package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// eventSource is the "View source" dialog: one event's JSON as the server
// has it and, for an encrypted one, as it reads decrypted.
type eventSource struct {
	Sheet     kit.SheetState
	Room, ID  string
	Original  string
	Decrypted string
	Err       string
	Loading   bool
	Tab       int // 0: as it reads (decrypted, if it was encrypted), 1: as the server has it
}

// openSource asks the source for an event's JSON and opens the dialog.
func (a *App) openSource(room, id string) {
	es, ok := a.src.(model.EventSourcer)
	if !ok {
		return
	}
	d := &a.source
	*d = eventSource{Room: room, ID: id, Loading: true}
	d.Sheet.Open = true
	es.EventSource(room, id, func(original, decrypted, err string) {
		if a.source.ID != id || a.source.Room != room {
			return // another event was asked for since
		}
		d := &a.source
		d.Original, d.Decrypted, d.Err, d.Loading = original, decrypted, err, false
		if decrypted == "" {
			d.Tab = 1 // nothing decrypted to show first
		}
	})
}

// sourceText is the JSON the open tab shows.
func (d *eventSource) text() string {
	if d.Tab == 0 && d.Decrypted != "" {
		return d.Decrypted
	}
	return d.Original
}

func (a *App) sourceDialogView(c *ui.Context) {
	d := &a.source
	if !d.Sheet.Open {
		return
	}
	p := kit.Of(c)
	body := func() {
		ui.Column(c).Gap(kit.S3).Children(func() {
			ui.Text(c, d.ID).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
			switch {
			case d.Loading:
				ui.Text(c, "Loading…").FontSize(kit.SizeBody).TextColor(p.TextMuted)
			case d.Original == "":
				ui.Text(c, d.Err).FontSize(kit.SizeBody).TextColor(p.Danger)
			default:
				if d.Decrypted != "" {
					kit.Tabs(c, &d.Tab, "Decrypted", "As the server has it")
				} else if d.Err != "" {
					ui.Text(c, d.Err).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
				}
				ui.Scroll(c.Key("source-json")).MaxHeight(380).Background(p.BgInset).Radius(kit.RadiusSm + 2).Padding(kit.S3).Children(func() {
					ui.RichText(c, ui.Span{Text: d.text(), Color: p.Text}).Font("monospace").FontSize(kit.SizeSmall).LineHeight(1.5)
				})
				if kit.Button(c, kit.Secondary, "Copy", kit.IconCopy).AlignSelf(ui.End).Clicked() {
					a.copyToClipboard(d.text(), "Copied")
				}
			}
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, "Message source", body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, "Message source", 640, body)
	}
}
