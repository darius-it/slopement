package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

const (
	stillMax  = 64   // pictures kept decoded
	stillSide = 1024 // the longest side kept, in pixels
)

// still is an attachment's picture: the decoded one when it has arrived,
// otherwise nil (the placeholder) while it is fetched, off the UI thread,
// from the source. Pictures are kept by URL, the oldest dropped first.
func (a *App) still(m model.Media) ui.ImageSource {
	if m.Kind != model.MediaImage || !strings.HasPrefix(m.URL, "mxc://") {
		return nil
	}
	if s, ok := a.stills[m.URL]; ok {
		return s
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.win == nil || a.stillBusy[m.URL] {
		return nil
	}
	a.stillBusy[m.URL] = true
	win, url := a.win, m.URL
	loader.LoadMedia(m, func(data []byte, err error) {
		var bmp *ui.Bitmap
		if err == nil {
			bmp, err = ui.DecodeBitmapScaled(data, stillSide)
		}
		win.Update(func() {
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
