package app

import (
	"slices"
	"strconv"
	"time"

	"slopement/kit"
	"slopement/model"
)

// quickSlots is how many emoji the hover toolbar offers to react with at once.
const quickSlots = 3

// quickReactions are the emoji the hover toolbar offers: the ones the user
// reacted with lately, newest first, then the usual ones to fill the places.
func (a *App) quickReactions() []string {
	var out []string
	if a.pick != nil {
		for _, k := range a.pick.RecentReactions() {
			if len(out) < quickSlots && !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	for _, k := range kit.QuickReactions {
		if len(out) < quickSlots && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// callLineText is the words of a call's line: what happened, and for an ended
// call how long it lasted ("The call ended · 12 min").
func callLineText(ev model.Event) string {
	if ev.Call == nil || !ev.Call.Ended || ev.Call.Duration <= 0 {
		return ev.Body
	}
	return ev.Body + " · " + callLength(ev.Call.Duration)
}

// callLength is a call's length in words: under a minute, "7 min", "1 h 5 min".
func callLength(d time.Duration) string {
	mins := int(d.Round(time.Minute) / time.Minute)
	switch {
	case d < time.Minute:
		return "under a minute"
	case mins < 60:
		return strconv.Itoa(mins) + " min"
	case mins%60 == 0:
		return strconv.Itoa(mins/60) + " h"
	}
	return strconv.Itoa(mins/60) + " h " + strconv.Itoa(mins%60) + " min"
}

// noteReaction remembers an emoji the user reacted with, for the toolbar. A
// reaction pressed to take back one already made is not a use.
func (a *App) noteReaction(ev model.Event, key string) {
	if a.pick == nil {
		return
	}
	for _, r := range ev.Reactions {
		if r.Key == key && r.Mine {
			return
		}
	}
	a.pick.UsedReaction(key)
}
