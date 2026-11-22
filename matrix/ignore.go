package matrix

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// Ignoring someone is the account data m.ignored_user_list: the homeserver
// stops sending their messages and invitations, and every client hides what
// it already holds. This app hides it in Timeline.

// ignoredData is the account data's name.
const ignoredData = "m.ignored_user_list"

// IsIgnored says the user ignores someone. UI thread.
func (s *Source) IsIgnored(mxid string) bool { return s.ignored[mxid] }

// IgnoredUsers lists who is ignored, sorted. UI thread.
func (s *Source) IgnoredUsers() []string {
	out := make([]string, 0, len(s.ignored))
	for u := range s.ignored {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// SetIgnored ignores someone, or stops: it shows at once and is written to
// the account (the whole list: another session's change since the last sync
// is read first, so it is not lost).
func (s *Source) SetIgnored(mxid string, on bool, done func(string)) {
	if s.cli == nil || mxid == "" || mxid == s.me.ID {
		done("You can't ignore this person.")
		return
	}
	had := s.ignored[mxid]
	s.setIgnoredLocal(mxid, on)
	s.changed()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var cur struct {
			Ignored map[string]map[string]any `json:"ignored_users"`
		}
		if err := s.cli.GetAccountData(ctx, ignoredData, &cur); err != nil || cur.Ignored == nil {
			cur.Ignored = map[string]map[string]any{}
		}
		if on {
			cur.Ignored[mxid] = map[string]any{}
		} else {
			delete(cur.Ignored, mxid)
		}
		err := s.cli.SetAccountData(ctx, ignoredData, cur)
		s.post(func() {
			if err != nil {
				s.setIgnoredLocal(mxid, had)
				done("Could not change who you ignore: " + plain(err))
				return
			}
			done("")
		})
	}()
}

func (s *Source) setIgnoredLocal(mxid string, on bool) {
	if s.ignored == nil {
		s.ignored = map[string]bool{}
	}
	if on {
		s.ignored[mxid] = true
	} else {
		delete(s.ignored, mxid)
	}
}

// onIgnored is the list as the account has it.
func (s *Source) onIgnored(ctx context.Context, evt *event.Event) {
	var c struct {
		Ignored map[string]json.RawMessage `json:"ignored_users"`
	}
	if json.Unmarshal(evt.Content.VeryRaw, &c) != nil {
		return
	}
	s.post(func() {
		s.ignored = map[string]bool{}
		for u := range c.Ignored {
			s.ignored[u] = true
		}
	})
}

// withoutIgnored drops what ignored people said.
func (s *Source) withoutIgnored(evs []model.Event) []model.Event {
	if len(s.ignored) == 0 {
		return evs
	}
	out := evs[:0:0]
	for _, e := range evs {
		if !s.ignored[e.Sender] {
			out = append(out, e)
		}
	}
	return out
}

var _ model.Ignorer = (*Source)(nil)
