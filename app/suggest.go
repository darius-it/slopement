package app

import (
	"sort"
	"strings"

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
	case strings.HasPrefix(word, ":") && len(word) >= 3 && !strings.Contains(word[1:], ":"):
		return emojiSuggestions(word[1:])
	}
	return nil
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

// applySuggestion takes a suggestion: it replaces the word being typed. A
// mention is remembered, to become a link to the person when the message is
// sent.
func (a *App) applySuggestion(room string, st *kit.ComposerState, it kit.SuggestItem) {
	_, start := wordAtEnd(st.Text)
	st.Text = st.Text[:start] + it.Insert + " "
	if it.ID != "" {
		if a.mentions[room] == nil {
			a.mentions[room] = map[string]string{}
		}
		a.mentions[room][it.Insert] = it.ID
	}
}

// expandMentions turns the mentions chosen in a draft into links to the
// people (https://matrix.to/#/@who), which the sender makes into pills and
// notifications; the ones that were edited away are left as they are. The
// room's remembered mentions are cleared.
func (a *App) expandMentions(room, text string) string {
	chosen := a.mentions[room]
	delete(a.mentions, room)
	inserts := make([]string, 0, len(chosen))
	for ins := range chosen {
		inserts = append(inserts, ins)
	}
	sort.Slice(inserts, func(i, j int) bool { return len(inserts[i]) > len(inserts[j]) }) // "@Alex Rivera" before "@Alex"
	for _, ins := range inserts {
		text = strings.ReplaceAll(text, ins, "["+ins+"](https://matrix.to/#/"+chosen[ins]+")")
	}
	return text
}
