package gallery

import (
	"slopement/embeds"
	"slopement/format"
	"slopement/kit"
)

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

func md(s string) *format.Node { return format.Parse(s) }

func msgs(rv kit.Reveals) []kit.MessageData {
	ytID := "jNQXAC9IVRw"
	return []kit.MessageData{
		{ID: "e1", Sender: "Mira", SenderID: "@mira:matrix.org", Time: "13:58", First: true,
			Doc: md("Pushed the new rail spacing. The indicator now grows from a pip to a **bar** when you switch spaces.")},
		{ID: "e2", Sender: "Mira", SenderID: "@mira:matrix.org", Time: "13:58",
			Doc: md("Also made unread counts `tabular-nums` so they stop jittering 🙂"), Reactions: []kit.Reaction{{Key: "👍", Count: 3, Mine: true}, {Key: "🔥", Count: 1}}},
		{ID: "e2b", Sender: "Mira", SenderID: "@mira:matrix.org", Time: "13:59", Media: demoMedia(3), Reveals: rv},
		{ID: "e3", Sender: "Alex Rivera", SenderID: "@alex:example.com", Time: "14:00", First: true,
			Reply: &kit.Quote{Sender: "Mira", SenderID: "@mira:matrix.org", Body: "Pushed the new rail spacing. The indicator now grows…"},
			Doc:   md("Looks great. Could the selected room use the *subtle* purple instead of the gray? Ending: ||it does||"), Reveals: rv},
		{ID: "e4", Sender: "Jun", SenderID: "@jun:kde.org", Time: "14:01", First: true, Unverified: true,
			Doc: md("@You can you check the mute menu durations?\n- 15m / 2h / 1d / 1w\n- and ~~never~~ **forever**"), Highlight: true},
		{ID: "e4b", Sender: "Jun", SenderID: "@jun:kde.org", Time: "14:01",
			Doc: md("the very first video, for science https://youtu.be/" + ytID),
			YouTube: &kit.YouTubeSpec{ID: ytID, Title: "Me at the zoo", Channel: "jawed", Duration: "0:19", Thumb: embeds.YouTubeThumb(ytID)}},
		{ID: "e5", Sender: "Sam", SenderID: "@sam:matrix.org", Time: "14:02", First: true, Undecryptable: true},
		// A pasted GIF link: shown as the GIF, without the link.
		{ID: "e5b", Sender: "Sam", SenderID: "@sam:matrix.org", Time: "14:02",
			Doc: md("https://static.klipy.com/ii/c3a19a0b747a76e98651f2b9a3cca5ff/8a/b1/uAaLRrFA.gif")},
		{ID: "e6", Sender: "Noor الحسيني", SenderID: "@noor:example.com", Time: "14:02", First: true, Edited: true,
			Doc: md("```go\nif err := client.Sync(ctx); err != nil {\n\treturn fmt.Errorf(\"sync: %w\", err)\n}\n```\nSupercalifragilisticexpialidocious_and_an_unbreakable_url_https://example.com/a/very/long/path/that/does/not/wrap")},
		{ID: "e7", Sender: "You", SenderID: "@me:matrix.org", Time: "14:03", First: true, Pending: true,
			Doc: md("On it — sending now")},
	}
}

// bubbles is the phone conversation (a DM with Alex).
func bubbles() []kit.BubbleData {
	alex := "@alex:example.com"
	b := func(id string, out bool, t, text string) kit.BubbleData {
		d := kit.BubbleData{ID: id, Out: out, Time: t, Doc: md(text)}
		if !out {
			d.Sender, d.SenderID = "Alex Rivera", alex
		}
		return d
	}
	list := []kit.BubbleData{
		b("b1", false, "13:40", "hey! are we still on for **tomorrow**?"),
		b("b2", false, "13:41", "I found the café ☕ https://maps.example.com/cafe"),
		b("b3", true, "13:44", "yes!! 10:00 works"),
		b("b4", true, "13:44", "I'll bring the ||birthday card|| 🎂"),
		b("b5", false, "13:47", "perfect. here's the plan:\n1. coffee\n2. walk\n3. *cake*"),
		b("b5b", false, "13:47", "https://static.klipy.com/ii/50d7c955398dfd7e3c8ba5281154280f/db/47/F5tYiwPfaxKo.gif"),
	}
	pics := demoMedia(2)
	list = append(list, kit.BubbleData{ID: "b6", Sender: "Alex Rivera", SenderID: alex, Time: "13:48", Media: pics})
	r := b("b7", true, "13:50", "omg the view 😍")
	r.Reply = &kit.Quote{Sender: "Alex Rivera", SenderID: alex, Body: "perfect. here's the plan: coffee, walk, cake"}
	r.Reactions = []kit.Reaction{{Key: "❤️", Count: 1}}
	r.Sent = true
	list = append(list, r)
	// Runs: First on a change of sender, Last before one.
	for i := range list {
		list[i].First = i == 0 || list[i-1].Out != list[i].Out
		list[i].Last = i == len(list)-1 || list[i+1].Out != list[i].Out
	}
	return list
}

var sas = []kit.SASEmoji{
	{Emoji: "🐶", Name: "Dog"}, {Emoji: "🔑", Name: "Key"}, {Emoji: "🌵", Name: "Cactus"}, {Emoji: "🎸", Name: "Guitar"},
	{Emoji: "🚀", Name: "Rocket"}, {Emoji: "🍄", Name: "Mushroom"}, {Emoji: "⚓", Name: "Anchor"},
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
