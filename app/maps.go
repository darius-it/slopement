package app

import (
	"context"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
	"slopement/osmmap"
)

// Maps of shared places. Off unless the user turned it on (Settings → Privacy →
// Maps): OpenStreetMap's tile servers then see this device's IP address and the
// area shown. A map is a few tiles joined by package osmmap, fetched off the UI
// thread, two maps at a time at most, kept decoded by place (the oldest dropped
// first), and tried once per place.

const (
	mapW, mapH = 256, 144 // pixels: at most four tiles
	mapZoom    = 16
	mapMax     = 48 // maps kept
	mapAtOnce  = 2  // fetched at the same time
)

// mapFetch gets one tile; a variable, so tests need no network.
var mapFetch osmmap.Fetch = osmmap.HTTPFetch(nil)

// mapKey names a map by its place, as a meter's worth.
func mapKey(l *model.Location) string {
	return strconv.FormatFloat(l.Lat, 'f', 5, 64) + "," + strconv.FormatFloat(l.Lon, 'f', 5, 64)
}

// mapPicture is the map of a place, or nil while it is fetched, when maps are
// off, or when it could not be drawn.
func (a *App) mapPicture(l *model.Location) ui.ImageSource {
	if l == nil || !a.prefs.showMaps {
		return nil
	}
	key := mapKey(l)
	if b, ok := a.maps[key]; ok {
		return b
	}
	if a.runUI == nil || a.mapBusy[key] || a.mapActive >= mapAtOnce {
		return nil
	}
	a.mapBusy[key] = true
	a.mapActive++
	runUI, lat, lon := a.runUI, l.Lat, l.Lon
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var bmp *ui.Bitmap
		data, err := osmmap.Render(ctx, mapFetch, lat, lon, mapZoom, mapW, mapH)
		if err == nil {
			bmp, err = ui.DecodeBitmapScaled(data, mapW*2)
		}
		runUI(func() {
			a.mapActive--
			if err != nil || bmp == nil {
				return // the plain card stays; busy stays set: one try per place
			}
			a.maps[key] = bmp
			a.mapOrder = append(a.mapOrder, key)
			if len(a.mapOrder) > mapMax {
				delete(a.maps, a.mapOrder[0])
				delete(a.mapBusy, a.mapOrder[0])
				a.mapOrder = a.mapOrder[1:]
			}
		})
	}()
	return nil
}
