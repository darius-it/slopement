package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A profile picture on the card is a button that asks to enlarge it; one with
// no picture is not.
func TestProfileCardPictureOpens(t *testing.T) {
	var got ProfileAction
	spec := ProfileSpec{Name: "Kai", MXID: "@kai:example.org", Picture: true}
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c)
		if a := ProfileCard(c, spec, &UserColors{}, time.Now()); a.OpenPicture {
			got = a
		}
	}, 600, 600)
	tt.Frame()
	if err := tt.Click("View profile picture"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !got.OpenPicture {
		t.Fatal("clicking the picture did not ask to enlarge it")
	}
	spec.Picture = false
	tt2 := ui.NewTester(func(c *ui.Context) {
		Apply(c)
		ProfileCard(c, spec, &UserColors{}, time.Now())
	}, 600, 600)
	tt2.Frame()
	if err := tt2.Click("View profile picture"); err == nil {
		t.Fatal("a card with no picture has a picture button")
	}
}

// The card's Ignore and Unignore buttons return explicit actions (not a
// toggle: a card can be built twice in a frame), not on one's own card.
func TestProfileCardIgnoreButtons(t *testing.T) {
	var spec ProfileSpec
	var got ProfileAction // what any frame returned
	colors := &UserColors{}
	view := func(c *ui.Context) {
		Apply(c)
		a := ProfileCard(c, spec, colors, time.Now())
		got.Ignore = got.Ignore || a.Ignore
		got.Unignore = got.Unignore || a.Unignore
	}
	press := func(label string) {
		got = ProfileAction{}
		tt := ui.NewTester(view, 600, 600)
		tt.Frame()
		if err := tt.Click(label); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		tt.Frame()
	}

	spec = ProfileSpec{Name: "Kai", MXID: "@kai:example.org", Ignorable: true}
	press("Ignore")
	if !got.Ignore || got.Unignore {
		t.Fatalf("Ignore pressed: %+v", got)
	}
	spec.Ignored = true
	press("Unignore")
	if !got.Unignore || got.Ignore {
		t.Fatalf("Unignore pressed: %+v", got)
	}
	// Your own card has neither.
	spec = ProfileSpec{Name: "Me", MXID: "@me:example.org", Ignorable: true, Self: true}
	tt := ui.NewTester(view, 600, 600)
	tt.Frame()
	if tt.HasText("Ignore") {
		t.Fatal("you can ignore yourself")
	}
}
