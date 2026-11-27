package app

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"slopement/kit"
	"slopement/model"
)

// Saving a file out of the app. A computer asks where first (saveFile) and the
// file is written there. A phone has no paths into other apps' folders: there
// saveFile only hands out a temporary file, and writeSaved then has the
// system's picker (Files on iOS, the storage framework on Android) take a copy.
// Everything that saves goes through both.

// webSaveLimit is the most a picture linked from the web may weigh to be saved.
const webSaveLimit = 50 << 20

// writeSaved writes data to the path saveFile gave and, on a phone, asks where
// the copy goes. then runs on the UI thread with the saved file's name, or ""
// when the picker was put away.
func (a *App) writeSaved(to string, data []byte, perm os.FileMode, then func(name string, err error)) {
	name := filepath.Base(to)
	if err := os.WriteFile(to, data, perm); err != nil {
		then("", err)
		return
	}
	if !kit.PhoneOS() {
		then(name, nil)
		return
	}
	saveCopy := a.saveCopyFn
	if saveCopy == nil {
		saveCopy = mygo.Dialog.SaveCopy
	}
	runUI := a.runUI
	finish := func() {
		saved, err := saveCopy(to, name)
		_ = os.Remove(to) // the temporary file
		done := func() {
			if saved == "" {
				then("", err)
			} else {
				then(name, err)
			}
		}
		if runUI == nil {
			done()
		} else {
			runUI(done)
		}
	}
	if a.saveCopyFn != nil {
		finish() // tests: at once
		return
	}
	go finish() // the picker waits for the user: off the UI thread
}

// downloadFile saves the file of a message: the bytes come from the source
// (downloaded and decrypted).
func (a *App) downloadFile(ev model.Event) {
	if len(ev.Media) > 0 {
		a.saveMedia(ev.Media[0])
	}
}

// saveMedia saves an attachment, or a picture linked from the web.
func (a *App) saveMedia(m model.Media) {
	load := a.mediaBytes(m)
	if load == nil {
		a.toast = "This account cannot download files"
		return
	}
	name := mediaFileName(m)
	if filepath.Ext(name) != "" {
		a.saveLoaded(name, load)
		return
	}
	// No extension to go by (a profile picture): the bytes say what it is.
	load(func(data []byte, err error) {
		a.later(func() {
			if err != nil {
				a.toast = "Could not save " + name + ": " + err.Error()
				return
			}
			a.saveLoaded(name+extensionOf(http.DetectContentType(data)), func(done func([]byte, error)) { done(data, nil) })
		})
	})
}

// saveLoaded asks where name goes and writes what load brings there.
func (a *App) saveLoaded(name string, load func(done func([]byte, error))) {
	a.saveFile("Save "+name, name, func(to string) {
		if kit.PhoneOS() {
			a.toast = "Downloading " + name + "…" // the picker comes after it
		}
		load(func(data []byte, err error) {
			a.later(func() {
				said := func(saved string, err error) {
					switch {
					case err != nil:
						a.toast = "Could not save " + name + ": " + err.Error()
					case saved != "":
						a.toast = "Saved " + saved
					}
				}
				if err != nil {
					said("", err)
					return
				}
				a.writeSaved(to, data, 0o644, said)
			})
		})
	})
}

// later runs fn on the UI thread (at once where there is no window: tests).
func (a *App) later(fn func()) {
	if a.runUI == nil {
		fn()
		return
	}
	a.runUI(fn)
}

// mediaBytes is how to get a medium's bytes, nil when there is no way.
func (a *App) mediaBytes(m model.Media) func(done func([]byte, error)) {
	if strings.HasPrefix(m.URL, "https://") {
		return func(done func([]byte, error)) {
			go func() {
				data, _, err := fetchPicture(m.URL, webSaveLimit)
				done(data, err)
			}()
		}
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok {
		return nil
	}
	return func(done func([]byte, error)) {
		if m.URL == "" {
			done(nil, errors.New("not uploaded yet"))
			return
		}
		loader.LoadMedia(m, done)
	}
}

// mediaFileName is what a saved medium is called: its own name without any
// folders, with an extension from its type or address if it has none.
func mediaFileName(m model.Media) string {
	name := m.Name
	if name == "" {
		if u, err := url.Parse(m.URL); err == nil && u.Scheme == "https" {
			name = path.Base(u.Path)
		}
	}
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/"))
	if name == "." || name == "/" || name == ".." {
		name = "file"
	}
	if path.Ext(name) == "" {
		name += extensionOf(m.Mime)
	}
	return name
}

// extensionOf is the extension of the media types the app is likely to save.
func extensionOf(mime string) string {
	return map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp",
		"video/mp4": ".mp4", "video/webm": ".webm", "audio/ogg": ".ogg", "audio/mpeg": ".mp3",
		"audio/mp4": ".m4a", "audio/wav": ".wav", "application/pdf": ".pdf",
	}[mime]
}
