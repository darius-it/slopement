package gallery

import "slopement/kit"

// Demo data for the gallery: fake, but shaped like Matrix's, with the
// worst cases mixed in (long names, huge counts, emoji, RTL).

var spaces = []kit.RailItemData{
	{ID: "!home", Name: "Home", Icon: kit.IconHome},
	{ID: "!design", Name: "Design Collective", Unread: true},
	{ID: "!matrix", Name: "Matrix HQ", Mention: 3},
	{ID: "!go", Name: "Gophers", Unread: true},
	{ID: "!fam", Name: "Family 🏡"},
	{ID: "!long", Name: "An Extremely Long Space Name That Never Ends"},
}

var rooms = []kit.RoomRowData{
	{ID: "!r1", Name: "design-crit", Preview: "Mira: pushed the new rail spacing, have a look", Time: "14:02", Unread: 4, Encrypted: true},
	{ID: "!r2", Name: "Alex Rivera", Avatar: kit.AvatarSpec{Presence: kit.PresenceOnline}, Preview: "sounds good, tomorrow 10?", Time: "13:48", Mention: 1, Encrypted: true, Fav: true},
	{ID: "!r3", Name: "general", Preview: "Sam: who's up for lunch", Time: "12:30", Muted: true, Unread: 128},
	{ID: "!r4", Name: "Weekly sync", Preview: "", Time: "11:05", Call: true, Typing: "Jun is typing…"},
	{ID: "!r5", Name: "Noor الحسيني", Avatar: kit.AvatarSpec{Presence: kit.PresenceAway}, Preview: "مرحبا! كيف حالك؟", Time: "Yesterday", Encrypted: true},
	{ID: "!r6", Name: "announcements", Preview: "Release 0.4 is out 🎉", Time: "Mon", Unread: 1200, Mention: 0},
	{ID: "!r7", Name: "a-very-long-room-name-that-keeps-going-and-going-forever", Preview: "This preview is also far too long to fit on one line, so it ends with an ellipsis", Time: "Sun"},
	{ID: "!r8", Name: "B", Preview: "k", Time: "12 Nov", Muted: true},
}

var people = []string{"@mira:matrix.org", "@alex:example.com", "@sam:matrix.org", "@jun:kde.org", "@noor:example.com", "@me:matrix.org"}

var names = map[string]string{
	"@mira:matrix.org":  "Mira",
	"@alex:example.com": "Alex Rivera",
	"@sam:matrix.org":   "Sam",
	"@jun:kde.org":      "Jun",
	"@noor:example.com": "Noor الحسيني",
	"@me:matrix.org":    "You",
}

func msgs() []kit.MessageData {
	return []kit.MessageData{
		{ID: "e1", Sender: "Mira", SenderID: "@mira:matrix.org", Time: "13:58", First: true,
			Body: "Pushed the new rail spacing. The indicator now grows from a pip to a bar when you switch spaces."},
		{ID: "e2", Sender: "Mira", SenderID: "@mira:matrix.org", Time: "13:58",
			Body: "Also made unread counts tabular so they stop jittering 🙂", Reactions: []kit.Reaction{{Key: "👍", Count: 3, Mine: true}, {Key: "🔥", Count: 1}}},
		{ID: "e3", Sender: "Alex Rivera", SenderID: "@alex:example.com", Time: "14:00", First: true,
			Reply: &kit.Quote{Sender: "Mira", SenderID: "@mira:matrix.org", Body: "Pushed the new rail spacing. The indicator now grows…"},
			Body:  "Looks great. Could the selected room use the subtle purple instead of the gray?"},
		{ID: "e4", Sender: "Jun", SenderID: "@jun:kde.org", Time: "14:01", First: true, Unverified: true,
			Body: "@You can you check the mute menu durations? 15m / 2h / 1d / 1w / forever", Highlight: true},
		{ID: "e5", Sender: "Sam", SenderID: "@sam:matrix.org", Time: "14:02", First: true, Undecryptable: true},
		{ID: "e6", Sender: "Noor الحسيني", SenderID: "@noor:example.com", Time: "14:02", First: true, Edited: true,
			Body: "Supercalifragilisticexpialidocious_and_an_unbreakable_url_https://example.com/a/very/long/path/that/does/not/wrap/nicely/at/all"},
		{ID: "e7", Sender: "You", SenderID: "@me:matrix.org", Time: "14:03", First: true, Pending: true,
			Body: "On it — sending now"},
	}
}

var sas = []kit.SASEmoji{
	{Emoji: "🐶", Name: "Dog"}, {Emoji: "🔑", Name: "Key"}, {Emoji: "🌵", Name: "Cactus"}, {Emoji: "🎸", Name: "Guitar"},
	{Emoji: "🚀", Name: "Rocket"}, {Emoji: "🍄", Name: "Mushroom"}, {Emoji: "⚓", Name: "Anchor"},
}

var gifs = []kit.GIFTile{
	{ID: "g:trending", Label: "Trending", Category: true},
	{ID: "g:hello", Label: "hello", Category: true},
	{ID: "g:lol", Label: "lol", Category: true},
	{ID: "g:thanks", Label: "thank you", Category: true},
	{ID: "g:yes", Label: "yes", Category: true},
	{ID: "g:no", Label: "no", Category: true},
	{ID: "g:1", Label: "GIF 1"}, {ID: "g:2", Label: "GIF 2"}, {ID: "g:3", Label: "GIF 3"}, {ID: "g:4", Label: "GIF 4"},
}

func switcherItems() []kit.SwitcherItem {
	items := []kit.SwitcherItem{}
	for i, r := range rooms {
		kind := kit.KindRoom
		sub := "Design Collective"
		alias := "#" + r.Name + ":matrix.org"
		if r.Avatar.Presence != kit.PresenceNone {
			kind, sub, alias = kit.KindPerson, "@"+r.Name, "@"+r.Name
		}
		items = append(items, kit.SwitcherItem{ID: r.ID, Name: r.Name, Sub: sub, Alias: alias, Kind: kind,
			Unread: r.Unread, Mention: r.Mention, Recent: i})
	}
	for i, s := range spaces[1:] {
		items = append(items, kit.SwitcherItem{ID: s.ID, Name: s.Name, Kind: kit.KindSpace, Recent: 20 + i})
	}
	for i, id := range people[:4] {
		items = append(items, kit.SwitcherItem{ID: "dm" + id, Name: names[id], Sub: id, Alias: id, Kind: kit.KindPerson, Recent: 30 + i})
	}
	return items
}
