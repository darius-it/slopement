package picker

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"slopement/kit"
)

func writeFile(t *testing.T, path string, data []byte, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func pngData() []byte {
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 6, 4))
	img.Set(1, 1, color.White)
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func jpegData() []byte {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil)
	return b.Bytes()
}

// The index holds pictures only, newest first, from the folder and the ones
// below it to a depth, not hidden folders or files that aren't pictures.
func TestScanFolders(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, filepath.Join(root, "old cat.gif"), tinyGIF(), now.Add(-3*time.Hour))
	writeFile(t, filepath.Join(root, "sub", "new dog.PNG"), pngData(), now.Add(-time.Hour))
	writeFile(t, filepath.Join(root, "notes.txt"), []byte("x"), now)
	writeFile(t, filepath.Join(root, "clip.mp4"), []byte("x"), now)
	writeFile(t, filepath.Join(root, ".hidden", "secret.gif"), tinyGIF(), now)
	writeFile(t, filepath.Join(root, "a", "b", "c", "d", "e", "too deep.gif"), tinyGIF(), now)
	writeFile(t, filepath.Join(root, "empty.gif"), nil, now)
	items := scanFolders([]string{root, root, filepath.Join(root, "missing")})
	var names []string
	for _, it := range items {
		names = append(names, it.Title)
		if !IsLocalURL(it.Full.URL) || !strings.HasSuffix(LocalPath(it.Full.URL), filepath.Base(LocalPath(it.Full.URL))) || it.Slug != "" {
			t.Errorf("item = %+v", it)
		}
	}
	if got := strings.Join(names, ","); got != "new dog,old cat" {
		t.Fatalf("indexed %q, want newest first and only pictures within the depth", got)
	}
	if items[0].ID == items[1].ID {
		t.Fatal("two files share an ID")
	}
}

// Setting the folders indexes them in the background; a search finds files by
// every word of their name; the front page offers them without a KLIPY key.
func TestFolderIndexSearchAndFront(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, filepath.Join(root, "happy cat dance.gif"), tinyGIF(), now)
	writeFile(t, filepath.Join(root, "sad dog.png"), pngData(), now.Add(-time.Minute))
	c := newAt(nil, filepath.Join(t.TempDir(), "p.json"))
	q := &queue{}
	c.SetUpdate(q.post)
	c.SetFolders([]string{root})
	q.settle(t, func() bool { return c.LocalCount() == 2 })
	if got := c.localMatches("CAT dance", 10); len(got) != 1 || got[0].Title != "happy cat dance" {
		t.Fatalf("matches = %+v", got)
	}
	if len(c.localMatches("cat dog", 10)) != 0 || len(c.localMatches("  ", 10)) != 0 {
		t.Fatal("matched what lacks a word, or nothing")
	}

	// No KLIPY key: favorites and folders are still there to browse.
	st := &kit.PickerState{Tab: kit.TabGIFs}
	st.Open = true
	m := c.Model(st)
	var folders bool
	for _, tl := range m.Front {
		if tl.Kind == kit.TileFolders {
			folders = true
		}
	}
	if !folders || m.Notice != "" {
		t.Fatalf("front = %+v, notice %q", m.Front, m.Notice)
	}
	st.Browse = "folders"
	m = c.Model(st)
	if len(m.Results) != 2 || m.Title != "My folders" {
		t.Fatalf("browse folders: %d tiles titled %q", len(m.Results), m.Title)
	}
	st.Browse, st.Query = "", "dog"
	m = c.Model(st)
	if len(m.Results) != 1 || m.Results[0].Label != "sad dog" {
		t.Fatalf("search without a key: %+v", m.Results)
	}

	// Taking the folders away empties the index.
	c.SetFolders(nil)
	q.drain()
	if c.LocalCount() != 0 {
		t.Fatal("the index stays after the folders are removed")
	}
}

// A local picture loads from its file (GIF animated, PNG and JPEG as one
// frame), and a file that is no picture fails.
func TestLocalFilesLoad(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, filepath.Join(root, "a.gif"), tinyGIF(), now)
	writeFile(t, filepath.Join(root, "b.png"), pngData(), now)
	writeFile(t, filepath.Join(root, "c.jpg"), jpegData(), now)
	writeFile(t, filepath.Join(root, "d.gif"), []byte("not a picture"), now)
	c := newAt(nil, filepath.Join(t.TempDir(), "p.json"))
	q := &queue{}
	c.SetUpdate(q.post)
	for _, f := range []string{"a.gif", "b.png", "c.jpg"} {
		url := localScheme + filepath.Join(root, f)
		c.mediaOf(url)
		q.settle(t, func() bool { _, ok := c.media.get(url); return ok })
	}
	bad := localScheme + filepath.Join(root, "d.gif")
	c.mediaOf(bad)
	q.settle(t, func() bool { return c.media.failed(bad) })
}
