package matrix

import (
	"bytes"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"maunium.net/go/mautrix"

	"slopement/model"
)

type editServer struct {
	mu     sync.Mutex
	reqs   []string
	bodies []string
	levels string // the power levels it answers a GET with
	refuse bool   // state writes answer M_FORBIDDEN
}

func (f *editServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Method+" "+r.URL.Path)
	f.bodies = append(f.bodies, string(body))
	levels, refuse := f.levels, f.refuse
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "m.room.power_levels"):
		_, _ = w.Write([]byte(levels))
	case strings.Contains(r.URL.Path, "/upload"):
		_, _ = w.Write([]byte(`{"content_uri":"mxc://example.org/room-pic"}`))
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/state/") && refuse:
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"errcode":"M_FORBIDDEN","error":"no"}`))
	default:
		_, _ = w.Write([]byte(`{"event_id":"$ok"}`))
	}
}

func (f *editServer) put(part string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.reqs {
		if strings.HasPrefix(r, "PUT ") && strings.Contains(r, part) {
			return f.bodies[i], true
		}
	}
	return "", false
}

func editSetup(t *testing.T, f *editServer) (*Source, *room) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	r.name, r.topic = "Old name", "old topic"
	return s, r
}

// What the user may change follows the power levels: each of name, topic and
// picture has the level to send it, and the user's own level is compared.
func TestRoomPermissions(t *testing.T) {
	f := &editServer{levels: `{"users":{"@me:example.org":50},"events":{"m.room.name":50,"m.room.topic":75,"m.room.avatar":0},"state_default":100}`}
	s, _ := editSetup(t, f)
	got := make(chan model.RoomPerms, 1)
	s.RoomPermissions("!a", func(p model.RoomPerms) { got <- p })
	select {
	case p := <-got:
		if !p.Name || p.Topic || !p.Avatar {
			t.Fatalf("perms at power 50 (name 50, topic 75, avatar 0): %+v", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no answer")
	}
	// No power levels readable: nothing is offered.
	f.mu.Lock()
	f.levels = `{"errcode":"M_NOT_FOUND"}`
	f.mu.Unlock()
	s2, _ := editSetup(t, &editServer{levels: `not json`})
	s2.RoomPermissions("!a", func(p model.RoomPerms) { got <- p })
	if p := <-got; p.Name || p.Topic || p.Avatar {
		t.Fatalf("unknown levels offered %+v", p)
	}
}

// Renaming, a new topic and a new picture show at once and go as state
// events; a refusal puts the old value back, in words.
func TestEditRoom(t *testing.T) {
	f := &editServer{}
	s, r := editSetup(t, f)
	done := make(chan string, 4)
	cb := func(err string) { done <- err }

	s.SetRoomName("!a", "  New name ", cb)
	if r.name != "New name" {
		t.Fatalf("not shown at once: %q", r.name)
	}
	if err := <-done; err != "" {
		t.Fatal(err)
	}
	if b, ok := f.put("/state/m.room.name"); !ok || !strings.Contains(b, `"name":"New name"`) {
		t.Fatalf("name request: %q %v", b, ok)
	}
	s.SetRoomName("!a", "   ", cb)
	if err := <-done; err == "" || r.name != "New name" {
		t.Fatal("an empty name was taken")
	}

	s.SetRoomTopic("!a", "a new topic", cb)
	<-done
	if b, ok := f.put("/state/m.room.topic"); !ok || !strings.Contains(b, `"topic":"a new topic"`) || r.topic != "a new topic" {
		t.Fatalf("topic: %q %q", b, r.topic)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, solid(300, 200, color.RGBA{G: 180, A: 255}))
	s.SetRoomAvatar("!a", model.File{Name: "p.png", Data: buf.Bytes()}, cb)
	if err := <-done; err != "" {
		t.Fatal(err)
	}
	if r.avatar != "mxc://example.org/room-pic" {
		t.Fatalf("avatar %q", r.avatar)
	}
	if b, ok := f.put("/state/m.room.avatar"); !ok || !strings.Contains(b, "mxc://example.org/room-pic") {
		t.Fatalf("avatar request: %q %v", b, ok)
	}
	s.RemoveRoomAvatar("!a", cb)
	<-done
	if r.avatar != "" {
		t.Fatal("the picture is still there")
	}

	// Refused: back as it was, and the reason in a line.
	f.mu.Lock()
	f.refuse = true
	f.mu.Unlock()
	s.SetRoomName("!a", "Not allowed", cb)
	if err := <-done; !strings.Contains(err, "permission") || r.name != "New name" {
		t.Fatalf("refusal: %q, name %q", err, r.name)
	}
	s.SetRoomTopic("!a", "nope", cb)
	if err := <-done; err == "" || r.topic != "a new topic" {
		t.Fatalf("topic refusal: %q %q", err, r.topic)
	}
}
