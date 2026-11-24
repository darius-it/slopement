package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// On a phone a GIF's favorite star is not over the picture in the timeline
// (there is no hover to bring it only when wanted); it is in the viewer's top
// bar, and pressing it toggles the favorite.
func TestPhoneGIFStarIsInTheViewer(t *testing.T) {
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	var calls []bool
	gif := kit.MediaSpec{ID: "g", Kind: kit.MediaGIF, W: 240, H: 180, Label: "a gif",
		Star: func(on bool) { calls = append(calls, on) }}

	// In the timeline: no star over it, though it has one to give.
	tt := ui.NewTester(func(c *ui.Context) {
		kit.Apply(c)
		ui.Column(c).Padding(20).Children(func() { kit.MediaPreview(c, gif, 240, 180, nil) })
	}, 400, 400)
	settle(tt)
	if _, ok := tt.Find("Add to favorite GIFs"); ok {
		t.Fatal("the star shows over a GIF on a phone")
	}

	// In the viewer: the star in the top bar, which toggles it.
	b := testApp()
	b.pushed = true
	b.viewerItems = []kit.MediaSpec{gif}
	b.viewerAt, b.viewer = 0, true
	vt := ui.NewTester(b.View, 400, 800)
	settle(vt)
	snap(t, "viewer-star", vt)
	if err := vt.Click("Add to favorite GIFs"); err != nil {
		t.Fatalf("no star in the viewer's top bar: %v (texts: %v)", err, vt.Texts())
	}
	settle(vt)
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("pressing the star called %v, want one call with true", calls)
	}
	if _, ok := vt.Find("Remove from favorite GIFs"); !ok {
		t.Fatalf("the star did not fill after it was pressed (texts: %v)", vt.Texts())
	}
	if err := vt.Click("Remove from favorite GIFs"); err != nil {
		t.Fatal(err)
	}
	settle(vt)
	if len(calls) != 2 || calls[1] {
		t.Fatalf("pressing it again called %v, want true then false", calls)
	}
}
