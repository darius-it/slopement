package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/klipy"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

type folderUploadSource struct {
	model.Source
	files chan model.File
	sent  chan string
}

func (u *folderUploadSource) SendFile(room string, f model.File, replyTo string) { u.files <- f }
func (u *folderUploadSource) Send(room, text, replyTo string)                    { u.sent <- text }

// Folders are added once, the picker is told, they are saved with the app's
// state and come back; removing one tells the picker too.
func TestGIFFoldersPersist(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cat.gif"), []byte("GIF89a"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.pick = picker.New(nil)
	a.addGIFFolder(dir)
	a.addGIFFolder(dir + "/") // the same folder again
	if len(a.gifFolders) != 1 {
		t.Fatalf("folders = %v", a.gifFolders)
	}
	st := a.state()
	b := testApp()
	b.pick = picker.New(nil)
	b.applyState(st)
	if len(b.gifFolders) != 1 || b.gifFolders[0] != filepath.Clean(dir) {
		t.Fatalf("restored = %v", b.gifFolders)
	}
	b.removeGIFFolder(dir)
	if len(b.gifFolders) != 0 || len(b.state().GifFolders) != 0 {
		t.Fatal("not removed")
	}
}

// The settings page lists the folders and has a way to add one.
func TestGIFSettingsPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := testApp()
	a.pick = picker.New(nil)
	dir := t.TempDir()
	a.addGIFFolder(dir)
	a.view, a.settings = viewSettings, "GIFs"
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Frame()
	tt.Frame()
	if !tt.HasText(filepath.Base(dir)) || !tt.HasText("Add a folder…") || !tt.HasText("GIF folders") {
		t.Fatal("the GIF folders page lacks the folder or the add row")
	}
}

// A picture from a folder is uploaded as a file — never sent as a link to a
// path on this disk, whatever the setting.
func TestLocalPictureIsUploaded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "dance.gif")
	if err := os.WriteFile(path, []byte("GIF89a-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := mock.New(time.Now())
	m.Delay = 0
	src := &folderUploadSource{Source: m, files: make(chan model.File, 1), sent: make(chan string, 1)}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }
	a.prefs.gifFiles = false
	url := "file://" + path
	a.sendPicked("r1", klipy.Item{Kind: klipy.GIFs, Full: klipy.Rendition{URL: url}})
	select {
	case f := <-src.files:
		if f.Name != "dance.gif" || string(f.Data) != "GIF89a-bytes" {
			t.Fatalf("file = %+v", f)
		}
	case text := <-src.sent:
		t.Fatalf("sent the text %q", text)
	case <-time.After(3 * time.Second):
		t.Fatal("nothing sent")
	}
}
