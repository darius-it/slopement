package app

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"slopement/model"
)

// twoFrameGIF is a small animated GIF.
func twoFrameGIF() []byte {
	pal := color.Palette{color.Black, color.White, color.RGBA{200, 30, 30, 255}}
	frame := func(c uint8) *image.Paletted {
		im := image.NewPaletted(image.Rect(0, 0, 24, 16), pal)
		for i := range im.Pix {
			im.Pix[i] = c
		}
		return im
	}
	var buf bytes.Buffer
	_ = gif.EncodeAll(&buf, &gif.GIF{Image: []*image.Paletted{frame(1), frame(2)}, Delay: []int{5, 5}})
	return buf.Bytes()
}

// gifLoader serves a GIF for any mxc:// URL.
type gifLoader struct {
	model.Source
	asked int
}

func (g *gifLoader) LoadMedia(m model.Media, done func([]byte, error)) { g.asked++; done(twoFrameGIF(), nil) }

// A GIF that arrives as a file plays — looping, from its bytes — instead of
// showing its first frame; downloaded once; paused (first frame) when
// "Animate GIFs" is off; and a "GIF" that is a video is not decoded as one.
func TestGIFFileAnimates(t *testing.T) {
	a := testApp()
	src := &gifLoader{Source: a.src}
	a.src = src
	a.runUI = func(f func()) { f() }

	gifMedia := model.Media{Kind: model.MediaGIF, Mime: "image/gif", URL: "mxc://example.org/anim", Name: "party.gif", W: 24, H: 16}
	ev := model.Event{ID: "$g", Kind: model.EventMedia, Body: "party.gif", Media: []model.Media{gifMedia}}

	_, media, _ := a.content(ev) // the first pass asks for the bytes
	if len(media) != 1 {
		t.Fatalf("media = %+v", media)
	}
	spec := media[0]
	for i := 0; i < 100 && spec.Player == nil; i++ {
		_, m2, _ := a.content(ev)
		spec = m2[0]
		time.Sleep(5 * time.Millisecond)
	}
	if spec.Player == nil {
		t.Fatal("the GIF has no player: it would show as one still")
	}
	if w, h := spec.Player.Size(); w != 24 || h != 16 {
		t.Fatalf("player size = %dx%d", w, h)
	}
	if !spec.Player.Loop() {
		t.Fatal("the GIF does not loop")
	}
	if spec.Paused {
		t.Fatal("paused with Animate GIFs on")
	}
	for i := 0; i < 5; i++ {
		a.content(ev)
	}
	if src.asked != 1 {
		t.Fatalf("downloaded %d times, want once", src.asked)
	}

	a.prefs.gifAutoplay = false
	if _, m3, _ := a.content(ev); !m3[0].Paused {
		t.Fatal("not paused with Animate GIFs off")
	}

	// A short video marked as a GIF is not GIF bytes: no decoding attempt.
	vid := model.Media{Kind: model.MediaGIF, Mime: "video/mp4", URL: "mxc://example.org/clip", Name: "clip.mp4"}
	before := src.asked
	a.content(model.Event{ID: "$v", Kind: model.EventMedia, Body: "clip.mp4", Media: []model.Media{vid}})
	if src.asked != before {
		t.Fatal("a video was downloaded as a GIF")
	}
}

// A GIF being sent plays from its own bytes at once.
func TestGIFBeingSentAnimates(t *testing.T) {
	a := testApp()
	a.runUI = func(f func()) { f() }
	m := model.Media{Kind: model.MediaGIF, Mime: "image/gif", Name: "mine.gif", Local: twoFrameGIF()}
	ev := model.Event{ID: "$l", Kind: model.EventMedia, Body: "mine.gif", Pending: true, Media: []model.Media{m}}
	for i := 0; i < 100; i++ {
		if _, media, _ := a.content(ev); media[0].Player != nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("a GIF being sent does not play")
}

// With "Send GIFs as files" off — the default — the picker's GIF goes as its
// link, and nothing is uploaded. On, it is fetched and uploaded as a picture;
// if it cannot be fetched, the link goes after all.
func TestGIFFromPickerLinkOrFile(t *testing.T) {
	var served int
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone.gif" {
			http.NotFound(w, r)
			return
		}
		served++
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write(twoFrameGIF())
	}))
	defer cdn.Close()

	a := testApp()
	src := &uploadSource{Source: a.src}
	a.src = src
	a.runUI = func(f func()) { f() }
	room := a.src.Rooms()[0].ID

	if a.prefs.gifFiles {
		t.Fatal("sending GIFs as files is on by default")
	}
	a.sendGIF(room, cdn.URL+"/funny.gif")
	if len(src.texts) != 1 || src.texts[0] != cdn.URL+"/funny.gif" || len(src.files) != 0 || served != 0 {
		t.Fatalf("default: texts=%v files=%d fetched=%d", src.texts, len(src.files), served)
	}

	a.prefs.gifFiles = true
	a.sendGIF(room, cdn.URL+"/funny.gif")
	for i := 0; i < 200 && len(src.files) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if len(src.files) != 1 || src.files[0].Name != "funny.gif" || src.files[0].Mime != "image/gif" || len(src.texts) != 1 {
		t.Fatalf("as a file: files=%+v texts=%v", src.files, src.texts)
	}

	a.sendGIF(room, cdn.URL+"/gone.gif") // cannot be fetched: the link goes
	for i := 0; i < 200 && len(src.texts) < 2; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if len(src.texts) != 2 || src.texts[1] != cdn.URL+"/gone.gif" || len(src.files) != 1 {
		t.Fatalf("fallback: files=%d texts=%v", len(src.files), src.texts)
	}
}
