package app

import "github.com/egoist/mygo/ui"

// A phone's keyboard follows the focus: it shows while a text input has it
// and goes when the focus is somewhere else. Leaving a room by Back, a swipe
// or a tab removes the composer from the tree while it still has the focus,
// and the keyboard, never told, stays up over the lists (sometimes: only
// when the composer was removed before anything else took the focus).
//
// blurSink is a one-pixel element that takes no room and holds the focus
// for no one: the focus is handed to it as a room closes, which ends the
// keyboard's text session.

// blurSinkView builds the sink. Call it once a frame, anywhere in the root.
func (a *App) blurSinkView(c *ui.Context) {
	ui.Box(c.Key("blur-sink")).Size(1, 1).Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).PassThrough().
		Focusable().TabStop(false).Label("").Bind(&a.blurSink)
}

// blurStep hands the focus to the sink in the frame a room closes (or the
// view changes away from it), so the keyboard goes with it.
func (a *App) blurStep(inRoom bool) {
	if a.wasInRoom && !inRoom {
		a.blurSink.Focus()
	}
	a.wasInRoom = inRoom
}
