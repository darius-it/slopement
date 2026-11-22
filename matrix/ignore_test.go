package matrix

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// Ignoring is the account's m.ignored_user_list, changed without losing what
// another session put in it; the ones ignored vanish from timelines, from
// search and from notifications, and come back when un-ignored.
func TestIgnoringPeople(t *testing.T) {
	fake := &actionServer{accData: map[string]string{ignoredData: `{"ignored_users":{"@old:example.org":{}}}`}}
	s, r := actionSetup(t, fake)
	r.appendEvent(model.Event{ID: "$2", Sender: "@noisy:example.org", Kind: model.EventText, Body: "spam about lunch", Time: at(2)}, "noisy", false)

	done := make(chan string, 2)
	s.SetIgnored("@noisy:example.org", true, func(err string) { done <- err })
	if !s.IsIgnored("@noisy:example.org") {
		t.Fatal("not ignored at once")
	}
	if err := <-done; err != "" {
		t.Fatal(err)
	}
	b, ok := fake.has("PUT", "/account_data/m.ignored_user_list")
	if !ok || !strings.Contains(b, "@noisy:example.org") || !strings.Contains(b, "@old:example.org") {
		t.Fatalf("the list written must hold the old entry too: %q", b)
	}
	for _, e := range s.Timeline("!a") {
		if e.Sender == "@noisy:example.org" {
			t.Fatal("an ignored person's message is still in the timeline")
		}
	}
	if len(s.localHits(model.ParseSearch("lunch"))) != 0 {
		t.Fatal("an ignored person's message is still found")
	}
	var told []model.Notice
	s.OnNotify(func(n model.Notice) { told = append(told, n) })
	dm := s.roomOf("!dm")
	dm.dm = true
	s.onMessage(context.Background(), liveMessage("!dm", "$n", "@noisy:example.org", "hello", 0, nil))
	if len(told) != 0 {
		t.Fatal("an ignored person notified")
	}

	s.SetIgnored("@noisy:example.org", false, func(err string) { done <- err })
	<-done
	if s.IsIgnored("@noisy:example.org") {
		t.Fatal("still ignored")
	}
	found := false
	for _, e := range s.Timeline("!a") {
		found = found || e.Sender == "@noisy:example.org"
	}
	if !found {
		t.Fatal("the message did not come back")
	}

	// You cannot ignore yourself.
	s.SetIgnored("@me:example.org", true, func(err string) { done <- err })
	if err := <-done; err == "" {
		t.Fatal("ignored oneself")
	}

	// The account's own list replaces what is held.
	evt := &event.Event{Content: event.Content{VeryRaw: json.RawMessage(`{"ignored_users":{"@a:x":{},"@b:x":{}}}`)}}
	s.onIgnored(nil, evt)
	if got := s.IgnoredUsers(); len(got) != 2 || got[0] != "@a:x" || got[1] != "@b:x" {
		t.Fatalf("ignored users: %v", got)
	}
}
