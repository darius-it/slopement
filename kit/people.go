package kit

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Person is someone the user has a DM with: a contact.
type Person struct {
	ID       string // MXID
	Name     string
	Avatar   AvatarSpec
	Presence Presence
	Status   Status
	// LastSeen is when they were last active, for those offline.
	LastSeen time.Time
	// LastChat is when the user and they last wrote to each other in their
	// DM; zero when they have not.
	LastChat time.Time
}

// What the People view says under each name.
const (
	PeopleSubStatus   = iota // their status, or how they are (Online, Away, last seen)
	PeopleSubLastChat        // when the user and they last chatted
)

// PeopleState is the People view's filter and search.
type PeopleState struct {
	Query  string
	Filter int // 0 all, 1 online
	// Sub is what shows under each name: PeopleSubStatus or
	// PeopleSubLastChat. Set by the app from its settings.
	Sub int
}

// personSub is the line under someone's name in the People view, and
// whether it is a quiet (faint) one. With Status it is the words of their
// status, and when they have none how they are: Online, Away or when they
// were last seen. With LastChat it is when they last chatted.
func personSub(ps Person, mode int, now time.Time) (text string, faint bool) {
	if mode == PeopleSubLastChat {
		return lastChatText(ps.LastChat, now), ps.LastChat.IsZero()
	}
	if ps.Status.Set(now) && ps.Status.Text != "" {
		return ps.Status.Text, false
	}
	switch ps.Presence {
	case PresenceAway:
		return "Away", false
	case PresenceNone:
		return lastSeenText(ps.LastSeen, now), true
	}
	return "Online", false
}

// lastChatText says when a chat with someone last happened.
func lastChatText(t, now time.Time) string {
	if t.IsZero() {
		return "No chat yet"
	}
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, x.Location()) }
	switch days := int(day(now).Sub(day(t)).Hours() / 24); {
	case days <= 0:
		return "Chatted today, " + t.Format("15:04")
	case days == 1:
		return "Chatted yesterday"
	case days < 7:
		return "Chatted " + t.Format("Monday")
	}
	return "Chatted " + t.Format("2 Jan")
}

// PeopleAction is what the user did in the People view.
type PeopleAction struct {
	Open    string // the person whose DM to open
	Profile string // the person whose card to show
}

// lastSeenText says when someone offline was last active.
func lastSeenText(t, now time.Time) string {
	if t.IsZero() {
		return "Offline"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "Last seen just now"
	case d < time.Hour:
		return "Last seen " + strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return "Last seen " + strconv.Itoa(int(d.Hours())) + "h ago"
	}
	return "Last seen " + t.Format("2 Jan")
}

// sortPeople puts those with a status first, then by name.
func sortPeople(list []Person, now time.Time) {
	sort.SliceStable(list, func(i, j int) bool {
		si, sj := list[i].Status.Set(now), list[j].Status.Set(now)
		if si != sj {
			return si
		}
		if list[i].Presence != list[j].Presence {
			return list[i].Presence == PresenceOnline
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
}

// People shows the user's contacts and their statuses at a glance (after
// Discord's Friends, Riot's Social and Campsite's people panel on Mobbin):
// a search, All / Online, then "Online" and "Offline" sections, each person
// with their presence on their avatar and, under their name, their status
// (or Away, or when they were last seen). Those with a status lead.
// splitPeople filters people by the search and splits them into online
// and offline, each sorted.
func splitPeople(st *PeopleState, people []Person, now time.Time) (online, offline []Person, q string) {
	q = strings.ToLower(strings.TrimSpace(st.Query))
	for _, ps := range people {
		if q != "" && !strings.Contains(strings.ToLower(ps.Name+" "+ps.ID+" "+ps.Status.Text), q) {
			continue
		}
		if ps.Presence == PresenceNone {
			offline = append(offline, ps)
		} else {
			online = append(online, ps)
		}
	}
	sortPeople(online, now)
	sortPeople(offline, now)
	return
}

// peopleHeader is the search and the All / Online filter.
func peopleHeader(c *ui.Context, st *PeopleState) {
	ui.Row(c).Gap(S2).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Column(c).Grow(1).MinWidth(200).Children(func() {
			SearchField(c.Key("people-search"), &st.Query, "Search people", "")
		})
		ui.Row(c).Gap(S1).Children(func() {
			for i, l := range []string{"All", "Online"} {
				if Chip(c.Key("pf"+l), l, st.Filter == i).Clicked() {
					st.Filter = i
				}
			}
		})
	})
}

func People(c *ui.Context, st *PeopleState, people []Person, colors *UserColors, now time.Time) (a PeopleAction) {
	p := Of(c)
	online, offline, q := splitPeople(st, people, now)
	ui.Column(c).Gap(S3).Children(func() {
		peopleHeader(c, st)
		section := func(title string, list []Person) {
			if len(list) == 0 {
				return
			}
			ui.Text(c.Key("sec"+title), title+" — "+strconv.Itoa(len(list))).FontSize(SizeCaption).
				FontWeight(WeightSemibold).TextColor(p.TextMuted).Padding(S2, S2, 0, S2)
			ui.Column(c.Key("list" + title)).Gap(1).Children(func() {
				for _, ps := range list {
					if act := personRow(c, ps, st.Sub, colors, now); act.Open != "" || act.Profile != "" {
						a = act
					}
				}
			})
		}
		section("Online", online)
		if st.Filter == 0 {
			section("Offline", offline)
		}
		if len(online) == 0 && (st.Filter == 1 || len(offline) == 0) {
			emptyNote(c, map[bool]string{true: "No one matches “" + st.Query + "”", false: "No one is online right now"}[q != ""])
		}
	})
	return a
}

// personRow is a contact: avatar with presence, name, and their status in
// one line under it; message and more on hover.
func personRow(c *ui.Context, ps Person, mode int, colors *UserColors, now time.Time) (a PeopleAction) {
	p := Of(c)
	b := ui.ButtonBase(c.Key(ps.ID)).MinHeight(56).PaddingX(S2).Gap(S3).Radius(RadiusMd - 2).AlignItems(ui.Center).
		Justify(ui.Start).Label(ps.Name)
	hov := b.Hovered() || b.FocusWithin()
	bg := ui.Transparent
	if b.Pressed() {
		bg = p.Pressed
	} else if hov {
		bg = p.Hover
	}
	b.Background(b.AnimateColor("bg", bg, Color))
	b.Children(func() {
		av := ps.Avatar
		av.Size, av.Name, av.ID, av.Colors, av.Presence = 40, ps.Name, ps.ID, colors, ps.Presence
		Avatar(c, av)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, ps.Name).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(p.Text).SingleLine()
			if mode == PeopleSubStatus && ps.Status.Set(now) {
				StatusLine(c, ps.Status, now, false) // emoji and words, as people wrote them
			} else {
				text, faint := personSub(ps, mode, now)
				col := p.TextMuted
				if faint {
					col = p.TextFaint
				}
				ui.Text(c, text).FontSize(SizeSmall).TextColor(col).SingleLine()
			}
		})
		if hov || Touch(c) {
			if IconButton(c, Ghost, IconMessage, "Message "+ps.Name).Clicked() {
				a.Open = ps.ID
			}
		}
	})
	if b.Clicked() && a.Open == "" {
		a.Profile = ps.ID
	}
	return a
}
