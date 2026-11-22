package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"slopement/format"
	"slopement/linkpreview"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// The three switches are real: they show on their pages without a Soon tag,
// and what they set survives a restart.
func TestFormerlySoonSwitchesAreBuilt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 1500)
	for _, tc := range []struct{ page, row string }{
		{"Appearance", "Animate profile pictures"},
		{"Appearance", "Hide joins and leaves"},
		{"Privacy", "Fetch previews from this device"},
	} {
		a.view, a.settings = viewSettings, tc.page
		settle(tt)
		if !tt.HasText(tc.row) {
			t.Fatalf("%s: no %q", tc.page, tc.row)
		}
		for _, acc := range tt.AccessAll() {
			if strings.Contains(acc.Name, tc.row) && strings.Contains(acc.Name, "coming soon") {
				t.Fatalf("%s: %q is still marked as coming soon", tc.page, tc.row)
			}
		}
	}
	a.prefs.animatedAvatars, a.prefs.hideJoins, a.prefs.previewsDirect = true, true, true
	b := testApp()
	b.applyState(a.state())
	if !b.prefs.animatedAvatars || !b.prefs.hideJoins || !b.prefs.previewsDirect {
		t.Fatalf("after a restart: %+v", b.prefs)
	}
}

// membershipSource adds lines of people joining, leaving and being banned to
// a room's timeline.
type membershipSource struct {
	model.Source
	room string
}

func (s *membershipSource) Timeline(room string) []model.Event {
	tl := append([]model.Event(nil), s.Source.Timeline(room)...)
	if room != s.room {
		return tl
	}
	now := time.Now()
	line := func(id, body, membership string) model.Event {
		return model.Event{ID: id, Room: room, Time: now, Kind: model.EventSystem, Body: body, Membership: membership}
	}
	return append(tl,
		line("$j", "Mira joined", "join"), line("$l", "Sam left", "leave"),
		line("$b", "Kai was banned", "ban"), line("$i", "Noor was invited", "invite"))
}

// "Hide joins and leaves" takes those lines out of the timeline, and only
// those: a ban or an invite is still said.
func TestHideJoinsAndLeaves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(&membershipSource{Source: m, room: "!crit"}, picker.New(nil))
	ids := func() map[string]bool {
		out := map[string]bool{}
		for _, r := range a.rows("!crit") {
			if r.ev != nil {
				out[r.ev.ID] = true
			}
		}
		return out
	}
	if got := ids(); !got["$j"] || !got["$l"] || !got["$b"] || !got["$i"] {
		t.Fatalf("the lines are not all shown by default: %v", got)
	}
	a.prefs.hideJoins = true
	got := ids()
	if got["$j"] || got["$l"] {
		t.Fatalf("joins and leaves still show: %v", got)
	}
	if !got["$b"] || !got["$i"] {
		t.Fatalf("a ban or an invite was hidden: %v", got)
	}
	if len(got) == 0 || len(a.rows("!crit")) == 0 {
		t.Fatal("the messages went too")
	}
}

// countingPreviewSource counts the questions put to the homeserver's preview.
type countingPreviewSource struct {
	model.Source
	asked atomic.Int32
}

func (p *countingPreviewSource) Room(id string) (model.Room, bool) {
	r, ok := p.Source.Room(id)
	r.Encrypted = false // previews in an encrypted room wait for the user's choice
	return r, ok
}

func (p *countingPreviewSource) LinkPreview(url string, done func(model.Preview, error)) {
	p.asked.Add(1)
	done(model.Preview{URL: url, Title: "From the homeserver"}, nil)
}

func pngPicture() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	return b.Bytes()
}

// With "Fetch previews from this device" on, the card comes from the page
// itself and nothing is asked of the homeserver; its picture is fetched from
// the site too. Turning it off asks the homeserver again.
func TestPreviewsFromThisDevice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/cover.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngPicture())
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><meta property="og:title" content="From the page">
<meta property="og:description" content="Its words"><meta property="og:image" content="/cover.png"></head></html>`))
		}
	}))
	defer srv.Close()

	m := mock.New(time.Now())
	m.Delay = 0
	src := &countingPreviewSource{Source: m}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }
	a.previewFetcher = &linkpreview.Client{AllowPrivate: true} // the test server is on this machine
	link := srv.URL + "/article"
	doc := format.Parse("worth a read " + link)
	ev := model.Event{ID: "$m1", Room: "!crit", Body: "worth a read " + link}
	wait := func(what string, ok func() bool) {
		t.Helper()
		for i := 0; i < 300; i++ {
			if ok() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", what)
	}

	// Off: the homeserver is asked, the page is not touched.
	a.linkCard(ev, doc)
	if got := a.linkCard(ev, doc); got == nil || got.Title != "From the homeserver" || src.asked.Load() != 1 || hits.Load() != 0 {
		t.Fatalf("via the homeserver: card %+v, asked %d, page hits %d", got, src.asked.Load(), hits.Load())
	}

	// On: it is asked again, from this device.
	a.prefs.previewsDirect = true
	a.linkCard(ev, doc)
	wait("the card from the page", func() bool { c := a.linkCard(ev, doc); return c != nil && c.Title == "From the page" })
	c := a.linkCard(ev, doc)
	if c.Description != "Its words" || src.asked.Load() != 1 {
		t.Fatalf("card %+v, the homeserver was asked %d times", c, src.asked.Load())
	}
	// Its picture comes from the site, by the same setting.
	if c.Image.Still == nil {
		wait("the picture", func() bool { c = a.linkCard(ev, doc); return c.Image.Still != nil })
	}
	if _, ok := a.stills[srv.URL+"/cover.png"]; !ok {
		t.Fatal("the picture was not fetched from the site")
	}

	// Off again: the homeserver once more, and the remote picture is not loaded.
	a.prefs.previewsDirect = false
	a.linkCard(ev, doc)
	if got := a.linkCard(ev, doc); got == nil || got.Title != "From the homeserver" || src.asked.Load() != 2 {
		t.Fatalf("back to the homeserver: %+v, asked %d", got, src.asked.Load())
	}
}

// An encrypted room's links wait for the user's choice in either mode.
func TestDirectPreviewsStillHonourEncryptedRooms(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Hello</title></head></html>`))
	}))
	defer srv.Close()
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.runUI = func(f func()) { f() }
	a.previewFetcher = &linkpreview.Client{AllowPrivate: true}
	a.prefs.previewsDirect = true
	var enc string
	for _, r := range m.Rooms() {
		if r.Encrypted {
			enc = r.ID
		}
	}
	if enc == "" {
		t.Skip("the mock has no encrypted room")
	}
	link := srv.URL + "/x"
	ev := model.Event{ID: "$e1", Room: enc, Body: link}
	a.prefs.previewsE2EE = false
	for i := 0; i < 3; i++ {
		if a.linkCard(ev, format.Parse(link)) != nil {
			t.Fatal("a card in an encrypted room the user has not allowed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if hits.Load() != 0 {
		t.Fatalf("the page was fetched %d times", hits.Load())
	}
}
