package app

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"slopement/klipy"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// stickerSource records the stickers sent through it.
type stickerSource struct {
	model.Source
	got  chan model.Sticker
	sent chan string
}

func (s *stickerSource) SendSticker(room string, st model.Sticker, replyTo string) { s.got <- st }
func (s *stickerSource) Send(room, text, replyTo string)                           { s.sent <- text }

// A sticker chosen in the picker is fetched and sent as an m.sticker, with its
// bytes; a GIF is still sent as its link.
func TestPickedStickerIsSentAsSticker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	m := mock.New(time.Now())
	m.Delay = 0
	src := &stickerSource{Source: m, got: make(chan model.Sticker, 1), sent: make(chan string, 2)}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }

	a.sendPicked("r1", klipy.Item{Kind: klipy.Stickers, Title: "waving", Full: klipy.Rendition{URL: srv.URL + "/s.png", Width: 4, Height: 4}})
	select {
	case st := <-src.got:
		if st.Body != "waving" || st.Mime != "image/png" || !bytes.Equal(st.Data, buf.Bytes()) || st.W != 4 {
			t.Fatalf("sticker = %+v", st)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no sticker sent")
	}

	a.sendPicked("r1", klipy.Item{Kind: klipy.GIFs, Full: klipy.Rendition{URL: "https://x/g.gif"}})
	select {
	case text := <-src.sent:
		if text != "https://x/g.gif" {
			t.Fatalf("GIF sent as %q", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("GIF not sent")
	}

	// A sticker that cannot be fetched falls back to its link, and says so.
	a.sendPicked("r1", klipy.Item{Kind: klipy.Stickers, Full: klipy.Rendition{URL: srv.URL + "/gone"}})
	select {
	case text := <-src.sent:
		if text != srv.URL+"/gone" || a.toast != "Could not load the sticker" {
			t.Fatalf("fallback sent %q, toast %q", text, a.toast)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no fallback")
	}
}
