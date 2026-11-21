package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// PeopleGrid is the People view on desktops: everyone as a tile, with a
// small thought bubble on the avatar of those who set a status. The bubble
// holds their emoji (a chat glyph when they chose none); the whole status is
// its tooltip, and — with the setting for it — the line under the name. Online,
// then offline; those with a status first.
func PeopleGrid(c *ui.Context, st *PeopleState, people []Person, colors *UserColors, now time.Time) (a PeopleAction) {
	p := Of(c)
	online, offline, q := splitPeople(st, people, now)
	ui.Column(c).Gap(S4).Children(func() {
		peopleHeader(c, st)
		section := func(title string, list []Person) {
			if len(list) == 0 {
				return
			}
			ui.Text(c.Key("sec"+title), title+" — "+strconv.Itoa(len(list))).FontSize(SizeCaption).
				FontWeight(WeightSemibold).TextColor(p.TextMuted).Padding(0, S1)
			ui.Row(c.Key("grid" + title)).Wrap().Gap(S2).Children(func() {
				for _, ps := range list {
					if act := personTile(c, ps, st.Sub, colors, now); act.Open != "" || act.Profile != "" {
						a = act
					}
				}
			})
		}
		section("Online", online)
		if st.Filter == 0 {
			section("Offline", offline)
		}
		if len(online) == 0 && (st.Filter == 1 || len(offline) == 0) {
			emptyNote(c, map[bool]string{true: "No one matches “" + st.Query + "”", false: "No one is online right now"}[q != ""])
		}
	})
	return a
}

const (
	tileW       = 168
	tileAvatar  = 64
	bubbleSpace = 14 // room over the avatar for the bubble's top
)

// personTile is a person in the grid: their avatar with presence and, if
// they set a status, its bubble; their name; and a line under it (their
// status words, or how they are, or when you last chatted — the setting). A
// click opens their card; Message shows on hover.
func personTile(c *ui.Context, ps Person, mode int, colors *UserColors, now time.Time) (a PeopleAction) {
	p := Of(c)
	b := ui.ButtonBase(c.Key(ps.ID)).Width(tileW).Padding(S2, S2, S3, S2).Gap(S1).Column().AlignItems(ui.Center).
		Radius(RadiusLg).Label(ps.Name)
	hov := b.Hovered() || b.FocusWithin()
	bg := ui.Transparent
	if b.Pressed() {
		bg = p.Pressed
	} else if hov {
		bg = p.Hover
	}
	b.Background(b.AnimateColor("bg", bg, Color))
	hasStatus := ps.Status.Set(now)
	if hasStatus {
		b.Description(statusWords(ps.Status))
	}
	b.Children(func() {
		ui.Box(c).Margin(bubbleSpace, 0, 0, 0).Children(func() {
			av := ps.Avatar
			av.Size, av.Name, av.ID, av.Colors, av.Presence = tileAvatar, ps.Name, ps.ID, colors, ps.Presence
			Avatar(c, av)
			if hasStatus {
				// At the avatar's top-right, trail dropping into it. It is a
				// small, fixed-size mark: it takes no room, covers no
				// neighbour, and does not move.
				ui.Box(c.Key("float")).Attach(ui.AnchorTopRight, ui.AnchorBottomLeft).Left(-14).Top(30).
					AlignItems(ui.Start).PassThrough().Children(func() {
					ThoughtBubble(c, ps.Status)
				})
			}
		})
		ui.Text(c, ps.Name).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(p.Text).SingleLine().MaxWidth(tileW-S4).
			Margin(S1, 0, 0, 0)
		text, faint := personSub(ps, mode, now)
		col := p.TextMuted
		if faint {
			col = p.TextFaint
		}
		sub := ui.Text(c, text).FontSize(SizeCaption).TextColor(col).SingleLine().MaxWidth(tileW - S4)
		if hasStatus && mode == PeopleSubStatus && ps.Status.Text != "" {
			sub.Tooltip(statusWords(ps.Status)) // the whole status, where the line is cut
		}
		// Message, on hover, at the tile's top-left corner.
		if hov || Touch(c) {
			m := ui.ButtonBase(c.Key("msg")).Size(28, 28).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Top(S2).Left(S2).Background(p.Raised).Border(1, p.Border).
				Label("Message " + ps.Name).Tooltip("Message")
			m.Children(func() { ui.Icon(c, IconMessage).Size(14, 14).TextColor(p.Text) })
			if m.Clicked() {
				a.Open = ps.ID
			}
		}
	})
	if b.Clicked() && a.Open == "" {
		a.Profile = ps.ID
	}
	return a
}

// statusWords is a status as one string: "🎧 Focusing on the kit".
func statusWords(s Status) string {
	switch {
	case s.Emoji == "":
		return s.Text
	case s.Text == "":
		return s.Emoji
	}
	return s.Emoji + " " + s.Text
}

// ThoughtBubble is a status as a small comic thought bubble: a rounded
// cloud of the raised surface holding the emoji — a chat glyph when the
// status has none — trailed by three dots shrinking toward whoever thinks
// it, below and to the left. Static, and its tooltip is the whole status.
func ThoughtBubble(c *ui.Context, s Status) ui.Element {
	p := Of(c)
	var cloud ui.Element
	ui.Column(c.Key("thought")).AlignItems(ui.Start).Children(func() {
		cloud = ui.Row(c.Key("cloud")).Height(28).MinWidth(28).PaddingX(S2).Radius(RadiusFull).
			AlignItems(ui.Center).Justify(ui.Center).Background(p.Raised).Border(1, p.Border).
			Shadow(0, 4, 12, -4, p.Shadow).Tooltip(statusWords(s)).Label(statusWords(s))
		cloud.Children(func() {
			if s.Emoji != "" {
				ui.Text(c, s.Emoji).FontSize(15)
			} else {
				ui.Icon(c, IconMessage).Size(14, 14).TextColor(p.TextMuted)
			}
		})
		// The trail: three dots, each smaller, down toward the avatar.
		ui.Box(c.Key("trail")).Size(30, 18).Children(func() {
			for i, d := range []struct{ x, y, s float32 }{{18, 2, 7}, {12, 9, 5}, {8, 14, 3}} {
				ui.Box(c.Key(i)).Size(d.s, d.s).Radius(RadiusFull).Background(p.Raised).Border(1, p.Border).
					Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Left(d.x).Top(d.y)
			}
		})
	})
	return cloud
}
