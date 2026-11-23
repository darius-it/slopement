package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// offlineSource is a source whose connection is down, or not.
type offlineSource struct {
	model.Source
	off *bool
}

func (o offlineSource) Offline() bool { return *o.off }

// With no connection the app says so across the top, in every layout, and the
// bar goes when the connection is back.
func TestOfflineBar(t *testing.T) {
	for _, nav := range []kit.NavLayout{kit.NavRail, kit.NavTabs, kit.NavInbox} {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		m := mock.New(time.Now())
		m.Delay = 0
		off := false
		a := New(offlineSource{m, &off}, picker.New(nil))
		a.nav = nav
		tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
		settle(tt)
		if tt.HasText("No internet connection") {
			t.Fatal("the bar shows while online")
		}
		off = true
		settle(tt)
		if !tt.HasText("No internet connection") {
			t.Fatalf("layout %v: no bar while offline", nav)
		}
		if nav == kit.NavRail {
			snap(t, "app-offline", tt)
		}
		off = false
		settle(tt)
		if tt.HasText("No internet connection") {
			t.Fatal("the bar stayed after the connection came back")
		}
	}
}

// retrySource keeps what the app asked to retry.
type retrySource struct {
	model.Source
	can   bool
	tried []string
}

func (r *retrySource) CanRetry(room, id string) bool { return r.can }
func (r *retrySource) Retry(room, id string)         { r.tried = append(r.tried, room+"/"+id) }

// A message that was not sent shows "Try again" where the source can send it
// again, and a press does; where it cannot, there is no button.
func TestTryAgainOnAFailedMessage(t *testing.T) {
	for _, nav := range []kit.NavLayout{kit.NavRail} {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		m := mock.New(time.Now())
		m.Delay = 0
		a := New(m, picker.New(nil))
		a.nav = nav
		me := a.src.Me().ID
		failed := model.Event{ID: "~local-f", Sender: me, Kind: model.EventText, Body: "never arrived", Time: time.Now(),
			Failed: true, Error: "Not sent: No internet connection."}
		rs := &retrySource{Source: timeline{a.src, []model.Event{failed}}, can: false}
		a.src = rs
		a.open("!crit")
		tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
		settle(tt)
		if tt.HasText("Try again") {
			t.Fatal("a button for what the source cannot retry")
		}
		rs.can = true
		settle(tt)
		if !tt.HasText("Try again") {
			t.Fatal("no \"Try again\" on a failed message")
		}
		snap(t, "app-try-again", tt)
		if err := tt.Click("Try again"); err != nil {
			t.Fatal(err)
		}
		settle(tt)
		if len(rs.tried) != 1 || rs.tried[0] != "!crit/~local-f" {
			t.Fatalf("retried: %v", rs.tried)
		}
	}
}

// A message that was not delivered says "Not sent" and why, once, and carries
// no check; the delivered one before it does.
func TestFailedMessageSaysWhy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.prefs.receipts = true
	me := a.src.Me().ID
	at := time.Now()
	tl := []model.Event{
		{ID: "$1", Sender: me, Kind: model.EventText, Body: "delivered", Time: at.Add(-time.Minute)},
		{ID: "~local-x", Sender: me, Kind: model.EventText, Body: "never arrived", Time: at, Failed: true, Error: "Not sent: No internet connection."},
	}
	a.src = timeline{a.src, tl}
	a.open("!crit")
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	if !tt.HasText("Not sent: No internet connection.") {
		t.Fatal("the reason is not said")
	}
	for _, acc := range tt.AccessAll() {
		if strings.Contains(acc.Name, "Not sent: Not sent") {
			t.Fatal("\"Not sent\" is said twice")
		}
	}
	if st := a.receiptStateOf("!crit"); st.sentID != "$1" {
		t.Fatalf("the check is on %q, want the delivered message", st.sentID)
	}
}
