package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// The chat lists build only the rows in view: with hundreds of chats a frame
// must not build a row for each (that made typing sluggish).
func TestChatListsBuildOnlyWhatShows(t *testing.T) {
	for _, layout := range []struct {
		name string
		nav  kit.NavLayout
	}{{"rail", kit.NavRail}, {"tabs", kit.NavTabs}, {"inbox", kit.NavInbox}} {
		t.Run(layout.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			m := mock.New(time.Now())
			m.Delay = 0
			m.AddBulk(600, 0, 0, 0)
			a := New(m, picker.New(nil))
			a.nav = layout.nav
			tt := ui.NewTesterWith(a.View, 1300, 800, ui.TesterOptions{Dark: true})
			settle(tt)
			rows := 0
			for _, acc := range tt.AccessAll() {
				if strings.HasPrefix(acc.Name, "Person ") {
					rows++
				}
			}
			if rows == 0 {
				t.Fatal("no chat rows at all")
			}
			if rows > 60 {
				t.Fatalf("%d chat rows built of 600: the list is not virtual", rows)
			}
		})
	}
}

// Changes to the source show at once, though the lists are kept between them.
func TestListsFollowTheSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 800, ui.TesterOptions{Dark: true})
	settle(tt)
	if tt.HasText("Zed Newcomer") {
		t.Fatal("already there")
	}
	m.AddBulk(1, 0, 0, 0) // "Person 0", and the source says it changed
	settle(tt)
	if !tt.HasText("Person 0") {
		t.Fatal("a room added to the source did not show: the cached list is stale")
	}
}
