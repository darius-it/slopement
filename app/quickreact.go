package app

import (
	"slices"

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
