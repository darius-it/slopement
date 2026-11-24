package kit

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// VoiceState is a voice message being recorded, as the composer shows it: set
// by the app before each build. While Active the composer's field is replaced by
// the recording bar.
type VoiceState struct {
	Active  bool
	Elapsed time.Duration
	// Levels are the loudness of the last stretches, 0 to 1, newest last: the
	// bars that move as the user speaks.
	Levels []float32
}

// VoiceBars is how many level bars the recording bar draws.
const VoiceBars = 36

// VoiceClock is a length as "0:07" (or "1:02:03" past an hour).
func VoiceClock(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// voiceRow is the recording bar: cancel (the bin), a red dot with the time, the
// level bars, and send. It reports what was pressed into st, which takePlus
// hands on. round: the phone's round buttons.
func voiceRow(c *ui.Context, st *ComposerState, round bool) {
	p := Of(c)
	v := st.Voice
	ui.Row(c.Key("voice-row")).Padding(S1+2, S1+2).Gap(S2).AlignItems(ui.Center).Children(func() {
		var cancel ui.Element
		if round {
			cancel = roundIcon(c, IconTrash, "Cancel recording", ui.Transparent, p.Danger)
		} else {
			cancel = IconButton(c, Ghost, IconTrash, "Cancel recording")
		}
		if cancel.Clicked() {
			st.wantVoiceCancel = true
		}
		ui.Box(c).Size(8, 8).Radius(RadiusFull).Background(p.Danger).Shrink(0)
		ui.Text(c, VoiceClock(v.Elapsed)).FontSize(SizeBody).FontWeight(WeightMedium).SingleLine().Shrink(0).
			Label("Recording " + VoiceClock(v.Elapsed))
		ui.Row(c).Grow(1).MinWidth(0).Gap(2).AlignItems(ui.Center).Height(28).Justify(ui.End).Children(func() {
			levels := v.Levels
			if len(levels) > VoiceBars {
				levels = levels[len(levels)-VoiceBars:]
			}
			for i, l := range levels {
				ui.Box(c.Key(fmt.Sprint("vb", i))).Width(3).Height(3 + 23*min(max(l, 0), 1)).Radius(1.5).Background(p.TextMuted)
			}
		})
		var send ui.Element
		if round {
			send = roundIcon(c.Key("voice-send"), IconSend, "Send voice message", p.Accent, p.OnAccent)
		} else {
			s := controlHeight(c)
			send = ui.ButtonBase(c.Key("voice-send")).Size(s, s).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label("Send voice message")
			bg := p.Accent
			if send.Pressed() {
				bg = p.AccentPressed
			} else if send.Hovered() {
				bg = p.AccentHover
			}
			send.Background(send.AnimateColor("bg", bg, Color))
			Pressable(send)
			send.Children(func() { ui.Icon(c, IconSend).Size(16, 16).TextColor(p.OnAccent) })
		}
		if send.Clicked() {
			st.wantVoiceSend = true
		}
	})
}
