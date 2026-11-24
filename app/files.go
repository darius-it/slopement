package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// maxFile is the largest file the composer takes (the Matrix layer refuses
// more, too): a bigger one is said so, not read into memory.
const maxFile = 50 << 20

// hasWords says a draft has something besides whitespace.
func hasWords(s string) bool { return strings.TrimSpace(s) != "" }

// sizeLabel is a file's size as people say it: "340 KB", "2.1 MB".
func sizeLabel(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}

// fileLimit is the largest file that can be sent: the server's limit when it has
// said one, never more than the app's own.
func (a *App) fileLimit() (limit int64, server bool) {
	limit = maxFile
	if ul, ok := a.src.(model.UploadLimiter); ok {
		if l := ul.UploadLimit(); l > 0 && l < limit {
			return l, true
		}
	}
	return limit, false
}

// tooLarge says why a file cannot be sent for its size ("" when it can): the
// server's limit is named, so the user knows it is not the app being fussy.
func (a *App) tooLarge(name string, size int64) string {
	return a.sizeChecker()(name, size)
}

// sizeChecker is tooLarge with the limit taken now, for work off the UI thread.
// Call it on the UI thread.
func (a *App) sizeChecker() func(name string, size int64) string {
	limit, server := a.fileLimit()
	return func(name string, size int64) string {
		if size <= limit {
			return ""
		}
		lim := fmt.Sprintf("%d MB", limit>>20)
		if limit%(1<<20) != 0 {
			lim = fmt.Sprintf("%.1f MB", float64(limit)/(1<<20))
		}
		if server {
			return fmt.Sprintf("%s (%s) is over your server's limit of %s", name, sizeLabel(int(size)), lim)
		}
		return fmt.Sprintf("%s is too large to send (over %s)", name, lim)
	}
}

// pickFiles opens the system's file dialog; what is chosen waits in the
// composer, to go with the message. The dialog is native, so it runs off the
// view.
func (a *App) pickFiles(room string) {
	if a.win == nil {
		return // no window (tests)
	}
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Send files", ButtonLabel: "Add", Multiple: true})
		if err != nil || len(paths) == 0 {
			return
		}
		a.addPaths(room, paths)
	}()
}

// addPaths reads the files at paths (off the UI thread) and has them wait
// in the room's composer. One that is too large, or unreadable, is said so.
func (a *App) addPaths(room string, paths []string) {
	runUI := a.runUI
	if runUI == nil {
		return
	}
	go func() {
		// The limit (the server's, once known) is read on the UI thread, once.
		got := make(chan func(string, int64) string, 1)
		runUI(func() { got <- a.sizeChecker() })
		var check func(string, int64) string
		select {
		case check = <-got:
		case <-time.After(2 * time.Second):
			check = func(name string, size int64) string { // the window is gone: the app's own limit
				if size > maxFile {
					return name + " is too large to send (over 50 MB)"
				}
				return ""
			}
		}
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil || info.IsDir() {
				runUI(func() { a.toast = "Could not read " + filepath.Base(p) })
				continue
			}
			if why := check(filepath.Base(p), info.Size()); why != "" {
				runUI(func() { a.toast = why })
				continue
			}
			data, err := os.ReadFile(p)
			if err != nil {
				runUI(func() { a.toast = "Could not read " + filepath.Base(p) })
				continue
			}
			f := model.File{Name: filepath.Base(p), Data: data}
			runUI(func() { a.addFile(room, f) })
		}
	}()
}

// addFile puts a file in the room's composer, to go with the next message.
func (a *App) addFile(room string, f model.File) {
	if why := a.tooLarge(f.Name, int64(len(f.Data))); why != "" {
		a.toast = why // pasted or dropped: turned away here, not after an upload
		return
	}
	a.files[room] = append(a.files[room], f)
	st := a.draft(room)
	a.fileSeq++
	cf := kit.ComposerFile{Name: f.Name, Size: sizeLabel(len(f.Data)), ID: a.fileSeq, Image: isPicture(f)}
	st.Files = append(st.Files, cf)
	if cf.Image {
		a.thumbnail(room, cf.ID, f.Data)
	}
}

// isPicture says a file is one to show as a picture: by its bytes, else by
// its name.
func isPicture(f model.File) bool {
	if f.Mime != "" {
		return strings.HasPrefix(f.Mime, "image/") && f.Mime != "image/svg+xml"
	}
	head := f.Data
	if len(head) > 512 {
		head = head[:512]
	}
	if strings.HasPrefix(http.DetectContentType(head), "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(f.Name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// thumbnail decodes a waiting picture to the thumbnail's size, off the UI
// thread, and hands it to the file's tile once it is ready. A picture that
// will not decode stays a placeholder (it still sends).
func (a *App) thumbnail(room string, id int, data []byte) {
	runUI := a.runUI
	if runUI == nil {
		return
	}
	go func() {
		// Twice the tile for a sharp one on a dense screen.
		bmp, err := ui.DecodeBitmapScaled(data, 160)
		if err != nil || bmp == nil {
			return
		}
		runUI(func() {
			st := a.draft(room)
			for i := range st.Files {
				if st.Files[i].ID == id {
					st.Files[i].Thumb = bmp
				}
			}
		})
	}()
}

// removeFile takes the i-th waiting file out.
func (a *App) removeFile(room string, i int) {
	fs, st := a.files[room], a.draft(room)
	if i < 0 || i >= len(fs) || i >= len(st.Files) {
		return
	}
	a.files[room] = append(fs[:i:i], fs[i+1:]...)
	st.Files = append(st.Files[:i:i], st.Files[i+1:]...)
}

// pasteStep is the paste shortcut pressed in the composer: a picture or
// files on the clipboard wait in the composer. The field pastes text by
// itself, and a clipboard with text on it (a copied page has a picture too)
// is text.
func (a *App) pasteStep(room string) {
	if a.win == nil {
		return
	}
	if mygo.Clipboard.ReadText() != "" {
		return
	}
	if paths, err := mygo.Clipboard.ReadFiles(); err == nil && len(paths) > 0 {
		a.addPaths(room, paths)
		return
	}
	if img := mygo.Clipboard.ReadImage(); len(img) > 0 {
		a.addFile(room, model.File{Name: "image.png", Mime: "image/png", Data: img})
	}
}

// sendFiles sends the files waiting in a room's composer, the first as the
// reply if the message is one. It reports whether there were any.
func (a *App) sendFiles(room string, reply string) bool {
	fs := a.files[room]
	if len(fs) == 0 {
		return false
	}
	up, ok := a.src.(model.Uploader)
	if !ok {
		a.toast = "This account cannot send files"
	} else {
		for i, f := range fs {
			r := ""
			if i == 0 {
				r = reply
			}
			up.SendFile(room, f, r)
		}
	}
	delete(a.files, room)
	a.draft(room).Files = nil
	return true
}

// downloadFile saves the file of a message where the user says: the bytes
// come from the source (downloaded and decrypted), the place from the
// system's save dialog.
func (a *App) downloadFile(ev model.Event) {
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.win == nil || len(ev.Media) == 0 {
		return
	}
	m, win := ev.Media[0], a.win
	name := m.Name
	if name == "" {
		name = "file"
	}
	go func() {
		to, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: "Save " + name, DefaultPath: name})
		if err != nil || to == "" {
			return
		}
		loader.LoadMedia(m, func(data []byte, err error) {
			if err == nil {
				err = os.WriteFile(to, data, 0o644)
			}
			win.Update(func() {
				if err != nil {
					a.toast = "Could not save " + name + ": " + err.Error()
				} else {
					a.toast = "Saved " + filepath.Base(to)
				}
			})
		})
	}()
}

// sendGIF sends a GIF or sticker from the picker: as its link, which costs
// the homeserver nothing and shows as the GIF here (the default), or — with
// "Send GIFs as files" on — as a picture uploaded to the media repo, which
// every client animates. The link also stands in when the file cannot be
// fetched or this account cannot upload.
func (a *App) sendGIF(room, url string) {
	up, ok := a.src.(model.Uploader)
	runUI := a.runUI
	if !a.prefs.gifFiles || !ok || runUI == nil {
		a.src.Send(room, url, "")
		return
	}
	go func() {
		ctx := &http.Client{Timeout: 20 * time.Second}
		resp, err := ctx.Get(url)
		var data []byte
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				data, err = io.ReadAll(io.LimitReader(resp.Body, 20<<20))
			} else {
				err = fmt.Errorf("status %d", resp.StatusCode)
			}
		}
		runUI(func() {
			if err != nil || len(data) == 0 {
				a.src.Send(room, url, "") // the link still shows as the GIF
				return
			}
			name := "image.gif"
			if i := strings.LastIndex(url, "/"); i >= 0 && i+1 < len(url) {
				name = strings.SplitN(url[i+1:], "?", 2)[0]
			}
			up.SendFile(room, model.File{Name: name, Data: data, Mime: strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]}, "")
		})
	}()
}
