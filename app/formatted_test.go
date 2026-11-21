package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/embeds"
	"slopement/format"
	"slopement/model"
)

// A message that came with HTML is drawn from it — a list has its bullets,
// whoever wrote it (this client's own sends included); one without is plain
// text, shown as typed, so a lone * or ~ and a dash are only characters;
// and a plain message that is only a GIF link is still the GIF.
func TestMessagesAreDrawnAsSent(t *testing.T) {
	a := testApp()
	room, _ := longTimeline(a, 2)
	now := time.Now()
	text := func(id, body string) model.Event {
		return model.Event{ID: id, Room: room.ID, Sender: room.Partner, Kind: model.EventText, Body: body, Time: now}
	}
	list := text("1", "- first\n- second")
	list.Formatted = "<ul>\n<li>first</li>\n<li>second</li>\n</ul>\n" // as Element writes it
	rich := text("2", "**big** ~~gone~~")
	rich.Formatted = "<strong>big</strong> <del>gone</del>"
	plain := text("3", "5 * 3 and ~a~ and - dash")
	plain.Plain = true
	gif := text("4", "https://static.klipy.com/ii/abc/x.gif")
	gif.Plain = true
	a.src = timeline{a.src, []model.Event{list, rich, plain, gif}}
	a.open(room.ID)

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	for _, want := range []string{"•", "first", "second", "big", "gone"} {
		if !tt.HasText(want) {
			t.Errorf("not drawn: %q", want)
		}
	}
	if tt.HasText("**big**") || tt.HasText("- first") {
		t.Error("the Markdown source is shown as well as the formatting")
	}
	if !tt.HasText("5 * 3 and ~a~ and - dash") {
		t.Error("plain text was not shown as typed")
	}

	if _, ok := embeds.SoleMedia(a.doc(gif)); !ok {
		t.Error("a plain message that is only a GIF link is no longer the GIF")
	}
	if got := format.HTML(a.doc(plain)); got != "5 * 3 and ~a~ and - dash" {
		t.Errorf("plain doc = %q", got)
	}
	// The mock's events say neither: Markdown, as before.
	md := model.Event{ID: "5", Body: "**x**"}
	if got := format.HTML(a.doc(md)); got != "<strong>x</strong>" {
		t.Errorf("markdown doc = %q", got)
	}
}
