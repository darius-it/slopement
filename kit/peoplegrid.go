package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// PeopleGrid is the People view on desktops: everyone as a tile, the
// status they set floating over their avatar in a comic thought bubble,
// trailed by three shrinking dots down to them. At rest a bubble holds the
// emoji alone; over the tile it opens to the whole status. Online, then
// offline; those with a status first.
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
					if act := personTile(c, ps, colors, now); act.Open != "" || act.Profile != "" {
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
	bubbleSpace = 14 // room over the avatar for the overlaid bubble's top
)

// personTile is a person in the grid: their thought bubble (if they set a
// status), avatar with presence, name, and how they are (Online, Away,
// last seen). A click opens their card; Message shows on hover.
func personTile(c *ui.Context, ps Person, colors *UserColors, now time.Time) (a PeopleAction) {
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
	if ps.Status.Set(now) {
		b.Description(ps.Status.Emoji + " " + ps.Status.Text)
	}
	b.Children(func() {
		// The avatar, the bubble overlaid on its top-right corner and off to
		// the side, its trail dropping into the avatar: only a little room
		// above for the cloud's top. It opens to the right, past the tile,
		// over its neighbours.
		ui.Box(c).Margin(bubbleSpace, 0, 0, 0).ZIndex(2).Children(func() {
			av := ps.Avatar
			av.Size, av.Name, av.ID, av.Colors, av.Presence = tileAvatar, ps.Name, ps.ID, colors, ps.Presence
			Avatar(c, av)
			if ps.Status.Set(now) {
				ui.Box(c.Key("float")).Attach(ui.AnchorTopRight, ui.AnchorBottomLeft).Left(-14).Top(30).Width(290).
					AlignItems(ui.Start).PassThrough().Children(func() {
					ThoughtBubble(c, ps.Status, now, hov)
				})
			}
		})
		ui.Text(c, ps.Name).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(p.Text).SingleLine().MaxWidth(tileW-S4).
			Margin(S1, 0, 0, 0)
		how, col := "Online", p.TextMuted
		switch {
		case ps.Presence == PresenceAway:
			how = "Away"
		case ps.Presence == PresenceNone:
			how, col = lastSeenText(ps.LastSeen, now), p.TextFaint
		}
		ui.Text(c, how).FontSize(SizeCaption).TextColor(col).SingleLine()
		// Message, on hover, at the tile's top-left corner, clear of the
		// bubble, which opens to the right.
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

// ThoughtBubble is a status as a comic thought bubble: a rounded cloud of
// the raised surface holding the emoji (and, open, the text), trailed by
// three dots shrinking toward whoever thinks it, below and to the left.
// It opens with a quick spring, growing from its trail.
func ThoughtBubble(c *ui.Context, s Status, now time.Time, open bool) ui.Element {
	p := Of(c)
	var cloud ui.Element
	wrap := ui.Column(c.Key("thought")).AlignItems(ui.Start)
	wrap.Children(func() {
		cloud = ui.Row(c.Key("cloud")).Height(30).PaddingX(S3).Gap(S15).Radius(RadiusFull).AlignItems(ui.Center).
			Background(p.Raised).Border(1, p.Border).Shadow(0, 4, 12, -4, p.Shadow).MaxWidth(280).
			TransformOrigin(0, 1).
			Transition(ui.ElementTransition{Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.12}},
				Width: true})
		cloud.Children(func() {
			ui.Text(c, s.Emoji).FontSize(15)
			if open && s.Text != "" {
				ui.Text(c.Key("txt"), s.Text).FontSize(SizeSmall).TextColor(p.Text).SingleLine().Shrink(1).MinWidth(0).
					Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{X: -4}})
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
