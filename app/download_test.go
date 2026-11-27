package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

type loadSource struct{ model.Source }

func (loadSource) LoadMedia(_ model.Media, done func([]byte, error)) {
	done([]byte("\x89PNG\r\n\x1a\nbytes"), nil)
}

// On a computer the file is written where the user said. On a phone it is
// written to a temporary file, of which the system's picker takes a copy, and
// putting the picker away says nothing.
func TestSaveMedia(t *testing.T) {
	a := testApp()
	a.src = loadSource{a.src}
	dir := t.TempDir()
	var path string
	a.saveFileFn = func(_, name string, done func(string)) { path = filepath.Join(dir, name); done(path) }
	m := model.Media{URL: "mxc://x/y", Name: "../photo.png"}

	a.saveMedia(m)
	if got, err := os.ReadFile(filepath.Join(dir, "photo.png")); err != nil || len(got) == 0 {
		t.Fatalf("not written: %v", err)
	}
	if a.toast != "Saved photo.png" {
		t.Fatalf("said %q", a.toast)
	}

	was := kit.SetPhoneOS(true)
	defer kit.SetPhoneOS(was)
	var copied string
	answer := "content://downloads/7"
	a.saveCopyFn = func(from, name string) (string, error) {
		if _, err := os.Stat(from); err != nil {
			t.Fatalf("nothing to copy: %v", err)
		}
		copied = name
		return answer, nil
	}
	a.saveMedia(m)
	if copied != "photo.png" || a.toast != "Saved photo.png" {
		t.Fatalf("copied %q, said %q", copied, a.toast)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the temporary file was left")
	}
	answer, a.toast = "", ""
	a.saveMedia(m)
	if a.toast != "Downloading photo.png…" {
		t.Fatalf("after putting the picker away it said %q", a.toast)
	}

	// A profile picture has no extension: the bytes give it one.
	a.saveMedia(model.Media{URL: "mxc://x/z", Name: "Kai"})
	if copied != "Kai.png" {
		t.Fatalf("copied %q", copied)
	}
}

// The picture viewer's Download button saves the picture shown.
func TestViewerDownload(t *testing.T) {
	a := testApp()
	pressed := 0
	a.viewerItems = []kit.MediaSpec{{ID: "p", Kind: kit.MediaImage, W: 4, H: 3, Progress: -1, Label: "p.png", Save: func() { pressed++ }}}
	a.viewer = true
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if err := tt.Click("Download"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if pressed != 1 {
		t.Fatalf("Download saved %d times", pressed)
	}
}

func TestMediaFileName(t *testing.T) {
	for _, c := range []struct {
		m    model.Media
		want string
	}{
		{model.Media{Name: `C:\x\a.pdf`}, "a.pdf"},
		{model.Media{Name: "cat", Mime: "image/jpeg"}, "cat.jpg"},
		{model.Media{URL: "https://example.org/g/wave.gif?x=1"}, "wave.gif"},
		{model.Media{Name: ".."}, "file"},
	} {
		if got := mediaFileName(c.m); got != c.want {
			t.Errorf("%+v: %q, want %q", c.m, got, c.want)
		}
	}
}
