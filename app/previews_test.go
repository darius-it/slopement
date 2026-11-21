package app

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// previewSource answers link previews, and counts the requests.
type previewSource struct {
	timeline
	asked []string
	fail  bool
	enc   bool
}

func (s *previewSource) LinkPreview(url string, done func(model.Preview, error)) {
	s.asked = append(s.asked, url)
	if s.fail {
		done(model.Preview{}, errors.New("no preview"))
		return
	}
	done(model.Preview{URL: url, Site: "Example Site", Title: "An article about things", Description: "What the page says."}, nil)
}

func (s *previewSource) Room(id string) (model.Room, bool) {
	r, ok := s.timeline.Source.Room(id)
	r.Encrypted = s.enc
	return r, ok
}

func previewApp(t *testing.T, src *previewSource, link string) (*App, string) {
	t.Helper()
	a := testApp()
	room, tl := longTimeline(a, 4)
	tl[3].Body = "look at " + link
	src.timeline = timeline{a.src, tl}
	a.src = src
	a.runUI = func(f func()) { f() }
	a.open(room.ID)
	return a, room.ID
}

// A link's card comes from the homeserver: asked once, shown when it has
// something to say; a failure is not asked again.
func TestLinkCardFromServer(t *testing.T) {
	src := &previewSource{}
	a, _ := previewApp(t, src, "https://example.org/article")
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	settle(tt)
	if len(src.asked) != 1 || src.asked[0] != "https://example.org/article" {
		t.Fatalf("asked = %v", src.asked)
	}
	if !tt.HasText("An article about things") || !tt.HasText("Example Site") {
		t.Fatal("the card does not show")
	}
	settle(tt)
	if len(src.asked) != 1 {
		t.Fatalf("asked again: %v", src.asked)
	}

	bad := &previewSource{fail: true}
	a2, _ := previewApp(t, bad, "https://example.org/gone")
	tt2 := ui.NewTester(a2.View, 1100, 700)
	for i := 0; i < 4; i++ {
		settle(tt2)
	}
	if len(bad.asked) != 1 {
		t.Fatalf("a failed preview was asked for %d times", len(bad.asked))
	}
	if tt2.HasText("An article about things") {
		t.Fatal("a card from a failed preview")
	}
}

// In an encrypted room the link would go to the homeserver, so nothing is
// asked until the user allows it; the setting off hides cards everywhere.
func TestLinkCardPrivacy(t *testing.T) {
	src := &previewSource{enc: true}
	a, _ := previewApp(t, src, "https://example.org/secret")
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	settle(tt)
	if len(src.asked) != 0 {
		t.Fatalf("a link in an encrypted room was sent to the server: %v", src.asked)
	}

	a.prefs.previewsE2EE = true
	settle(tt)
	settle(tt)
	if len(src.asked) != 1 {
		t.Fatalf("allowed, but not asked: %v", src.asked)
	}

	off := &previewSource{}
	a2, _ := previewApp(t, off, "https://example.org/x")
	a2.prefs.linkPreviews = false
	tt2 := ui.NewTester(a2.View, 1100, 700)
	settle(tt2)
	settle(tt2)
	if len(off.asked) != 0 {
		t.Fatal("asked with link previews turned off")
	}
}
