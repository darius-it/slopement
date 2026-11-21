package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// overlays are the dialogs, sheets and popovers any screen can open.
func (a *App) overlays(c *ui.Context) {
	a.switcherOverlay(c)

	me := a.src.Me()
	switch act := kit.StatusEditor(c, &a.statusDraft, a.avatar(me, 0), me.Name, a.now); {
	case act.Save:
		s := a.statusDraft.Status(a.now)
		a.src.SetStatus(model.Status{Emoji: s.Emoji, Text: s.Text, Until: s.Until})
	case act.Clear:
		a.src.SetStatus(model.Status{})
	}

	if kit.PollDialog(c, &a.pollSheet, &a.pollDraft) {
		d := a.pollDraft
		pl := model.Poll{Question: d.Question, Multiple: d.Multiple, Hidden: d.Hidden}
		if du := kit.PollDurations[d.Duration].For; du > 0 {
			pl.Ends = a.now.Add(du)
		}
		for _, an := range d.Answers {
			if an != "" {
				pl.Answers = append(pl.Answers, model.PollAnswer{Text: an})
			}
		}
		a.src.CreatePoll(a.pollRoom, pl)
		a.pollDraft = kit.NewPollDraft()
	}

	// A person's card opened from People, members, or a phone header.
	if a.profileSheet.Open && a.profileOpen != "" {
		spec := a.profile(a.profileOpen)
		person := a.profileOpen
		body := func() { a.profileActs(person, kit.ProfileCard(c, *spec, &a.colors, a.now)) }
		if kit.Compact(c) {
			kit.Sheet(c, &a.profileSheet, "", body)
		} else {
			kit.Dialog(c, &a.profileSheet.Open, "", 340, body)
		}
	} else if a.profileSheet.Open {
		a.profileSheet.Open = false
	}

	if r, ok := a.src.Room(a.leaving); ok {
		if done, yes := kit.Confirm(c, &a.leave, kit.ConfirmSpec{
			Title:   "Leave " + r.Name + "?",
			Body:    "You can rejoin later if it's public, or if someone invites you. Its history stays on the server.",
			Confirm: "Leave", Danger: true,
		}); done {
			if yes {
				a.src.Leave(r.ID)
				if a.room == r.ID {
					a.room, a.panel = "", panelNone
				}
			}
			a.leaving = ""
		}
	}

	kit.MediaViewer(c, &a.viewer, a.viewerItems, &a.viewerAt)
	a.verifyDialog(c)
	a.phoneSheets(c)
}

// switcherOverlay is the ⌘K switcher over rooms, people and communities.
func (a *App) switcherOverlay(c *ui.Context) {
	if !a.switcher.Open {
		return
	}
	recent := map[string]int{}
	for i, id := range a.lastRooms {
		recent[id] = i
	}
	rank := func(id string) int {
		if i, ok := recent[id]; ok {
			return i
		}
		return 100
	}
	var items []kit.SwitcherItem
	for _, r := range a.src.Rooms() {
		it := kit.SwitcherItem{ID: r.ID, Name: r.Name, Unread: r.Unread, Mention: r.Mentions, Recent: rank(r.ID),
			Avatar: kit.AvatarSpec{Name: r.Name, ID: r.ID, Colors: &a.colors}}
		switch r.Kind {
		case model.KindDM:
			it.Kind, it.Sub, it.Alias = kit.KindPerson, r.Partner, r.Partner
		default:
			it.Kind = kit.KindRoom
			if cm, ok := a.communityOf(r.Community); ok {
				it.Sub = cm.Name
			}
			it.Alias = "#" + r.Name
		}
		items = append(items, it)
	}
	for _, cm := range a.src.Communities() {
		items = append(items, kit.SwitcherItem{ID: cm.ID, Name: cm.Name, Kind: kit.KindSpace, Recent: 200,
			Avatar: kit.AvatarSpec{Name: cm.Name, ID: cm.ID, Square: true}})
	}
	chosen, searchMsgs := kit.Switcher(c, &a.switcher, items, &a.colors)
	switch {
	case chosen != nil && chosen.Kind == kit.KindSpace:
		a.community, a.tab, a.view, a.phoneTab = chosen.ID, 1, viewRoom, 1
	case chosen != nil:
		a.open(chosen.ID)
		if r, ok := a.src.Room(chosen.ID); ok {
			a.community = r.Community
			if r.Community != "" {
				a.tab = 1
			} else {
				a.tab = 0
			}
		}
	case searchMsgs:
		a.toast = "Message search comes with the Matrix layer"
	}
}

var demoSAS = []kit.SASEmoji{
	{Emoji: "🐶", Name: "Dog"}, {Emoji: "🔑", Name: "Key"}, {Emoji: "🌵", Name: "Cactus"}, {Emoji: "🎸", Name: "Guitar"},
	{Emoji: "🚀", Name: "Rocket"}, {Emoji: "🍄", Name: "Mushroom"}, {Emoji: "⚓", Name: "Anchor"},
}

// verifyDialog walks through verifying this session (the flow, on fake
// emoji: the crypto comes with the Matrix layer).
func (a *App) verifyDialog(c *ui.Context) {
	if !a.verify.Open {
		return
	}
	body := func() {
		switch a.verifyStep {
		case 0:
			switch kit.VerifyStart(c) {
			case kit.VerifyWithDevice:
				a.verifyStep = 1
			case kit.VerifyWithKey:
				a.toast = "Security Key entry comes with the Matrix layer"
				a.verify.Open = false
			case kit.VerifyReset:
				a.toast = "Resetting your identity comes with the Matrix layer"
				a.verify.Open = false
			}
		case 1:
			switch kit.VerifyEmoji(c, "your other device", demoSAS) {
			case kit.VerifyMatch:
				a.verifyStep = 2
			case kit.VerifyMismatch:
				a.toast = "Verification cancelled: the emoji didn't match"
				a.verify.Open = false
			}
		default:
			p := kit.Of(c)
			ui.Column(c).AlignItems(ui.Center).Gap(kit.S3).Padding(kit.S4).Children(func() {
				ui.Icon(c, kit.IconShield).Size(40, 40).TextColor(p.Success)
				ui.Text(c, "This session is verified").FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold)
				if kit.Button(c, kit.Primary, "Done", nil).Clicked() {
					a.verify.Open = false
				}
			})
		}
	}
	if kit.Compact(c) {
		kit.Sheet(c, &a.verify, "Verify this session", body)
	} else {
		kit.Dialog(c, &a.verify.Open, "Verify this session", 460, body)
	}
}

// phoneSheets are the phone's sheets: a room's actions and its info.
func (a *App) phoneSheets(c *ui.Context) {
	if r, ok := a.src.Room(a.roomSheetID); ok {
		kit.Sheet(c, &a.roomSheet, r.Name, func() {
			st := a.menuState(r)
			row := func(icon *ui.SVG, title string, act kit.RoomAction) {
				if kit.ChoiceRow(c, icon, title, "").Clicked() {
					a.roomAction(r.ID, act)
					a.roomSheet.Open = false
				}
			}
			fav := "Favourite"
			if st.Favourite {
				fav = "Remove from favourites"
			}
			row(kit.IconStar, fav, kit.RoomAction{ToggleFav: true})
			if st.Muted {
				row(kit.IconBell, "Unmute", kit.RoomAction{Unmute: true})
			} else if d := kit.MuteList(c, a.now); d != nil {
				a.roomAction(r.ID, kit.RoomAction{Mute: d})
				a.roomSheet.Open = false
			}
			row(kit.IconLink, "Copy link", kit.RoomAction{CopyLink: true})
			row(kit.IconLogOut, "Leave", kit.RoomAction{Leave: true})
		})
	}
	if r, ok := a.src.Room(a.room); ok && kit.Compact(c) {
		kit.Sheet(c, &a.roomInfo, "", func() { a.roomInfoBody(c, r) })
	}
}
