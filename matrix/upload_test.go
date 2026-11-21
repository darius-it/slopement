package matrix

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// A fake homeserver for sending files: it keeps what was uploaded and the
// message that was sent.
type sendServer struct {
	mu       sync.Mutex
	uploaded []byte
	uploadAs string
	sent     map[string]any
}

func (f *sendServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(r.URL.Path, "/upload"):
		f.uploaded, f.uploadAs = body, r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"content_uri":"mxc://example.org/uploaded"}`))
	case strings.Contains(r.URL.Path, "/send/m.room.message/"):
		_ = json.Unmarshal(body, &f.sent)
		_, _ = w.Write([]byte(`{"event_id":"$sent"}`))
	default:
		http.NotFound(w, r)
	}
}

func (f *sendServer) message() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent
}

func pngBytes(w, h int) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)))
	return buf.Bytes()
}

// A picture sent to a plain room: uploaded as it is, sent as an m.image
// with its size and url; shown at once, pending, and settled when sent.
func TestSendFileToPlainRoom(t *testing.T) {
	fake := &sendServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	r := s.roomOf("!a")
	data := pngBytes(4, 3)

	s.SendFile("!a", model.File{Name: "pic.png", Data: data}, "")
	if len(r.events) != 1 || !r.events[0].Pending || r.events[0].Kind != model.EventMedia ||
		r.events[0].Media[0].W != 4 || r.events[0].Media[0].H != 3 || len(r.events[0].Media[0].Local) == 0 {
		t.Fatalf("not shown at once as a pending picture: %+v", r.events)
	}
	waitFor(t, "the message to settle", func() bool { return len(r.events) == 1 && r.events[0].ID == "$sent" })

	if r.events[0].Pending || r.events[0].Failed {
		t.Fatalf("settled event = %+v", r.events[0])
	}
	if !bytes.Equal(fake.uploaded, data) || fake.uploadAs != "image/png" {
		t.Fatalf("uploaded %d bytes as %q, want the picture as image/png", len(fake.uploaded), fake.uploadAs)
	}
	msg := fake.message()
	info, _ := msg["info"].(map[string]any)
	if msg["msgtype"] != "m.image" || msg["url"] != "mxc://example.org/uploaded" || msg["body"] != "pic.png" ||
		info["w"] != float64(4) || info["h"] != float64(3) || msg["file"] != nil {
		t.Fatalf("message = %v", msg)
	}
}

// To an encrypted room the file is encrypted before it goes up, and its key
// travels in the message instead of a plain url.
func TestSendFileToEncryptedRoom(t *testing.T) {
	fake := &sendServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	r := s.roomOf("!a")
	r.encrypted = true
	data := []byte("some secret document, long enough to see the difference")

	s.SendFile("!a", model.File{Name: "notes.txt", Data: data}, "")
	waitFor(t, "the message to settle", func() bool { return len(r.events) == 1 && r.events[0].ID == "$sent" })

	if bytes.Contains(fake.uploaded, []byte("secret document")) || fake.uploadAs != "application/octet-stream" {
		t.Fatalf("the file went up in the clear (as %q)", fake.uploadAs)
	}
	msg := fake.message()
	file, _ := msg["file"].(map[string]any)
	if msg["url"] != nil || file == nil || file["url"] != "mxc://example.org/uploaded" || file["key"] == nil || file["iv"] == nil {
		t.Fatalf("message = %v", msg)
	}
	if msg["msgtype"] != "m.file" {
		t.Fatalf("msgtype = %v", msg["msgtype"])
	}
}

// The preview comes from the homeserver's preview_url, with the picture
// as an mxc:// URL on it.
func TestLinkPreviewAsksTheHomeserver(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("url")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"og:title":"A title","og:description":"About it","og:site_name":"Site",` +
			`"og:image":"mxc://example.org/pic","og:image:width":640,"og:image:height":"360"}`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	s.cli, _ = mautrix.NewClient(srv.URL, "@me:example.org", "token")

	got := make(chan model.Preview, 1)
	s.LinkPreview("https://example.org/page", func(p model.Preview, err error) {
		if err != nil {
			t.Error(err)
		}
		got <- p
	})
	p := <-got
	if asked != "https://example.org/page" {
		t.Fatalf("the homeserver was asked about %q", asked)
	}
	if p.Title != "A title" || p.Site != "Site" || p.Description != "About it" || p.Image != "mxc://example.org/pic" || p.W != 640 || p.H != 360 {
		t.Fatalf("preview = %+v", p)
	}
}

// Too large, or refused by the server: the message stays and is marked.
func TestSendFileFailureIsMarked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(`{"errcode":"M_TOO_LARGE","error":"File too big"}`))
	}))
	defer srv.Close()

	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	r := s.roomOf("!a")
	s.SendFile("!a", model.File{Name: "big.bin", Data: []byte("x")}, "")
	waitFor(t, "the failure", func() bool { return len(r.events) == 1 && r.events[0].Failed })
	if r.events[0].Pending || !strings.Contains(r.events[0].Error, "Not sent") {
		t.Fatalf("event = %+v", r.events[0])
	}
}
