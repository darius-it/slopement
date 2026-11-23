package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// canRetry says a failed message can be sent again (the source kept how).
func (a *App) canRetry(room, id string) bool {
	rt, ok := a.src.(model.Retrier)
	return ok && rt.CanRetry(room, id)
}

// offlineBar is the error across the top while the connection to the server is
// down: what is happening, and what it means for what the user sends. It goes
// when the sync gets an answer again.
func (a *App) offlineBar(c *ui.Context) {
	conn, ok := a.src.(model.Connectivity)
	if !ok || !conn.Offline() {
		return
	}
	p := kit.Of(c)
	ui.Row(c.Key("offline")).Shrink(0).Padding(kit.S2, kit.S3).Gap(kit.S2).AlignItems(ui.Center).
		Background(p.DangerSubtl).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Label("No internet connection").Children(func() {
		ui.Icon(c, kit.IconAlert).Size(16, 16).TextColor(p.Danger)
		ui.Text(c, "No internet connection. Reconnecting… What you send now will not be delivered.").
			FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).TextColor(p.Danger).Grow(1)
	})
}
