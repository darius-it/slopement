package gallery

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

func TestLayoutDefaults(t *testing.T) {
	want := kit.NavRail
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		want = kit.NavTabs
	}
	if got := kit.DefaultNavLayout(); got != want {
		t.Fatalf("default = %v, want %v", got, want)
	}
	if testApp().navLayout != want {
		t.Fatal("the gallery does not start on the platform's default")
	}
	// Phones have one layout, the tabs, whatever was chosen; wide windows
	// show the choice.
	if kit.NavRail.Resolve(true) != kit.NavTabs || kit.NavRail.Resolve(false) != kit.NavRail ||
		kit.NavInbox.Resolve(true) != kit.NavTabs || kit.NavInbox.Resolve(false) != kit.NavInbox ||
		kit.NavTabs.Resolve(false) != kit.NavTabs {
		t.Fatal("Resolve")
	}
	if len(kit.NavLayouts) != 3 {
		t.Fatalf("%d layouts, want Rail, Tabs, One inbox", len(kit.NavLayouts))
	}
}

// TestPickLayout picks One inbox in Settings; the Desktop screen follows.
func TestPickLayout(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Settings", 1
	tt := ui.NewTester(a.View, 1440, 1000)
	tt.Frame()
	if err := tt.Click("One inbox"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.navLayout != kit.NavInbox {
		t.Fatalf("layout = %v", a.navLayout)
	}
	a.page = "Desktop"
	tt.Frame()
	if !tt.HasText("Inbox") || !tt.HasText("Communities") {
		t.Fatal("the Desktop screen is not the inbox")
	}
	// Tabs: the Chats | Communities switch.
	a.navLayout = kit.NavTabs
	tt.Frame()
	if _, ok := tt.Find("Chats"); !ok {
		t.Fatal("no Chats tab")
	}
}
