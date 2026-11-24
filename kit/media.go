package kit

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/netimage"
)

// MediaKind is the kind of an attachment.
type MediaKind int

const (
	MediaImage MediaKind = iota
	MediaGIF
	MediaVideo
)

// MediaSpec is an image, GIF or video to preview.
type MediaSpec struct {
	ID   string
	Kind MediaKind
	// The picture: Player animates (GIFs, playing video), Still shows a
	// decoded thumbnail, URL loads one from the web (link embeds). None
	// shows the placeholder, in the item's own tint.
	Player *ui.VideoPlayer
	Still  ui.ImageSource
	URL    string
	// Full, when set, gives the picture at full resolution for the viewer
	// (nil until it has been fetched: the Still shows meanwhile). The
	// timeline's Still is decoded for the preview's size, which looks soft
	// once it fills the window and is zoomed.
	Full func() ui.ImageSource
	// W and H are the media's size: the preview keeps its aspect ratio
	// from the start, so the timeline never jumps as it loads.
	W, H     float32
	Duration string // videos: "0:42"
	Spoiler  bool
	// Progress is an upload's progress, 0 to 1; negative when done.
	Progress float32
	Label    string // for screen readers: the file's name or the caption
	// Star, when set, shows a star on hover (GIFs) that adds the media to
	// the picker's favorites; Starred fills it. Star gets the new state.
	Star    func(on bool)
	Starred bool
	// Paused shows a GIF's first frame without playing it (the app's
	// "Animate GIFs" setting).
	Paused bool
	// Sticker shows at its own size, never enlarged, as a GIF does.
	Sticker bool
}

// fit returns w×h scaled into maxW×maxH, keeping the aspect ratio, never
// smaller than a usable tile.
func fit(w, h, maxW, maxH float32) (float32, float32) {
	if w <= 0 || h <= 0 {
		w, h = 4, 3
	}
	s := min(maxW/w, maxH/h)
	if s > 1 && w*s > maxW {
		s = maxW / w
	}
	rw, rh := w*s, h*s
	if rw < 96 {
		rw = 96
	}
	if rh < 72 {
		rh = 72
	}
	return rw, rh
}

// MediaPreview shows an attachment in the timeline: fitted into maxW×maxH
// at its own aspect ratio, rounded, with a play button and duration on
// videos, a GIF badge, a blur over spoilers until clicked, and a ring while
// it uploads. Clicked opens the viewer (or reveals a spoiler first).
func MediaPreview(c *ui.Context, m MediaSpec, maxW, maxH float32, rv Reveals) ui.Element {
	w, h := fit(m.W, m.H, maxW, maxH)
	if (m.Kind == MediaGIF || m.Sticker) && m.W > 0 && m.H > 0 {
		// A GIF shows at its own size (never enlarged), shrunk only to fit.
		s := min(1, maxW/m.W, maxH/m.H)
		w, h = m.W*s, m.H*s
	}
	return mediaTile(c, m, w, h, RadiusMd, rv)
}

func mediaTile(c *ui.Context, m MediaSpec, w, h, radius float32, rv Reveals) ui.Element {
	p := Of(c)
	hidden := m.Spoiler && !(rv != nil && rv["media:"+m.ID])
	b := ui.ButtonBase(c.Key("media:"+m.ID)).Size(w, h).Radius(radius).Clip().Background(p.BgInset).Label(m.Label)
	b.Children(func() {
		var pic ui.Element
		switch {
		case m.Player != nil && m.Kind == MediaGIF:
			pic = ui.Video(c, m.Player).Fill().Fit(ui.Contain) // never cropped
			// Paused (the "Animate GIFs" setting off) holds the first
			// frame; Pause is idempotent, so calling it every frame is
			// fine, and a paused player draws no further frames.
			if m.Paused {
				m.Player.Pause()
			}
		case m.Player != nil:
			pic = ui.Video(c, m.Player).Fill().Fit(ui.Cover)
		case m.Still != nil:
			pic = ui.Image(c, m.Still).Fill().Fit(ui.Cover)
		case m.URL != "":
			pic = netimage.Image(c, m.URL, netimage.Options{MaxSide: int(max(w, h) * 2)}).Fill().Fit(ui.Cover)
		default:
			pic = ui.Box(c).Fill().Background(p.Users[HashIndex(m.ID)].Alpha(0.14).Over(p.BgInset))
		}
		if hidden {
			// The picture itself blurs (the spoiler shows its colors, not
			// its content), and clears smoothly once revealed.
			pic.Blur(b.AnimateWith("blur", 12, DurBase, EaseOut))
		} else if m.Spoiler {
			pic.Blur(b.AnimateWith("blur", 0, DurBase, EaseOut))
		}
		if hidden {
			ui.Column(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Background(p.Bg.Alpha(0.25)).
				AlignItems(ui.Center).Justify(ui.Center).Gap(S1).PassThrough().Children(func() {
				ui.Icon(c, IconEye).Size(20, 20).TextColor(p.Text)
				ui.Text(c, "Spoiler").FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(p.Text)
			})
		}
		overlay := ui.RGBA(10, 8, 20, 0.55)
		white := ui.Hex("#FFFFFF")
		switch {
		case m.Kind == MediaVideo && !hidden:
			ui.Box(c).Size(48, 48).Radius(RadiusFull).Background(overlay).Attach(ui.AnchorCenter, ui.AnchorCenter).
				AlignItems(ui.Center).Justify(ui.Center).PassThrough().Children(func() {
				ui.Icon(c, IconPlay).Size(20, 20).TextColor(white).Translate(1.5, 0)
			})
			if m.Duration != "" {
				pillText(c, m.Duration, 20, SizeCaption, WeightMedium, white, overlay).
					Attach(ui.AnchorBottomLeft, ui.AnchorBottomLeft).Left(S2).Top(-S2)
			}
		case m.Kind == MediaGIF && !hidden:
			pillText(c, "GIF", 18, SizeCaption-1, WeightSemibold, white, overlay).
				Attach(ui.AnchorBottomLeft, ui.AnchorBottomLeft).Left(S15).Top(-S15)
		}
		if m.Progress >= 0 && m.Progress < 1 {
			ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Background(ui.RGBA(10, 8, 20, 0.3)).PassThrough()
			progressRing(c, m.Progress, white)
		}
		// The favorite star appears at the top-right corner on pointer hover,
		// never just because the GIF is starred — nor while the tile merely holds
		// the keyboard focus, which a click leaves behind; starred is a filled
		// gold star. Not on a touch screen, which has no hover to bring it only
		// when wanted: it would sit over every GIF. There it is in the viewer's
		// top bar (MediaViewer).
		if m.Star != nil && !hidden && b.Hovered() && !Touch(c) {
			label := "Add to favorite GIFs"
			if m.Starred {
				label = "Remove from favorite GIFs"
			}
			s := ui.ButtonBase(c.Key("star")).Size(28, 28).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(S15).Left(-S15).Label(label).Tooltip(label).
				Transition(ui.ElementTransition{Duration: DurFast, ExitDuration: DurExit, Enter: &ui.Motion{Scale: 0.9}, Exit: &ui.Motion{Scale: 0.9}})
			bg := overlay
			if s.Hovered() {
				bg = ui.RGBA(10, 8, 20, 0.75)
			}
			s.Background(s.AnimateColor("bg", bg, Color))
			s.Children(func() {
				col, ic := white, IconStar
				if m.Starred {
					col, ic = p.Warning, IconStarFilled // gold, as the picker's favorites
				}
				ui.Icon(c, ic).Size(15, 15).TextColor(col)
			})
			Pressable(s)
			if s.Clicked() {
				m.Star(!m.Starred)
			}
		}
	})
	if b.Clicked() && hidden && rv != nil {
		rv["media:"+m.ID] = true
		// The picture's veil dissolves into dust across it as the blur
		// clears.
		startDust(c, b, "media:"+m.ID, "reveal", dustBurst{x: w / 2, y: h / 2, w: w * 0.9, h: h * 0.9, n: 90,
			grains: ui.Hex("#FFFFFF")})
	}
	playDust(b, "media:"+m.ID)
	return b
}

// progressRing is an upload's progress around a center dot.
func progressRing(c *ui.Context, v float32, col ui.Color) {
	ring := ui.Box(c).Size(40, 40).Attach(ui.AnchorCenter, ui.AnchorCenter).PassThrough()
	shown := ring.AnimateWith("p", v, DurBase, EaseOut)
	ring.Draw(func(pt *ui.Painter, r ui.Rect) {
		var bg, fg ui.Path
		cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2-3
		bg.Circle(cx, cy, rad)
		pt.StrokePath(&bg, 3, col.Alpha(0.3))
		fg.Circle(cx, cy, rad).Cap(ui.RoundCap).Trim(0, max(shown, 0.01))
		pt.StrokePath(&fg, 3, col)
	})
}

// MediaGrid shows several attachments sent together: one fills the
// width, two side by side, three as one large and two small, four or more
// as a 2×2 grid whose last tile says how many more. It returns the index
// clicked, or -1.
func MediaGrid(c *ui.Context, items []MediaSpec, width float32, rv Reveals) int {
	clicked := -1
	if len(items) == 0 {
		return clicked
	}
	if len(items) == 1 {
		if MediaPreview(c, items[0], width, width*0.8, rv).Clicked() {
			clicked = 0
		}
		return clicked
	}
	const gap = 3
	half := (width - gap) / 2
	tile := func(i int, w, h float32) {
		e := mediaTile(c, items[i], w, h, 4, rv)
		if e.Clicked() {
			clicked = i
		}
	}
	ui.Column(c.Key("grid")).Gap(gap).Radius(RadiusMd).Clip().Width(width).Children(func() {
		switch len(items) {
		case 2:
			ui.Row(c).Gap(gap).Children(func() { tile(0, half, half); tile(1, half, half) })
		case 3:
			ui.Row(c).Gap(gap).Children(func() {
				tile(0, half, half)
				ui.Column(c).Gap(gap).Children(func() { tile(1, half, (half-gap)/2); tile(2, half, (half-gap)/2) })
			})
		default:
			for r := 0; r < 2; r++ {
				ui.Row(c.Key(r)).Gap(gap).Children(func() {
					for col := 0; col < 2; col++ {
						i := r*2 + col
						if i == 3 && len(items) > 4 {
							ui.Box(c.Key("more")).Size(half, half*0.75).Children(func() {
								tile(3, half, half*0.75)
								ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Background(ui.RGBA(10, 8, 20, 0.5)).
									AlignItems(ui.Center).Justify(ui.Center).PassThrough().Children(func() {
									ui.Text(c, "+"+strconv.Itoa(len(items)-3)).FontSize(SizeHeading).FontWeight(WeightSemibold).
										TextColor(ui.Hex("#FFFFFF"))
								})
							})
							continue
						}
						tile(i, half, half*0.75)
					}
				})
			}
		}
	})
	return clicked
}

// FileSpec is a file attachment.
type FileSpec struct {
	Name, Size string
	Progress   float32 // upload, 0 to 1; negative when done
}

// FileCard shows a file to download. Clicked downloads it.
func FileCard(c *ui.Context, f FileSpec) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key("file:"+f.Name)).Gap(S3).Padding(S2, S3, S2, S2).Radius(RadiusMd).
		Background(p.Raised).Border(1, p.Border).AlignItems(ui.Center).MaxWidth(360).Label("Download " + f.Name)
	if b.Hovered() {
		b.Border(1, p.BorderStrong)
	}
	b.Children(func() {
		ui.Box(c).Size(40, 40).Radius(RadiusSm + 2).Background(p.AccentSubtle).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, IconFile).Size(20, 20).TextColor(p.AccentStrong) })
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, f.Name).FontSize(SizeBody).FontWeight(WeightMedium).TruncateMiddle()
			ui.Text(c, f.Size).FontSize(SizeSmall).TextColor(p.TextMuted)
		})
		ui.Icon(c, IconDownload).Size(18, 18).TextColor(p.TextMuted)
	})
	return b
}

// LocationSpec is a shared place: a static card, never a live map, with a
// link that opens it in the user's maps.
type LocationSpec struct {
	ID          string
	Title       string // what the sender called it ("" for "Shared location")
	Coordinates string // "52.5200, 13.4050"
	URL         string // where "Open in maps" goes
	// Map is a picture of the place (OpenStreetMap's tiles, with a pin) once it
	// has been fetched and the user allowed it; nil: the plain card.
	Map ui.ImageSource
}

// MapWidth and MapHeight are the size of a map on the card, in DIP.
const (
	MapWidth  = 256
	MapHeight = 144
)

// LocationCard shows a LocationSpec.
func LocationCard(c *ui.Context, l LocationSpec) ui.Element {
	p := Of(c)
	title := l.Title
	if title == "" {
		title = "Shared location"
	}
	if l.Map != nil {
		return mapCard(c, l, title)
	}
	card := ui.Row(c.Key("loc:"+l.ID)).Gap(S3).Padding(S2, S3, S2, S2).Radius(RadiusMd).
		Background(p.Raised).Border(1, p.Border).AlignItems(ui.Center).MaxWidth(360).Label("Location: " + title)
	card.Children(func() {
		ui.Box(c).Size(40, 40).Radius(RadiusSm + 2).Background(p.AccentSubtle).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, IconMapPin).Size(20, 20).TextColor(p.AccentStrong) })
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(SizeBody).FontWeight(WeightMedium).MaxLines(2)
			ui.Text(c, l.Coordinates).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
			ui.Link(c, "Open in maps", l.URL).FontSize(SizeSmall).TextColor(p.AccentStrong)
		})
	})
	return card
}

// mapCard is a place drawn as a map, its words and link under it, and
// OpenStreetMap's credit, which its tile policy asks to be shown with every map.
func mapCard(c *ui.Context, l LocationSpec, title string) ui.Element {
	p := Of(c)
	card := ui.Column(c.Key("loc:"+l.ID)).Radius(RadiusMd).Background(p.Raised).Border(1, p.Border).Clip().
		Width(MapWidth).MaxWidth(MapWidth).Label("Location: " + title)
	card.Children(func() {
		ui.Image(c, l.Map).Size(MapWidth, MapHeight).Fit(ui.Cover).Label("Map of " + title)
		ui.Column(c).Gap(1).Padding(S2, S3).Children(func() {
			ui.Text(c, title).FontSize(SizeBody).FontWeight(WeightMedium).MaxLines(2)
			ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
				ui.Link(c, "Open in maps", l.URL).FontSize(SizeSmall).TextColor(p.AccentStrong)
				ui.Text(c, "© OpenStreetMap contributors").FontSize(SizeCaption).TextColor(p.TextFaint).SingleLine().Grow(1).Justify(ui.End)
			})
		})
	})
	return card
}

// LinkCard is a link's preview from OpenGraph: a neutral edge, the site,
// the title as a link, two lines of description, and a thumbnail at the
// side (small pictures) or below (large ones). Dismissed reports the ×.
type LinkCard struct {
	URL, Site, Title, Description string
	Image                         MediaSpec // URL or Still; zero for none
}

// Embeds carry no colored mark: the site's name over the title says where
// a card is from (chosen over a brand-colored rule, 2026-11-21). Their
// corners are tighter than chat media's, so a card reads as a quoted
// document rather than a bubble.
const (
	embedRadius = RadiusSm + 2 // the card
	embedMedia  = RadiusSm     // the picture inside it
)

// embedHeader is a card's site, title (a link) and subtitle.
func embedHeader(c *ui.Context, site, title, url, sub string, subLines int) {
	p := Of(c)
	ui.Column(c).Gap(2).MinWidth(0).Children(func() {
		if site != "" {
			ui.Text(c, site).FontSize(SizeCaption).FontWeight(WeightMedium).TextColor(p.TextMuted).SingleLine()
		}
		if url != "" {
			ui.Link(c, title, url).FontSize(SizeBody).FontWeight(WeightSemibold).TextColor(p.AccentStrong).MaxLines(2)
		} else {
			ui.Text(c, title).FontSize(SizeBody).FontWeight(WeightSemibold).TextColor(p.AccentStrong).MaxLines(2)
		}
		if sub != "" {
			ui.Text(c, sub).FontSize(SizeSmall).TextColor(p.TextMuted).MaxLines(subLines)
		}
	})
}

// LinkPreview shows a LinkCard; it reports whether its × was pressed.
func LinkPreview(c *ui.Context, l LinkCard, width float32) (dismiss bool) {
	p := Of(c)
	hasPic := l.Image.URL != "" || l.Image.Still != nil
	large := hasPic && l.Image.W >= 400
	card := ui.Column(c.Key("link:" + l.URL)).Width(width).MaxWidthPercent(100).Radius(embedRadius).Background(p.BgInset).
		Padding(S3).Gap(S2)
	hov := card.Hovered()
	card.Children(func() {
		ui.Row(c).Gap(S3).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				embedHeader(c, l.Site, l.Title, l.URL, l.Description, 3)
			})
			if hasPic && !large {
				mediaTile(c, l.Image, 64, 64, embedMedia, nil)
			}
		})
		if large {
			w := width - S6
			mediaTile(c, l.Image, w, min(w*l.Image.H/max(l.Image.W, 1), 240), embedMedia, nil)
		}
		if hov {
			x := ui.ButtonBase(c).Size(20, 20).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(S1).Left(-S1).Background(p.Raised).Label("Remove preview")
			x.Children(func() { ui.Icon(c, IconX).Size(12, 12).TextColor(p.TextMuted) })
			dismiss = x.Clicked()
		}
	})
	return dismiss
}

// YouTubeCard is a YouTube link: the site, title and channel, then the
// thumbnail at 16:9 with a play button and the duration. It reports a
// press of play.
func YouTubeCard(c *ui.Context, y YouTubeSpec, width float32) (play bool) {
	p := Of(c)
	ui.Column(c.Key("yt:" + y.ID)).Width(width).MaxWidthPercent(100).Gap(S2).Padding(S3).Radius(embedRadius).
		Background(p.BgInset).Children(func() {
		embedHeader(c, "YouTube", y.Title, "", y.Channel, 1)
		w := width - S6
		m := MediaSpec{ID: y.ID, Kind: MediaVideo, URL: y.Thumb, W: 16, H: 9, Duration: y.Duration, Label: "Play " + y.Title}
		if mediaTile(c, m, w, w*9/16, embedMedia, nil).Clicked() {
			play = true
		}
	})
	return play
}

// viewZoom is the viewer's zoom on the picture shown: Scale 1 is fitted,
// and X, Y the pan in DIPs from the center. It resets with the item.
var viewZoom struct {
	ID       string
	Scale    float32
	X, Y     float32
	dragging bool
	lx, ly   float32
}

const (
	viewZoomMax = 8
	viewZoomMin = 1
)

// zoomAt changes the zoom to scale, keeping the point (px, py), from the
// center of the area, where it is on screen.
func zoomAt(scale, px, py float32) {
	scale = min(max(scale, viewZoomMin), viewZoomMax)
	k := scale / viewZoom.Scale
	viewZoom.X = px - (px-viewZoom.X)*k
	viewZoom.Y = py - (py-viewZoom.Y)*k
	viewZoom.Scale = scale
	if scale == 1 {
		viewZoom.X, viewZoom.Y = 0, 0
	}
}

// clampPan keeps the picture from being dragged out of the area: its edges
// stop at the area's.
func clampPan(mw, mh, aw, ah float32) {
	s := viewZoom.Scale
	limX := max((mw*s-aw)/2, 0)
	limY := max((mh*s-ah)/2, 0)
	viewZoom.X = min(max(viewZoom.X, -limX), limX)
	viewZoom.Y = min(max(viewZoom.Y, -limY), limY)
}

// viewDismiss is where a finger has put the picture, down or up, to close the
// viewer (flickAway).
var viewDismiss ui.Point

// flickAway lets a finger take the picture away to close the viewer, as
// Photos' and Instagram's do: it follows the finger up or down (shrinking a
// little), the backdrop clearing as it goes; let go past a third of the way, or
// flicked, it carries on off the screen and the viewer closes; let go before,
// it springs back. It is a vertical Draggable, so a finger going across or two
// pinching are not taken. Only at 1×: a zoomed picture is panned instead. It
// returns how far the picture is gone, 0 to 1.
func flickAway(c *ui.Context, pic ui.Element, open *bool) float32 {
	_, wh := c.Size()
	reach := wh * 0.33 // past this (counting the throw), it goes
	pic.Draggable(&viewDismiss, ui.DraggableOptions{
		Axis: ui.Vertical,
		Snap: func(p ui.Point) ui.Point {
			switch {
			case p.Y > reach:
				return ui.Point{Y: wh}
			case p.Y < -reach:
				return ui.Point{Y: -wh}
			}
			return ui.Point{}
		},
	})
	away := min(float32(math.Abs(float64(pic.DragOffset().Y)))/(wh*0.45), 1)
	sc := 1 - 0.12*away
	pic.Scale(sc, sc)
	// It went off the screen: close, once it is there.
	if math.Abs(float64(viewDismiss.Y)) >= float64(wh) && !pic.Transitioning() {
		viewDismiss = ui.Point{}
		*open = false
	}
	return away
}

// MediaViewer shows attachments full screen over a dark backdrop: the
// picture fitted at full resolution, zoomed with the wheel, a pinch,
// double-click or +/−, and dragged when zoomed; arrows (and keys) between
// items, a counter, download and close. *index is the item shown.
func MediaViewer(c *ui.Context, open *bool, items []MediaSpec, index *int) {
	if len(items) == 0 {
		return
	}
	*index = min(max(*index, 0), len(items)-1)
	p := Of(c)
	white := ui.Hex("#FFFFFF")
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(8, 6, 14, 0.94)).Transition(ui.ElementTransition{Duration: DurBase, ExitDuration: DurExit, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
		w, h := c.Size()
		panel.Size(w, h).Transition(FadeIn()).Label("Media viewer")
		m := items[*index]
		if viewZoom.ID != m.ID || viewZoom.Scale == 0 {
			viewZoom.ID, viewZoom.Scale, viewZoom.X, viewZoom.Y, viewZoom.dragging = m.ID, 1, 0, 0, false
			viewDismiss = ui.Point{}
		}
		// The panel is the whole window, bars included: on a phone the header
		// keeps below the status bar (clock, battery) and the picture above the
		// home indicator.
		safeTop, _, safeBottom, _ := c.SafeArea()
		areaW, areaH := w-S8*2, h-140-safeTop-safeBottom
		// The header sits clear of the window's controls (the traffic lights
		// on macOS), as every other bar does.
		bar := c.TitleBar()
		ui.Row(c).Height(max(52, bar.Height)+safeTop).Padding(safeTop, bar.Right+S3, 0, bar.Left+S3).Gap(S2).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Text(c, m.Label).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(white).Grow(1).SingleLine().MinWidth(0)
			ui.Text(c, strconv.Itoa(*index+1)+" / "+strconv.Itoa(len(items))).FontSize(SizeSmall).FontFeatures("tnum").TextColor(white.Alpha(0.7))
			type viewAction struct {
				icon  *ui.SVG
				label string
				gold  bool // a starred favorite is gold, as the picker's
			}
			actions := []viewAction{{IconMinus, "Zoom out", false}, {IconPlus, "Zoom in", false}, {IconDownload, "Download", false}, {IconX, "Close", false}}
			if m.Star != nil {
				// A GIF's favorite star lives here, in the top bar, not over the
				// picture in the timeline (where a finger has no hover to show it
				// only when wanted). Before Download.
				fav := viewAction{IconStar, "Add to favorite GIFs", false}
				if m.Starred {
					fav = viewAction{IconStarFilled, "Remove from favorite GIFs", true}
				}
				actions = append(actions[:2:2], append([]viewAction{fav}, actions[2:]...)...)
			}
			for _, a := range actions {
				if Touch(c) && (a.label == "Zoom in" || a.label == "Zoom out") {
					continue // a finger pinches
				}
				bs := float32(36)
				if Touch(c) {
					bs = TouchTarget
				}
				b := ui.ButtonBase(c.Key(a.label)).Size(bs, bs).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).Label(a.label)
				if b.Hovered() {
					b.Background(white.Alpha(0.12))
				}
				b.Children(func() {
					col := white
					if a.gold {
						col = p.Warning
					}
					ui.Icon(c, a.icon).Size(18, 18).TextColor(col)
				})
				if b.Clicked() {
					switch a.label {
					case "Add to favorite GIFs", "Remove from favorite GIFs":
						m.Star(!m.Starred)
						items[*index].Starred = !m.Starred // the viewer shows it at once
					case "Close":
						*open = false
					case "Zoom in":
						zoomAt(viewZoom.Scale*1.5, 0, 0)
					case "Zoom out":
						zoomAt(viewZoom.Scale/1.5, 0, 0)
					}
				}
			}
		})
		// A click around the picture (not on it) closes the viewer.
		var picBox ui.Rect // the picture, last frame, in window DIPs
		area := ui.Row(c.Key("area")).Grow(1).AlignItems(ui.Center).Justify(ui.Center).PaddingX(S4).Clip()
		var mw, mh float32
		var away float32 // 0..1: how far a finger has taken the picture off, to close it
		area.Children(func() {
			mw, mh = fit(m.W, m.H, areaW, areaH)
			s := viewZoom.Scale
			pic := ui.Box(c.Key(m.ID)).Size(mw, mh).Radius(RadiusSm).Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{Scale: 0.98}})
			if s != 1 {
				pic.Scale(s, s).Translate(viewZoom.X/s, viewZoom.Y/s)
			} else if Touch(c) {
				away = flickAway(c, pic, open)
			}
			picBox = pic.Bounds()
			var full ui.ImageSource
			if m.Full != nil {
				full = m.Full()
			}
			pic.Children(func() {
				switch {
				case m.Player != nil:
					ui.Video(c, m.Player).Fill().Fit(ui.Contain)
				case full != nil:
					ui.Image(c, full).Fill().Fit(ui.Contain)
				case m.Still != nil:
					ui.Image(c, m.Still).Fill().Fit(ui.Contain)
				case m.URL != "":
					netimage.Image(c, m.URL).Fill().Fit(ui.Contain)
				}
			})
		})
		if away > 0 {
			// The backdrop clears as the picture goes: the chat comes back under it.
			backdrop.Background(ui.RGBA(8, 6, 14, 0.94*(1-away)))
		}
		ab := area.Bounds() // as of the last frame: the callback outlives this element
		area.HandleInput(func(ev ui.InputEvent) bool {
			px, py := ev.X-ab.W/2, ev.Y-ab.H/2 // from the area's center
			switch ev.Kind {
			case ui.InputZoom:
				zoomAt(viewZoom.Scale*ev.Scale, px, py)
			case ui.InputScroll:
				switch {
				case ev.Mods&ui.Ctrl != 0 || !ev.Precise:
					// A wheel's notches (or Ctrl held) zoom, around the pointer.
					zoomAt(viewZoom.Scale*float32(math.Exp(float64(-ev.DY)*0.01)), px, py)
				case viewZoom.Scale > 1:
					viewZoom.X -= ev.DX // a touchpad pans what is zoomed
					viewZoom.Y -= ev.DY
				default:
					return false
				}
			case ui.InputPointerDown:
				if ev.Button != 0 {
					return false
				}
				if ev.Clicks == 2 {
					if viewZoom.Scale > 1 {
						zoomAt(1, px, py)
					} else {
						zoomAt(2.5, px, py)
					}
					clampPan(mw, mh, ab.W, ab.H)
					return true
				}
				viewZoom.dragging, viewZoom.lx, viewZoom.ly = viewZoom.Scale > 1, ev.X, ev.Y
				return false // a plain click still reaches Clicked below
			case ui.InputPointerMove:
				if !viewZoom.dragging {
					return false
				}
				viewZoom.X += ev.X - viewZoom.lx
				viewZoom.Y += ev.Y - viewZoom.ly
				viewZoom.lx, viewZoom.ly = ev.X, ev.Y
			case ui.InputPointerUp:
				viewZoom.dragging = false
				return false
			default:
				return false
			}
			clampPan(mw, mh, ab.W, ab.H)
			return true
		})
		if area.Clicked() && viewZoom.Scale == 1 {
			x, y, _ := area.PointerPosition()
			px, py := ab.X+x, ab.Y+y
			if px < picBox.X || px > picBox.X+picBox.W || py < picBox.Y || py > picBox.Y+picBox.H {
				*open = false
			}
		}
		if c.Shortcut(0, ui.KeyRight) && *index < len(items)-1 {
			*index++
		}
		if c.Shortcut(0, ui.KeyLeft) && *index > 0 {
			*index--
		}
		if c.Shortcut(0, ui.KeyEqual) || c.Shortcut(ui.Shift, ui.KeyEqual) {
			zoomAt(viewZoom.Scale*1.5, 0, 0)
		}
		if c.Shortcut(0, ui.KeyMinus) {
			zoomAt(viewZoom.Scale/1.5, 0, 0)
		}
		if c.Shortcut(0, ui.Key0) {
			zoomAt(1, 0, 0)
		}
	})
}
