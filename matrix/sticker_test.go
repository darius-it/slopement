package matrix

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// stickerServer keeps what was uploaded, and the event sent and its type.
type stickerServer struct {
	mu       sync.Mutex
	uploads  int
	uploaded []byte
	evType   string
	sent     map[string]any
}

func (f *stickerServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(r.URL.Path, "/upload"):
		f.uploads++
		f.uploaded = body
		_, _ = w.Write([]byte(`{"content_uri":"mxc://example.org/sticker"}`))
	case strings.Contains(r.URL.Path, "/send/"):
		parts := strings.Split(r.URL.Path, "/send/")
		f.evType = strings.SplitN(parts[1], "/", 2)[0]
		_ = json.Unmarshal(body, &f.sent)
		_, _ = w.Write([]byte(`{"event_id":"$sticker"}`))
	default:
		http.NotFound(w, r)
	}
}

// A sticker goes up as a picture and is sent as an m.sticker — also to an
// encrypted room, where the picture stays a plain url as clients send them.
func TestSendStickerUploadsAndSendsMSticker(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		fake := &stickerServer{}
		srv := httptest.NewServer(http.HandlerFunc(fake.handler))
		s, _ := testSynced()
		cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
		s.cli = cli
		r := s.roomOf("!a")
		r.encrypted = encrypted
		data := pngBytes(8, 6)

		s.SendSticker("!a", model.Sticker{Body: "waving cat", Mime: "image/png", W: 8, H: 6, Data: data}, "")
		if len(r.events) != 1 || !r.events[0].Pending || !r.events[0].Media[0].Sticker {
			t.Fatalf("not shown at once as a pending sticker: %+v", r.events)
		}
		waitFor(t, "the sticker to settle", func() bool { return len(r.events) == 1 && r.events[0].ID == "$sticker" })
		fake.mu.Lock()
		if fake.evType != "m.sticker" || !bytes.Equal(fake.uploaded, data) {
			t.Fatalf("encrypted=%v: sent type %q, uploaded %d bytes", encrypted, fake.evType, len(fake.uploaded))
		}
		info, _ := fake.sent["info"].(map[string]any)
		if fake.sent["url"] != "mxc://example.org/sticker" || fake.sent["body"] != "waving cat" ||
			info["w"] != float64(8) || info["mimetype"] != "image/png" || fake.sent["msgtype"] != nil {
			t.Fatalf("encrypted=%v: content = %v", encrypted, fake.sent)
		}
		fake.mu.Unlock()
		srv.Close()
	}
}

// A pack's sticker is already on the server: nothing is uploaded.
func TestSendStickerByURL(t *testing.T) {
	fake := &stickerServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	r := s.roomOf("!a")
	s.SendSticker("!a", model.Sticker{Body: "hi", Mime: "image/webp", URL: "mxc://example.org/pack1"}, "$reply")
	waitFor(t, "the sticker to settle", func() bool { return len(r.events) == 1 && r.events[0].ID == "$sticker" })
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.uploads != 0 || fake.sent["url"] != "mxc://example.org/pack1" {
		t.Fatalf("uploads %d, content %v", fake.uploads, fake.sent)
	}
	if rel, _ := fake.sent["m.relates_to"].(map[string]any); rel == nil {
		t.Fatalf("the reply is lost: %v", fake.sent)
	}
}

// An m.sticker arrives as a media event marked as a sticker, a GIF one
// animating.
func TestReceiveSticker(t *testing.T) {
	c := &event.MessageEventContent{Body: "dancing", URL: "mxc://example.org/d",
		Info: &event.FileInfo{MimeType: "image/gif", Width: 120, Height: 120}}
	evt := msgEvent("$s", "!r", "@ada:example.org", 1, c)
	evt.Type = event.EventSticker
	e := textEvent("@me:example.org", evt)
	if e.Kind != model.EventMedia || len(e.Media) != 1 {
		t.Fatalf("event = %+v", e)
	}
	if m := e.Media[0]; !m.Sticker || m.Kind != model.MediaGIF || m.URL != "mxc://example.org/d" || m.W != 120 {
		t.Fatalf("media = %+v", m)
	}
}
