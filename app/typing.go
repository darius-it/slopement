package app

import "time"

// Sending typing notices: while the user writes in a room's composer the room
// is told, again every 25 seconds (the server lets a notice last 30), and
// told they stopped five seconds after the last change, when the text is
// emptied or sent, when another room opens, and when the setting is turned
// off. Only the room whose composer is built is ever typed in.

// (Variables, for the tests.)
var (
	typingIdle    = 5 * time.Second  // no change for this long: stopped
	typingRefresh = 25 * time.Second // a notice is sent again after this
)

// typingRoom is one room's typing as told to it.
type typingRoom struct {
	text   string    // the draft as last seen
	last   time.Time // when it last changed (with words in it)
	sentAt time.Time // when the last "typing" went
	active bool      // the room has been told, and not that it stopped
}

// sendTyping tells a room, if the source can.
func (a *App) sendTyping(room string, typing bool) {
	if t, ok := a.src.(interface{ SetTyping(string, bool) }); ok {
		t.SetTyping(room, typing)
	}
}

// typingStep is called each frame for the room whose composer is shown, with
// its draft: it starts (and keeps up) the typing notice while the text
// changes.
func (a *App) typingStep(room, text string) {
	if !a.prefs.typing {
		return
	}
	if _, ok := a.src.(interface{ SetTyping(string, bool) }); !ok {
		return
	}
	if a.typing == nil {
		a.typing = map[string]*typingRoom{}
	}
	t := a.typing[room]
	if t == nil {
		t = &typingRoom{}
		a.typing[room] = t
	}
	now := time.Now()
	if text != t.text {
		t.text = text
		if hasWords(text) {
			t.last = now
		}
	}
	if hasWords(text) && now.Sub(t.last) < typingIdle {
		if !t.active || now.Sub(t.sentAt) >= typingRefresh {
			a.sendTyping(room, true)
			t.active, t.sentAt = true, now
		}
		if a.win != nil { // a frame when it will have been idle long enough
			win := a.win
			time.AfterFunc(typingIdle+50*time.Millisecond, func() { win.Update(func() {}) })
		}
	}
}

// typingTick stops the typing notices that have gone quiet, or all of them
// when the setting is off. Called every frame, whatever is on screen.
func (a *App) typingTick() {
	if len(a.typing) == 0 {
		return
	}
	now := time.Now()
	for room, t := range a.typing {
		if !t.active {
			continue
		}
		if !a.prefs.typing || !hasWords(t.text) || now.Sub(t.last) >= typingIdle {
			a.sendTyping(room, false)
			t.active = false
		}
	}
}

// stopTyping tells a room the user stopped (they left it for another).
func (a *App) stopTyping(room string) {
	if t := a.typing[room]; t != nil && t.active {
		a.sendTyping(room, false)
		t.active = false
	}
}
