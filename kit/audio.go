package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// AudioData is an audio message as its card shows it.
type AudioData struct {
	ID      string
	Name    string
	Length  string // "0:42", when known
	Elapsed string // "0:12" while it has played some
	// Progress is how far it has played, 0 to 1.
	Progress float32
	Playing  bool
	Loading  bool
	// Note is a quiet line under it: why it cannot play here.
	Note string
}

// AudioAction is what the card asked for.
type AudioAction struct {
	Toggle bool    // the play / pause button
	Seek   bool    // the bar was pressed: go to Where
	Where  float32 // 0 to 1
}

// AudioCard is an audio or voice message: a play button, the bar (a press
// on it seeks), and the time. A quiet line says when it cannot be played
// here, which the file card beside it still lets be downloaded.
func AudioCard(c *ui.Context, d AudioData) (act AudioAction) {
	p := Of(c)
	ui.Column(c.Key("audio:"+d.ID)).Gap(S1).MaxWidth(360).Padding(S2, S3, S2, S2).Radius(RadiusMd).
		Background(p.BgInset).Border(1, p.Border).Children(func() {
		ui.Row(c).Gap(S3).AlignItems(ui.Center).Children(func() {
			icon, label := IconPlay, "Play "+d.Name
			if d.Playing {
				icon, label = IconPause, "Pause "+d.Name
			}
			b := ui.ButtonBase(c.Key("audio-play")).Size(36, 36).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Background(p.Accent).Label(label)
			if d.Loading {
				b.Opacity(0.6)
			}
			Pressable(b)
			b.Children(func() { ui.Icon(c, icon).Size(16, 16).TextColor(p.OnAccent) })
			if b.Clicked() && !d.Loading {
				act.Toggle = true
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(S1).Children(func() {
				ui.Text(c, d.Name).FontSize(SizeSmall).FontWeight(WeightMedium).SingleLine()
				// The bar: a press seeks to where it was.
				bar := ui.ButtonBase(c.Key("audio-bar")).Height(14).AlignItems(ui.Center).Label("Seek in " + d.Name)
				track := ui.Box(c.Key("audio-track")).Height(4).Radius(2).Background(p.Border)
				bar.Children(func() {
					track.Children(func() {
						w := track.Bounds().W
						ui.Box(c.Key("audio-fill")).Height(4).Radius(2).Background(p.Accent).Width(w * min(max(d.Progress, 0), 1))
					})
				})
				if bar.Clicked() {
					x, _, _ := bar.PointerPosition()
					if bw := bar.Bounds().W; bw > 0 {
						act.Seek, act.Where = true, min(max(x/bw, 0), 1)
					}
				}
			})
			t := d.Length
			if d.Elapsed != "" {
				t = d.Elapsed
				if d.Length != "" {
					t += " / " + d.Length
				}
			}
			if t != "" {
				ui.Text(c, t).FontSize(SizeCaption).FontFeatures("tnum").TextColor(p.TextMuted).SingleLine().Shrink(0)
			}
		})
		if d.Note != "" {
			ui.Text(c, d.Note).FontSize(SizeCaption).TextColor(p.TextMuted)
		}
	})
	return act
}

// clock is a duration in seconds as "0:42" or "1:05:09".
func Clock(sec int) string {
	if sec < 0 {
		sec = 0
	}
	h, m, s := sec/3600, sec%3600/60, sec%60
	two := func(n int) string {
		if n < 10 {
			return "0" + strconv.Itoa(n)
		}
		return strconv.Itoa(n)
	}
	if h > 0 {
		return strconv.Itoa(h) + ":" + two(m) + ":" + two(s)
	}
	return strconv.Itoa(m) + ":" + two(s)
}
