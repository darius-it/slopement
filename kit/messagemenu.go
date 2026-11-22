package kit

import "github.com/egoist/mygo/ui"

// MessageMenuState says what a message's menu offers: what can be done to
// this message by this user.
type MessageMenuState struct {
	// Mine: sent by the user, so it can be edited and deleted.
	Mine bool
	// HasText: it has words to copy (and to edit).
	HasText bool
	// Pinned: it is pinned in its room, so the menu offers to unpin it.
	Pinned bool
	// Off hides the menu (a message still being sent, or a lock tile).
	Off bool
	// Thread offers "Reply in thread" (where the account has threads and
	// there is room for the thread panel).
	Thread bool
	// Sticker offers "Add to my emoji": it is a sticker (or custom emoji)
	// that can go in the user's own pack.
	Sticker bool
}

// MessageChoice is what was chosen from a message's menu.
type MessageChoice int

const (
	MsgNone MessageChoice = iota
	MsgReply
	MsgCopyText
	MsgCopyLink
	MsgEdit
	MsgPin
	MsgUnpin
	MsgDelete
	MsgThread // reply in (or open) the message's thread
	// MsgAddToPack puts the message's sticker in the user's own pack.
	MsgAddToPack
	// MsgOpenSheet asks for the menu as a sheet (a long press on a phone).
	MsgOpenSheet
)

// MessageMenu fills m with what can be done to a message and reports the
// one chosen. Use it as the ⋯ button's menu and as the message's context
// menu (right click; a long press on phones).
func MessageMenu(m *ui.Menu, s MessageMenuState) (ch MessageChoice) {
	if m.Item("Reply").Icon(MenuIcon(IconReply)).Chosen() {
		ch = MsgReply
	}
	if s.Thread && m.Item("Reply in thread").Icon(MenuIcon(IconMessage)).Chosen() {
		ch = MsgThread
	}
	if s.HasText && m.Item("Copy text").Icon(MenuIcon(IconCopy)).Chosen() {
		ch = MsgCopyText
	}
	if m.Item("Copy message link").Icon(MenuIcon(IconLink)).Chosen() {
		ch = MsgCopyLink
	}
	if s.Sticker && m.Item("Add to my emoji").Icon(MenuIcon(IconPlus)).Chosen() {
		ch = MsgAddToPack
	}
	if s.Pinned {
		if m.Item("Unpin").Icon(MenuIcon(IconPin)).Chosen() {
			ch = MsgUnpin
		}
	} else if m.Item("Pin").Icon(MenuIcon(IconPin)).Chosen() {
		ch = MsgPin
	}
	if s.Mine {
		m.Separator()
		if s.HasText && m.Item("Edit").Icon(MenuIcon(IconPencil)).Chosen() {
			ch = MsgEdit
		}
		if m.Item("Delete").Icon(MenuIcon(IconTrash)).Danger().Chosen() {
			ch = MsgDelete
		}
	}
	return ch
}
