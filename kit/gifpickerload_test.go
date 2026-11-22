package kit

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
)

// In a long page of GIFs only the tiles in the view ask to load; the ones
// scrolled out of it take their request back.
func TestPickerTilesLoadOnlyInView(t *testing.T) {
	const n = 120
	wanted, dropped := map[string]bool{}, map[string]bool{}
	m := &PickerModel{}
	for i := 0; i < n; i++ {
		id := "t" + strconv.Itoa(i)
		m.Results = append(m.Results, PickerTile{ID: id, Label: id, W: 100, H: 100,
			Want: func() { wanted[id] = true }, Drop: func() { dropped[id] = true }})
	}
	st := &PickerState{Tab: TabGIFs, Browse: "favorites"}
	st.Open = true
	view := func(c *ui.Context) {
		Apply(c)
		btn := ui.Button(c.Key("anchor"), "GIF")
		Picker(c, btn, st, m)
	}
	tt := ui.NewTester(view, 900, 700)
	for i := 0; i < 4; i++ {
		tt.Frame()
	}
	if len(wanted) == 0 || len(wanted) > n/2 {
		t.Fatalf("%d of %d tiles asked to load: only those in the view should", len(wanted), n)
	}
	if len(dropped) == 0 {
		t.Fatal("no tile out of the view took its request back")
	}
}

// A tile still loading shows the skeleton; the wave is a drawn frame only
// while it loads, so a loaded tile costs nothing.
func TestPickerSkeletonOnlyWhileLoading(t *testing.T) {
	loading := PickerTile{ID: "a", Label: "a", W: 100, H: 100, Want: func() {}, Drop: func() {}}
	failed := loading
	failed.ID, failed.Label, failed.Failed = "b", "b", true
	for name, tile := range map[string]PickerTile{"loading": loading, "failed": failed} {
		m := &PickerModel{Results: []PickerTile{tile}}
		st := &PickerState{Tab: TabGIFs, Browse: "favorites"}
		st.Open = true
		tt := ui.NewTester(func(c *ui.Context) {
			Apply(c)
			Picker(c, ui.Button(c.Key("anchor"), "GIF"), st, m)
		}, 900, 700)
		for i := 0; i < 3; i++ {
			tt.Frame()
		}
		if got := tt.HasText("Tap to retry"); got != (name == "failed") {
			t.Errorf("%s: retry mark shown = %v", name, got)
		}
	}
}
