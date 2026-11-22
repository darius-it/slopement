package app

import (
	"sort"
	"strings"
	"unicode/utf8"

	"slopement/kit"
	"slopement/model"
)

// suggestMax is how many suggestions show at once.
const suggestMax = 6

// wordAtEnd is the word being typed — the text after the last space — and
// where it starts.
func wordAtEnd(text string) (word string, start int) {
	start = strings.LastIndexAny(text, " \n\t") + 1
	return text[start:], start
}

// suggestions are what the word being typed could become: after an @, the
// people of the room (who have written in it, and the other person of a DM);
// after a : and two letters, emoji by name. Nothing for any other word, nor
// while the user has put them away.
func (a *App) suggestions(room string, st *kit.ComposerState) []kit.SuggestItem {
	if st.SuggestionsOff() || st.Editing != "" {
		return nil
	}
	word, _ := wordAtEnd(st.Text)
	switch {
	case strings.HasPrefix(word, "@"):
		return a.mentionSuggestions(room, strings.ToLower(word[1:]))
	case strings.HasPrefix(word, "#"):
		return a.roomSuggestions(room, strings.ToLower(word[1:]))
	case strings.HasPrefix(word, ":") && len(word) >= 3 && !strings.Contains(word[1:], ":"):
		out := append(a.packSuggestions(room, strings.ToLower(word[1:])), emojiSuggestions(word[1:])...)
		return out[:min(len(out), suggestMax)] // custom emoji lead
	}
	return nil
}

// packSuggestions are custom emoji (of the packs on offer in the room) whose
// shortcode contains the word after the colon, those it starts with first.
// Picking one writes ":shortcode:", which the source sends as the picture.
func (a *App) packSuggestions(room, query string) []kit.SuggestItem {
	ps, ok := a.src.(model.Packs)
	if !ok {
		return nil
	}
	var starts, contains []kit.SuggestItem
	for _, p := range ps.EmojiPacks(room) {
		for _, e := range p.Emoji {
			code := strings.ToLower(e.Shortcode)
			if !e.Emoticon || !strings.Contains(code, query) {
				continue
			}
			it := kit.SuggestItem{Label: e.Shortcode, Sub: p.Name, Insert: ":" + e.Shortcode + ":", Image: a.packImage(e)}
			if strings.HasPrefix(code, query) {
				starts = append(starts, it)
			} else {
				contains = append(contains, it)
			}
		}
	}
	out := append(starts, contains...)
	if len(out) > suggestMax {
		out = out[:suggestMax]
	}
	return out
}

// roomSuggestions are the rooms (not chats with a person, nor communities)
// whose name or address starts with the query, then those that contain it:
// picking one writes its address and makes the word a link to it, which
// other clients show as a room pill.
func (a *App) roomSuggestions(current, query string) []kit.SuggestItem {
	type scored struct {
		r     model.Room
		score int
	}
	var found []scored
	for _, r := range a.src.Rooms() { // already most recently active first
		if r.Kind == model.KindDM || r.ID == current {
			continue
		}
		name := strings.ToLower(r.Name)
		alias := strings.ToLower(strings.TrimPrefix(r.Alias, "#"))
		switch {
		case query == "":
			found = append(found, scored{r, 2})
		case strings.HasPrefix(name, query) || strings.HasPrefix(alias, query):
			found = append(found, scored{r, 2})
		case strings.Contains(name, query) || strings.Contains(alias, query):
			found = append(found, scored{r, 1})
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	var out []kit.SuggestItem
	for _, f := range found {
		if len(out) == suggestMax {
			break
		}
		target := f.r.Alias // linked by its address; a room with none, by its ID
		insert := f.r.Alias
		if target == "" {
			target, insert = f.r.ID, "#"+f.r.Name
		}
		out = append(out, kit.SuggestItem{Label: f.r.Name, Sub: f.r.Alias,
			Avatar: kit.AvatarSpec{Name: f.r.Name, ID: f.r.ID, Size: 24, Square: true, Colors: &a.colors, Image: a.avatarPic(f.r.Avatar)},
			Insert: insert, ID: target})
	}
	return out
}

// mentionSuggestions are people of the room whose name or address starts
// with the query, then those that contain it.
func (a *App) mentionSuggestions(room, query string) []kit.SuggestItem {
	me := a.src.Me().ID
	seen := map[string]bool{me: true}
	var ids []string
	if r, ok := a.src.Room(room); ok && r.Partner != "" && !seen[r.Partner] {
		seen[r.Partner] = true
		ids = append(ids, r.Partner)
	}
	tl := a.src.Timeline(room)
	for i := len(tl) - 1; i >= 0; i-- { // the most recent speakers first
		if s := tl[i].Sender; s != "" && !seen[s] && tl[i].Kind != model.EventSystem {
			seen[s] = true
			ids = append(ids, s)
		}
	}
	type scored struct {
		p     model.Person
		score int
	}
	var found []scored
	for _, id := range ids {
		p, _ := a.src.Person(id)
		name, local := strings.ToLower(p.Name), strings.ToLower(strings.TrimPrefix(id, "@"))
		switch {
		case query == "":
			found = append(found, scored{p, 2})
		case strings.HasPrefix(name, query) || strings.HasPrefix(local, query):
			found = append(found, scored{p, 2})
		case strings.Contains(name, query) || strings.Contains(local, query):
			found = append(found, scored{p, 1})
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	var out []kit.SuggestItem
	for _, f := range found {
		if len(out) == suggestMax {
			break
		}
		out = append(out, kit.SuggestItem{Label: f.p.Name, Sub: f.p.ID, Avatar: a.avatar(f.p, 24),
			Insert: "@" + f.p.Name, ID: f.p.ID})
	}
	return out
}

// emojiSuggestions are emoji whose name matches the word after the colon
// (":smi", ":thumbs_up").
func emojiSuggestions(query string) []kit.SuggestItem {
	var out []kit.SuggestItem
	for _, e := range kit.SearchEmoji(strings.ReplaceAll(query, "_", " ")) {
		out = append(out, kit.SuggestItem{Label: e.Name, Sub: ":" + strings.ReplaceAll(e.Name, " ", "_") + ":",
			Emoji: e.Char, Insert: e.Char})
		if len(out) == suggestMax {
			break
		}
	}
	return out
}

// applySuggestion takes a suggestion: it replaces the word being typed, with
// the caret after it, the user going on typing. A mention is marked with the
// person it is of — the field draws it in a frame, and the draft sends it as
// a link to them (kit.ComposerState.Draft).
func (a *App) applySuggestion(room string, st *kit.ComposerState, it kit.SuggestItem) {
	_, start := wordAtEnd(st.Text)
	st.Complete(utf8.RuneCountInString(st.Text[:start]), it)
}
