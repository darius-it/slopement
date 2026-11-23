package app

import (
	"strconv"
	"strings"

	"slopement/model"
)

// A run of people joining (or leaving) one after another is one quiet line, as
// Element makes it: "Mira, Sam and 3 others joined" — a room of hundreds, or
// a bridge bringing everyone in, would otherwise fill the timeline.

// collapseMembership returns the events with each run of two or more joins, or
// of leaves, in a row replaced by one line. Bans, invitations and everything
// else stay as they are, and break a run.
func (a *App) collapseMembership(evs []model.Event) []model.Event {
	out := make([]model.Event, 0, len(evs))
	me := a.src.Me().ID
	for i := 0; i < len(evs); i++ {
		e := evs[i]
		if e.Kind != model.EventSystem || (e.Membership != "join" && e.Membership != "leave") {
			out = append(out, e)
			continue
		}
		j := i + 1
		for j < len(evs) && evs[j].Kind == model.EventSystem && evs[j].Membership == e.Membership {
			j++
		}
		if j-i < 2 {
			out = append(out, e)
			continue
		}
		var names []string
		seen := map[string]bool{}
		for _, m := range evs[i:j] {
			if seen[m.Sender] {
				continue
			}
			seen[m.Sender] = true
			if m.Sender == me {
				names = append(names, "You")
				continue
			}
			p, _ := a.person(m.Sender)
			names = append(names, p.Name)
		}
		verb := "joined"
		if e.Membership == "leave" {
			verb = "left"
		}
		line := e
		line.ID = "members:" + e.ID
		line.Body = joinNames(names) + " " + verb
		out = append(out, line)
		i = j - 1
	}
	return out
}

// joinNames is "A", "A and B", "A, B and C", and from four "A, B and 2 others".
func joinNames(names []string) string {
	switch n := len(names); {
	case n == 0:
		return ""
	case n == 1:
		return names[0]
	case n <= 3:
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	default:
		return names[0] + ", " + names[1] + " and " + strconv.Itoa(n-2) + " others"
	}
}
