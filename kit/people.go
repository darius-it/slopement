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
}

// PeopleState is the People view's filter and search.
type PeopleState struct {
	Query  string
	Filter int // 0 all, 1 online
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
					if act := personRow(c, ps, colors, now); act.Open != "" || act.Profile != "" {
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
func personRow(c *ui.Context, ps Person, colors *UserColors, now time.Time) (a PeopleAction) {
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
			switch {
			case ps.Status.Set(now):
				StatusLine(c, ps.Status, now, false)
			case ps.Presence == PresenceAway:
				ui.Text(c, "Away").FontSize(SizeSmall).TextColor(p.TextMuted)
			case ps.Presence == PresenceOnline:
				ui.Text(c, "Online").FontSize(SizeSmall).TextColor(p.TextMuted)
			default:
				ui.Text(c, lastSeenText(ps.LastSeen, now)).FontSize(SizeSmall).TextColor(p.TextFaint)
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
