package matrix

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// ---- The rail's order -------------------------------------------------------

// The order of the communities on the rail is the account data
// com.slopement.rail_order, {"order": [space IDs]}: set by dragging, the same
// on every session. Communities not in it follow, by name.

const railOrderData = "com.slopement.rail_order"

var railOrderType = event.Type{Type: railOrderData, Class: event.AccountDataEventType}

// SetCommunityOrder sets the rail's order: it shows at once and is saved to
// the account.
func (s *Source) SetCommunityOrder(ids []string) {
	s.railOrder = append([]string(nil), ids...)
	s.changed()
	if s.cli == nil {
		return
	}
	order := append([]string(nil), ids...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.cli.SetAccountData(ctx, railOrderData, map[string][]string{"order": order})
	}()
}

// onRailOrder is the order as the account has it.
func (s *Source) onRailOrder(ctx context.Context, evt *event.Event) {
	var c struct {
		Order []string `json:"order"`
	}
	if json.Unmarshal(evt.Content.VeryRaw, &c) != nil {
		return
	}
	s.post(func() { s.railOrder = c.Order })
}

// sortCommunities puts communities in the rail's order, the unlisted after,
// by name.
func (s *Source) sortCommunities(out []model.Community) {
	rank := map[string]int{}
	for i, id := range s.railOrder {
		rank[id] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, iok := rank[out[i].ID]
		rj, jok := rank[out[j].ID]
		switch {
		case iok && jok:
			return ri < rj
		case iok != jok:
			return iok
		}
		return false // both unlisted: the by-name order they came in
	})
}

var _ model.CommunityOrder = (*Source)(nil)

// ---- Reset the local cache ---------------------------------------------------

// ResetCache empties the room cache (rooms, timelines, the sync token) and
// stops writing to it for this run, so the next launch syncs from nothing. The
// encryption keys and the session are in other stores and stay. done runs on
// the UI thread when it is cleared.
func (s *Source) ResetCache(done func()) {
	cache := s.cache
	s.cache = nil // nothing more is queued for it this run
	s.dirty, s.gone = map[string]bool{}, nil
	s.dirtyMeta, s.tokenDirty = false, false
	if cache == nil {
		done()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cache.flush() // let what was queued land first, then drop it
		cache.reset(ctx)
		s.post(done)
	}()
}

var _ model.CacheResetter = (*Source)(nil)

// ---- A space's rooms, joined or not ------------------------------------------

// SpaceRooms lists the rooms a community offers from its hierarchy, with the
// ones the user is in marked: what there is to join from the community's home.
// A sub-space comes marked Space, and the space itself is left out. The
// rooms come in the space's own order, as everywhere else.
func (s *Source) SpaceRooms(space string, done func([]model.PublicRoom, string)) {
	if s.cli == nil {
		done(nil, "Not signed in.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := s.cli.Hierarchy(ctx, id.RoomID(space), &mautrix.ReqHierarchy{Limit: 50, MaxDepth: ptrInt(1)})
		if err != nil {
			s.post(func() { done(nil, "Could not load the community's rooms: "+plain(err)) })
			return
		}
		// The space's own entry says where each child belongs: the order
		// field of its m.space.child, and when it was listed.
		order, listed := map[string]string{}, map[string]int64{}
		for _, info := range resp.Rooms {
			if info.RoomID.String() != space {
				continue
			}
			for _, ev := range info.ChildrenState {
				if ev == nil || ev.StateKey == nil {
					continue
				}
				_ = ev.Content.ParseRaw(ev.Type)
				if c := ev.Content.AsSpaceChild(); c != nil {
					order[*ev.StateKey] = c.Order
				}
				listed[*ev.StateKey] = ev.Timestamp
			}
			break
		}
		var out []model.PublicRoom
		for _, info := range resp.Rooms {
			if info.RoomID.String() == space {
				continue
			}
			pr := model.PublicRoom{
				ID: info.RoomID.String(), Alias: info.CanonicalAlias.String(), Name: info.Name, Topic: info.Topic,
				Members: info.NumJoinedMembers, Avatar: string(info.AvatarURL), Space: info.RoomType == event.RoomTypeSpace,
			}
			if pr.Name == "" {
				pr.Name = pr.Alias
			}
			if pr.Name == "" {
				pr.Name = pr.ID
			}
			out = append(out, pr)
		}
		// By the order field, then when the room was listed, then by ID —
		// the same rule the room list follows (MSC1772).
		sort.SliceStable(out, func(i, j int) bool {
			oi, oj := order[out[i].ID], order[out[j].ID]
			if oi != oj {
				if oi == "" {
					return false
				}
				if oj == "" {
					return true
				}
				return oi < oj
			}
			ti, tj := listed[out[i].ID], listed[out[j].ID]
			if ti != tj {
				return ti < tj
			}
			return out[i].ID < out[j].ID
		})
		s.post(func() {
			for i := range out {
				_, out[i].Joined = s.rooms[out[i].ID]
			}
			done(out, "")
		})
	}()
}

func ptrInt(n int) *int { return &n }

var _ model.SpaceBrowser = (*Source)(nil)
