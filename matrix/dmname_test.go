package matrix

import (
	"testing"

	"slopement/model"
)

// A chat with oneself is "Notes to self", not a nameless "?"; a chat whose
// partner has no profile yet shows their username.
func TestSelfChatAndUnknownPartnerHaveNames(t *testing.T) {
	s, _ := testSynced()
	s.me.ID = "@me:example.org"
	r := s.roomOf("!self")
	r.dm, r.partner = true, "@me:example.org" // no name at all, as a chat made with oneself
	if got, _ := s.Room("!self"); got.Name != selfChatName {
		t.Fatalf("self chat is %q", got.Name)
	}
	u := s.roomOf("!new")
	u.dm, u.partner = true, "@ada:example.org"
	if got, _ := s.Room("!new"); got.Name != "ada" {
		t.Fatalf("a partner with no profile is %q, want their username", got.Name)
	}
	r.name, r.nameSet = "My notes", true
	if got, _ := s.Room("!self"); got.Name != "My notes" {
		t.Fatalf("a chosen name was replaced: %q", got.Name)
	}
}

// A chat with one person is called by their display name as it is now. It was
// stored with the name known when the chat was first seen — the username, when
// the profile had not arrived — and stayed "kf" though the person is KaiFireborn.
func TestDMIsNamedByThePartnersDisplayName(t *testing.T) {
	s, _ := testSynced()
	r := s.roomOf("!dm")
	r.dm, r.partner, r.name = true, "@kf:example.org", "kf" // first seen: the profile was not known

	if got, _ := s.Room("!dm"); got.Name != "kf" {
		t.Fatalf("before the profile: %q (the username is all there is)", got.Name)
	}
	// The profile arrives.
	s.people["@kf:example.org"] = &model.Person{ID: "@kf:example.org", Name: "KaiFireborn"}
	if got, _ := s.Room("!dm"); got.Name != "KaiFireborn" {
		t.Fatalf("Room: the DM is %q, want the display name", got.Name)
	}
	var listed string
	for _, rm := range s.Rooms() {
		if rm.ID == "!dm" {
			listed = rm.Name
		}
	}
	if listed != "KaiFireborn" {
		t.Fatalf("Rooms: the DM is %q, want the display name", listed)
	}
	// And follows a change of the person's name.
	s.people["@kf:example.org"].Name = "Kai"
	if got, _ := s.Room("!dm"); got.Name != "Kai" {
		t.Fatalf("a changed display name was not followed: %q", got.Name)
	}

	// A name somebody gave the chat is kept; so is a group's.
	r.name, r.nameSet = "Our corner", true
	if got, _ := s.Room("!dm"); got.Name != "Our corner" {
		t.Fatalf("a chosen name was replaced: %q", got.Name)
	}
	g := s.roomOf("!grp")
	g.name = "Plans"
	if got, _ := s.Room("!grp"); got.Name != "Plans" {
		t.Fatalf("a group's name changed: %q", got.Name)
	}
}

// The flag is kept in the cache.
func TestRoomNameSetSurvivesTheCache(t *testing.T) {
	r := &room{id: "!a", name: "Named", nameSet: true}
	if got := r.record().room(); !got.nameSet || got.name != "Named" {
		t.Fatalf("after a restart: %+v", got)
	}
}
