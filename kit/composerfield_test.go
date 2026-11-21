package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// composerRig is a composer in a window, with what it asked for each frame.
type composerRig struct {
	tt   *ui.Tester
	st   *ComposerState
	sent int
}

func newRig(t *testing.T, w, h int, phone bool) *composerRig {
	t.Helper()
	r := &composerRig{st: &ComposerState{}}
	colors := &UserColors{}
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Fill().Justify(ui.End).Children(func() {
			var res ComposerResult
			if phone {
				res = PhoneComposer(c.Key("c"), r.st, "Message", colors)
			} else {
				res = Composer(c.Key("c"), r.st, "Message", colors)
			}
			if res.Send {
				r.sent++
			}
		})
	}
	r.tt = ui.NewTester(view, w, h)
	r.tt.Frame()
	if err := r.tt.Click("Message"); err != nil {
		t.Fatalf("no field to click: %v", err)
	}
	r.tt.Frame()
	return r
}

func (r *composerRig) settle() { r.tt.Frame(); r.tt.Frame() }

// What is typed is what is sent: no Markdown shortcut turns it into
// something else, no formatting key formats it, and Enter sends while
// Shift+Enter breaks the line.
func TestComposerFieldIsPlainText(t *testing.T) {
	r := newRig(t, 600, 300, false)
	r.tt.Type("# not a heading")
	r.settle()
	r.tt.Key(ui.Super, ui.KeyB) // ⌘B must not bold
	r.tt.Key(ui.Ctrl, ui.KeyB)
	r.tt.Type(" **not bold**")
	r.settle()
	if r.st.Text != "# not a heading **not bold**" {
		t.Fatalf("text = %q", r.st.Text)
	}
	if got := r.st.Draft(); got != r.st.Text {
		t.Fatalf("draft = %q, text = %q", got, r.st.Text)
	}
	for _, para := range r.st.doc.Text().Paragraphs() {
		if para.Style.Heading != 0 {
			t.Fatal("a Markdown shortcut made a heading")
		}
		for _, run := range para.Runs {
			if run.Attributes.Bold() || run.Attributes.Italic || run.Attributes.Underline {
				t.Fatalf("formatted run %+v", run)
			}
		}
	}

	r.tt.Key(ui.Shift, ui.KeyEnter)
	r.tt.Type("second line")
	r.settle()
	if r.st.Draft() != "# not a heading **not bold**\nsecond line" || r.sent != 0 {
		t.Fatalf("Shift+Enter: draft %q, sent %d", r.st.Draft(), r.sent)
	}
	r.tt.Key(0, ui.KeyEnter)
	r.settle()
	if r.sent != 1 {
		t.Fatalf("Enter did not send (sent %d)", r.sent)
	}
}

// A mention chosen from the suggestions is a marked run — drawn in a frame —
// and goes as a link to the person; what is typed around it is plain; text
// set from outside (the field cleared, a draft restored) has no marks.
func TestMentionMarkAndDraft(t *testing.T) {
	r := newRig(t, 600, 300, false)
	r.tt.Type("hi @al")
	r.settle()
	r.st.Complete(3, SuggestItem{Insert: "@Alex Rivera", ID: "@alex:example.org"})
	r.tt.Frame()
	r.tt.Type("see you")
	r.settle()
	if r.st.Text != "hi @Alex Rivera see you" {
		t.Fatalf("text = %q", r.st.Text)
	}
	if got, want := r.st.Draft(), "hi [@Alex Rivera](https://matrix.to/#/@alex:example.org) see you"; got != want {
		t.Fatalf("draft = %q, want %q", got, want)
	}
	marked := 0
	for _, para := range r.st.doc.Text().Paragraphs() {
		for _, run := range para.Runs {
			if run.Attributes.Tag != "" {
				marked++
				if run.Text != "@Alex Rivera" {
					t.Fatalf("the mark covers %q", run.Text)
				}
			}
		}
	}
	if marked != 1 {
		t.Fatalf("marked runs = %d", marked)
	}
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "composer-mention.png"))
		_ = png.Encode(f, r.tt.Image())
		f.Close()
	}

	r.st.Text = "" // as a send clears it
	r.tt.Frame()
	if r.st.Draft() != "" {
		t.Fatalf("draft after clearing = %q", r.st.Draft())
	}
	r.st.Text = "restored @Alex Rivera"
	if strings.Contains(r.st.Draft(), "matrix.to") {
		t.Fatal("a restored draft has a mention mark it never had")
	}
}

// Several lines grow the field only up to a limit, and the phone's composer
// is the same field.
func TestComposerFieldGrowsToALimit(t *testing.T) {
	for _, phone := range []bool{false, true} {
		r := newRig(t, 500, 700, phone)
		one, _ := r.tt.Find("Message")
		_ = one
		for i := 0; i < 30; i++ {
			r.tt.Key(ui.Shift, ui.KeyEnter)
			r.tt.Type("line")
		}
		r.settle()
		var field ui.Rect
		for _, a := range r.tt.AccessAll() {
			if a.Name == "Message" && a.Bounds.H > field.H {
				field = a.Bounds
			}
		}
		if field.H == 0 || field.H > 250 {
			t.Fatalf("phone=%v: the field is %.0f DIP tall with 30 lines: no limit", phone, field.H)
		}
		if got := strings.Count(r.st.Draft(), "\n"); got != 30 {
			t.Fatalf("phone=%v: %d line breaks kept", phone, got)
		}
	}
}
