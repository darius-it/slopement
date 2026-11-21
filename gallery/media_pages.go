package gallery

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"

	"slopement/embeds"
	"slopement/format"
	"slopement/kit"
)

const formattingSample = `# Release notes
Shipping **0.4** today, *finally*. ***Huge*** thanks to __everyone__ who tested.
~~Friday~~ Thursday, and the ending is ||the rail grows a pip||.

> The best UI is the one you don't notice.
> — someone, probably

Changes:
- ` + "`⌘K`" + ` switcher, *instant*
- Mute for 15m / 2h / 1d / 1w
  - and **forever**
1. update
2. restart

` + "```go\nfunc main() {\n\tfmt.Println(\"hello, matrix\")\n}\n```" + `
Docs: [matrix.org](https://matrix.org) or https://spec.matrix.org/latest. Escaped: \*not italic\*, \|\|not a spoiler\|\|`

// demoImage draws a soft, photo-like gradient: offline pictures for the
// gallery (real ones come from the media repository).
func demoImage(seed int, w, h int) *ui.Bitmap {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	h1 := float64(seed*67%360) * math.Pi / 180
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			// Two hues blended diagonally, with a sun-like glow.
			t := (fx + fy) / 2
			gx, gy := fx-0.7, fy-0.3
			glow := math.Max(0, 1-math.Sqrt(gx*gx+gy*gy)*2.2)
			r := 0.55 + 0.35*math.Cos(h1+t*2) + glow*0.25
			g := 0.55 + 0.35*math.Cos(h1+t*2+2.1) + glow*0.2
			b := 0.6 + 0.35*math.Cos(h1+t*2+4.2) + glow*0.1
			clamp := func(v float64) uint8 { return uint8(math.Max(0, math.Min(1, v)) * 255) }
			img.Set(x, y, color.RGBA{clamp(r), clamp(g), clamp(b), 255})
		}
	}
	return ui.NewBitmap(img)
}

var (
	demoOnce sync.Once
	demoPics []*ui.Bitmap
)

func pics() []*ui.Bitmap {
	demoOnce.Do(func() {
		sizes := [][2]int{{320, 240}, {240, 320}, {320, 180}, {280, 280}, {320, 240}, {300, 200}}
		for i, s := range sizes {
			demoPics = append(demoPics, demoImage(i+2, s[0], s[1]))
		}
	})
	return demoPics
}

func demoMedia(n int) []kit.MediaSpec {
	out := make([]kit.MediaSpec, 0, n)
	for i := 0; i < n; i++ {
		b := pics()[i%len(pics())]
		w, h := b.Size()
		out = append(out, kit.MediaSpec{ID: "pic" + string(rune('a'+i)), Still: b, W: float32(w), H: float32(h), Progress: -1, Label: "Photo"})
	}
	return out
}

func (a *App) formattingPage(c *ui.Context) {
	p := kit.Of(c)
	if a.fmtDoc == nil || a.fmtWas != a.fmtSrc {
		a.fmtDoc, a.fmtWas = format.Parse(a.fmtSrc), a.fmtSrc
	}
	a.specimen(c, "Playground", "Type on the left: the right shows the message as everyone sees it. Below, the formatted_body other Matrix clients receive.", true, func() {
		ui.Row(c).Gap(kit.S4).AlignItems(ui.Stretch).Wrap().Children(func() {
			ui.Column(c).Grow(1).Basis(320).MinWidth(280).Gap(kit.S2).Children(func() {
				ui.Text(c, "Markdown").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted)
				ui.TextArea(c, &a.fmtSrc).Font("monospace").FontSize(kit.SizeSmall).Lines(18, 30).Label("Markdown")
			})
			ui.Column(c).Grow(1).Basis(320).MinWidth(280).Gap(kit.S2).Children(func() {
				ui.Text(c, "Message").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted)
				ui.Column(c).Padding(kit.S3).Radius(kit.RadiusMd).Border(1, p.Border).Children(func() {
					kit.MessageBody(c, "playground", a.fmtDoc, a.reveals, kit.BodyStyle{})
				})
			})
		})
		ui.Text(c, "formatted_body").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted)
		ui.Text(c, format.HTML(a.fmtDoc)).Font("monospace").FontSize(kit.SizeCaption).TextColor(p.TextMuted).Selectable().
			Padding(kit.S3).Radius(kit.RadiusSm).Background(p.BgInset)
	})
	a.specimen(c, "In a bubble", "The same body in an outgoing phone bubble: links, code and quotes recolor to read on purple.", false, func() {
		ui.Column(c).Padding(kit.S3).Radius(kit.RadiusMd).Background(kit.Of(c).Wallpaper).Children(func() {
			kit.Bubble(c, kit.BubbleData{ID: "fmt-bubble", Out: true, First: true, Last: true, Read: true, Time: "14:05",
				Doc: format.Parse("**Shipping** today ||(surprise)||, see `v0.4` and https://matrix.org\n> quoted bit\n- one\n- two")},
				&a.colors, a.reveals)
		})
	})
}

func (a *App) mediaPage(c *ui.Context) {
	a.specimen(c, "Attachments", "Fitted at their own aspect ratio from the first frame. Click to open the viewer (← → between items). The spoiler blurs until clicked; the video shows its length; uploads show a ring.", true, func() {
		one := demoMedia(1)
		ui.Row(c).Gap(kit.S4).Wrap().AlignItems(ui.Start).Children(func() {
			if kit.MediaPreview(c, one[0], 320, 240, a.reveals).Clicked() {
				a.openViewer(one, 0)
			}
			vid := demoMedia(3)[2]
			vid.ID, vid.Kind, vid.Duration = "vid", kit.MediaVideo, "0:42"
			kit.MediaPreview(c, vid, 320, 240, a.reveals)
			sp := demoMedia(2)[1]
			sp.ID, sp.Spoiler = "spoiled", true
			kit.MediaPreview(c, sp, 200, 240, a.reveals)
			up := demoMedia(4)[3]
			up.ID, up.Progress = "uploading", 0.62
			kit.MediaPreview(c, up, 180, 180, a.reveals)
		})
	})
	a.specimen(c, "Albums", "Two side by side, three as one and two, more as a grid whose last tile counts the rest.", true, func() {
		ui.Row(c).Gap(kit.S4).Wrap().AlignItems(ui.Start).Children(func() {
			for _, n := range []int{2, 3, 6} {
				items := demoMedia(n)
				for i := range items {
					items[i].ID = items[i].ID + string(rune('0'+n))
				}
				ui.Box(c.Key(n)).Children(func() {
					if i := kit.MediaGrid(c, items, 300, a.reveals); i >= 0 {
						a.openViewer(items, i)
					}
				})
			}
		})
	})
	a.specimen(c, "Files", "", false, func() {
		kit.FileCard(c, kit.FileSpec{Name: "design-kit-v0.4-final-final.fig", Size: "18.2 MB", Progress: -1})
	})
	yt := embeds.Classify("https://youtu.be/jNQXAC9IVRw")
	ytSpec := kit.YouTubeSpec{ID: yt.VideoID, Title: "Me at the zoo", Channel: "jawed", Duration: "0:19", Thumb: embeds.YouTubeThumb(yt.VideoID)}
	article := kit.LinkCard{URL: "https://matrix.org/blog/2026/11/release", Site: "Matrix.org",
		Title:       "Matrix 2.0 is here: sliding sync, OIDC and native group calls",
		Description: "A huge release for the protocol: instant login and sync, next-gen auth, and VoIP built in.",
		Image:       demoMedia(3)[2]}
	article.Image.ID, article.Image.W, article.Image.H = "article-pic", 1200, 630 // an OpenGraph image
	a.specimen(c, "Link previews", "From the server's preview_url, off by default in encrypted rooms. No colored marks: the site's name over the title says where it's from. Corners tighter than chat media (8 for the card, 6 for the picture). YouTube plays via youtube-nocookie on press; large pictures go below the words, small ones beside.", false, func() {
		ui.Column(c).Gap(kit.S3).Children(func() {
			if kit.YouTubeCard(c, ytSpec, 400) {
				a.did = "Would play " + embeds.YouTubePlayer(yt.VideoID, yt.Start)
			}
			kit.LinkPreview(c, article, 400)
			small := article
			small.URL, small.Image.W = small.URL+"#small", 120
			kit.LinkPreview(c, small, 400)
		})
	})
	a.specimen(c, "Imgur & direct media", "Shown as the media itself, not as a card.", false, func() {
		img := demoMedia(5)[4]
		img.ID, img.Kind, img.Label = "imgur", kit.MediaGIF, "imgur.com/AbC123x"
		ui.Column(c).Gap(kit.S1).Children(func() {
			ui.Text(c, "i.imgur.com/AbC123x.gifv").FontSize(kit.SizeSmall).TextColor(kit.Of(c).AccentStrong)
			kit.MediaPreview(c, img, 320, 220, a.reveals)
		})
	})
}
