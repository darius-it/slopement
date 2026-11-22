package matrix

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"

	"slopement/model"
)

func solid(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// A profile picture is the middle square of what was chosen, no more than
// 512 across: a wide picture loses its sides, a huge one is scaled down, an
// opaque one is a JPEG and one with transparency stays a PNG.
func TestPrepareAvatar(t *testing.T) {
	// 300×100: left third red, middle blue, right third red — the middle
	// square (100×100) is all blue.
	wide := image.NewRGBA(image.Rect(0, 0, 300, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 300; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= 100 && x < 200 {
				c = color.RGBA{B: 255, A: 255}
			}
			wide.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, wide)
	out, mime, err := prepareAvatar(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" {
		t.Fatalf("an opaque picture is %s", mime)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 100 || b.Dy() != 100 {
		t.Fatalf("size %v, want the middle 100×100", b)
	}
	r, g, bl, _ := img.At(50, 50).RGBA()
	if bl>>8 < 200 || r>>8 > 60 || g>>8 > 60 {
		t.Fatalf("the middle is not blue: %d %d %d", r>>8, g>>8, bl>>8)
	}
	if r, _, _, _ := img.At(2, 50).RGBA(); r>>8 > 60 {
		t.Fatal("the picture kept its red sides")
	}

	// Huge: scaled down to 512.
	buf.Reset()
	_ = png.Encode(&buf, solid(1600, 1200, color.RGBA{G: 200, A: 255}))
	out, _, err = prepareAvatar(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	img, _ = jpeg.Decode(bytes.NewReader(out))
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("a large picture is %v, want 512×512", b)
	}

	// Transparent: stays a PNG.
	buf.Reset()
	_ = png.Encode(&buf, solid(64, 64, color.RGBA{R: 10, G: 10, B: 10, A: 100}))
	_, mime, err = prepareAvatar(buf.Bytes())
	if err != nil || mime != "image/png" {
		t.Fatalf("transparent: %s %v", mime, err)
	}

	// Not a picture.
	if _, _, err := prepareAvatar([]byte("not an image")); err == nil {
		t.Fatal("a text file was taken as a picture")
	}
}

type profileServer struct {
	mu       sync.Mutex
	reqs     []string
	bodies   []string
	mediaBy  []byte
	devices  string
	deleteSC int
}

func (f *profileServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Method+" "+r.URL.Path)
	f.bodies = append(f.bodies, string(body))
	if strings.Contains(r.URL.Path, "/upload") {
		f.mediaBy = body
	}
	devices, sc := f.devices, f.deleteSC
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "/upload"):
		_, _ = w.Write([]byte(`{"content_uri":"mxc://example.org/abc"}`))
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/devices"):
		_, _ = w.Write([]byte(devices))
	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/devices/"):
		if sc != 0 {
			w.WriteHeader(sc)
			_, _ = w.Write([]byte(`{"flows":[{"stages":["m.login.sso"]}],"session":"s"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	default:
		_, _ = w.Write([]byte(`{}`))
	}
}

func (f *profileServer) find(method, part string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.reqs {
		if strings.HasPrefix(r, method+" ") && strings.Contains(r, part) {
			return f.bodies[i], true
		}
	}
	return "", false
}

func profileSetup(t *testing.T, f *profileServer) *Source {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	cli.DeviceID = "OWN"
	s.cli = cli
	return s
}

// A new name shows at once and goes to the profile; a refusal gives it
// back. A new picture is uploaded cropped and set; removing it clears it.
func TestProfileEdits(t *testing.T) {
	f := &profileServer{}
	s := profileSetup(t, f)
	s.me.Name = "old"

	got := make(chan string, 4)
	s.SetDisplayName("  New Name ", func(err string) { got <- err })
	if s.me.Name != "New Name" {
		t.Fatalf("name = %q at once", s.me.Name)
	}
	if err := <-got; err != "" {
		t.Fatal(err)
	}
	if b, ok := f.find("PUT", "/profile/@me:example.org/displayname"); !ok || !strings.Contains(b, "New Name") {
		t.Fatalf("displayname request: %q %v", b, ok)
	}
	s.SetDisplayName("   ", func(err string) { got <- err })
	if err := <-got; err == "" || s.me.Name != "New Name" {
		t.Fatalf("an empty name was taken: %q %q", err, s.me.Name)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, solid(900, 600, color.RGBA{B: 200, A: 255}))
	s.SetAvatar(model.File{Name: "me.png", Data: buf.Bytes()}, func(err string) { got <- err })
	select {
	case err := <-got:
		if err != "" {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the avatar was never set")
	}
	if s.me.Avatar != "mxc://example.org/abc" {
		t.Fatalf("avatar = %q", s.me.Avatar)
	}
	img, err := jpeg.Decode(bytes.NewReader(f.mediaBy))
	if err != nil || img.Bounds().Dx() != 512 || img.Bounds().Dy() != 512 {
		t.Fatalf("what was uploaded: %v %v", img, err)
	}
	if b, ok := f.find("PUT", "/profile/@me:example.org/avatar_url"); !ok || !strings.Contains(b, "mxc://example.org/abc") {
		t.Fatalf("avatar_url request: %q %v", b, ok)
	}

	s.RemoveAvatar(func(err string) { got <- err })
	if s.me.Avatar != "" {
		t.Fatal("the picture is still shown")
	}
	<-got

	s.SetAvatar(model.File{Name: "x.txt", Data: []byte("nope")}, func(err string) { got <- err })
	if err := <-got; err == "" {
		t.Fatal("a text file was set as the picture")
	}
}

// Colors chosen for names are one account data event, shown at once, and
// the account's own copy replaces what is held.
func TestUserColors(t *testing.T) {
	f := &profileServer{}
	s := profileSetup(t, f)
	s.SetUserColor("@kai:example.org", 5)
	s.SetUserColor("@me:example.org", 2)
	if got := s.UserColors(); got["@kai:example.org"] != 5 || got["@me:example.org"] != 2 {
		t.Fatalf("colors = %v", got)
	}
	waitFor(t, "the account data", func() bool {
		b, ok := f.find("PUT", "/account_data/com.slopement.user_colors")
		return ok && strings.Contains(b, `"@kai:example.org":5`)
	})
	s.SetUserColor("@kai:example.org", -1)
	if _, ok := s.UserColors()["@kai:example.org"]; ok {
		t.Fatal("the automatic color was not restored")
	}
	// Another session changed them.
	evt := &event.Event{Content: event.Content{VeryRaw: json.RawMessage(`{"@ada:example.org":1}`)}}
	s.onUserColors(nil, evt)
	if got := s.UserColors(); len(got) != 1 || got["@ada:example.org"] != 1 {
		t.Fatalf("after the account's own: %v", got)
	}
}

// The devices list has this one first, then the others by last seen; a
// homeserver that asks to confirm a sign-out in a browser is said so.
func TestSessionsList(t *testing.T) {
	now := time.Now().UnixMilli()
	f := &profileServer{devices: `{"devices":[` +
		`{"device_id":"OLD","display_name":"Old laptop","last_seen_ip":"1.2.3.4","last_seen_ts":1000},` +
		`{"device_id":"OWN","display_name":"slopement","last_seen_ip":"5.6.7.8","last_seen_ts":` + itoa(now) + `},` +
		`{"device_id":"NEW","display_name":"Phone","last_seen_ts":` + itoa(now-1000) + `}]}`}
	s := profileSetup(t, f)
	s.RefreshSessions()
	waitFor(t, "the devices", func() bool { return len(s.Sessions()) == 3 })
	got := s.Sessions()
	if got[0].ID != "OWN" || !got[0].Current || got[1].ID != "NEW" || got[2].ID != "OLD" {
		t.Fatalf("order: %v %v %v", got[0].ID, got[1].ID, got[2].ID)
	}
	if got[2].Name != "Old laptop" || got[2].IP != "1.2.3.4" || got[2].LastSeen.UnixMilli() != 1000 {
		t.Fatalf("a device: %+v", got[2])
	}

	f.mu.Lock()
	f.deleteSC = 401
	f.mu.Unlock()
	res := make(chan [2]any, 1)
	s.SignOutSession("OLD", func(err string, browser bool) { res <- [2]any{err, browser} })
	if r := <-res; r[1] != true {
		t.Fatalf("a 401 should send the user to the account page: %v", r)
	}
	f.mu.Lock()
	f.deleteSC = 0
	f.mu.Unlock()
	s.SignOutSession("OLD", func(err string, browser bool) { res <- [2]any{err, browser} })
	if r := <-res; r[0] != "" || r[1] != false {
		t.Fatalf("sign out: %v", r)
	}
	for _, ss := range s.Sessions() {
		if ss.ID == "OLD" {
			t.Fatal("the signed-out session is still listed")
		}
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
