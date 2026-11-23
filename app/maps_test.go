package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// A shared place is a plain card with its coordinates and nothing is fetched,
// until the user turns maps on; then it is drawn from tiles, with
// OpenStreetMap's credit under it, and the tiles are asked for once per place.
func TestSharedPlaceMapIsOptIn(t *testing.T) {
	var asked atomic.Int32
	old := mapFetch
	mapFetch = func(context.Context, string) ([]byte, error) {
		asked.Add(1)
		var b bytes.Buffer
		_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 256, 256)))
		return b.Bytes(), nil
	}
	defer func() { mapFetch = old }()

	a := testApp()
	a.nav = kit.NavRail
	a.runUI = func(fn func()) { fn() }
	room, tl := longTimeline(a, 3)
	tl = append(tl, model.Event{ID: "$place", Room: room.ID, Sender: room.Partner, Kind: model.EventText,
		Body: "Cafe", Time: time.Now(), Location: &model.Location{Lat: 52.52, Lon: 13.405, Description: "Cafe Einstein"}})
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)

	if !tt.HasText("Cafe Einstein") || !tt.HasText("Open in maps") {
		t.Fatal("the place is not shown as a card")
	}
	if a.prefs.showMaps || defaultPrefs().showMaps {
		t.Fatal("maps are on by default")
	}
	time.Sleep(50 * time.Millisecond)
	settle(tt)
	if asked.Load() != 0 || tt.HasText("© OpenStreetMap contributors") {
		t.Fatalf("a map was fetched with maps off (%d tiles)", asked.Load())
	}

	a.prefs.showMaps = true
	waitUntil(t, tt, "the map", func() bool { return tt.HasText("© OpenStreetMap contributors") })
	if asked.Load() < 1 || asked.Load() > 4 {
		t.Fatalf("asked for %d tiles", asked.Load())
	}
	n := asked.Load()
	for i := 0; i < 5; i++ {
		settle(tt)
	}
	if asked.Load() != n {
		t.Fatal("the same place was fetched again")
	}
	if !a.prefs.state().prefs().showMaps {
		t.Fatal("the choice was not kept")
	}
}
