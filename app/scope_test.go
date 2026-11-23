package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// Going from a community back to Chats brings back the chat last open there,
// with the keyboard in its composer; a list with no remembered room shows the
// empty "Select" pane.
func TestListsRememberTheirLastRoom(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	cms := a.communities()
	if len(cms) == 0 {
		t.Skip("needs a community")
	}
	a.open("!dm-alex")
	settle(tt)
	if a.scopeRooms[""] != "!dm-alex" {
		t.Fatalf("Chats did not remember its room: %v", a.scopeRooms)
	}

	a.community = cms[0].ID // into a space that has had no room open
	settle(tt)
	if a.room != "" || !tt.HasText("Select a room in "+cms[0].Name) {
		t.Fatalf("a community with no chosen room should be empty, room %q", a.room)
	}

	a.community = "" // back to Chats: by default the empty pane
	settle(tt)
	if a.room != "" || !tt.HasText("Select a chat") {
		t.Fatalf("without the setting, Chats should be empty, room %q", a.room)
	}

	a.prefs.jumpLast = true // opted in: the last chat comes back
	a.community = cms[0].ID
	settle(tt)
	a.focusRoom = ""
	a.community = ""
	settle(tt)
	if a.room != "!dm-alex" {
		t.Fatalf("the last chat did not come back: %q", a.room)
	}
	tt.Type("hi")
	settle(tt)
	if got := a.draft("!dm-alex").Text; got != "hi" {
		t.Fatalf("the composer was not focused, draft %q", got)
	}
}
