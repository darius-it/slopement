package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/format"
)

// TestMentionPill renders a mention: the parser makes a Mention node, and
// the body must show its name in a pill rather than drop it.
func TestMentionPill(t *testing.T) {
	doc := format.Parse("hey @Robin can you check?")
	for _, onAccent := range []bool{false, true} {
		view := func(c *ui.Context) {
			Apply(c)
			ui.Column(c).Padding(10).Children(func() {
				MessageBody(c, "m1", doc, Reveals{}, BodyStyle{OnAccent: onAccent})
			})
		}
		tt := ui.NewTester(view, 600, 120)
		tt.Frame()
		if !tt.HasText("@Robin") {
			t.Fatalf("OnAccent %v: the mention's name was not shown", onAccent)
		}
	}
}
