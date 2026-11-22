package app

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

func testApp() *App {
	src := mock.New(time.Now())
	src.Delay = 0 // no simulated replies
	return New(src, picker.New(nil))
}

func snap(t *testing.T, name string, tt *ui.Tester) {
	dir := os.Getenv("SNAP_DIR")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Logf("snapshot: %v", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Logf("snapshot: %v", err)
	}
}

func settle(tt *ui.Tester) {
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
}

// TestSettingsPages renders each settings page; with SNAP_DIR it writes
// each, for the account (avatar editor) and the choice rows (security,
// advanced) to be eyeballed.
func TestSettingsPages(t *testing.T) {
	a := testApp()
	a.view = viewSettings
	tt := ui.NewTester(a.View, 1200, 800)
	for _, pg := range settingsPages {
		a.settings = pg.name
		settle(tt)
		snap(t, "settings-"+strings.ToLower(pg.name), tt)
	}
}

// TestLayouts renders every layout, wide and phone, light and dark; with
// SNAP_DIR it writes each.
func TestLayouts(t *testing.T) {
	for _, l := range []kit.NavLayout{kit.NavRail, kit.NavTabs, kit.NavInbox} {
		for _, dark := range []bool{false, true} {
			name := map[kit.NavLayout]string{kit.NavRail: "rail", kit.NavTabs: "tabs", kit.NavInbox: "inbox"}[l] +
				map[bool]string{false: "-light", true: "-dark"}[dark]
			a := testApp()
			a.nav = l
			tt := ui.NewTesterWith(a.View, 1440, 900, ui.TesterOptions{Dark: dark})
			settle(tt)
			if !tt.HasText("Alex Rivera") {
				t.Fatalf("%s: no chats", name)
			}
			snap(t, "app-"+name, tt)
			p := ui.NewTesterWith(a.View, 390, 844, ui.TesterOptions{Dark: dark})
			a.pushed = false
			settle(p)
			snap(t, "app-phone-"+name, p)
		}
	}
}

// TestRoomInfo shows the room info panel, then mutes from its Mute menu:
// how long it's muted shows under the name, and Mute becomes Unmute.
func TestRoomInfo(t *testing.T) {
	a := testApp()
	a.nav = kit.NavRail
	a.open("!crit")
	a.panel = panelInfo
	tt := ui.NewTesterWith(a.View, 1440, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	snap(t, "app-info", tt)
	if err := tt.Click("Mute"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	snap(t, "app-info-menu", tt)
	if err := tt.ChooseMenuItem("For 2 hours"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if r, _ := a.src.Room("!crit"); !r.Muted || !tt.HasText("Unmute") {
		t.Fatal("not muted from the menu")
	}
	snap(t, "app-info-muted", tt)
}

// TestSendAndNavigate sends a message, opens a community room, People and
// Settings, and the switcher.
func TestSendAndNavigate(t *testing.T) {
	a := testApp()
	a.nav = kit.NavRail
	tt := ui.NewTester(a.View, 1440, 900)
	settle(tt)
	if a.room != "!dm-alex" {
		t.Fatalf("opened %q first", a.room)
	}
	a.draft(a.room).Text = "lunch tomorrow?"
	settle(tt)
	if err := tt.Click("Send"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !tt.HasText("lunch tomorrow?") {
		t.Fatal("the message wasn't sent")
	}
	// A community, then one of its rooms.
	if err := tt.Click("Design Collective"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if err := tt.Click("design-crit"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.room != "!crit" || !tt.HasText("Where should the offsite be?") {
		t.Fatalf("design-crit not open: %q", a.room)
	}
	snap(t, "app-crit", tt)
	if err := tt.Click("Members"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	snap(t, "app-members", tt)
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !tt.HasText("Your name's colour") {
		t.Fatal("settings didn't open")
	}
	snap(t, "app-settings", tt)
	a.switcher.Toggle()
	settle(tt)
	snap(t, "app-switcher", tt)
}
