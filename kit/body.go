package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/format"
)

// Reveals holds which spoilers the user has clicked open, by key. The app
// keeps one for the session: spoilers close again on restart.
type Reveals map[string]bool

// BodyStyle is how a body renders: in the timeline or in a bubble.
type BodyStyle struct {
	Size  float32
	Color ui.Color // the text's color; zero is the theme's
	// OnAccent is set inside an outgoing bubble on the accent, which
	// recolors links, code and quotes to read on it.
	OnAccent bool
	// OnClick, when set, runs on a plain click (a press and release in one
	// place) on the text — a drag still selects it. For bodies shown in a
	// card that goes somewhere (a pinned message): selectable text takes
	// the press, so the card cannot see the click itself.
	OnClick func()
	// Shared leaves the selection to a Selectable container around the
	// body (the timeline's list), so one drag can cross messages as it does
	// on the web; without it the body is one selection of its own.
	Shared bool
	// Menu is the menu of a right click on the body's text. Selectable text
	// has a menu of its own (Copy, Select All) that, being innermost, would
	// hide the message's: this puts the message's there, with the editing
	// items (m.EditItems) below it.
	Menu func(m *ui.Menu)
}

// MessageBody renders a message's formatted tree. key identifies the
// message, for its spoilers' state. One selection spans the whole message:
// a drag crosses its paragraphs, list items and code, and Copy takes what
// is selected.
func MessageBody(c *ui.Context, key string, doc *format.Node, rv Reveals, s BodyStyle) {
	if doc == nil {
		return
	}
	if s.Size == 0 {
		s.Size = SizeBody
	}
	b := &bodyBuilder{c: c, key: key, rv: rv, s: s, p: Of(c)}
	col := ui.Column(c).Gap(S15).MinWidth(0)
	if !s.Shared {
		col.Selectable()
	}
	col.Children(func() {
		for i, n := range doc.Children {
			b.block(n, strconv.Itoa(i))
		}
	})
}

type bodyBuilder struct {
	c   *ui.Context
	key string
	rv  Reveals
	s   BodyStyle
	p   *Palette
	// para is the paragraph being built, over which a revealed spoiler's
	// dust draws, and paraID its identity.
	para   ui.Element
	paraID string
}

func (b *bodyBuilder) textColor() ui.Color {
	if b.s.Color.A > 0 {
		return b.s.Color
	}
	return b.p.Text
}

func (b *bodyBuilder) muted() ui.Color {
	if b.s.OnAccent {
		return b.textColor().Alpha(0.72)
	}
	return b.p.TextMuted
}

func (b *bodyBuilder) block(n *format.Node, path string) {
	c, p := b.c, b.p
	switch n.Kind {
	case format.Paragraph:
		b.paragraph(n.Children, b.s.Size, WeightRegular, path)
	case format.Heading:
		size := map[int]float32{1: SizeHeading, 2: SizeTitle, 3: SizeLabel + 1}[n.Level]
		b.paragraph(n.Children, size, WeightSemibold, path)
	case format.Quote:
		ui.Row(c.Key(path)).Gap(S2).AlignItems(ui.Stretch).Children(func() {
			bar := p.BorderStrong
			if b.s.OnAccent {
				bar = b.textColor().Alpha(0.4)
			}
			ui.Box(c).Width(3).Radius(2).Background(bar).Shrink(0)
			ui.Column(c).Gap(S1).Grow(1).MinWidth(0).Children(func() {
				for i, ch := range n.Children {
					b.block(ch, path+"."+strconv.Itoa(i))
				}
			})
		})
	case format.CodeBlock:
		b.codeBlock(n, path)
	case format.List:
		ui.Column(c.Key(path)).Gap(S05).Children(func() {
			for i, item := range n.Children {
				marker := "•"
				if n.Ordered {
					marker = strconv.Itoa(n.Start+i) + "."
				}
				ui.Row(c.Key(i)).Gap(S15).AlignItems(ui.Start).Children(func() {
					ui.Text(c, marker).FontSize(b.s.Size).FontFeatures("tnum").TextColor(b.muted()).
						MinWidth(14).TextAlign(ui.End).Shrink(0).Unselectable()
					ui.Column(c).Gap(S05).Grow(1).MinWidth(0).Children(func() {
						for j, ch := range item.Children {
							b.block(ch, path+"."+strconv.Itoa(i)+"."+strconv.Itoa(j))
						}
					})
				})
			}
		})
	default:
		b.paragraph([]*format.Node{n}, b.s.Size, WeightRegular, path)
	}
}

func (b *bodyBuilder) paragraph(inl []*format.Node, size float32, weight int, path string) {
	// Selectable comes from the body's container, so the selection is one.
	para := ui.RichText(b.c.Key(path)).FontSize(size).FontWeight(weight).LineHeight(1.45).TextColor(b.textColor())
	if click := b.s.OnClick; click != nil && para.Clicked() {
		click()
	}
	if b.s.Menu != nil {
		ContextMenu(b.c, para, b.s.Menu)
	}
	prev, prevID := b.para, b.paraID
	b.para, b.paraID = para, b.key+"|"+path
	para.Children(func() {
		for i, n := range inl {
			b.inline(n, path+":"+strconv.Itoa(i))
		}
	})
	playDust(para, b.paraID) // revealed spoilers' dust, over the text
	b.para, b.paraID = prev, prevID
}

func (b *bodyBuilder) inline(n *format.Node, path string) {
	c, p := b.c, b.p
	kids := func(e ui.Element) {
		e.Children(func() {
			for i, ch := range n.Children {
				b.inline(ch, path+"."+strconv.Itoa(i))
			}
		})
	}
	switch n.Kind {
	case format.Text:
		ui.Text(c, n.Text)
	case format.Break:
		ui.Text(c, "\n")
	case format.Bold:
		kids(ui.Text(c, "").FontWeight(WeightSemibold + 100))
	case format.Italic:
		kids(ui.Text(c, "").Italic())
	case format.Underline:
		kids(ui.Text(c, "").Underline())
	case format.Strike:
		kids(ui.Text(c, "").Strikethrough())
	case format.Code:
		bg := p.Text.Alpha(0.08)
		if b.s.OnAccent {
			bg = b.textColor().Alpha(0.16)
		}
		ui.RichText(c, ui.Span{Text: n.Text, Font: "monospace", Size: b.s.Size - 1, Background: bg, Radius: 4, PaddingX: 3})
	case format.Link:
		// Links are told apart by color alone; ui.Link underlines them
		// while the pointer is over them.
		col := p.AccentStrong
		if b.s.OnAccent {
			col = p.LinkOnBubble
		}
		link := ui.Link(c, format.Plain(n), n.Href).TextColor(col).Tooltip(n.Href)
		// A right click on the link itself: open it, or copy it. (The
		// innermost menu wins, so the message's own does not open here.)
		href := n.Href
		ContextMenu(c, link, func(m *ui.Menu) {
			if m.Item("Open link").Icon(MenuIcon(IconLink)).Chosen() {
				c.Services().OpenURL(href)
			}
			if m.Item("Copy link").Icon(MenuIcon(IconCopy)).Chosen() {
				c.Services().WriteClipboard(href)
				c.AddToast(ui.Toast{Title: "Link copied"})
			}
		})
	case format.Mention:
		// A mention is a pill: a tinted box around the name, in the accent,
		// so a ping stands out in the sentence without shouting. On an
		// accent bubble it becomes a lighter box on the fill instead.
		bg, fg := p.AccentSubtle, p.AccentStrong
		if b.s.OnAccent {
			bg, fg = b.textColor().Alpha(0.18), b.textColor()
		}
		ui.RichText(c, ui.Span{Text: n.Text, Weight: WeightMedium, Color: fg,
			Background: bg, Radius: RadiusSm, PaddingX: 4})
	case format.Spoiler:
		key := b.key + "#" + path
		open := b.rv != nil && b.rv[key]
		veil := p.TextMuted
		if b.s.OnAccent {
			veil = b.textColor().Alpha(0.55)
		}
		s := ui.Text(c, "").Label("Spoiler")
		// The veil clears and the words fade in as the dust flies.
		bg, fg := veil, ui.Transparent
		if open {
			bg, fg = p.Text.Alpha(0.06), b.textColor()
		} else {
			s.Cursor(ui.CursorPointer).Tooltip("Spoiler: click to show")
		}
		fade := ui.Animation{Duration: 260 * time.Millisecond, Ease: EaseOut}
		s.Background(s.AnimateColor("veil", bg, fade)).TextColor(s.AnimateColor("ink", fg, fade))
		kids(s)
		if s.Clicked() && b.rv != nil {
			if !open {
				x, y, _ := b.para.PointerPosition()
				startDust(c, b.para, b.paraID, key, dustBurst{x: x, y: y, w: 64, h: b.s.Size, n: 26, grains: veil})
			}
			b.rv[key] = !open
		}
	}
}

func (b *bodyBuilder) codeBlock(n *format.Node, path string) {
	c, p := b.c, b.p
	bg, border := p.BgInset, p.Border
	if b.s.OnAccent {
		bg, border = ui.RGBA(0, 0, 0, 0.14), ui.Transparent
	}
	ui.Column(c.Key(path)).Radius(RadiusSm+2).Background(bg).Border(1, border).Clip().MinWidth(0).Children(func() {
		ui.Row(c).Height(28).PaddingStart(S3).PaddingEnd(S1).Gap(S2).AlignItems(ui.Center).Children(func() {
			lang := n.Lang
			if lang == "" {
				lang = "code"
			}
			ui.Text(c, lang).FontSize(SizeCaption).FontWeight(WeightMedium).TextColor(b.muted()).Grow(1).Unselectable()
			cp := ui.ButtonBase(c.Key("copy")).Height(22).PaddingX(S15).Gap(S1).Radius(RadiusSm).AlignItems(ui.Center).
				Label("Copy code")
			if cp.Hovered() {
				cp.Background(p.Hover)
			}
			cp.Children(func() {
				ui.Icon(c, IconCopy).Size(12, 12).TextColor(b.muted())
				ui.Text(c, "Copy").FontSize(SizeCaption).TextColor(b.muted())
			})
			if cp.Clicked() {
				c.Services().WriteClipboard(n.Text)
				c.AddToast(ui.Toast{Title: "Code copied"})
			}
		})
		ui.Scroll(c).Row().Scrollbars(false).Padding(0, S3, S3, S3).Children(func() {
			ui.RichText(c).Font("monospace").FontSize(SizeSmall).LineHeight(1.5).NoWrap().Children(func() {
				// Quiet highlighting: comments, strings, numbers and the
				// language's keywords only (format.Highlight, no dependency).
				for _, tok := range format.Highlight(n.Lang, n.Text) {
					col := b.textColor()
					switch {
					case b.s.OnAccent && tok.Kind == format.CodeComment:
						col = b.muted()
					case !b.s.OnAccent && tok.Kind == format.CodeKeyword:
						col = p.AccentStrong
					case !b.s.OnAccent && tok.Kind == format.CodeString:
						col = p.Success
					case !b.s.OnAccent && tok.Kind == format.CodeComment:
						col = p.TextFaint
					case !b.s.OnAccent && tok.Kind == format.CodeNumber:
						col = p.Warning
					}
					ui.RichText(c, ui.Span{Text: tok.Text, Color: col, Italic: tok.Kind == format.CodeComment})
				}
			})
		})
	})
}
