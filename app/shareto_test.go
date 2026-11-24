package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// shareSource records what is sent.
type shareSource struct {
	timeline
	files []model.File
	rooms []string
	said  []string
}

func (s *shareSource) SendFile(room string, f model.File, reply string) {
	s.files, s.rooms = append(s.files, f), append(s.rooms, room)
}
func (s *shareSource) Send(room, text, reply string) { s.said = append(s.said, room+": "+text) }

// held is the burst's timer, kept until the test says the burst has ended.
var held func()

func endBurst() {
	if f := held; f != nil {
		held = nil
		f()
	}
}

func shareApp(t *testing.T) (*App, *shareSource, model.Room, *ui.Tester) {
	t.Helper()
	old := shareAfter
	held = nil
	shareAfter = func(_ time.Duration, f func()) { held = f }
	t.Cleanup(func() { shareAfter = old; held = nil })
	a := testApp()
	a.nav = kit.NavRail
	a.runUI = func(fn func()) { fn() }
	room, tl := longTimeline(a, 3)
	src := &shareSource{timeline: timeline{a.src, tl}}
	a.src = src
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	return a, src, room, tt
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A share asks where to send it, says what it is, and sends nothing until a chat
// is picked; then the files go as files, the words as a message, and the chat opens.
func TestShareAsksThenSends(t *testing.T) {
	a, src, room, tt := shareApp(t)
	pic := writeTemp(t, "holiday.png", "PNGDATA")
	doc := writeTemp(t, "notes.txt", "hello")

	a.ReceiveShare(Share{Paths: []string{pic}})
	a.ReceiveShare(Share{Paths: []string{doc}, Text: "see attached"}) // a second call of the same burst
	if a.shareDlg.Sheet.Open {
		t.Fatal("asked before the burst ended")
	}
	endBurst()
	settle(tt)
	if !a.shareDlg.Sheet.Open || len(a.shareDlg.Share.Paths) != 2 {
		t.Fatalf("dialog open=%v, paths=%v", a.shareDlg.Sheet.Open, a.shareDlg.Share.Paths)
	}
	if !tt.HasText("Send to…") || !tt.HasText("2 files and “see attached”") {
		t.Fatal("the dialog does not say what is being sent")
	}
	if len(src.files) != 0 || len(src.said) != 0 {
		t.Fatal("something was sent before a chat was picked")
	}
	if err := tt.Click("Send to " + room.Name); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(src.files) != 2 || src.files[0].Name != "holiday.png" || string(src.files[1].Data) != "hello" || src.rooms[0] != room.ID {
		t.Fatalf("files sent: %+v", src.files)
	}
	if len(src.said) != 1 || src.said[0] != room.ID+": see attached" {
		t.Fatalf("words sent: %v", src.said)
	}
	if a.shareDlg.Sheet.Open || a.room != room.ID || a.view != viewRoom {
		t.Fatalf("after: open=%v room=%q view=%v", a.shareDlg.Sheet.Open, a.room, a.view)
	}
	if _, err := os.Stat(pic); err != nil {
		t.Fatal("a file the user owns was removed")
	}
}

// Closing "Send to…" without choosing removes the copies made for the share (and
// only those: a file the user owns stays).
func TestShareCancelledCleansUpCopies(t *testing.T) {
	a, _, _, tt := shareApp(t)
	tmp := writeTemp(t, "copy.jpg", "JPG")
	mine := writeTemp(t, "mine.jpg", "JPG")

	a.ReceiveShare(Share{Paths: []string{tmp}, Temp: true})
	endBurst()
	settle(tt)
	a.shareDlg.Sheet.Open = false
	settle(tt)
	if _, err := os.Stat(tmp); err == nil {
		t.Fatal("the copy was left behind after cancelling")
	}

	a.ReceiveShare(Share{Paths: []string{mine}})
	endBurst()
	settle(tt)
	a.shareDlg.Sheet.Open = false
	settle(tt)
	if _, err := os.Stat(mine); err != nil {
		t.Fatal("cancelling removed the user's own file")
	}
}

// Nothing handed over is no question; files made for the share (Android's) are
// removed once read; a file that cannot be read is said, and nothing is sent.
func TestShareEdgeCases(t *testing.T) {
	a, src, room, tt := shareApp(t)
	a.ReceiveShare(Share{})
	a.ReceiveShare(Share{Text: "   "})
	endBurst()
	settle(tt)
	if a.shareDlg.Sheet.Open || held != nil {
		t.Fatal("an empty share asked a question")
	}

	tmp := writeTemp(t, "from-android.jpg", "JPG")
	a.ReceiveShare(Share{Paths: []string{tmp}, Temp: true})
	endBurst()
	settle(tt)
	a.sendShare(a.shareDlg.Share, room.ID, room.Name)
	settle(tt)
	if len(src.files) != 1 {
		t.Fatalf("sent %d", len(src.files))
	}
	if _, err := os.Stat(tmp); err == nil {
		t.Fatal("the copy made for the share was left behind")
	}

	a.ReceiveShare(Share{Paths: []string{filepath.Join(t.TempDir(), "gone.pdf")}})
	endBurst()
	settle(tt)
	a.sendShare(a.shareDlg.Share, room.ID, room.Name)
	settle(tt)
	if len(src.files) != 1 || a.shareDlg.Err == "" || !a.shareDlg.Sheet.Open {
		t.Fatalf("a missing file: sent=%d err=%q open=%v", len(src.files), a.shareDlg.Err, a.shareDlg.Sheet.Open)
	}
}
