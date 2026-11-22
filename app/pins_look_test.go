package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// A pinned message in the panel is drawn like the timeline draws it —
// formatting and all — and its Unpin shows only while the pointer is on its
// card, inside it.
func TestPinCardRichAndHoverUnpin(t *testing.T) {
	a, src, room := actApp(t)
	rich := model.Event{ID: "$rich", Room: room.ID, Sender: room.Partner, Kind: model.EventText,
		Body: "- one\n- **two**", Formatted: "<ul><li>one</li><li><strong>two</strong></li></ul>", Time: a.now}
	src.timeline = timeline{src.timeline.Source, append(src.tl, rich)}
	a.messageChoice(room.ID, rich, kit.MsgPin)
	a.panel = panelPins

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if !tt.HasText("•") || !tt.HasText("two") {
		t.Fatal("the pinned list is not drawn as a list")
	}
	for _, src := range []string{"**two**", "<ul>", "- one"} {
		if tt.HasText(src) {
			t.Fatalf("the pinned message shows its source %q", src)
		}
	}
	if _, ok := tt.Find("Unpin"); ok {
		t.Fatal("Unpin shows with no pointer on the card")
	}
	// The pointer on the card's list (the panel is the right 300).
	tt.Move(1000, 120)
	settle(tt)
	snap(t, "pins-panel-hover", tt)
	u, ok := tt.Find("Unpin")
	if !ok {
		t.Fatal("Unpin does not show on hover")
	}
	if u.X < 1100-300 {
		t.Fatalf("Unpin %v is outside the panel", u)
	}
}

// The verify dialog: the two ways sit close, and the way out apart below.
func TestVerifyChoicesSpacing(t *testing.T) {
	a := testApp()
	a.verifier = &stubVerify{st: model.VerifyState{Phase: model.VerifyIdle, Needed: true}}
	tt := uiTester(a)
	tt.Frame()
	tt.Frame()
	snap(t, "verify-choices", tt)
	one, ok1 := tt.Find("Use another device")
	two, ok2 := tt.Find("Use your Security Key")
	out, ok3 := tt.Find("Can't verify? Reset")
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("the choices are not drawn")
	}
	between := two.Y - (one.Y + one.H)
	below := out.Y - (two.Y + two.H)
	if between > 8 {
		t.Errorf("the two ways are %v apart, want them together", between)
	}
	if below < 12 {
		t.Errorf("the way out is only %v under the last way", below)
	}
}
