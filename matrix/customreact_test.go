package matrix

import (
	"strings"
	"testing"
	"time"
)

// A custom emoji reaction's key is its mxc:// URI, and its shortcode goes with
// it, under both names clients read (MSC4027 and Beeper's). A plain URL is not
// an emoji: nothing is sent for it.
func TestReactWithACustomEmoji(t *testing.T) {
	fake := &recordServer{}
	s, _, done := actionSource(t, fake)
	defer done()

	s.ReactCustom("!a", "$other", "mxc://example.org/wave", "wave")
	if got := reactionsOf(s, "$other"); got["mxc://example.org/wave"].Count != 1 || !got["mxc://example.org/wave"].Mine {
		t.Fatalf("not shown at once: %+v", got)
	}
	waitFor(t, "the reaction to be sent", func() bool { _, ok := fake.find("/send/m.reaction/"); return ok })
	body, _ := fake.find("/send/m.reaction/")
	for _, want := range []string{`"rel_type":"m.annotation"`, `"key":"mxc://example.org/wave"`, `"shortcode":"wave"`,
		`"com.beeper.reaction.shortcode":"wave"`, `"$other"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("reaction event %s lacks %s", body, want)
		}
	}

	// Not an mxc:// URI: ignored.
	fake.mu.Lock()
	before := len(fake.reqs)
	fake.mu.Unlock()
	s.ReactCustom("!a", "$other", "https://example.org/wave.png", "wave")
	time.Sleep(50 * time.Millisecond)
	fake.mu.Lock()
	after := len(fake.reqs)
	fake.mu.Unlock()
	if after != before {
		t.Fatal("a web URL was sent as a custom emoji")
	}
}
