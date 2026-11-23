package app

import (
	"reflect"
	"testing"
	"time"

	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// The toolbar offers the emoji reacted with lately, newest first, then the
// usual three to fill the places; taking a reaction back is not a use.
func TestQuickReactionsAreTheRecentOnes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	if got := a.quickReactions(); !reflect.DeepEqual(got, []string{"👍", "❤️", "😂"}) {
		t.Fatalf("by default: %v", got)
	}
	ev := model.Event{ID: "$e", Room: "!crit"}
	a.noteReaction(ev, "🔥")
	a.noteReaction(ev, "🎉")
	if got := a.quickReactions(); !reflect.DeepEqual(got, []string{"🎉", "🔥", "👍"}) {
		t.Fatalf("after two: %v", got)
	}
	a.noteReaction(ev, "👍") // used again: first, and not twice
	a.noteReaction(ev, "🔥")
	a.noteReaction(ev, "🎉")
	a.noteReaction(ev, "🚀")
	if got := a.quickReactions(); !reflect.DeepEqual(got, []string{"🚀", "🎉", "🔥"}) {
		t.Fatalf("after more: %v", got)
	}
	// Taking one back is not a use; a custom emoji is not offered here.
	mine := model.Event{ID: "$e", Reactions: []model.Reaction{{Key: "🧡", Count: 1, Mine: true}}}
	a.noteReaction(mine, "🧡")
	a.noteReaction(ev, "mxc://x/wave")
	if got := a.quickReactions(); got[0] != "🚀" {
		t.Fatalf("a take-back or a custom emoji took a place: %v", got)
	}
	// Kept for the next run.
	b := New(mock.New(time.Now()), picker.New(nil))
	if got := b.quickReactions(); got[0] != "🚀" {
		t.Fatalf("not kept: %v", got)
	}
}
