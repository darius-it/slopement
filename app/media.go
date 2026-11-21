package app

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

const (
	stillMax  = 64   // pictures kept decoded
	stillSide = 1024 // the longest side kept, in pixels
)

const (
	avatarMax  = 300 // profile pictures kept decoded
	avatarSide = 128 // pixels a side: sharp up to 40 DIP at 3x
)

// avatarPic is the decoded profile picture of an mxc:// URL, or nil while
// it is fetched (initials show meanwhile) or when it cannot be: one try
// per URL, on the source's thumbnail, off the UI thread. Kept by URL, the
// oldest dropped first.
func (a *App) avatarPic(url string) *ui.Bitmap {
	if url == "" || !strings.HasPrefix(url, "mxc://") {
		return nil
	}
	if b, ok := a.avatars[url]; ok {
		return b
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.runUI == nil || a.avatarBusy[url] {
		return nil
	}
	a.avatarBusy[url] = true
	runUI := a.runUI
	loader.LoadMedia(model.Media{URL: url, Kind: model.MediaImage, Square: avatarSide}, func(data []byte, err error) {
		var bmp *ui.Bitmap
		if err == nil {
			bmp, err = ui.DecodeBitmapScaled(data, avatarSide)
		}
		runUI(func() {
			if err != nil || bmp == nil {
				return // initials stay; busy stays set, so no retry storm
			}
			a.avatars[url] = bmp
			a.avatarOrder = append(a.avatarOrder, url)
			if len(a.avatarOrder) > avatarMax {
				delete(a.avatars, a.avatarOrder[0])
				delete(a.avatarBusy, a.avatarOrder[0])
				a.avatarOrder = a.avatarOrder[1:]
			}
		})
	})
	return nil
}

// localStill is the picture of a message being sent, decoded from the bytes
// the user chose (off the UI thread, once).
func (a *App) localStill(m model.Media) ui.ImageSource {
	key := "local:" + m.Name + ":" + strconv.Itoa(len(m.Local))
	if s, ok := a.stills[key]; ok {
		return s
	}
	if a.runUI == nil || a.stillBusy[key] {
		return nil
	}
	a.stillBusy[key] = true
	runUI, data := a.runUI, m.Local
	go func() {
		bmp, err := ui.DecodeBitmapScaled(data, stillSide)
		runUI(func() {
			if err != nil {
				return
			}
			a.stills[key] = bmp
			a.stillOrder = append(a.stillOrder, key)
		})
	}()
	return nil
}

// still is an attachment's picture: the decoded one when it has arrived,
// otherwise nil (the placeholder) while it is fetched, off the UI thread,
// from the source. Pictures are kept by URL, the oldest dropped first.
func (a *App) still(m model.Media) ui.ImageSource {
	if len(m.Local) > 0 && m.Kind == model.MediaImage {
		return a.localStill(m) // being sent: shown from its own bytes
	}
	if m.Kind != model.MediaImage || !strings.HasPrefix(m.URL, "mxc://") {
		return nil
	}
	if s, ok := a.stills[m.URL]; ok {
		return s
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.runUI == nil || a.stillBusy[m.URL] {
		return nil
	}
	a.stillBusy[m.URL] = true
	runUI, url := a.runUI, m.URL
	loader.LoadMedia(m, func(data []byte, err error) {
		var bmp *ui.Bitmap
		if err == nil {
			bmp, err = ui.DecodeBitmapScaled(data, stillSide)
		}
		runUI(func() {
			if err != nil || bmp == nil {
				return // the placeholder stays; busy stays set, so no retry storm
			}
			delete(a.stillBusy, url)
			a.stills[url] = bmp
			a.stillOrder = append(a.stillOrder, url)
			if len(a.stillOrder) > stillMax {
				delete(a.stills, a.stillOrder[0])
				a.stillOrder = a.stillOrder[1:]
			}
		})
	})
	return nil
}
