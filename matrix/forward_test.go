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
)

func TestForwardContent(t *testing.T) {
	reply := map[string]any{
		"msgtype": "m.text", "body": "> <@a:x> quoted\n\nmy answer",
		"format": "org.matrix.custom.html", "formatted_body": "<mx-reply><blockquote>quoted</blockquote></mx-reply>my answer",
		"m.relates_to": map[string]any{"m.in_reply_to": map[string]any{"event_id": "$q"}},
		"m.mentions":   map[string]any{"user_ids": []any{"@a:x"}},
	}
	got := forwardContent(reply)
	if got["body"] != "my answer" || got["formatted_body"] != "my answer" {
		t.Fatalf("the reply's fallback stayed: %v", got)
	}
	for _, k := range []string{"m.relates_to", "m.mentions", "m.new_content"} {
		if _, ok := got[k]; ok {
			t.Fatalf("%s was forwarded", k)
		}
	}

	// A message that merely starts with "> " (a quote the user wrote) keeps it.
	quote := forwardContent(map[string]any{"msgtype": "m.text", "body": "> a quote I wrote"})
	if quote["body"] != "> a quote I wrote" {
		t.Fatalf("a typed quote was cut: %v", quote)
	}
	// An attachment keeps its file.
	img := forwardContent(map[string]any{"msgtype": "m.image", "body": "cat.png", "url": "mxc://x/cat",
		"info": map[string]any{"mimetype": "image/png"}})
	if img["url"] != "mxc://x/cat" {
		t.Fatalf("the attachment's url was lost: %v", img)
	}
	if forwardContent(nil) != nil || forwardContent(map[string]any{}) != nil {
		t.Fatal("an empty content forwarded")
	}
}

// Forward reads the message from the server and sends it, without its reply,
// to the other room.
func TestForwardSendsACopy(t *testing.T) {
	var mu sync.Mutex
	var sentPath string
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/event/"):
			_, _ = w.Write([]byte(`{"event_id":"$m","type":"m.room.message","sender":"@ada:example.org","origin_server_ts":1,
			 "room_id":"!a","content":{"msgtype":"m.text","body":"hello there",
			 "m.relates_to":{"m.in_reply_to":{"event_id":"$q"}}}}`))
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/send/"):
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			sentPath = r.URL.Path
			_ = json.Unmarshal(b, &sent)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"event_id":"$new"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	s.roomOf("!a")
	s.roomOf("!b")

	var gotErr string
	finished := make(chan struct{})
	s.Forward("!a", "$m", "!b", func(err string) { gotErr = err; close(finished) })
	waitFor(t, "the forward", func() bool {
		select {
		case <-finished:
			return true
		default:
			return false
		}
	})
	if gotErr != "" {
		t.Fatalf("forward failed: %s", gotErr)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(sentPath, "/rooms/!b/send/m.room.message/") && !strings.Contains(sentPath, "%21b/send/m.room.message/") {
		t.Fatalf("sent to %q", sentPath)
	}
	if sent["body"] != "hello there" {
		t.Fatalf("content = %v", sent)
	}
	if _, ok := sent["m.relates_to"]; ok {
		t.Fatal("the reply came along")
	}

	// To a room that is not held: refused at once.
	var why string
	s.Forward("!a", "$m", "!nowhere", func(err string) { why = err })
	if why == "" {
		t.Fatal("forwarded to a room the user is not in")
	}
}
