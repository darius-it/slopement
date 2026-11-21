package gallery

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/embeds"
	"slopement/format"
	"slopement/kit"
)

// social is the gallery's state for polls, statuses and profiles.
type social struct {
	railAdd     kit.RailAddAction
	myStatus    kit.Status
	statusDraft kit.StatusDraft
	pollSheet   kit.SheetState
	pollDraft   kit.PollDraft
	// demoPoll is Mira's poll in the timelines; sentPolls the user's.
	demoPoll  kit.PollData
	sentPolls []*kit.PollData
	// profileOpen shows the DM partner's profile card from the header.
	profileOpen bool
	people      kit.PeopleState
	// phoneTab is the phone mock's tab: 0 chats, 1 communities, 2 people.
	phoneTab int
	// navLayout is the Settings → Appearance → Layout choice.
	navLayout kit.NavLayout
	// favChats are the starred chats; profileMsg the message whose
	// sender's card is open; profileCard the person whose card People
	// opened.
	favChats     map[string]bool
	profileMsg   string
	profileCard  string
	profileSheet kit.SheetState
}

// contacts are the user's people in the mocks: a mix of statuses,
// presence and last-seen times.
func (a *App) contacts() []kit.Person {
	now := a.now
	return []kit.Person{
		{ID: "@alex:example.com", Name: "Alex Rivera", Presence: kit.PresenceOnline, Status: alexStatus(now)},
		{ID: "@mira:matrix.org", Name: "Mira", Presence: kit.PresenceOnline,
			Status: kit.Status{Emoji: "🌴", Text: "In Lisbon, slow replies", Until: kit.ClearTime(4, now)}},
		{ID: "@sam:matrix.org", Name: "Sam", Presence: kit.PresenceOnline,
			Status: kit.Status{Emoji: "🍔", Text: "Lunch", Until: now.Add(25 * time.Minute)}},
		{ID: "@jun:kde.org", Name: "Jun", Presence: kit.PresenceAway},
		{ID: "@kai:example.org", Name: "Kai", Status: kit.Status{Emoji: "✈️", Text: "Travelling this month"},
			LastSeen: now.Add(-26 * time.Hour)},
		{ID: "@noor:example.com", Name: "Noor الحسيني", LastSeen: now.Add(-2 * time.Hour)},
		{ID: "@ola:matrix.org", Name: "Ola", LastSeen: now.Add(-12 * time.Minute)},
	}
}

// peopleAct applies the People view's action.
func (a *App) peopleAct(act kit.PeopleAction) {
	switch {
	case act.Open != "":
		a.openChatWith(act.Open)
	case act.Profile != "":
		a.profileCard = act.Profile
		a.profileSheet.Open = true
	}
}

// openChatWith opens the DM with mxid.
func (a *App) openChatWith(mxid string) {
	cs := a.concepts()
	for _, ch := range a.conceptChats() {
		if ch.mxid == mxid {
			cs.chatOpen, a.room, a.phoneTab = ch.id, "", 0
			return
		}
	}
	a.did = "Would start a chat with " + mxid
}

// person returns the contact of mxid.
func (a *App) person(mxid string) (kit.Person, bool) {
	for _, ps := range a.contacts() {
		if ps.ID == mxid {
			return ps, true
		}
	}
	return kit.Person{}, false
}

// profileOf is the card of mxid.
func (a *App) profileOf(mxid, name string) *kit.ProfileSpec {
	spec := &kit.ProfileSpec{Name: name, MXID: mxid}
	if ps, ok := a.person(mxid); ok {
		spec.Name, spec.Status = ps.Name, ps.Status
		spec.Avatar.Presence = ps.Presence
	}
	switch mxid {
	case "@alex:example.com":
		spec.Pronouns, spec.Trust = "they/them", kit.TrustVerified
	case "@jun:kde.org":
		spec.Trust = kit.TrustWarning
	}
	return spec
}

// profileDialog shows the card chosen in People: a dialog on desktops, a
// sheet on phones.
func (a *App) profileDialog(c *ui.Context) {
	if a.profileCard == "" {
		return
	}
	spec := a.profileOf(a.profileCard, a.profileCard)
	body := func() {
		if act := kit.ProfileCard(c, *spec, &a.colors, a.now); act.Message {
			a.profileSheet.Open = false
			a.openChatWith(spec.MXID)
		}
	}
	if kit.Compact(c) {
		kit.Sheet(c, &a.profileSheet, "", body)
	} else {
		kit.Dialog(c, &a.profileSheet.Open, "", 340, body)
	}
}

// asMedia shows a message that is only a link to a GIF (or an image or
// video) as that media, without the link: GIFs animate (loaded by the
// picker's loader, which keeps a bounded number).
func (a *App) asMedia(doc **format.Node, media *[]kit.MediaSpec, id string) {
	e, ok := embeds.SoleMedia(*doc)
	if !ok {
		return
	}
	m := kit.MediaSpec{ID: id + ":media", Kind: kit.MediaImage, URL: e.MediaURL, W: 4, H: 3, Progress: -1, Label: "GIF"}
	switch {
	case embeds.IsGIF(e.MediaURL):
		m.Kind, m.URL, m.W, m.H = kit.MediaGIF, "", 320, 240
		m.Player = a.pick.Animation(e.MediaURL)
	case e.Kind == embeds.Video || strings.HasSuffix(e.MediaURL, ".mp4"):
		m.Kind = kit.MediaVideo
	}
	*doc, *media = nil, append([]kit.MediaSpec{m}, *media...)
}

// profileAct applies a message's profile action: open or close its card.
func (a *App) profileAct(id string, act kit.MessageAction) {
	switch {
	case act.Profile:
		if a.profileMsg == id {
			a.profileMsg = ""
		} else {
			a.profileMsg = id
		}
	case act.ProfileClosed:
		a.profileMsg = ""
		if act.ProfileAct.Message {
			a.did = "Would open the DM"
		}
	}
}

// peopleScreen is the People view in the Desktop mock's main area: the
// grid, statuses as thought bubbles.
func (a *App) peopleScreen(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		ui.Row(c).Height(52).PaddingX(kit.S4).Gap(kit.S2).AlignItems(ui.Center).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
			ui.Icon(c, kit.IconUsers).Size(18, 18).TextColor(p.TextMuted)
			ui.Text(c, "People").FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).Grow(1)
			kit.Button(c, kit.Secondary, "Add person", kit.IconPlus)
		})
		ui.Scroll(c).Grow(1).Padding(kit.S4).Children(func() {
			a.peopleAct(kit.PeopleGrid(c, &a.people, a.contacts(), &a.colors, a.now))
		})
	})
}

func newSocial(now time.Time) social {
	return social{
		navLayout: kit.DefaultNavLayout(), // the rail on desktops, tabs on phones
		favChats:  map[string]bool{"c-alex": true, "c-fam": true},
		pollDraft: kit.NewPollDraft(),
		demoPoll: kit.PollData{ID: "poll1", Question: "Where should the offsite be?", Ends: now.Add(3*time.Hour + 20*time.Minute),
			Answers: []kit.PollAnswer{
				{Text: "Lisbon", Votes: 5, Voters: voters("mira", "kai", "noor", "sam", "ola")},
				{Text: "Berlin", Votes: 3, Voters: voters("jun", "alex")}, // one not loaded yet: "and 1 more"
				{Text: "Somewhere with a lake", Votes: 4, Voters: voters("fam", "lena", "theo", "ola")},
			}},
	}
}

// voterNames are the demo polls' voters.
var voterNames = map[string]kit.PollVoter{
	"mira": {ID: "@mira:matrix.org", Name: "Mira"}, "alex": {ID: "@alex:example.com", Name: "Alex Rivera"},
	"sam": {ID: "@sam:matrix.org", Name: "Sam"}, "jun": {ID: "@jun:kde.org", Name: "Jun"},
	"kai": {ID: "@kai:example.org", Name: "Kai"}, "noor": {ID: "@noor:example.com", Name: "Noor الحسيني"},
	"ola": {ID: "@ola:matrix.org", Name: "Ola"}, "fam": {ID: "@barbara:example.com", Name: "Barbara Klein"},
	"lena": {ID: "@lena:matrix.org", Name: "Lena"}, "theo": {ID: "@theo:example.org", Name: "Theo"},
}

// voters lists the demo voters by their short names.
func voters(names ...string) []kit.PollVoter {
	list := make([]kit.PollVoter, 0, len(names))
	for _, n := range names {
		list = append(list, voterNames[n])
	}
	return list
}

// alexStatus is the DM partner's status in the mocks.
func alexStatus(now time.Time) kit.Status {
	end := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
	if !end.After(now) {
		end = now.Add(2 * time.Hour)
	}
	return kit.Status{Emoji: "🎧", Text: "Focusing on the kit", Until: end}
}

func (a *App) myStatusTip() string {
	if a.myStatus.Set(a.now) {
		return a.myStatus.Emoji + " " + a.myStatus.Text + " · click to change"
	}
	return "Set a status"
}

// vote toggles answer i of pd for the user: one choice replaces the
// other, several add up.
func vote(pd *kit.PollData, i int) {
	if i < 0 || i >= len(pd.Answers) || pd.Ended {
		return
	}
	an := &pd.Answers[i]
	if an.Mine {
		an.Mine, an.Votes = false, an.Votes-1
		return
	}
	if !pd.Multiple {
		for j := range pd.Answers {
			if pd.Answers[j].Mine {
				pd.Answers[j].Mine, pd.Answers[j].Votes = false, pd.Answers[j].Votes-1
			}
		}
	}
	an.Mine, an.Votes = true, an.Votes+1
}

// pollAct applies a poll card's action.
func (a *App) pollAct(pd *kit.PollData, act kit.MessageAction) {
	if act.Vote >= 0 {
		vote(pd, act.Vote)
	}
	if act.EndPoll {
		pd.Ended = true
	}
}

// globalOverlays are the dialogs any page can open: the status editor,
// the new poll, a person's card.
func (a *App) globalOverlays(c *ui.Context) {
	a.profileDialog(c)
	switch act := kit.StatusEditor(c, &a.statusDraft, kit.AvatarSpec{ID: "@me:matrix.org", Colors: &a.colors}, "You", a.now); {
	case act.Save:
		a.myStatus = a.statusDraft.Status(a.now)
		a.did = "Status: " + a.myStatus.Text
	case act.Clear:
		a.myStatus = kit.Status{}
		a.did = "Status cleared"
	}
	if kit.PollDialog(c, &a.pollSheet, &a.pollDraft) {
		d := a.pollDraft
		pd := &kit.PollData{ID: "mypoll" + strconv.Itoa(len(a.sentPolls)), Question: d.Question, Multiple: d.Multiple,
			Hidden: d.Hidden, Mine: true}
		if du := kit.PollDurations[d.Duration].For; du > 0 {
			pd.Ends = a.now.Add(du)
		}
		for _, an := range d.Answers {
			if an != "" {
				pd.Answers = append(pd.Answers, kit.PollAnswer{Text: an})
			}
		}
		a.sentPolls = append(a.sentPolls, pd)
		a.pollDraft = kit.NewPollDraft()
		a.did = "Poll created"
	}
}

func (a *App) pollsPage(c *ui.Context) {
	a.specimenFit(c, "New poll", "From the composer's + (Create poll). Two to twenty answers, Enter in the last adds one; one or several choices; results as votes come in or only at the end; how long it runs (the creator's client ends it). After WhatsApp, Discord and LinkedIn on Mobbin.", func() {
		ui.Column(c).Width(400).Children(func() { kit.PollEditor(c, &a.pollDraft) })
	})
	open := a.demoPoll
	voted := a.demoPoll
	voted.ID = "voted"
	voted.Answers = append([]kit.PollAnswer(nil), a.demoPoll.Answers...)
	if !voted.Answers[0].Mine {
		vote(&voted, 0)
	}
	multi := kit.PollData{ID: "multi", Question: "Which days work for you?", Multiple: true, Mine: true,
		Answers: []kit.PollAnswer{{Text: "Monday", Votes: 2, Voters: voters("sam", "jun")},
			{Text: "Wednesday", Votes: 6, Mine: true, Voters: voters("mira", "alex", "kai", "lena", "theo")},
			{Text: "Friday", Votes: 5, Mine: true, Voters: voters("mira", "noor", "ola", "fam")}}}
	hidden := kit.PollData{ID: "hidden", Question: "Best name for the release?", Hidden: true, Ends: a.now.Add(26 * time.Hour),
		Answers: []kit.PollAnswer{{Text: "Lavender", Votes: 4, Mine: true, Voters: voters("mira", "kai", "ola")},
			{Text: "Quiet Hours", Votes: 7, Voters: voters("alex", "sam", "jun", "noor", "lena", "theo", "fam")}}}
	ended := kit.PollData{ID: "ended", Question: "Tabs or spaces?", Ended: true,
		Answers: []kit.PollAnswer{{Text: "Tabs", Votes: 12, Voters: voters("alex", "sam", "jun", "kai", "lena")},
			{Text: "Spaces", Votes: 9, Mine: true, Voters: voters("mira", "noor", "ola", "theo")}}}
	for _, pd := range []*kit.PollData{&open, &voted, &multi, &hidden, &ended} {
		pd.Colors = &a.colors
	}
	a.specimen(c, "In the timeline", "Before voting the answers are choices; after, a bar of each one's share fills in behind it, the leader's in the accent. Hidden polls keep the bars until they end. Once results show, \"N votes\" is a link (brighter and underlined on hover) to who voted for what; while a hidden poll runs, it stays plain text.", false, func() {
		ui.Row(c).Gap(kit.S4).Wrap().AlignItems(ui.Start).Children(func() {
			if act := kit.PollCard(c, open, a.now, 340); act.Vote >= 0 {
				vote(&a.demoPoll, act.Vote)
			}
			kit.PollCard(c, voted, a.now, 340)
			kit.PollCard(c, multi, a.now, 340)
			kit.PollCard(c, hidden, a.now, 340)
			kit.PollCard(c, ended, a.now, 340)
		})
	})
}

func (a *App) statusPage(c *ui.Context) {
	p := kit.Of(c)
	a.specimen(c, "Your status", "Click your avatar at the foot of the rail (Desktop) or here. Your own emoji and words; it clears itself after the time chosen, or never. After Discord on Mobbin.", true, func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			if kit.Button(c, kit.Primary, "Set a status", nil).Clicked() {
				a.statusDraft.Edit(a.myStatus)
			}
			if a.myStatus.Set(a.now) {
				kit.StatusLine(c, a.myStatus, a.now, true)
			} else {
				ui.Text(c, "No status").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			}
		})
	})
	st := alexStatus(a.now)
	a.specimen(c, "Where it shows", "A small thought bubble on the avatar in the chat list (hover the bubble for the whole status), under the name in the chat's header, and in the profile card. Others never see when it clears.", false, func() {
		pp := kit.Of(c)
		r := rooms[1]
		r.Status = st
		ui.Column(c).Width(300).Radius(kit.RadiusMd).Background(pp.BgSubtle).Padding(kit.S2).Children(func() {
			kit.RoomRow(c, r, false, nil)
		})
		ui.Row(c).Gap(kit.S2).Padding(kit.S2, kit.S3).Radius(kit.RadiusMd).Border(1, pp.Border).AlignItems(ui.Center).Width(300).Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: "Alex Rivera", ID: "@alex:example.com", Size: 28, Colors: &a.colors})
			ui.Column(c).MinWidth(0).Children(func() {
				ui.Text(c, "Alex Rivera").FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
				kit.StatusLine(c, st, a.now, false)
			})
		})
	})
	a.specimen(c, "In chats", "Every chat with someone who set a status shows it as a small thought bubble on their avatar; hovering the bubble opens it to the whole status, over the row. So statuses show in Chats in every layout; People is an extra.", false, func() {
		ui.Column(c).Width(320).Radius(kit.RadiusMd).Background(kit.Of(c).BgSubtle).Padding(kit.S2).Children(func() {
			for _, ch := range a.conceptChats()[:3] {
				a.chatRow(c, ch, false)
			}
		})
	})
	a.specimen(c, "People on desktop: thought bubbles", "Everyone as a tile; a status floats over the avatar as a comic thought bubble, trailed by three dots: the emoji at rest, the whole status over the tile. Click a person for their card.", true, func() {
		a.peopleAct(kit.PeopleGrid(c, &a.people, a.contacts(), &a.colors, a.now))
	})
	a.specimen(c, "People on phones", "The list: the status under each name.", false, func() {
		ui.Column(c).MaxWidth(420).Children(func() {
			a.peopleAct(kit.People(c, &a.people, a.contacts(), &a.colors, a.now))
		})
	})
	a.specimenFit(c, "Profile card", "Opens from a user's avatar or name. Their status sits in a soft bubble; an unverified user gets a nudge to verify.", func() {
		ui.Row(c).Gap(kit.S6).Wrap().Children(func() {
			kit.ProfileCard(c, kit.ProfileSpec{Name: "Alex Rivera", MXID: "@alex:example.com", Pronouns: "they/them", Status: st,
				Trust: kit.TrustVerified, Avatar: kit.AvatarSpec{Presence: kit.PresenceOnline}}, &a.colors, a.now)
			kit.ProfileCard(c, kit.ProfileSpec{Name: "Jun", MXID: "@jun:kde.org", Trust: kit.TrustWarning}, &a.colors, a.now)
		})
	})
}
