package mock

import (
	"time"

	"slopement/model"
)

// seed fills the account: people, DMs and group chats, three communities,
// and a conversation in each room, with the worst cases mixed in (long
// names, RTL, huge counts).
func (s *Source) seed(now time.Time) {
	min := func(m int) time.Time { return now.Add(-time.Duration(m) * time.Minute) }
	s.me = model.Person{ID: "@me:matrix.org", Name: "Robin Ellis", Presence: model.Online, Trust: model.TrustVerified}

	for _, p := range []model.Person{
		{ID: "@alex:example.com", Name: "Alex Rivera", Presence: model.Online, Pronouns: "they/them", Trust: model.TrustVerified, Contact: true,
			Status: model.Status{Emoji: "🎧", Text: "Focusing on the kit", Until: now.Add(2 * time.Hour)}},
		{ID: "@mira:matrix.org", Name: "Mira", Presence: model.Online, Contact: true,
			Status: model.Status{Emoji: "🌴", Text: "In Lisbon, slow replies", Until: now.Add(72 * time.Hour)}},
		{ID: "@sam:matrix.org", Name: "Sam", Presence: model.Online, Contact: true,
			Status: model.Status{Emoji: "🍔", Text: "Lunch", Until: now.Add(25 * time.Minute)}},
		{ID: "@jun:kde.org", Name: "Jun", Presence: model.Away, Trust: model.TrustWarning, Contact: true},
		{ID: "@kai:example.org", Name: "Kai", LastSeen: min(60 * 26), Contact: true,
			Status: model.Status{Emoji: "✈️", Text: "Travelling this month"}},
		{ID: "@noor:example.com", Name: "Noor الحسيني", LastSeen: min(120), Contact: true},
		{ID: "@ola:matrix.org", Name: "Ola", LastSeen: min(12), Contact: true},
		{ID: "@lena:matrix.org", Name: "Lena", Presence: model.Online},
		{ID: "@theo:example.org", Name: "Theo"},
		{ID: "@barbara:example.com", Name: "Barbara Klein", Presence: model.Away},
	} {
		p := p
		s.people[p.ID] = &p
	}

	room := func(r model.Room) { rr := r; s.rooms[r.ID] = &rr }
	// Chats: DMs and small groups.
	room(model.Room{ID: "!dm-alex", Kind: model.KindDM, Name: "Alex Rivera", Partner: "@alex:example.com", Members: 2, Encrypted: true,
		Mentions: 1, Favourite: true, LastActivity: min(4), Preview: "sounds good, tomorrow 10?"})
	room(model.Room{ID: "!fam", Kind: model.KindGroup, Name: "Family 🏡", Members: 5, Encrypted: true, Unread: 3, Favourite: true,
		LastActivity: min(30), Preview: "Barbara Klein: dinner on Sunday?"})
	room(model.Room{ID: "!dm-mira", Kind: model.KindDM, Name: "Mira", Partner: "@mira:matrix.org", Members: 2, Encrypted: true,
		LastActivity: min(110), Preview: "You: the rail spacing is in"})
	room(model.Room{ID: "!trip", Kind: model.KindGroup, Name: "Lisbon trip", Members: 4, Encrypted: true,
		LastActivity: min(60 * 20), Preview: "Kai: flights booked ✈️"})
	room(model.Room{ID: "!dm-noor", Kind: model.KindDM, Name: "Noor الحسيني", Partner: "@noor:example.com", Members: 2, Encrypted: true,
		LastActivity: min(60 * 22), Preview: "مرحبا! كيف حالك؟"})
	room(model.Room{ID: "!dm-sam", Kind: model.KindDM, Name: "Sam", Partner: "@sam:matrix.org", Members: 2, Encrypted: true,
		Muted: true, LastActivity: min(60 * 70), Preview: "who's up for lunch"})

	// Communities and their rooms.
	type cr struct {
		id, name, topic, preview string
		unread, mentions, ago    int
		members                  int
		call                     bool
	}
	comm := func(c model.Community, rs []cr) {
		for _, r := range rs {
			room(model.Room{ID: r.id, Kind: model.KindRoom, Name: r.name, Topic: r.topic, Community: c.ID, Members: r.members,
				Encrypted: r.members < 200, Unread: r.unread, Mentions: r.mentions, LastActivity: min(r.ago), Preview: r.preview, Call: r.call})
			c.Rooms = append(c.Rooms, r.id)
		}
		s.comms = append(s.comms, c)
	}
	comm(model.Community{ID: "!design", Name: "Design Collective", About: "People who care about the details", Members: 412, Online: 38}, []cr{
		{"!announce", "announcements", "News from the collective", "Release 0.4 is out 🎉", 1200, 0, 60 * 30, 412, false},
		{"!crit", "design-crit", "Design reviews, every Tuesday", "Mira: pushed the new rail spacing", 4, 0, 2, 12, false},
		{"!general", "general", "Anything goes", "Sam: who's up for lunch", 0, 0, 90, 380, false},
		{"!sync", "weekly-sync", "Thursdays, 10:00", "", 0, 0, 300, 9, true},
		{"!showcase", "showcase", "Show what you made", "Lena: new poster series", 2, 0, 60 * 5, 300, false},
	})
	comm(model.Community{ID: "!matrixhq", Name: "Matrix HQ", About: "The Matrix protocol and its ecosystem", Members: 31200, Online: 1870}, []cr{
		{"!matrix", "matrix", "Talk about Matrix", "Theo: sliding sync is on", 120, 3, 20, 31200, false},
		{"!matrixdev", "matrix-dev", "Building on Matrix", "Jun: MSC4133 landed in 1.16", 9, 0, 45, 4200, false},
		{"!offtopic", "offtopic", "Everything else", "", 0, 0, 60 * 50, 2400, false},
	})
	comm(model.Community{ID: "!gophers", Name: "Gophers", About: "Go, the language", Members: 5600, Online: 240}, []cr{
		{"!gonuts", "go-nuts", "General Go", "Kai: generics, again", 1, 0, 60 * 3, 5600, false},
		{"!golangdev", "golang-dev", "Developing Go itself", "", 0, 0, 60 * 30, 900, false},
		{"!jobs", "jobs", "Go jobs", "", 0, 0, 60 * 90, 3100, false},
	})

	// Conversations.
	at := func(m int) time.Time { return min(m) }
	msg := func(room, sender string, ago int, body string) model.Event {
		return model.Event{ID: s.id("$e"), Room: room, Sender: sender, Time: at(ago), Body: body}
	}
	add := func(room string, evs ...model.Event) { s.timeline[room] = append(s.timeline[room], evs...) }

	alex, mira, sam, jun, kai, noor, ola := "@alex:example.com", "@mira:matrix.org", "@sam:matrix.org", "@jun:kde.org", "@kai:example.org", "@noor:example.com", "@ola:matrix.org"
	me := s.me.ID

	add("!dm-alex",
		msg("!dm-alex", alex, 12, "hey! are we still on for **tomorrow**?"),
		msg("!dm-alex", alex, 12, "I found the café ☕ https://maps.example.com/cafe"),
		msg("!dm-alex", me, 9, "yes!! 10:00 works"),
		msg("!dm-alex", me, 9, "I'll bring the ||birthday card|| 🎂"),
		msg("!dm-alex", alex, 6, "perfect. here's the plan:\n1. coffee\n2. walk\n3. *cake*"),
		msg("!dm-alex", alex, 5, "https://static.klipy.com/ii/50d7c955398dfd7e3c8ba5281154280f/db/47/F5tYiwPfaxKo.gif"),
		msg("!dm-alex", alex, 4, "sounds good, tomorrow 10?"),
	)
	s.timeline["!dm-alex"][2].Reactions = []model.Reaction{{Key: "❤️", Count: 1}}

	crit := []model.Event{
		{ID: s.id("$s"), Room: "!crit", Sender: mira, Time: at(40), Kind: model.EventSystem, Body: "Mira changed the topic to “Design reviews, every Tuesday”"},
		msg("!crit", mira, 9, "Pushed the new rail spacing. The indicator now grows from a pip to a **bar** when you switch spaces."),
		msg("!crit", mira, 9, "Also made unread counts `tabular-nums` so they stop jittering 🙂"),
		msg("!crit", alex, 7, "Looks great. Could the selected room use the *subtle* purple instead of the gray? Ending: ||it does||"),
		msg("!crit", jun, 6, "@Robin can you check the mute menu durations?\n- 15m / 2h / 1d / 1w\n- and ~~never~~ **forever**"),
		msg("!crit", jun, 6, "the very first video, for science https://youtu.be/jNQXAC9IVRw"),
		{ID: s.id("$u"), Room: "!crit", Sender: sam, Time: at(5), Kind: model.EventUndecryptable},
		msg("!crit", sam, 5, "https://static.klipy.com/ii/c3a19a0b747a76e98651f2b9a3cca5ff/8a/b1/uAaLRrFA.gif"),
		msg("!crit", noor, 4, "```go\nif err := client.Sync(ctx); err != nil {\n\treturn fmt.Errorf(\"sync: %w\", err)\n}\n```"),
		{ID: s.id("$p"), Room: "!crit", Sender: mira, Time: at(3), Kind: model.EventPoll, Poll: &model.Poll{
			Question: "Where should the offsite be?", Ends: now.Add(3*time.Hour + 20*time.Minute),
			Answers: []model.PollAnswer{
				{ID: "a", Text: "Lisbon", Voters: []string{mira, kai, noor, sam, ola}},
				{ID: "b", Text: "Berlin", Voters: []string{jun, alex}},
				{ID: "c", Text: "Somewhere with a lake", Voters: []string{"@barbara:example.com", "@lena:matrix.org", "@theo:example.org"}},
			}}},
		msg("!crit", mira, 2, "pushed the new rail spacing"),
	}
	crit[2].Reactions = []model.Reaction{{Key: "👍", Count: 3, Mine: true}, {Key: "🔥", Count: 1}}
	crit[4].Mentions = true
	add("!crit", crit...)

	add("!fam",
		msg("!fam", "@barbara:example.com", 60, "Who's coming on Sunday?"),
		msg("!fam", me, 50, "me! bringing dessert"),
		msg("!fam", "@barbara:example.com", 30, "dinner on Sunday?"),
	)
	add("!dm-mira",
		msg("!dm-mira", mira, 130, "did the rail land?"),
		msg("!dm-mira", me, 110, "the rail spacing is in"),
	)
	add("!trip",
		msg("!trip", kai, 60*21, "found a place near Alfama"),
		msg("!trip", kai, 60*20, "flights booked ✈️"),
	)
	add("!dm-noor", msg("!dm-noor", noor, 60*22, "مرحبا! كيف حالك؟"))
	add("!dm-sam", msg("!dm-sam", sam, 60*70, "who's up for lunch"))
	add("!general",
		model.Event{ID: s.id("$s"), Room: "!general", Sender: "@lena:matrix.org", Time: at(200), Kind: model.EventSystem, Body: "Lena joined"},
		msg("!general", sam, 90, "who's up for lunch"),
	)
	add("!announce", msg("!announce", mira, 60*30, "Release 0.4 is out 🎉"))
	add("!showcase", msg("!showcase", "@lena:matrix.org", 60*5, "new poster series"))
	add("!matrix", msg("!matrix", "@theo:example.org", 20, "sliding sync is on"))
	add("!matrixdev", msg("!matrixdev", jun, 45, "MSC4133 landed in 1.16"))
	add("!gonuts", msg("!gonuts", kai, 60*3, "generics, again"))

	s.sessions = []model.Session{
		{ID: "ABCDEFGH", Name: "slopement on macOS", LastSeen: now, IP: "203.0.113.4", Client: "slopement 0.1", Verified: true, Current: true},
		{ID: "PHONE123", Name: "Element X on iPhone", LastSeen: min(60 * 3), IP: "198.51.100.7", Client: "Element X iOS", Verified: true},
		{ID: "OLDLAPTP", Name: "Element Desktop", LastSeen: now.AddDate(0, -4, 0), IP: "192.0.2.1", Client: "Element Desktop 1.11"},
	}
}
