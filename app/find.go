package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Find in this chat (⌘F): a bar under the room's header that steps through
// the messages that say something, jumping to each. It searches this room
// only — and, because an encrypted room's old messages are not held until
// they are fetched and decrypted, it pages older history in as it looks (a
// bounded number of pages, said in the bar) so that what was said last year
// is found too.

// findPagesAtOnce is how many pages of history a find asks for before it
// offers to keep looking: 100 events a page.
const findPagesAtOnce = 30

type findState struct {
	Open    bool
	Room    string
	Text    string
	Asked   string // the text Hits answer
	Hits    []model.SearchHit
	At      int    // the match shown (0 is the newest)
	Cur     string // its event, kept as older matches arrive
	Paging  bool   // history is being paged in
	More    bool   // the room has older messages than were searched
	Pages   int    // pages asked for so far
	focused bool

	waiting  string
	lastText string // the text of the last search run: a different one is a new query
	dueAt    time.Time
	gen      int
}

// openFind opens the bar on the open room (a second ⌘F puts the cursor back
// in it).
func (a *App) openFind() {
	if a.view != viewRoom || a.room == "" {
		return
	}
	f := &a.find
	if !f.Open || f.Room != a.room {
		*f = findState{Open: true, Room: a.room, Asked: noSearch}
	}
	f.focused = false
}

func (a *App) closeFind() { a.find = findState{} }

// findStep runs the search once the text has rested, and starts paging
// older history in. Each frame the bar is open.
func (a *App) findStep(s model.Searcher) {
	f := &a.find
	if f.Room != a.room { // another room opened: the bar goes
		a.closeFind()
		return
	}
	if f.Text == f.Asked {
		return
	}
	now := time.Now()
	if f.waiting != f.Text {
		f.waiting, f.dueAt = f.Text, now.Add(searchDebounce)
		if a.win != nil && searchDebounce > 0 {
			win := a.win
			time.AfterFunc(searchDebounce+20*time.Millisecond, func() { win.Update(func() {}) })
		}
	}
	if now.Before(f.dueAt) {
		return
	}
	f.gen++
	g := f.gen
	f.Asked = f.Text
	q := model.SearchQuery{Words: strings.TrimSpace(f.Text), Room: f.Room}
	if q.Empty() {
		f.Hits, f.At, f.Cur = nil, 0, ""
		return
	}
	if f.Text != f.lastText { // a new query starts at its newest match
		f.Cur, f.lastText = "", f.Text
	}
	s.Search(q, func(hits []model.SearchHit, final bool, err string) {
		if g != a.find.gen {
			return // an answer to an older text
		}
		a.setFindHits(hits)
	})
	if f.Pages == 0 {
		a.startPaging() // once per find: older history comes in as the user looks
	}
}

// setFindHits takes new hits: the match shown stays the same message when it
// is still there (older matches arriving do not move the user), else the
// newest match is shown, and jumped to.
func (a *App) setFindHits(hits []model.SearchHit) {
	f := &a.find
	f.Hits = hits
	for i, h := range hits {
		if h.Event == f.Cur {
			f.At = i
			return
		}
	}
	f.At = 0
	if len(hits) > 0 {
		a.findShow(0)
	} else {
		f.Cur = ""
	}
}

// findShow makes match i the one shown and jumps to it.
func (a *App) findShow(i int) {
	f := &a.find
	if len(f.Hits) == 0 {
		return
	}
	i = (i%len(f.Hits) + len(f.Hits)) % len(f.Hits) // round the ends
	f.At, f.Cur = i, f.Hits[i].Event
	a.jumpTo(f.Room, f.Cur)
	a.jumpAt = f.Hits[i].Time
}

// startPaging has a batch of older history paged in while the user looks:
// the first when the first search runs, and another when they ask to keep
// looking. Each page re-runs the search over what arrived.
func (a *App) startPaging() {
	f := &a.find
	deep, ok := a.src.(model.DeepSearch)
	if !ok || f.Paging {
		return
	}
	f.Paging, f.More = true, true
	f.Pages += findPagesAtOnce
	room := f.Room
	deep.PageBack(room, findPagesAtOnce, func(done, more bool) {
		if a.find.Room != room {
			return
		}
		a.find.More = more
		if done {
			a.find.Paging = false
		}
		a.find.Asked = noSearch // search again over what arrived
	})
}

// findBar is the bar under the header.
func (a *App) findBar(c *ui.Context) {
	f := &a.find
	if !f.Open {
		return
	}
	searcher, ok := a.src.(model.Searcher)
	if !ok {
		return
	}
	a.findStep(searcher)
	if !f.Open {
		return
	}
	p := kit.Of(c)
	locked := 0
	if f.Text != "" {
		for _, e := range a.src.Timeline(f.Room) {
			if e.Kind == model.EventUndecryptable {
				locked++
			}
		}
	}
	ui.Column(c.Key("findbar")).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Background(p.BgSubtle).Children(func() {
		ui.Row(c).Gap(kit.S2).Padding(kit.S2, kit.S3).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				in := kit.SearchField(c.Key("find-q"), &f.Text, "Find in this chat", "")
				if !f.focused {
					in.AutoFocus()
					f.focused = true
				}
				// Enter goes to the next match, older.
				if in.Submitted() {
					a.findShow(f.At + 1)
				}
			})
			ui.Text(c, a.findCount()).FontSize(kit.SizeSmall).FontFeatures("tnum").TextColor(p.TextMuted).SingleLine().Shrink(0)
			if kit.IconButton(c, kit.Ghost, kit.IconChevronL, "Newer match").Clicked() {
				a.findShow(f.At - 1)
			}
			if kit.IconButton(c, kit.Ghost, kit.IconChevronR, "Older match").Clicked() {
				a.findShow(f.At + 1)
			}
			if kit.IconButton(c, kit.Ghost, kit.IconX, "Close find").Clicked() {
				a.closeFind()
			}
		})
		if line := a.findStatus(locked); line != "" {
			ui.Row(c).Gap(kit.S2).Padding(0, kit.S4, kit.S2, kit.S4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, line).FontSize(kit.SizeCaption).TextColor(p.TextMuted)
				if !f.Paging && f.More && f.Text != "" {
					if kit.Button(c, kit.Ghost, "Keep looking", nil).Height(22).Clicked() {
						a.startPaging()
					}
				}
			})
		}
	})
}

// findCount is "3 of 12", or why there is no match yet.
func (a *App) findCount() string {
	f := &a.find
	switch {
	case strings.TrimSpace(f.Text) == "":
		return ""
	case len(f.Hits) > 0:
		return strconv.Itoa(f.At+1) + " of " + strconv.Itoa(len(f.Hits))
	case f.Paging || f.Text != f.Asked:
		return "Searching…"
	}
	return "No matches"
}

// findStatus is the line under the bar: what has been searched.
func (a *App) findStatus(locked int) string {
	f := &a.find
	if strings.TrimSpace(f.Text) == "" {
		return ""
	}
	var parts []string
	switch {
	case f.Paging:
		parts = append(parts, "Looking through older messages…")
	case f.More:
		parts = append(parts, "Searched the last "+strconv.Itoa(f.Pages*100)+" messages.")
	}
	if locked > 0 {
		parts = append(parts, strconv.Itoa(locked)+" locked messages can't be searched until their keys arrive.")
	}
	return strings.Join(parts, " ")
}
