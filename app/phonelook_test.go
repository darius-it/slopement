package app

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// typersSource says someone types in every room.
type typersSource struct {
	model.Source
	who []string
}

func (s typersSource) Room(id string) (model.Room, bool) {
	r, ok := s.Source.Room(id)
	r.Typing = s.who
	return r, ok
}

// On a phone, who is typing is a bubble of its own under the last message,
// in DMs and groups alike: it never sits over a message.
func TestPhoneTypingBubbleTakesItsRoom(t *testing.T) {
	for _, room := range []string{"!dm-alex", "!crit"} {
		a := testApp()
		r, _ := a.src.Room(room)
		who := "Alex Rivera"
		if r.Kind != model.KindDM {
			who = "Sam"
		}
		a.src = typersSource{a.src, []string{who}}
		a.open(room)
		a.pushed = true
		tt := ui.NewTester(a.View, 390, 844)
		settle(tt)
		snap(t, "phone-typing-"+room[1:], tt)
		if !tt.HasText(who + " is typing") {
			t.Fatalf("%s: no typing bubble (texts: %v)", room, tt.Texts())
		}
	}
}

// The phone's Communities tab is one page of cards: each community with its
// liveliest rooms under it, "All N rooms" unfolding the rest in place, and a
// room one tap away. No page to go into, no "All communities" back.
func TestPhoneCommunityCards(t *testing.T) {
	was := kit.SetPhoneOS(true)
	defer kit.SetPhoneOS(was)
	for _, dark := range []bool{false, true} {
		a := testApp()
		a.pushed = false
		a.phoneTab = 1
		tt := ui.NewTesterWith(a.View, 390, 844, ui.TesterOptions{Dark: dark})
		settle(tt)
		snap(t, "phone-communities-"+map[bool]string{false: "light", true: "dark"}[dark], tt)
		if tt.HasText("All communities") {
			t.Fatal("the old back is still there")
		}
		comms := a.communities()
		if len(comms) < 2 || !tt.HasText(comms[0].Name) || !tt.HasText(comms[1].Name) {
			t.Fatal("not every community has its card")
		}
		// The first card unfolds to all its rooms.
		cm := comms[0]
		ids := a.communityRoomIDs(cm, 0)
		if len(ids) <= phoneCommunityPeek {
			t.Fatalf("the mock's %s needs more than %d rooms", cm.Name, phoneCommunityPeek)
		}
		folded := a.peekRooms(ids)
		var hidden model.Room
		for _, id := range ids {
			shown := false
			for _, r := range folded {
				shown = shown || r.ID == id
			}
			if !shown {
				hidden, _ = a.roomByID(id)
			}
		}
		if tt.HasText(hidden.Name) {
			t.Fatalf("%q shows on a folded card", hidden.Name)
		}
		if err := tt.Click("All " + strconv.Itoa(len(ids)) + " rooms"); err != nil {
			t.Fatal(err)
		}
		settle(tt)
		snap(t, "phone-communities-open-"+map[bool]string{false: "light", true: "dark"}[dark], tt)
		if !tt.HasText(hidden.Name) {
			t.Fatalf("%q not shown once unfolded", hidden.Name)
		}
		// A room opens from the overview.
		if err := tt.Click(folded[0].Name); err != nil {
			t.Fatal(err)
		}
		settle(tt)
		if a.room != folded[0].ID || !a.pushed {
			t.Fatalf("room %q pushed %v, want %q", a.room, a.pushed, folded[0].ID)
		}
	}
}

// On a phone the GIF picker opens without the keyboard (its search is focused
// only when tapped), and has no footer row; on a computer the search takes the
// keys at once.
func TestPickerKeyboardOnlyOnTap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, phone := range []bool{true, false} {
		was := kit.SetPhoneOS(phone)
		a := testApp()
		a.open("!dm-alex")
		a.pushed = true
		w, h := 390, 844
		if !phone {
			w, h = 1200, 800
		}
		tt := ui.NewTesterWith(a.View, w, h, ui.TesterOptions{Dark: true})
		settle(tt)
		a.picker.Open = true
		settle(tt)
		name := map[bool]string{true: "phone", false: "desktop"}[phone]
		snap(t, "picker-"+name, tt)
		if got := tt.Focused("Search KLIPY"); got != !phone {
			t.Errorf("%s: search focused = %v", name, got)
		}
		kit.SetPhoneOS(was)
	}
}

// The phone's + opens a sheet of large tiles.
func TestPhonePlusSheet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	was := kit.SetPhoneOS(true)
	defer kit.SetPhoneOS(was)
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTesterWith(a.View, 390, 844, ui.TesterOptions{Dark: true})
	settle(tt)
	if err := tt.Click("Attach or create"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	snap(t, "phone-plus", tt)
	for _, l := range []string{"Photos", "Camera", "File", "Poll"} {
		if !tt.HasText(l) {
			t.Fatalf("no %q tile (texts: %v)", l, tt.Texts())
		}
	}
}
