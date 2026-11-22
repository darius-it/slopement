package matrix

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

const catPack = `{"pack":{"display_name":"Cats","usage":["emoticon"]},
 "images":{
  "wave":{"url":"mxc://x/wave","body":"waving","info":{"w":64,"h":64,"mimetype":"image/png"}},
  "big":{"url":"mxc://x/big","usage":["sticker"]},
  "both":{"url":"mxc://x/both","usage":["emoticon","sticker"]},
  "broken":{"url":"https://not-mxc/x.png"}}}`

// The pack's usage is the default, an image's own wins, an image with a url
// that isn't mxc:// is left out.
func TestParsePack(t *testing.T) {
	p, ok := parsePack("r|cats", "Room", "!r", []byte(catPack))
	if !ok || p.Name != "Cats" || p.Room != "!r" || len(p.Emoji) != 3 {
		t.Fatalf("pack = %+v", p)
	}
	by := map[string]model.PackEmoji{}
	for _, e := range p.Emoji {
		by[e.Shortcode] = e
	}
	if w := by["wave"]; !w.Emoticon || w.Sticker || w.W != 64 || w.Mime != "image/png" || w.Body != "waving" {
		t.Errorf("wave = %+v", w)
	}
	if b := by["big"]; b.Emoticon || !b.Sticker {
		t.Errorf("big = %+v", b)
	}
	if b := by["both"]; !b.Emoticon || !b.Sticker {
		t.Errorf("both = %+v", b)
	}
	if _, ok := parsePack("x", "n", "", []byte(`{"images":{}}`)); ok {
		t.Error("an empty pack counts")
	}
	if p, _ := parsePack("x", "Fallback", "", []byte(`{"images":{"a":{"url":"mxc://x/a"}}}`)); p.Name != "Fallback" || !p.Emoji[0].Emoticon || !p.Emoji[0].Sticker {
		t.Errorf("no usage at all = %+v", p)
	}
}

// What is on offer in a room: the user's own, then the room's, then the packs
// turned on elsewhere (only the keys turned on).
func TestEmojiPacksOnOffer(t *testing.T) {
	s, _ := testSynced()
	s.packs.user = map[string]any{"images": map[string]any{"mine": map[string]any{"url": "mxc://x/mine"}}}
	s.setRoomPack("!here", "a", []byte(catPack))
	s.setRoomPack("!away", "on", []byte(`{"pack":{"display_name":"Away on"},"images":{"o":{"url":"mxc://x/o"}}}`))
	s.setRoomPack("!away", "off", []byte(`{"pack":{"display_name":"Away off"},"images":{"o":{"url":"mxc://x/p"}}}`))
	s.packs.enabled = map[string]map[string]any{"!away": {"on": map[string]any{}}, "!here": {"a": map[string]any{}}}
	got := s.EmojiPacks("!here")
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "My emoji,Cats,Away on" {
		t.Fatalf("packs = %v (the room's own pack must not repeat, an unlisted one must not show)", names)
	}
	if !got[0].Mine || got[1].Mine {
		t.Fatal("only the user's own pack is mine")
	}
	// A pack that is emptied goes.
	s.setRoomPack("!here", "a", []byte(`{"images":{}}`))
	if n := len(s.EmojiPacks("!here")); n != 2 {
		t.Fatalf("%d packs after the room's was emptied", n)
	}
}

// ":wave:" of an enabled pack becomes its image; a name that isn't one, a
// sticker-only image, code and attributes stay as typed.
func TestWithCustomEmoji(t *testing.T) {
	s, _ := testSynced()
	s.setRoomPack("!r", "a", []byte(catPack))
	got := s.withCustomEmoji("!r", `<p>hi :wave: and :nope: and :big: <code>:wave:</code> <a href="x:wave:">:wave:</a></p>`)
	img := `<img data-mx-emoticon src="mxc://x/wave" alt=":wave:" title=":wave:" height="32" />`
	if want := `<p>hi ` + img + ` and :nope: and :big: <code>:wave:</code> <a href="x:wave:">` + img + `</a></p>`; got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
	if other := "<p>no emoji here</p>"; s.withCustomEmoji("!elsewhere", `<p>:wave:</p>`) != `<p>:wave:</p>` || s.withCustomEmoji("!r", other) != other {
		t.Fatal("changed what it should not")
	}
}

// Sending a message with a custom emoji's shortcode puts the image in the
// formatted body, and leaves the plain body as typed.
func TestSendUsesCustomEmoji(t *testing.T) {
	var mu sync.Mutex
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/send/") {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(b, &sent)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"event_id":"$m"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.packs.userAsked = true // the fake homeserver has no account data
	s.packs.asked = map[string]bool{"!a": true}
	s.setRoomPack("!a", "p", []byte(catPack))
	r := s.roomOf("!a")
	s.Send("!a", "look :wave:", "")
	waitFor(t, "the send", func() bool { return len(r.events) == 1 && r.events[0].ID == "$m" })
	mu.Lock()
	defer mu.Unlock()
	if sent["body"] != "look :wave:" || sent["format"] != "org.matrix.custom.html" ||
		!strings.Contains(sent["formatted_body"].(string), `<img data-mx-emoticon src="mxc://x/wave"`) {
		t.Fatalf("content = %v", sent)
	}
}

// "Add to my pack" saves the image under its shortcode in the account's pack,
// a taken name gets a number, the same picture twice is kept once, and what
// was in the pack stays.
func TestAddToPack(t *testing.T) {
	var mu sync.Mutex
	var saved map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/account_data/im.ponies.user_emotes") {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(b, &saved)
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.packs.user = map[string]any{"pack": map[string]any{"display_name": "Mine"}, "extra": "kept",
		"images": map[string]any{"wave": map[string]any{"url": "mxc://x/old"}}}
	res := make(chan string, 3)
	s.AddToPack(model.PackEmoji{Shortcode: "Wave", URL: "mxc://x/new", Mime: "image/gif", W: 32, H: 32}, func(p string) { res <- p })
	waitFor(t, "the save", func() bool { mu.Lock(); defer mu.Unlock(); return saved != nil })
	if p := <-res; p != "" {
		// done runs through post; the test source runs it inline or later
		t.Fatalf("problem %q", p)
	}
	mu.Lock()
	images, _ := saved["images"].(map[string]any)
	mu.Unlock()
	if len(images) != 2 || images["wave"].(map[string]any)["url"] != "mxc://x/old" || images["wave_2"].(map[string]any)["url"] != "mxc://x/new" ||
		saved["extra"] != "kept" || saved["pack"].(map[string]any)["display_name"] != "Mine" {
		t.Fatalf("saved = %v", saved)
	}
	s.AddToPack(model.PackEmoji{Shortcode: "again", URL: "mxc://x/new"}, func(p string) { res <- p })
	if p := <-res; p != "" || len(s.packs.user["images"].(map[string]any)) != 2 {
		t.Fatalf("the same picture twice: %q, %v", p, s.packs.user)
	}
	s.AddToPack(model.PackEmoji{Shortcode: "x", URL: "https://not-mxc"}, func(p string) { res <- p })
	if p := <-res; p == "" {
		t.Fatal("added a picture that is not on the media repo")
	}
}
