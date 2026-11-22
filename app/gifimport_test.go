package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"slopement/model/mock"
	"slopement/picker"
)

// A Discord export chosen in the dialog lands in the favorites and says so.
func TestImportDiscordFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the picker keeps its favorites under the config dir
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := mock.New(time.Now())
	src.Delay = 0
	pick := picker.New(nil)
	a := New(src, pick)
	done := make(chan struct{}, 1)
	a.runUI = func(f func()) { f(); done <- struct{}{} }

	path := filepath.Join(t.TempDir(), "discord-favorite-gifs.json")
	data := `{"https://tenor.com/view/cat-gif-123": {"format": 1, "src": "https://media.tenor.com/abc/cat.gif", "width": 10, "height": 10, "order": 1}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	a.importDiscordFile(path)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("no result")
	}
	if a.toast != "Imported 1 GIFs from Discord" {
		t.Fatalf("toast %q", a.toast)
	}
	a.importDiscordFile(filepath.Join(t.TempDir(), "missing.json"))
	<-done
	if a.toast == "" || a.toast == "Imported 1 GIFs from Discord" {
		t.Fatalf("toast %q", a.toast)
	}
}
