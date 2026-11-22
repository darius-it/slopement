package mock

import (
	"sort"
	"strings"

	"slopement/model"
)

// Search looks through every message of every room, as the real source does
// of what it holds (the mock has no server to ask).
func (s *Source) Search(q model.SearchQuery, done func(hits []model.SearchHit, final bool, err string)) {
	if q.Empty() {
		done(nil, true, "")
		return
	}
	var out []model.SearchHit
	for id, r := range s.rooms {
		if q.Room != "" && id != q.Room {
			continue
		}
		for _, e := range s.timeline[id] {
			if e.Kind == model.EventSystem || e.Kind == model.EventUndecryptable || e.Pending {
				continue
			}
			var text strings.Builder
			text.WriteString(e.Body)
			for _, m := range e.Media {
				text.WriteString(" " + m.Name)
			}
			p, _ := s.Person(e.Sender)
			if q.Match(text.String(), r.Name, "", e.Sender, p.Name, e, true) {
				out = append(out, model.SearchHit{Room: id, Event: e.ID, Sender: e.Sender, Time: e.Time, Body: e.Body})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	if len(out) > 200 {
		out = out[:200]
	}
	done(out, true, "")
}

var _ model.Searcher = (*Source)(nil)
