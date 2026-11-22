package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/embeds"
	"slopement/format"
	"slopement/kit"
	"slopement/model"
)

// Conversions from the model to the kit's view data. The kit knows nothing
// of the model, and the model nothing of the kit.

func presence(p model.Presence) kit.Presence {
	switch p {
	case model.Online:
		return kit.PresenceOnline
	case model.Away:
		return kit.PresenceAway
	}
	return kit.PresenceNone
}

func trust(t model.Trust) kit.Trust {
	switch t {
	case model.TrustVerified:
		return kit.TrustVerified
	case model.TrustWarning:
		return kit.TrustWarning
	}
	return kit.TrustUnknown
}

func status(s model.Status) kit.Status {
	return kit.Status{Emoji: s.Emoji, Text: s.Text, Until: s.Until}
}

func (a *App) avatar(p model.Person, size float32) kit.AvatarSpec {
	return kit.AvatarSpec{Name: p.Name, ID: p.ID, Size: size, Presence: presence(p.Presence), Colors: &a.colors,
		Image: a.avatarPic(p.Avatar)}
}

func (a *App) kitPerson(p model.Person) kit.Person {
	return kit.Person{ID: p.ID, Name: p.Name, Avatar: a.avatar(p, 0), Presence: presence(p.Presence),
		Status: status(p.Status), LastSeen: p.LastSeen}
}

// contacts are the people listed in People: those with a DM.
func (a *App) contacts() []kit.Person {
	// When the user last chatted with each person: the last activity of
	// their DM.
	lastChat := map[string]time.Time{}
	for _, r := range a.src.Rooms() {
		if r.Kind == model.KindDM && r.Partner != "" && r.LastActivity.After(lastChat[r.Partner]) {
			lastChat[r.Partner] = r.LastActivity
		}
	}
	var out []kit.Person
	for _, p := range a.src.People() {
		if p.Contact {
			kp := a.kitPerson(p)
			kp.LastChat = lastChat[p.ID]
			out = append(out, kp)
		}
	}
	return out
}

func (a *App) profile(id string) *kit.ProfileSpec {
	p, _ := a.src.Person(id)
	return &kit.ProfileSpec{Name: p.Name, MXID: p.ID, Avatar: a.avatar(p, 0), Status: status(p.Status),
		Trust: trust(p.Trust), Pronouns: p.Pronouns, Self: p.ID == a.src.Me().ID,
		Recolor: true, ColorIndex: a.colorOf(p.ID)}
}

// fullTime is a moment in full, for a tooltip: "Sunday 7th October, 12:02",
// with the year when it is not this one ("… October 2024, 12:02").
func fullTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	day := t.Day()
	suffix := "th"
	if day%100 < 11 || day%100 > 13 {
		switch day % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	s := t.Format("Monday ") + strconv.Itoa(day) + suffix + t.Format(" January")
	if t.Year() != now.Year() {
		s += t.Format(" 2006")
	}
	return s + t.Format(", 15:04")
}

// timeLabel is a room row's time: 14:02, Yesterday, Mon, 12 Nov.
func timeLabel(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	y1, m1, d1 := t.Date()
	y2, m2, d2 := now.Date()
	switch {
	case y1 == y2 && m1 == m2 && d1 == d2:
		return t.Format("15:04")
	case now.Sub(t) < 48*time.Hour && now.AddDate(0, 0, -1).Day() == d1:
		return "Yesterday"
	case now.Sub(t) < 6*24*time.Hour:
		return t.Format("Mon")
	}
	return t.Format("2 Jan")
}

// dayLabel is a date separator's: Today, Yesterday, Monday 17 November.
func dayLabel(t, now time.Time) string {
	switch timeLabel(t, now) {
	case t.Format("15:04"):
		return "Today"
	case "Yesterday":
		return "Yesterday"
	}
	return t.Format("Monday 2 January")
}

// row is a room as a row of a list: a DM shows its partner's presence and
// status, a group chat a squared avatar.
func (a *App) row(r model.Room) kit.RoomRowData {
	d := kit.RoomRowData{ID: r.ID, Name: r.Name, Preview: r.Preview, Time: timeLabel(r.LastActivity, a.now),
		Unread: r.Unread, Mention: r.Mentions, Muted: r.Muted, Fav: r.Favourite, Call: r.Call, Encrypted: r.Encrypted,
		Avatar: kit.AvatarSpec{Name: r.Name, ID: r.ID, Colors: &a.colors}}
	switch r.Kind {
	case model.KindDM:
		if p, ok := a.src.Person(r.Partner); ok {
			d.Avatar.ID, d.Avatar.Presence = p.ID, presence(p.Presence)
			d.Avatar.Image = a.avatarPic(p.Avatar)
			if p.Status.Set(a.now) {
				d.Status = status(p.Status)
			}
		}
	case model.KindGroup:
		d.Avatar.Square = true
	}
	if r.Kind != model.KindDM {
		d.Avatar.Image = a.avatarPic(r.Avatar)
	}
	if len(r.Typing) > 0 {
		d.Typing = r.Typing[0] + " is typing…"
	}
	return d
}

func (a *App) menuState(r model.Room) kit.RoomMenuState {
	return kit.RoomMenuState{Muted: r.Muted, MutedUntil: r.MutedUntil, Favourite: r.Favourite, LowPrio: r.LowPriority,
		Unread: r.Unread > 0 || r.Mentions > 0}
}

// pollData is a poll as the kit's card shows it.
func (a *App) pollData(id string, p model.Poll, mine bool) *kit.PollData {
	me := a.src.Me().ID
	pd := &kit.PollData{ID: id, Question: p.Question, Multiple: p.Multiple, Hidden: p.Hidden, Ends: p.Ends, Ended: p.Ended,
		Mine: mine, Colors: &a.colors}
	for _, an := range p.Answers {
		ka := kit.PollAnswer{Text: an.Text, Votes: len(an.Voters)}
		for _, v := range an.Voters {
			if v == me {
				ka.Mine = true
				continue
			}
			p, _ := a.src.Person(v)
			ka.Voters = append(ka.Voters, kit.PollVoter{ID: v, Name: p.Name})
		}
		pd.Answers = append(pd.Answers, ka)
	}
	return pd
}

// doc parses an event's Markdown once (and again only if it changes).
// doc is what a message says, as the tree the timeline draws. A message
// that came with HTML is drawn from it — the formatting its sender wrote, a
// list's bullets and all, whichever client wrote it; one that came without is
// plain text, shown as typed (its links still links); only an event that says
// neither (the mock's) is Markdown.
func (a *App) doc(ev model.Event) *format.Node {
	key := ev.Body
	if ev.Formatted != "" {
		key = "html:" + ev.Formatted
	} else if ev.Plain {
		key = "plain:" + ev.Body
	}
	if d, ok := a.docs[ev.ID]; ok && d.body == key {
		return d.doc
	}
	var n *format.Node
	switch {
	case ev.Formatted != "":
		n = format.ParseHTML(ev.Formatted)
	case ev.Plain:
		n = format.ParsePlain(ev.Body)
	default:
		n = format.Parse(ev.Body)
	}
	a.docs[ev.ID] = parsed{key, n}
	return n
}

type parsed struct {
	body string
	doc  *format.Node
}

// fileSpec is the card of a message that is a file (a document, audio), or
// nil.
func (a *App) fileSpec(ev model.Event) *kit.FileSpec {
	for _, m := range ev.Media {
		if m.Kind == model.MediaFile {
			return &kit.FileSpec{Name: m.Name, Size: sizeLabel(int(m.Size)), Progress: -1}
		}
	}
	return nil
}

// content fills a message's body, media and embeds: a message that is
// only a GIF link shows as the GIF; a YouTube link gets its card.
func (a *App) content(ev model.Event) (doc *format.Node, media []kit.MediaSpec, yt *kit.YouTubeSpec) {
	doc = a.doc(ev)
	if ev.Kind == model.EventMedia && len(ev.Media) > 0 && strings.TrimSpace(ev.Body) == ev.Media[0].Name {
		doc = nil // the body of a picture or file is its file name: the card or picture says it
	}
	for i, m := range ev.Media {
		if m.Kind == model.MediaFile {
			continue // a file card (fileSpec), not a picture
		}
		spec := kit.MediaSpec{ID: ev.ID + ":" + strconv.Itoa(i), Kind: kit.MediaImage, W: float32(m.W), H: float32(m.H),
			Progress: -1, Label: m.Name, Spoiler: m.Spoiler}
		if isGIFFile(m) {
			// A GIF sent as a file: it plays, looping and muted, as the
			// picker's does — and, with "Animate GIFs" off, holds its first
			// frame.
			spec.Kind, spec.Paused = kit.MediaGIF, !a.prefs.gifAutoplay
			spec.Player = a.gifPlayer(m)
			if spec.Player != nil {
				if w, h := spec.Player.Size(); w > 0 && h > 0 {
					spec.W, spec.H = float32(w), float32(h)
				}
			}
		}
		spec.Still = a.still(m)
		if m.Kind == model.MediaImage && !isGIFFile(m) {
			mm := m
			spec.Full = func() ui.ImageSource { return a.fullPic(mm) }
		}
		media = append(media, spec)
	}
	if doc == nil {
		return nil, media, nil // nothing written beside the picture or file: no links to preview
	}
	if e, ok := embeds.SoleMedia(doc); ok {
		gif := embeds.IsGIF(e.MediaURL)
		// With a kind of preview off, the link stays a link.
		if gif && !a.prefs.gifPreviews || e.Kind == embeds.Video && !a.prefs.videoPreviews {
			return doc, media, nil
		}
		m := kit.MediaSpec{ID: ev.ID + ":media", Kind: kit.MediaImage, W: 4, H: 3, Progress: -1, Label: "GIF"}
		if gif {
			// A placeholder until the GIF loads; then its own size and
			// shape, never cropped. Paused honors "Animate GIFs": the
			// first frame shows, nothing plays.
			m.Kind, m.W, m.H = kit.MediaGIF, 240, 180
			m.Paused = !a.prefs.gifAutoplay
			if a.pick != nil {
				url := e.MediaURL
				m.Player = a.pick.Animation(url)
				if m.Player != nil {
					if w, h := m.Player.Size(); w > 0 && h > 0 {
						m.W, m.H = float32(w), float32(h)
					}
				}
				// The star adds it to the picker's favorite GIFs.
				w, h := int(m.W), int(m.H)
				m.Starred = a.pick.FavoriteURL(url)
				m.Star = func(on bool) {
					a.pick.SetFavoriteURL(url, w, h, on)
					if on {
						a.toast = "Added to your favorite GIFs"
					} else {
						a.toast = "Removed from your favorite GIFs"
					}
				}
			}
		}
		return nil, append([]kit.MediaSpec{m}, media...), nil
	}
	if a.prefs.linkPreviews {
		for _, l := range embeds.Links(doc, 3) {
			if e := embeds.Classify(l); e.Kind == embeds.YouTube {
				yt = &kit.YouTubeSpec{ID: e.VideoID, Title: "YouTube video", Channel: "youtube.com", Thumb: embeds.YouTubeThumb(e.VideoID)}
				break
			}
		}
	}
	return doc, media, yt
}

func reactions(rs []model.Reaction) []kit.Reaction {
	out := make([]kit.Reaction, len(rs))
	for i, r := range rs {
		out[i] = kit.Reaction{Key: r.Key, Count: r.Count, Mine: r.Mine}
	}
	return out
}

// quote is the event replied to, as the reply's quote.
func (a *App) quote(room, id string) *kit.Quote {
	if id == "" {
		return nil
	}
	for _, ev := range a.src.Timeline(room) {
		if ev.ID == id {
			p, _ := a.src.Person(ev.Sender)
			return &kit.Quote{Sender: p.Name, SenderID: p.ID, Body: format.Plain(a.doc(ev)), ID: id, Avatar: a.avatar(p, 16)}
		}
	}
	return nil
}
