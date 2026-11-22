package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// AccountEntry is one account signed in on this device, as the account
// menu lists it.
type AccountEntry struct {
	User    string // the MXID
	Server  string
	Current bool
}

// Accounts is what the host (main.go, which owns the sign-in and the sync)
// gives the app to switch between the accounts of this device. Nil, as on
// the mock, leaves the menu with the status and the profile only.
type Accounts struct {
	// List is the accounts, the current one marked. Cheap: called as the
	// menu builds.
	List func() []AccountEntry
	// Switch makes another account the one the app shows.
	Switch func(user string)
	// Add opens the sign-in to add another account.
	Add func()
	// Remove logs an account out on this device (its sign-in; the messages
	// stay on the server).
	Remove func(user string)
}

// openDM opens the chat with someone and calls then with its room: the DM
// there is, or one made now (a source that can: the real account's), or
// nothing — said as a toast — when it could not be made.
func (a *App) openDM(person string, then func(room string)) {
	if room := a.src.OpenDM(person); room != "" {
		then(room)
		return
	}
	mk, ok := a.src.(model.DMCreator)
	if !ok {
		a.toast = "Could not start a chat with " + person
		return
	}
	a.toast = "Starting the chat…"
	mk.OpenDMAsync(person, func(room string) {
		if room == "" {
			a.toast = "Could not start a chat with " + person
			return
		}
		a.toast = ""
		then(room)
	})
}

// SetAccounts gives the app its accounts.
func (a *App) SetAccounts(acc *Accounts) { a.accounts = acc }

// meMenu fills the menu of the user's picture: the status, the profile, the
// accounts of this device with one to switch to, adding one, and removing.
func (a *App) meMenu(m *ui.Menu) {
	me := a.src.Me()
	m.Item(me.Name).Subtitle(me.ID).Disabled(true)
	m.Separator()
	if m.Item("Set status…").Chosen() {
		a.statusDraft.Edit(status(me.Status))
	}
	if m.Item("View profile").Chosen() {
		a.profileOpen, a.profileSheet.Open = me.ID, true
	}
	acc := a.accounts
	if acc == nil || acc.List == nil {
		return
	}
	m.Separator()
	// One submenu for all of it, as Discord's: a row per account (picture,
	// name, a mark on the current one, Log out) and Add account as the last.
	m.Submenu("Switch account", func(m *ui.Menu) {
		for _, e := range acc.List() {
			a.accountRow(m, e)
		}
		m.Separator()
		if acc.Add == nil {
			return
		}
		// The plus is drawn in the row, in the pictures' column: an item
		// Icon would give every row an icon column, a gap between the
		// current account's check mark and its picture.
		add := m.Item("Add account").ID("account:add")
		add.Content(func(x ui.MenuItemContent) {
			c := x.Context
			ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				checkSlot(c, false, x.Text)
				ui.Row(c).Size(28, 28).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, kit.IconPlus).Size(16, 16).TextColor(x.Muted)
				})
				ui.Text(c, "Add account").FontSize(kit.SizeBody).TextColor(x.Text)
			})
		})
		if add.Chosen() {
			acc.Add()
		}
	})
}

// checkSlot is the account rows' leading column: a check on the current
// account, the same room left empty on the others.
func checkSlot(c *ui.Context, on bool, color ui.Color) {
	ui.Row(c).Width(12).Shrink(0).Justify(ui.Center).Children(func() {
		if on {
			ui.Icon(c, kit.IconCheck).Size(12, 12).TextColor(color)
		}
	})
}

// accountRow is one account of the submenu: its picture and name, a mark if
// it is the one shown, and a Log out button that takes its own press (a
// press elsewhere on the row switches to it).
func (a *App) accountRow(m *ui.Menu, e AccountEntry) {
	acc := a.accounts
	name := e.User
	if i := strings.Index(strings.TrimPrefix(name, "@"), ":"); i > 0 {
		name = strings.TrimPrefix(name, "@")[:i] // the part before the server, until a name is known
	}
	spec := kit.AvatarSpec{Name: name, ID: e.User, Size: 28, Colors: &a.colors}
	if e.Current {
		me := a.src.Me()
		name = me.Name
		spec = a.avatar(me, 28)
	}
	// The check is drawn in the row, in a narrow slot: the menu's own mark
	// column (Checked) is wider and adds its gap before the picture. The
	// label says which is current, for assistive technology.
	label := name
	if e.Current {
		label = name + ", current account"
	}
	it := m.Item(label).ID("account:" + e.User)
	it.Content(func(x ui.MenuItemContent) {
		c := x.Context
		p := kit.Of(c)
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			checkSlot(c, e.Current, x.Text)
			kit.Avatar(c, spec)
			ui.Column(c).Grow(1).MinWidth(0).Margin(0, kit.S2, 0, 0).Children(func() {
				ui.Text(c, name).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).SingleLine()
				ui.Text(c, e.User).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
			})
			if acc != nil && acc.Remove != nil {
				label := "Log out of " + e.User
				out := ui.ButtonBase(c.Key("logout:"+e.User)).Size(28, 28).Shrink(0).Radius(kit.RadiusSm).
					AlignItems(ui.Center).Justify(ui.Center).Label(label).Tooltip(label)
				if out.Hovered() {
					out.Background(p.DangerSubtl)
				}
				out.Children(func() { ui.Icon(c, kit.IconLogOut).Size(16, 16).TextColor(p.Danger) })
				if out.Clicked() {
					a.removing, a.removeAccount.Open = e.User, true
				}
			}
		})
	})
	if it.Chosen() && !e.Current && acc != nil && acc.Switch != nil {
		acc.Switch(e.User)
	}
}

// removeAccountDialog asks before an account is forgotten on this device.
func (a *App) removeAccountDialog(c *ui.Context) {
	if a.removing == "" || a.accounts == nil || a.accounts.Remove == nil {
		return
	}
	user := a.removing
	if done, yes := kit.Confirm(c, &a.removeAccount, kit.ConfirmSpec{
		Title:   "Log out of " + user + "?",
		Body:    "It is signed out on this device. Its messages stay on the server, and you can sign in again later.",
		Confirm: "Log out", Danger: true,
	}); done {
		if yes {
			a.accounts.Remove(user)
		}
		a.removing = ""
	}
}
