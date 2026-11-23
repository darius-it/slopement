package matrix

import (
	"context"
	"encoding/json"
	"time"

	"maunium.net/go/mautrix/event"
)

// A resumed sync (from the cache's token) sends only the account data that
// changed since: what the user set long ago — the colors chosen for names,
// the order of the rail, mutes, who is ignored, their status — would never
// arrive, and a restart would show none of it. So at the start each is asked
// for, and goes through the handler a synced one takes.
func (s *Source) fetchAccountData(ctx context.Context) {
	for _, d := range []struct {
		typ event.Type
		fn  func(context.Context, *event.Event)
	}{
		{userColorsType, s.onUserColors},
		{railOrderType, s.onRailOrder},
		{mutesType, s.onMutes},
		{statusType, s.onStatusData},
		{personNotesType, s.onPersonNotes},
		{event.AccountDataIgnoredUserList, s.onIgnored},
	} {
		if ctx.Err() != nil {
			return
		}
		c, cancel := context.WithTimeout(ctx, 15*time.Second)
		var raw json.RawMessage
		err := s.cli.GetAccountData(c, d.typ.Type, &raw)
		cancel()
		if err != nil || len(raw) == 0 {
			continue // not set (404), or unreachable: the sync may still bring it
		}
		d.fn(ctx, &event.Event{Type: d.typ, Content: event.Content{VeryRaw: raw}})
	}
}
