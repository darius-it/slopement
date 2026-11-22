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

	a.removeAccountDialog(c)
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

// verifyDialog walks through verifying this session, on the source's
// Verify (the crypto layer's real flow; the mock's fake walks the same
// screens). An incoming request opens it by itself.
func (a *App) verifyDialog(c *ui.Context) {
	v := a.verifier
	if v == nil {
		a.verify.Open = false
		return
	}
	st := v.State()
	// Unverified with messages that cannot be read: the prompt shows once
	// a run (declining it leaves the banner in the room and the settings
	// entry), never nagging.
	if st.Needed && !a.verifyAsked {
		a.verifyAsked = true
		a.verify.Open = true
	}
	if st.Phase == model.VerifyAsked && !a.verify.Open {
		a.verify.Open = true
	}
	if !a.verify.Open {
		return
	}
	err := func() { // one quiet line, on whichever screen says why
		if st.Err != "" {
			ui.Text(c, st.Err).FontSize(kit.SizeSmall).TextColor(kit.Of(c).Danger).TextAlign(ui.Center)
		}
	}
	body := func() {
		// The key entry is a step of its own until submitted.
		if a.verifyStep == 1 {
			if kit.VerifyKey(c, &a.verifyKey, st.Phase == model.VerifyWorking) {
				a.verifyStep = 0
				v.UseKey(a.verifyKey)
				a.verifyKey = ""
			}
			err()
			return
		}
		switch st.Phase {
		case model.VerifyAsked:
			accept, decline := kit.VerifyAsk(c, st.From)
			switch {
			case accept:
				v.Accept()
			case decline:
				v.Decline()
			}
		case model.VerifyWaiting:
			if kit.VerifyWait(c) {
				v.Decline()
			}
		case model.VerifyEmoji:
			switch kit.VerifyEmoji(c, st.From, sasOf(st.Emoji)) {
			case kit.VerifyMatch:
				v.Confirm(true)
			case kit.VerifyMismatch:
				v.Confirm(false)
			}
		case model.VerifyWorking:
			p := kit.Of(c)
			ui.Column(c).AlignItems(ui.Center).Gap(kit.S3).Padding(kit.S4).Children(func() {
				ui.Icon(c, kit.IconShield).Size(40, 40).TextColor(p.Accent)
				ui.Text(c, "Working…").FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold)
				if st.From != "" {
					ui.Text(c, "With "+st.From).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
				}
			})
		case model.VerifyDone:
			p := kit.Of(c)
			ui.Column(c).AlignItems(ui.Center).Gap(kit.S3).Padding(kit.S4).Children(func() {
				ui.Icon(c, kit.IconShield).Size(40, 40).TextColor(p.Success)
				ui.Text(c, "This session is verified").FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold)
				if st.Err != "" { // the backup note: verified, history pending
					ui.Text(c, st.Err).FontSize(kit.SizeSmall).TextColor(p.TextMuted).TextAlign(ui.Center)
				} else {
					ui.Text(c, "Your encrypted messages will appear as the keys arrive.").
						FontSize(kit.SizeSmall).TextColor(p.TextMuted).TextAlign(ui.Center)
				}
				if kit.Button(c, kit.Primary, "Done", nil).Clicked() {
					a.verify.Open = false
				}
			})
		default: // idle: how to verify
			switch kit.VerifyStart(c) {
			case kit.VerifyWithDevice:
				v.BeginWithDevice()
			case kit.VerifyWithKey:
				a.verifyStep = 1
			case kit.VerifyReset:
				a.toast = "Resetting your identity comes with the Matrix layer"
				a.verify.Open = false
			}
			err()
		}
	}
	if kit.Compact(c) {
		kit.Sheet(c, &a.verify, "Verify this session", body)
	} else {
		kit.Dialog(c, &a.verify.Open, "Verify this session", 460, body)
	}
}

// sasOf maps the model's emoji to the kit's.
func sasOf(emoji []model.SASEmoji) []kit.SASEmoji {
	out := make([]kit.SASEmoji, len(emoji))
	for i, e := range emoji {
		out[i] = kit.SASEmoji{Emoji: e.Emoji, Name: e.Name}
	}
	return out
}

// phoneSheets are the phone's sheets: a room's actions and its info.
func (a *App) phoneSheets(c *ui.Context) {
	if a.msgID != "" {
		a.messageSheet(c)
	}
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
