package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// uploadSource records what is sent to it.
type uploadSource struct {
	model.Source
	files   []model.File
	replies []string
	texts   []string
}

func (s *uploadSource) SendFile(room string, f model.File, replyTo string) {
	s.files = append(s.files, f)
	s.replies = append(s.replies, replyTo)
}
func (s *uploadSource) Send(room, text, replyTo string) { s.texts = append(s.texts, text) }

// Files chosen wait in the composer as chips (removable), and go with the
// message: first the files, then the words.
func TestComposerFiles(t *testing.T) {
	a := testApp()
	src := &uploadSource{Source: a.src}
	a.src = src
	a.runUI = func(f func()) { f() }
	room := a.src.Rooms()[0]
	a.open(room.ID)

	dir := t.TempDir()
	doc := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(doc, []byte("%PDF- pretend"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.addPaths(room.ID, []string{doc, filepath.Join(dir, "missing.txt")})
	for i := 0; i < 100 && len(a.files[room.ID]) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if len(a.files[room.ID]) != 1 || a.files[room.ID][0].Name != "report.pdf" {
		t.Fatalf("waiting files = %+v", a.files[room.ID])
	}
	time.Sleep(50 * time.Millisecond) // the missing one is toasted on its way

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if !tt.HasText("report.pdf") {
		t.Fatal("the waiting file does not show as a chip")
	}
	if _, ok := tt.Find("Send"); !ok {
		t.Fatal("no Send button with only a file waiting")
	}

	// Taken out, then put back; sent with a message that replies.
	if err := tt.Click("Remove report.pdf"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(a.files[room.ID]) != 0 || tt.HasText("report.pdf") {
		t.Fatal("the chip's ✕ did not take the file out")
	}
	a.addFile(room.ID, model.File{Name: "one.png", Data: []byte("1")})
	a.addFile(room.ID, model.File{Name: "two.txt", Data: []byte("22")})
	a.replyTo[room.ID] = "$quoted"
	if !a.sendFiles(room.ID, "$quoted") {
		t.Fatal("nothing was sent")
	}
	if len(src.files) != 2 || src.files[0].Name != "one.png" || src.files[1].Name != "two.txt" {
		t.Fatalf("sent files = %+v", src.files)
	}
	if src.replies[0] != "$quoted" || src.replies[1] != "" {
		t.Fatalf("the reply goes with the first file only: %v", src.replies)
	}
	if len(a.files[room.ID]) != 0 || len(a.draft(room.ID).Files) != 0 {
		t.Fatal("the composer still holds what was sent")
	}
}

// A message that is only a picture or a file (its body is the file name)
// renders — on desktop and on a phone, sent, still uploading, and failed.
// (It once crashed the window: no words beside the media meant no document
// to look for links in.)
func TestMediaOnlyMessagesRender(t *testing.T) {
	a := testApp()
	room, _ := longTimeline(a, 2)
	now := time.Now()
	media := func(id, name string, kind model.MediaKind, pending, failed bool) model.Event {
		return model.Event{ID: id, Room: room.ID, Sender: room.Partner, Kind: model.EventMedia, Body: name, Time: now,
			Media: []model.Media{{Kind: kind, Name: name, W: 40, H: 30, Size: 2048}}, Pending: pending, Failed: failed, Error: "no"}
	}
	tl := []model.Event{
		media("1", "photo.png", model.MediaImage, false, false),
		media("2", "report.pdf", model.MediaFile, false, false),
		media("3", "uploading.jpg", model.MediaImage, true, false),
		media("4", "broken.png", model.MediaImage, false, true),
		{ID: "5", Room: room.ID, Sender: a.src.Me().ID, Kind: model.EventMedia, Body: "mine.png", Time: now,
			Media: []model.Media{{Kind: model.MediaImage, Name: "mine.png", W: 10, H: 10, Local: []byte("not an image")}}, Pending: true},
	}
	a.src = timeline{a.src, tl}
	a.runUI = func(f func()) { f() }
	a.open(room.ID)
	for _, size := range [][2]int{{1100, 700}, {390, 844}} {
		tt := ui.NewTester(a.View, size[0], size[1])
		settle(tt)
		settle(tt)
		if size[0] > 400 && !tt.HasText("report.pdf") {
			t.Fatal("the file card does not show")
		}
	}
}

func TestSizeLabel(t *testing.T) {
	for n, want := range map[int]string{0: "0 B", 900: "900 B", 2048: "2 KB", 3 << 20: "3.0 MB"} {
		if got := sizeLabel(n); got != want {
			t.Errorf("sizeLabel(%d) = %q, want %q", n, got, want)
		}
	}
}
