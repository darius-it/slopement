package kit

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// readStackShown is how many readers show as avatars before "+N".
const readStackShown = 3

// ReadStack is who has read up to a message: their pictures in the place
// (and at the size) of the hollow "sent" check, as if it filled with them —
// overlapping in a group, as Element X's and Messenger's, each ringed in the
// background so the edges read — then "+N". They come in with a short fade
// from a little smaller, and name everyone in the tooltip and label.
func ReadStack(c *ui.Context, who []AvatarSpec) {
	ReadStackOn(c, who, Of(c).Bg)
}

// ReadStackOn is ReadStack over a background other than the page's (a
// phone's chat wallpaper), which rings the pictures.
func ReadStackOn(c *ui.Context, who []AvatarSpec, ring ui.Color) {
	if len(who) == 0 {
		return
	}
	p := Of(c)
	names := make([]string, len(who))
	for i, w := range who {
		names[i] = w.Name
	}
	label := "Read by " + strings.Join(names, ", ")
	shown, extra := who, 0
	if len(who) > readStackShown {
		shown, extra = who[:readStackShown], len(who)-readStackShown
	}
	ui.Row(c.Key("seen")).Gap(-5).AlignItems(ui.Center).Shrink(0).Label(label).Tooltip(label).
		Transition(ui.ElementTransition{Duration: DurBase, Enter: &ui.Motion{Scale: 0.8}, Ease: EaseOut}).Children(func() {
		for i, w := range shown {
			w.Size, w.Presence, w.Trust = 15, PresenceNone, TrustUnknown
			// Later ones under earlier ones would hide the first reader:
			// each sits over the next, ringed so the overlap reads.
			ui.Box(c.Key(i)).Padding(1.5).Radius(RadiusFull).Background(ring).ZIndex(len(shown) - i).Children(func() {
				Avatar(c, w)
			})
		}
		if extra > 0 {
			ui.Text(c, "+"+strconv.Itoa(extra)).FontSize(SizeCaption - 1).FontFeatures("tnum").TextColor(p.TextMuted)
		}
	})
}
