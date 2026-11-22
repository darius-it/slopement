package app

import (
	"errors"
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

const (
	gifMax      = 24       // looping players kept (each holds its frames)
	gifMaxBytes = 20 << 20 // a GIF larger than this stays a still
)

// isGIFFile reports an attachment that is a real GIF (image/gif bytes),
// which animates; a "GIF" that is a short video is not one.
func isGIFFile(m model.Media) bool {
	return m.Mime == "image/gif" && m.Kind == model.MediaGIF
}

// gifPlayer is the looping player of a GIF attachment, once its bytes are
// here and decoded (nil until then: a placeholder shows, then it plays):
// downloaded (and decrypted) off the UI thread, or taken from the bytes of
// one the user is sending. A GIF that cannot be decoded stays a placeholder,
// asked for once. The newest players are kept.
func (a *App) gifPlayer(m model.Media) *ui.VideoPlayer {
	key := m.URL
	if len(m.Local) > 0 {
		key = "local:" + m.Name + ":" + strconv.Itoa(len(m.Local))
	}
	if key == "" {
		return nil
	}
	if p, ok := a.gifs[key]; ok {
		return p
	}
	if a.runUI == nil || a.gifBusy[key] {
		return nil
	}
	runUI := a.runUI
	finish := func(data []byte, err error) {
		var d ui.VideoDecoder
		if err == nil && len(data) <= gifMaxBytes {
			d, err = ui.NewGIFDecoder(data)
		} else if err == nil {
			err = errors.New("gif too large")
		}
		runUI(func() {
			if err != nil {
				return // stays a placeholder; busy stays set: no retry storm
			}
			pl, perr := ui.NewVideoPlayer(d)
			if perr != nil {
				return
			}
			pl.SetLoop(true).SetMuted(true)
			a.gifs[key] = pl
			a.gifOrder = append(a.gifOrder, key)
			if len(a.gifOrder) > gifMax {
				delete(a.gifs, a.gifOrder[0])
				delete(a.gifBusy, a.gifOrder[0])
				a.gifOrder = a.gifOrder[1:]
			}
		})
	}
	if len(m.Local) > 0 {
		a.gifBusy[key] = true
		data := m.Local
		go finish(data, nil)
		return nil
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok || !strings.HasPrefix(m.URL, "mxc://") {
		return nil
	}
	a.gifBusy[key] = true
	loader.LoadMedia(m, finish)
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

const (
	fullMax  = 4    // full-resolution pictures kept
	fullSide = 4096 // the longest side, in pixels: sharp when zoomed on a large display
)

// fullPic is an attachment's picture for the viewer at full resolution, nil
// until it has been fetched (the preview's still shows meanwhile). It is
// asked only while the viewer shows it, once per picture.
func (a *App) fullPic(m model.Media) ui.ImageSource {
	key := m.URL
	if len(m.Local) > 0 || !strings.HasPrefix(key, "mxc://") {
		return nil
	}
	if b, ok := a.fulls[key]; ok {
		return b
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.runUI == nil || a.fullBusy[key] {
		return nil
	}
	a.fullBusy[key] = true
	runUI := a.runUI
	loader.LoadMedia(m, func(data []byte, err error) {
		var bmp *ui.Bitmap
		if err == nil {
			bmp, err = ui.DecodeBitmapScaled(data, fullSide)
		}
		runUI(func() {
			if err != nil || bmp == nil {
				return
			}
			a.fulls[key] = bmp
			a.fullOrder = append(a.fullOrder, key)
			if len(a.fullOrder) > fullMax {
				delete(a.fulls, a.fullOrder[0])
				delete(a.fullBusy, a.fullOrder[0])
				a.fullOrder = a.fullOrder[1:]
			}
		})
	})
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
