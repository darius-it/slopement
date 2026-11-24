package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"slopement/model"
)

// limitSource is a source whose server takes files up to a limit.
type limitSource struct {
	model.Source
	limit int64
}

func (s *limitSource) UploadLimit() int64 { return s.limit }

// A file over the server's limit is turned away when it is added (dropped,
// picked, pasted) with a message that names the server's limit; one under it is
// added; without a known limit, only the app's own 50 MB applies.
func TestFileOverTheServersLimitIsTurnedAway(t *testing.T) {
	a := testApp()
	src := &limitSource{Source: a.src, limit: 5 << 20} // 5 MB
	a.src = src
	a.runUI = func(f func()) { f() }
	room := a.src.Rooms()[0]
	a.open(room.ID)

	dir := t.TempDir()
	small := filepath.Join(dir, "small.bin")
	big := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(small, make([]byte, 1<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(big, make([]byte, 6<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	var said []string
	a.addPaths(room.ID, []string{big, small})
	for i := 0; i < 200 && len(a.files[room.ID]) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	said = append(said, a.toast)
	if len(a.files[room.ID]) != 1 || a.files[room.ID][0].Name != "small.bin" {
		t.Fatalf("waiting files = %+v", a.files[room.ID])
	}
	if !strings.Contains(said[0], "big.bin") || !strings.Contains(said[0], "your server's limit of 5 MB") {
		t.Fatalf("toast = %q", said[0])
	}

	// Pasted or dropped bytes (addFile) get the same check.
	a.toast = ""
	a.addFile(room.ID, model.File{Name: "pasted.png", Data: make([]byte, 6<<20)})
	if len(a.files[room.ID]) != 1 || !strings.Contains(a.toast, "pasted.png") {
		t.Fatalf("a pasted file over the limit: files=%d toast=%q", len(a.files[room.ID]), a.toast)
	}

	// Server limit unknown (0) or higher than the app's: the app's own limit.
	src.limit = 0
	if why := a.tooLarge("x", 60<<20); !strings.Contains(why, "over 50 MB") || strings.Contains(why, "server") {
		t.Fatalf("no server limit: %q", why)
	}
	if a.tooLarge("x", 40<<20) != "" {
		t.Fatal("40 MB refused with no server limit")
	}
	src.limit = 500 << 20
	if why := a.tooLarge("x", 60<<20); !strings.Contains(why, "over 50 MB") {
		t.Fatalf("a high server limit: %q", why)
	}
}
