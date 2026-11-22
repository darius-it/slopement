package app

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Message search: one screen with the query, chips for the filters that
// have to be reached for (the rest are typed: in: from: before: after:
// mentions:me), and the hits grouped by room, the words highlighted. A press
// on a hit opens the room at the message. It is reached from the switcher's
// "Search messages for …" and by ⌘⇧F.

// searchDebounce is how long the text rests before it is searched (the
// server is asked too: not once per keystroke).
var searchDebounce = 250 * time.Millisecond

// searchState is the screen's.
type searchState struct {
	Text    string // what is typed
	Asked   string // the text Hits answer
	Hits    []model.SearchHit
	Final   bool // the server has been asked too, and has answered
	Err     string
	gen     int       // the search the answer must be for
	waiting string    // the text a search is waiting to run for
	dueAt   time.Time // when it runs

	// Looking further back: the encrypted rooms whose history is paged in a
	// room at a time (each a few pages), and where it is.
	Crawling  bool
	CrawlAt   int
	CrawlN    int
	CrawlRoom string
	crawled   bool // it has run: "Search further back" says "again"
}

// crawlPages is how many pages of history each room gets when looking
// further back (100 messages a page).
const crawlPages = 10

// crawlRooms are the rooms worth paging for a search: the encrypted ones
// (the server can search the others), the most recent first, those the
// in: filter keeps, a dozen at most.
func (a *App) crawlRooms(q model.SearchQuery) []model.Room {
	var out []model.Room
	for _, r := range a.src.Rooms() {
		if !r.Encrypted || (q.Room != "" && r.ID != q.Room) {
			continue
		}
		if q.In != "" && !strings.Contains(strings.ToLower(r.Name), strings.ToLower(q.In)) &&
			!strings.Contains(strings.ToLower(r.Alias), strings.ToLower(q.In)) {
			continue
		}
		out = append(out, r)
		if len(out) == 12 {
			break
		}
	}
	return out
}

// searchFurther pages older history in for the encrypted rooms, one after
// another, and searches again as each page arrives: how what was said in an
// encrypted chat long ago is found at all.
func (a *App) searchFurther(q model.SearchQuery) {
	deep, ok := a.src.(model.DeepSearch)
	st := &a.srch
	rooms := a.crawlRooms(q)
	if !ok || st.Crawling || len(rooms) == 0 {
		return
	}
	st.Crawling, st.crawled, st.CrawlAt, st.CrawlN = true, true, 0, len(rooms)
	var next func()
	next = func() {
		if st.CrawlAt >= len(rooms) {
			st.Crawling, st.CrawlRoom = false, ""
			return
		}
		r := rooms[st.CrawlAt]
		st.CrawlAt++
		st.CrawlRoom = r.Name
		deep.PageBack(r.ID, crawlPages, func(done, more bool) {
			st.Asked = noSearch // search again over what arrived
			if done {
				next()
			}
		})
	}
	next()
}

// searchDeeper is the line (and button) under the results for looking
// further back in encrypted rooms.
func (a *App) searchDeeper(c *ui.Context, q model.SearchQuery) {
	if _, ok := a.src.(model.DeepSearch); !ok || len(a.crawlRooms(q)) == 0 {
		return
	}
	p := kit.Of(c)
	st := &a.srch
	ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Padding(kit.S2).Children(func() {
		switch {
		case st.Crawling:
			ui.Text(c, "Looking further back in "+st.CrawlRoom+" ("+strconv.Itoa(st.CrawlAt)+" of "+strconv.Itoa(st.CrawlN)+")…").
				FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		default:
			label := "Search further back"
			if st.crawled {
				label = "Search even further back"
			}
			ui.Text(c, "Encrypted chats are searched in the messages loaded here.").FontSize(kit.SizeSmall).TextColor(p.TextFaint).Grow(1)
			if kit.Button(c, kit.Secondary, label, nil).Height(28).Clicked() {
				a.searchFurther(q)
			}
		}
	})
}

// openSearch shows the search screen, with a query typed in already.
func (a *App) openSearch(text string) {
	a.srch = searchState{Text: text, Asked: noSearch, Final: true}
	a.view = viewSearch
}

// searchStep runs the search for the text once it has rested. Each frame.
func (a *App) searchStep(s model.Searcher) {
	st := &a.srch
	if st.Text == st.Asked {
		return
	}
	now := time.Now()
	if st.waiting != st.Text {
		st.waiting, st.dueAt = st.Text, now.Add(searchDebounce)
		if a.win != nil && searchDebounce > 0 {
			win := a.win
			time.AfterFunc(searchDebounce+20*time.Millisecond, func() { win.Update(func() {}) })
		}
	}
	if now.Before(st.dueAt) {
		return
	}
	st.gen++
	g := st.gen
	st.Asked = st.Text
	q := model.ParseSearch(st.Text)
	if q.Empty() {
		st.Hits, st.Final, st.Err = nil, true, ""
		return
	}
	s.Search(q, func(hits []model.SearchHit, final bool, err string) {
		if g != a.srch.gen {
			return // an answer to an older text
		}
		a.srch.Hits, a.srch.Final, a.srch.Err = hits, final, err
	})
}

// searchView is the search screen. back is a phone's (a Back button); a
// desktop's closes with its ✕.
func (a *App) searchView(c *ui.Context, back bool) {
	p := kit.Of(c)
	st := &a.srch
	searcher, ok := a.src.(model.Searcher)
	if ok {
		a.searchStep(searcher)
	}
	q := model.ParseSearch(st.Text)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		bar := c.TitleBar()
		ui.Row(c).Height(max(52, bar.Height)).Padding(0, kit.S3+bar.Right, 0, kit.S3).Gap(kit.S2).AlignItems(ui.Center).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
			if back && kit.IconButton(c, kit.Ghost, kit.IconChevronL, "Back").Clicked() {
				a.view = viewRoom
			}
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				kit.SearchField(c.Key("msgsearch"), &st.Text, "Search messages", "").AutoFocus()
			})
			if !back && kit.IconButton(c, kit.Ghost, kit.IconX, "Close search").Clicked() {
				a.view = viewRoom
			}
		})
		// The filters that are easier to press than to type.
		ui.Scroll(c.Key("filters")).Row().Scrollbars(false).Gap(kit.S1).Padding(kit.S2, kit.S3, kit.S1, kit.S3).Shrink(0).Children(func() {
			for _, kind := range []string{"image", "gif", "video", "file", "link"} {
				if kit.Chip(c.Key("has:"+kind), strings.ToUpper(kind[:1])+kind[1:], q.HasKind(kind)).Clicked() {
					st.Text = q.Toggle(kind).Text()
				}
			}
			if kit.Chip(c.Key("mentions"), "Mentions me", q.Mentions).Clicked() {
				q.Mentions = !q.Mentions
				st.Text = q.Text()
			}
		})
		ui.Scroll(c.Key("results")).Grow(1).Padding(kit.S2, kit.S4, kit.S6, kit.S4).Gap(kit.S1).Children(func() {
			switch {
			case !ok:
				emptyNote(c, "This account can't search messages here.")
			case q.Empty():
				a.searchHelp(c)
			default:
				a.searchResults(c, q)
			}
		})
	})
}

// searchHelp is what the empty screen says: how to narrow a search.
func (a *App) searchHelp(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Gap(kit.S2).Padding(kit.S4, 0).Children(func() {
		ui.Text(c, "Search your messages").FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
		for _, row := range [][2]string{
			{"in:design", "in a room, by name or address"},
			{"from:kai", "from a person, by name or address"},
			{"has:image", "image, gif, video, file or link"},
			{"after:2026-01-15", "on or after a day; before: for the other end"},
			{"mentions:me", "messages that mention you"},
		} {
			ui.Row(c).Gap(kit.S3).Children(func() {
				ui.Text(c, row[0]).FontSize(kit.SizeSmall).Font("monospace").TextColor(p.AccentStrong).MinWidth(140)
				ui.Text(c, row[1]).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			})
		}
		ui.Text(c, "Encrypted chats are searched on this device, in the messages loaded here: \"Search further back\" loads older ones. Other rooms are searched on your homeserver too. ⌘F finds within the open chat.").
			FontSize(kit.SizeSmall).TextColor(p.TextFaint)
	})
}

// searchResults lists the hits, grouped by room (the room with the newest
// hit first).
func (a *App) searchResults(c *ui.Context, q model.SearchQuery) {
	p := kit.Of(c)
	st := &a.srch
	if len(st.Hits) == 0 {
		switch {
		case st.Text != st.Asked || !st.Final:
			emptyNote(c, "Searching…")
		default:
			emptyNote(c, "Nothing found.")
		}
		if st.Err != "" {
			ui.Text(c, st.Err).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		}
		a.searchDeeper(c, q)
		return
	}
	var order []string
	byRoom := map[string][]model.SearchHit{}
	for _, h := range st.Hits {
		if _, seen := byRoom[h.Room]; !seen {
			order = append(order, h.Room)
		}
		byRoom[h.Room] = append(byRoom[h.Room], h)
	}
	words := strings.Fields(strings.ToLower(q.Words))
	for _, rid := range order {
		name := rid
		if r, ok := a.src.Room(rid); ok {
			name = r.Name
		}
		a.sectionHeader(c, name+" · "+strconv.Itoa(len(byRoom[rid])), nil)
		for _, h := range byRoom[rid] {
			a.hitRow(c, h, words)
		}
	}
	switch {
	case !st.Final:
		ui.Text(c, "Asking your homeserver too…").FontSize(kit.SizeSmall).TextColor(p.TextFaint).Padding(kit.S2)
	case st.Err != "":
		ui.Text(c, st.Err).FontSize(kit.SizeSmall).TextColor(p.TextMuted).Padding(kit.S2)
	}
	a.searchDeeper(c, q)
}

// hitRow is one message found: who, when, and what it says with the words
// highlighted. A press opens the room at it.
func (a *App) hitRow(c *ui.Context, h model.SearchHit, words []string) {
	p := kit.Of(c)
	sender, _ := a.src.Person(h.Sender)
	b := ui.ButtonBase(c.Key("hit:" + h.Event)).Gap(kit.S3).Padding(kit.S2, kit.S3).Radius(kit.RadiusSm).
		AlignItems(ui.Start).Justify(ui.Start).Label("Go to the message from " + sender.Name)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	kit.Pressable(b)
	b.Children(func() {
		kit.Avatar(c, a.avatar(sender, 28))
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, sender.Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightSemibold).TextColor(a.colors.Color(p, h.Sender)).SingleLine()
				ui.Text(c, timeLabel(h.Time, a.now)).FontSize(kit.SizeCaption).TextColor(p.TextFaint).SingleLine().
					Tooltip(fullTime(h.Time, a.now))
			})
			spans := []ui.Span{}
			for _, part := range highlightParts(snippet(h.Body, words, 220), words) {
				s := ui.Span{Text: part.Text, Color: p.Text}
				if part.Hit {
					s.Background, s.Color, s.Weight, s.Radius = p.AccentSubtle, p.AccentStrong, kit.WeightMedium, 3
				}
				spans = append(spans, s)
			}
			ui.RichText(c, spans...).FontSize(kit.SizeBody).LineHeight(1.4).MaxLines(3)
		})
	})
	if b.Clicked() {
		a.open(h.Room)
		a.jumpTo(h.Room, h.Event)
		a.jumpAt = h.Time
	}
}

// snippet is the part of a message to show: whitespace collapsed, and — for
// a long one — a window around the first word found, with … where it is cut.
func snippet(body string, words []string, limit int) string {
	text := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	at := 0
	lower := strings.ToLower(text)
	for _, w := range words {
		if i := strings.Index(lower, w); i >= 0 {
			if r := utf8.RuneCountInString(text[:i]); at == 0 || r < at {
				at = r
			}
		}
	}
	start := max(at-limit/4, 0)
	end := min(start+limit, len(runes))
	start = max(end-limit, 0)
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}

// part is a stretch of text, a match of a searched word or not.
type part struct {
	Text string
	Hit  bool
}

// highlightParts splits text at every occurrence of any word (compared
// without regard to case), so the matches can be drawn marked.
func highlightParts(text string, words []string) []part {
	if len(words) == 0 {
		return []part{{Text: text}}
	}
	lower := strings.ToLower(text)
	if len(lower) != len(text) {
		return []part{{Text: text}} // case folding changed the lengths: no marks, no wrong marks
	}
	mark := make([]bool, len(text))
	for _, w := range words {
		if w == "" {
			continue
		}
		for from := 0; ; {
			i := strings.Index(lower[from:], w)
			if i < 0 {
				break
			}
			for k := from + i; k < from+i+len(w); k++ {
				mark[k] = true
			}
			from += i + len(w)
		}
	}
	var out []part
	start := 0
	for i := 1; i <= len(text); i++ {
		if i == len(text) || mark[i] != mark[start] {
			out = append(out, part{Text: text[start:i], Hit: mark[start]})
			start = i
		}
	}
	return out
}
