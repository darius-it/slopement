package picker

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg" // stills of a folder's pictures
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"

	"slopement/animwebp"
	"slopement/klipy"
)

// Folders on the user's disk as a source of GIFs (Settings → GIFs): their
// GIF, APNG, PNG, JPEG and WebP files show as "My folders" on the GIF front
// page and in a search (by file name). They are indexed in the background
// when the folders are set and again when the picker opens after a while, and
// each tile is thumbnailed from its file as it comes into view. (A file that
// is not a picture, or a video — mygo plays no MP4 — is left out.)

// localScheme marks an item's URL as a file on this disk. Such a URL never
// leaves the app: sending one uploads the file.
const localScheme = "file://"

const (
	maxLocalFiles = 3000 // the most one index holds
	maxLocalDepth = 4    // folders below a chosen one that are looked into
	rescanAfter   = 30 * time.Second
	maxLocalFile  = 32 << 20 // a bigger file is not shown
)

var localExts = map[string]bool{".gif": true, ".png": true, ".apng": true, ".jpg": true, ".jpeg": true, ".webp": true}

// IsLocalURL reports whether url names a file on this disk.
func IsLocalURL(url string) bool { return strings.HasPrefix(url, localScheme) }

// LocalPath is the file path of a local URL.
func LocalPath(url string) string { return strings.TrimPrefix(url, localScheme) }

// scanFolders lists the picture files in dirs and in the folders below them
// (to a depth), as items, newest file first, at most maxLocalFiles. A folder
// that can't be read is skipped. Pure but for the disk, for the tests.
func scanFolders(dirs []string) []klipy.Item {
	type file struct {
		path string
		mod  time.Time
	}
	var files []file
	seen := map[string]bool{}
	for _, root := range dirs {
		root = filepath.Clean(root)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if rel, _ := filepath.Rel(root, p); rel != "." && strings.Count(rel, string(filepath.Separator)) >= maxLocalDepth {
					return fs.SkipDir
				}
				if strings.HasPrefix(d.Name(), ".") && p != root {
					return fs.SkipDir // hidden folders
				}
				return nil
			}
			if !localExts[strings.ToLower(filepath.Ext(p))] || seen[p] || len(files) >= maxLocalFiles*2 {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.Size() == 0 || info.Size() > maxLocalFile {
				return nil
			}
			seen[p] = true
			files = append(files, file{p, info.ModTime()})
			return nil
		})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	if len(files) > maxLocalFiles {
		files = files[:maxLocalFiles]
	}
	items := make([]klipy.Item, 0, len(files))
	for _, f := range files {
		sum := sha1.Sum([]byte(f.path))
		r := klipy.Rendition{URL: localScheme + f.path}
		items = append(items, klipy.Item{
			ID:    "file:" + hex.EncodeToString(sum[:8]),
			Title: strings.TrimSuffix(filepath.Base(f.path), filepath.Ext(f.path)),
			Kind:  klipy.GIFs, Thumb: r, Still: r, Full: r,
		})
	}
	return items
}

// SetFolders sets the folders the picker looks in and indexes them in the
// background; nothing happens when they are the ones already set.
func (c *Controller) SetFolders(dirs []string) {
	if sameStrings(dirs, c.folders) {
		return
	}
	c.folders = append([]string(nil), dirs...)
	c.scanned = time.Time{}
	c.rescan()
}

// rescanIfStale indexes the folders again when it has been a while: the
// picker opening is when a new file would be looked for.
func (c *Controller) rescanIfStale() {
	if len(c.folders) > 0 && time.Since(c.scanned) > rescanAfter {
		c.rescan()
	}
}

func (c *Controller) rescan() {
	if c.scanning {
		c.rescanAgain = true
		return
	}
	dirs := append([]string(nil), c.folders...)
	if len(dirs) == 0 {
		c.local = nil
		c.refreshLocalViews()
		return
	}
	c.scanning = true
	go func() {
		items := scanFolders(dirs)
		c.post(func() {
			c.scanning = false
			c.scanned = time.Now()
			if sameStrings(dirs, c.folders) {
				c.local = items
				for _, it := range items {
					c.items[it.ID] = it
				}
				c.refreshLocalViews()
			}
			if c.rescanAgain {
				c.rescanAgain = false
				c.rescan()
			}
		})
	}()
}

// refreshLocalViews updates what is on screen that is made of local files.
func (c *Controller) refreshLocalViews() {
	if c.browse == "folders" && c.shownQuery == "" {
		c.gifs.Title, c.gifs.Results = "My folders", c.tiles(c.local)
		c.gifs.Notice = ""
		if len(c.local) == 0 {
			c.gifs.Notice = "No pictures in your folders (Settings → GIFs)"
		}
	}
}

// LocalCount is how many files the folders hold (as indexed).
func (c *Controller) LocalCount() int { return len(c.local) }

// localMatches are the local files whose name has every word of query.
func (c *Controller) localMatches(query string, limit int) []klipy.Item {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil
	}
	var out []klipy.Item
	for _, it := range c.local {
		name := strings.ToLower(it.Title)
		ok := true
		for _, w := range words {
			if !strings.Contains(name, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// readLocal reads a local item's file (the URL's path), within the size the
// index allows.
func readLocal(url string) ([]byte, error) {
	f, err := os.Open(LocalPath(url))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLocalFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLocalFile {
		return nil, errors.New("picker: file too large")
	}
	return data, nil
}

// newDecoder decodes a picture's bytes as an animation: a GIF or an APNG
// plays, any other picture is a video of one frame.
func newDecoder(data []byte) (ui.VideoDecoder, error) {
	switch {
	case bytes.HasPrefix(data, []byte("GIF8")):
		return ui.NewGIFDecoder(data)
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		return ui.NewAPNGDecoder(data)
	case animwebp.Animated(data):
		return animwebp.NewDecoder(data)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return &stillDecoder{img: img}, nil
}

// stillDecoder is a picture as a video of one frame.
type stillDecoder struct {
	img  image.Image
	done bool
}

func (s *stillDecoder) Next() (ui.VideoFrame, error) {
	if s.done {
		return ui.VideoFrame{}, io.EOF
	}
	s.done = true
	return ui.VideoFrame{Image: s.img, Duration: time.Hour}, nil
}

func (s *stillDecoder) Rewind() error { s.done = false; return nil }
