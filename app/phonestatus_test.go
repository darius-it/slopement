package app

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// On a phone, as on desktop, your messages have a small circle on the
// bubble's side: hollow with a check while nobody has read it, filled with the
// reader's picture once they have. Alex answered in the mock DM, so his picture
// shows whatever the Read receipts setting is (his answer is public).
func TestPhoneOwnMessagesShowTheirStatus(t *testing.T) {
	for _, on := range []bool{false, true} {
		a := testApp()
		a.prefs.receipts = on
		a.open("!dm-alex")
		a.pushed = true
		tt := ui.NewTester(a.View, 400, 800)
		settle(tt)
		snap(t, "phone-status-"+map[bool]string{false: "off", true: "on"}[on], tt)
		if !tt.HasText("Read by") {
			t.Errorf("receipts %v: no reader circle on an answered message (texts: %v)", on, tt.Texts())
		}
	}
}

// The tab bar says where things are waiting: a count on the Chats tab and on
// the Communities tab, each from its own rooms (muted ones left out), the
// mentions in the accent. The tab's name for assistive technology carries it.
func TestPhoneTabBarCounts(t *testing.T) {
	a := testApp()
	a.pushed = false
	chats, comms := a.tabCounts()
	if chats.unread == 0 || comms.unread == 0 {
		t.Fatalf("the mock has nothing unread in both: chats %+v, communities %+v", chats, comms)
	}
	// By hand, from the rooms: the same numbers.
	var wantChats, wantComms tabCount
	for _, r := range a.roomList() {
		if r.Muted {
			continue
		}
		w := &wantChats
		if r.Community != "" {
			w = &wantComms
		}
		w.unread += r.Unread
		w.mentions += r.Mentions
	}
	if chats != wantChats || comms != wantComms {
		t.Fatalf("counts %+v %+v, want %+v %+v", chats, comms, wantChats, wantComms)
	}
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	snap(t, "phone-tabbar-counts", tt)
	for name, n := range map[string]tabCount{"Chats": chats, "Communities": comms} {
		label := name + ", " + strconv.Itoa(n.unread) + " unread"
		if n.mentions > 0 {
			label = name + ", " + strconv.Itoa(n.mentions) + " mentions"
		}
		if _, ok := tt.Find(label); !ok {
			t.Errorf("no tab labeled %q (texts: %v)", label, tt.Texts())
		}
	}
}

// unreadSource says the open room has unread messages, as it does when they
// arrive while the user has scrolled up.
type unreadSource struct {
	model.Source
	room            string
	unread, mention int
}

func (u unreadSource) Room(id string) (model.Room, bool) {
	r, ok := u.Source.Room(id)
	if ok && id == u.room {
		r.Unread, r.Mentions = u.unread, u.mention
	}
	return r, ok
}

// The jump arrow carries the room's unread count: what came while the user was
// scrolled away.
func TestPhoneJumpToLatestShowsUnread(t *testing.T) {
	a := testApp()
	a.src = unreadSource{Source: a.src, room: "!dm-alex", unread: 7}
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 360)
	settle(tt)
	a.listState("!dm-alex").ScrollTo(0, ui.Start)
	settle(tt)
	snap(t, "phone-jump-unread", tt)
	if _, ok := tt.Find("Jump to latest message, 7 unread"); !ok {
		t.Fatalf("the arrow does not carry the count (texts: %v)", tt.Texts())
	}
}

// Scrolled away from the newest messages, a small arrow takes you back to
// them; at the end it is not there.
func TestPhoneJumpToLatest(t *testing.T) {
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 360) // short, so the messages overflow
	settle(tt)
	if _, ok := tt.Find("Jump to latest message"); ok {
		t.Fatal("the arrow shows at the newest message")
	}
	ls := a.listState("!dm-alex")
	ls.ScrollTo(0, ui.Start)
	settle(tt)
	snap(t, "phone-jump", tt)
	if _, ok := tt.Find("Jump to latest message"); !ok {
		t.Fatalf("no arrow after scrolling up (texts: %v)", tt.Texts())
	}
	if err := tt.Click("Jump to latest message"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	settle(tt)
	if !ls.AtEnd() {
		t.Fatal("the arrow did not take the list to its end")
	}
	if _, ok := tt.Find("Jump to latest message"); ok {
		t.Fatal("the arrow stays at the end")
	}
}
