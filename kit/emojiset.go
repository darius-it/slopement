package kit

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/forPelevin/gomoji"
	"github.com/rivo/uniseg"
)

// The full emoji set comes from Unicode's emoji list (through gomoji's copy
// of it): every emoji, in Discord's categories, without the skin-tone
// variants (the picker has no tone chooser; the yellow default is sent).

// emojiGroups maps Unicode's groups to the picker's categories, in order.
var emojiGroups = []struct {
	name   string
	groups []string
}{
	{"People", []string{"Smileys & Emotion", "People & Body"}},
	{"Nature", []string{"Animals & Nature"}},
	{"Food", []string{"Food & Drink"}},
	{"Activities", []string{"Activities"}},
	{"Travel", []string{"Travel & Places"}},
	{"Objects", []string{"Objects"}},
	{"Symbols", []string{"Symbols"}},
	{"Flags", []string{"Flags"}},
}

// emojiSubgroups is Unicode's order of the subgroups, so each category reads
// as the standard list does: faces, then hands, people, and so on.
var emojiSubgroups = strings.Fields(`
face-smiling face-affection face-tongue face-hand face-neutral-skeptical face-sleepy face-unwell face-hat
face-glasses face-concerned face-negative face-costume cat-face monkey-face emotion
hand-fingers-open hand-fingers-partial hand-single-finger hand-fingers-closed hands hand-prop body-parts
person person-gesture person-role person-fantasy person-activity person-sport person-resting family person-symbol
animal-mammal animal-bird animal-amphibian animal-reptile animal-marine animal-bug plant-flower plant-other
food-fruit food-vegetable food-prepared food-asian food-marine food-sweet drink dishware
place-map place-geographic place-building place-religious place-other transport-ground transport-water transport-air
hotel time sky-weather
event award-medal sport game arts-crafts
clothing sound music musical-instrument phone computer light-video book-paper money mail writing office lock tool
science medical household other-object
transport-sign warning arrow religion zodiac av-symbol gender math punctuation currency other-symbol keycap alphanum geometric
flag country-flag subdivision-flag`)

// emojiPart is one emoji of the data, sorted.
type emojiPart struct {
	e     Emoji
	cat   int
	sub   int
	rank  int   // its place in the curated list, if it is on it
	order []int // the code points, for a stable order within a subgroup
}

// buildEmojiCategories assembles the categories from the data: the emoji of
// the standard groups (not the extras the data adds), once each, qualified for
// display, named in lowercase, in Unicode's order of subgroups.
func buildEmojiCategories() []EmojiCategory {
	catOf := map[string]int{}
	for i, g := range emojiGroups {
		for _, name := range g.groups {
			catOf[name] = i
		}
	}
	subOf := map[string]int{}
	for i, s := range emojiSubgroups {
		subOf[s] = i
	}
	// The curated words, found by the emoji without its variation selector.
	curated := map[string]string{}
	rank := map[string]int{} // the curated lists are in Unicode's order: the common ones lead their subgroup
	for _, cat := range curatedCategories {
		for _, e := range cat.Emojis {
			curated[bare(e.Char)] = e.Words
			rank[bare(e.Char)] = len(rank)
		}
	}

	var parts []emojiPart
	at := map[string]int{} // by the emoji without its selectors: where it is in parts
	for _, d := range gomoji.AllEmojis() {
		cat, ok := catOf[d.Group]
		if !ok || !isUnicodeName(d.UnicodeName) || strings.Contains(d.UnicodeName, "skin tone") {
			continue
		}
		char, order := fromCodePoints(d.CodePoint)
		if char == "" {
			continue
		}
		sub, known := subOf[d.SubGroup]
		if !known {
			sub = len(emojiSubgroups)
		}
		name := emojiName(d.UnicodeName)
		words := curated[bare(char)]
		if sg := strings.ReplaceAll(d.SubGroup, "-", " "); sg != "" {
			words += " " + sg // "flag", "food asian": a way to browse by search
		}
		r, listed := rank[bare(char)]
		if !listed {
			r = 1 << 20
		}
		p := emojiPart{Emoji{Char: char, Name: name, Words: strings.TrimSpace(strings.ToLower(words))}, cat, sub, r, order}
		// The data holds an emoji written with and without its selectors as
		// two entries: keep the fully qualified one (most selectors), which
		// is how other clients write it, so reactions and text match.
		if i, dup := at[bare(char)]; dup {
			old := parts[i].e.Char
			if n, o := strings.Count(char, "️"), strings.Count(old, "️"); n > o || (n == o && char < old) {
				parts[i] = p
			}
			continue
		}
		at[bare(char)] = len(parts)
		parts = append(parts, p)
	}
	sort.Slice(parts, func(i, j int) bool {
		a, b := parts[i], parts[j]
		if a.cat != b.cat {
			return a.cat < b.cat
		}
		if a.sub != b.sub {
			return a.sub < b.sub
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		for k := 0; k < len(a.order) && k < len(b.order); k++ {
			if a.order[k] != b.order[k] {
				return a.order[k] < b.order[k]
			}
		}
		return len(a.order) < len(b.order)
	})
	cats := make([]EmojiCategory, len(emojiGroups))
	for i, g := range emojiGroups {
		cats[i].Name = g.name
	}
	for _, p := range parts {
		cats[p.cat].Emojis = append(cats[p.cat].Emojis, p.e)
	}
	return cats
}

// isUnicodeName reports whether a data name is Unicode's own ("E0.6 grinning
// face"), as opposed to the extra emoticon entries the data holds.
func isUnicodeName(n string) bool {
	return len(n) > 3 && n[0] == 'E' && unicode.IsDigit(rune(n[1])) && strings.Contains(n[:min(len(n), 8)], " ")
}

// emojiName is a data name as the picker says it: without its version
// ("E0.6 "), lowercase, the colon and commas of "flag: Kuwait" taken out.
func emojiName(n string) string {
	if _, rest, ok := strings.Cut(n, " "); ok {
		n = rest
	}
	n = strings.NewReplacer(":", "", ",", "", "’", "'").Replace(strings.ToLower(n))
	return strings.Join(strings.Fields(n), " ")
}

// fromCodePoints builds an emoji from the data's code points ("1F9D9 200D
// 2642 FE0F"), and qualifies it: a single character that is text by default
// (✔, 🏞) gets its emoji selector, as other clients write it. It returns the
// code points too, for ordering.
func fromCodePoints(cp string) (string, []int) {
	var b strings.Builder
	var order []int
	for _, f := range strings.Fields(cp) {
		n, err := strconv.ParseInt(f, 16, 32)
		if err != nil {
			return "", nil
		}
		b.WriteRune(rune(n))
		order = append(order, int(n))
	}
	s := b.String()
	if len(order) == 1 && uniseg.StringWidth(s) == 1 {
		s += "️"
	}
	return s, order
}

// bare is an emoji without its emoji selectors, to match one written either way.
func bare(s string) string { return strings.ReplaceAll(s, "️", "") }
