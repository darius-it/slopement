package kit

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func allEmoji() map[string]Emoji {
	out := map[string]Emoji{}
	for _, cat := range EmojiCategories {
		for _, e := range cat.Emojis {
			out[e.Char] = e
		}
	}
	return out
}

// The picker holds the whole Unicode set in Discord's categories: flags and
// symbols included, none twice, no skin-tone variants and none of the data's
// extra emoticon entries.
func TestFullEmojiSet(t *testing.T) {
	var names []string
	total := 0
	for _, cat := range EmojiCategories {
		names = append(names, cat.Name)
		total += len(cat.Emojis)
		if len(cat.Emojis) == 0 {
			t.Errorf("%s is empty", cat.Name)
		}
	}
	if got := strings.Join(names, ","); got != "People,Nature,Food,Activities,Travel,Objects,Symbols,Flags" {
		t.Fatalf("categories %s", got)
	}
	if total < 1700 || total > 2300 {
		t.Fatalf("%d emoji: not the whole base set", total)
	}
	all := allEmoji()
	if len(all) != total {
		t.Fatalf("%d distinct of %d: some are there twice", len(all), total)
	}
	for _, want := range []string{"😀", "👍", "❤️", "🔥", "🥲", "🫠", "🇩🇪", "🇯🇵", "🏳️‍🌈", "🏴‍☠️", "✔️", "🏞️", "#️⃣", "🧙‍♂️", "🫶", "🅰️"} {
		if _, ok := all[want]; !ok {
			t.Errorf("%q (%+q) is missing", want, want)
		}
	}
	for ch, e := range all {
		if strings.Contains(e.Name, "skin tone") || strings.ContainsAny(ch, "\U0001F3FB\U0001F3FC\U0001F3FD\U0001F3FE\U0001F3FF") {
			t.Errorf("a skin-tone variant: %q %s", ch, e.Name)
		}
		if e.Name == "" || strings.ContainsAny(e.Name, ":,") || e.Name != strings.ToLower(e.Name) {
			t.Errorf("name of %q is %q", ch, e.Name)
		}
	}
	flags := EmojiCategories[len(EmojiCategories)-1]
	if flags.Name != "Flags" || len(flags.Emojis) < 250 {
		t.Fatalf("%d flags", len(flags.Emojis))
	}
}

// A character that is text by default (✔, 🏞) is written with its emoji
// selector, as other clients write it, so the same reaction key comes out; one
// that is an emoji already (😀) is left as it is.
func TestEmojiAreQualified(t *testing.T) {
	all := allEmoji()
	for _, ch := range []string{"✔️", "🏞️", "☑️", "❤️"} {
		if _, ok := all[ch]; !ok {
			t.Errorf("%+q is not written with its selector", ch)
		}
	}
	for _, ch := range []string{"😀", "✅", "⭐"} {
		if _, ok := all[ch]; !ok {
			t.Errorf("%+q should be written bare", ch)
		}
	}
}

// Names are findable by the full name, by the curated words people use, and
// by the kind of thing ("flag"); the footer's shortcode joins the words.
func TestEmojiSearch(t *testing.T) {
	has := func(query, want string) {
		t.Helper()
		for _, e := range SearchEmoji(query) {
			if e.Char == want {
				return
			}
		}
		t.Errorf("a search for %q does not find %s", query, want)
	}
	has("grinning", "😀")
	has("tears of joy", "😂")
	has("joy tears laugh", "😂") // the curated words stay
	has("flag germany", "🇩🇪")
	has("rainbow flag", "🏳️‍🌈")
	has("thumbs_up", "👍")
	has("pirate", "🏴‍☠️")
	has("pizza", "🍕")
	has("food asian", "🍱")
	if len(SearchEmoji("zzzzz")) != 0 {
		t.Error("found something for nonsense")
	}
	if got := EmojiName("😂"); got != "face with tears of joy" {
		t.Errorf("name = %q", got)
	}
	if EmojiName("❤️") == "" || EmojiName("❤") == "" {
		t.Error("a heart is nameless written either way")
	}
	if got := emojiShortcode("face with tears of joy"); got != "face_with_tears_of_joy" {
		t.Errorf("shortcode = %q", got)
	}
}

// Each category reads in Unicode's order of subgroups: the smiling faces
// first, then affection; the people's category starts with faces.
func TestEmojiOrder(t *testing.T) {
	people := EmojiCategories[0].Emojis
	if people[0].Char != "😀" {
		t.Fatalf("first is %q", people[0].Char)
	}
	idx := func(ch string) int {
		for i, e := range people {
			if e.Char == ch {
				return i
			}
		}
		return -1
	}
	if !(idx("😀") < idx("😍") && idx("😍") < idx("😛") && idx("😛") < idx("👋")) {
		t.Fatalf("order: grinning %d, heart-eyes %d, tongue %d, wave %d", idx("😀"), idx("😍"), idx("😛"), idx("👋"))
	}
	// The data comes in a random order; the build is the same every time.
	again := buildEmojiCategories()
	for i, cat := range again {
		for j, e := range cat.Emojis {
			if e.Char != EmojiCategories[i].Emojis[j].Char {
				t.Fatalf("a second build differs at %s[%d]", cat.Name, j)
			}
		}
	}
}

// With the whole set in the grid only the rows near the view build their
// cells: the emoji tab opens fast, and scrolling to the flags builds those.
func TestEmojiGridBuildsOnlyNearRows(t *testing.T) {
	st := &EmojiPickerState{Open: true}
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c)
		EmojiPopover(c, ui.Button(c.Key("anchor"), "e"), st, nil)
	}, 700, 700)
	start := time.Now()
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("three frames of the emoji grid took %v", took)
	}
	cells := 0
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name != "" {
			cells++
		}
	}
	if cells == 0 || cells > 400 {
		t.Fatalf("%d emoji cells built: only the rows near the view should be", cells)
	}
	if !tt.HasText("People") {
		t.Fatal("the first category's name is not shown")
	}
}
