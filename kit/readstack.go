package kit

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// readStackShown is how many readers show as avatars before "+N".
const readStackShown = 3

// ReadStack is who has read up to a message: a few small avatars, then
// "+N". It sits at the end of the message's row, quiet — no color, no
// motion — and names everyone in its tooltip and label.
func ReadStack(c *ui.Context, who []AvatarSpec) {
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
	ui.Row(c.Key("seen")).Gap(2).AlignItems(ui.Center).Shrink(0).Label(label).Tooltip(label).Children(func() {
		for _, w := range shown {
			w.Size, w.Presence, w.Trust = 16, PresenceNone, TrustUnknown
			Avatar(c, w)
		}
		if extra > 0 {
			ui.Text(c, "+"+strconv.Itoa(extra)).FontSize(SizeCaption - 1).FontFeatures("tnum").TextColor(p.TextMuted)
		}
	})
}
