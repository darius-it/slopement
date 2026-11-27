package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// ComposerFile is a file waiting in the composer to be sent with the
// message: shown as a chip with its name and size, removable.
type ComposerFile struct {
	Name, Size string
	// Image says the file is a picture: it waits as a thumbnail, not a chip.
	// Thumb is that thumbnail once decoded (nil meanwhile: a placeholder of
	// the same size holds its place, so nothing moves when it arrives).
	Image bool
	Thumb ui.ImageSource
	// ID tells the file apart from the others waiting, for the decode that
	// finishes later.
	ID int
}

// thumbSide is the side of a waiting picture's thumbnail.
const thumbSide = 72

// fileTray shows the files waiting to be sent, above the text field, and
// returns the one whose ✕ was pressed (-1: none). Nothing without files.
func fileTray(c *ui.Context, st *ComposerState) (removed int) {
	removed = -1
	if len(st.Files) == 0 {
		return
	}
	p := Of(c)
	ui.Row(c.Key("files")).Wrap().Gap(S1).Padding(S2, S2, 0, S2).Children(func() {
		for i, f := range st.Files {
			if f.Image {
				// A picture shows as itself, with the ✕ on its corner and its
				// name for the pointer and the screen reader.
				tile := ui.Box(c.Key("file:"+strconv.Itoa(f.ID))).Size(thumbSide, thumbSide).Radius(RadiusSm + 2).Clip().
					Background(p.BgInset).Border(1, p.Border).Tooltip(f.Name + " · " + f.Size).Label(f.Name)
				tile.Children(func() {
					if f.Thumb != nil {
						ui.Image(c, f.Thumb).Fill().Fit(ui.Cover)
					}
					side := rowHeightFor(c, 22)
					x := ui.ButtonBase(c).Size(side, side).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
						Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(3).Left(-3).Background(ui.RGBA(10, 8, 20, 0.6)).
						Label("Remove " + f.Name)
					Pressable(x)
					x.Children(func() { ui.Icon(c, IconX).Size(12, 12).TextColor(ui.Hex("#FFFFFF")) })
					if x.Clicked() {
						removed = i
					}
				})
				continue
			}
			ui.Row(c.Key("file:"+f.Name)).Gap(S1).Padding(S1, S1, S1, S2).Radius(RadiusSm+2).Background(p.Raised).
				Border(1, p.Border).AlignItems(ui.Center).MaxWidth(280).Children(func() {
				ui.Icon(c, IconPaperclip).Size(14, 14).TextColor(p.TextMuted)
				ui.Text(c, f.Name).FontSize(SizeSmall).SingleLine().Shrink(1).MinWidth(0)
				ui.Text(c, f.Size).FontSize(SizeCaption).TextColor(p.TextFaint).SingleLine().Shrink(0)
				side := rowHeightFor(c, 20) // a finger's target on a touch screen
				x := ui.ButtonBase(c).Size(side, side).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label("Remove " + f.Name)
				if x.Hovered() {
					x.Background(p.Hover)
				}
				x.Children(func() { ui.Icon(c, IconX).Size(12, 12).TextColor(p.TextMuted) })
				if x.Clicked() {
					removed = i
				}
			})
		}
	})
	return
}

// editBanner says a message is being edited, with a ✕ to stop: the line
// above the field while st.Editing is set.
func editBanner(c *ui.Context, st *ComposerState) {
	if st.Editing == "" {
		return
	}
	p := Of(c)
	ui.Row(c.Key("editing")).Gap(S2).Padding(S2, S2, 0, S3).AlignItems(ui.Center).
		Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).
		Children(func() {
			ui.Icon(c, IconPencil).Size(14, 14).TextColor(p.AccentStrong)
			ui.Text(c, "Editing message").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted).Grow(1).SingleLine()
			x := ui.ButtonBase(c).Size(22, 22).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label("Cancel editing")
			if x.Hovered() {
				x.Background(p.Hover)
			}
			x.Children(func() { ui.Icon(c, IconX).Size(14, 14).TextColor(p.TextMuted) })
			if x.Clicked() {
				st.EndEdit()
			}
		})
}

// PinBar is the line under a room's header saying a message is pinned: its
// place among the pins ("2 of 3"), who wrote it and its start. Pressing it
// goes to the message, and the bar moves on to the next pin.
func PinBar(c *ui.Context, at, of int, sender, text string) (pressed bool) {
	p := Of(c)
	b := ui.ButtonBase(c.Key("pinbar")).Height(rowHeightFor(c, 36)).PaddingX(S4).Gap(S2).AlignItems(ui.Center).Justify(ui.Start).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Label("Go to pinned message")
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		ui.Icon(c, IconPin).Size(14, 14).TextColor(p.AccentStrong)
		label := "Pinned"
		if of > 1 {
			label = "Pinned " + itoa(at+1) + " of " + itoa(of)
		}
		ui.Text(c, label).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.AccentStrong).SingleLine().Shrink(0)
		ui.RichText(c,
			ui.Span{Text: sender + "  ", Weight: WeightMedium, Color: p.TextMuted},
			ui.Span{Text: text, Color: p.TextMuted},
		).FontSize(SizeSmall).SingleLine().Shrink(1).MinWidth(0).Grow(1)
	})
	return b.Clicked()
}

func itoa(n int) string { return strconv.Itoa(n) }

// pasteKey reports whether a key in the text field is the paste shortcut
// (⌘V, Ctrl+V): the app then looks for a picture or files on the clipboard,
// while the field pastes whatever text there is on its own.
func pasteKey(k ui.TextKey) bool {
	return k.Key == ui.KeyV && (k.Mods == ui.Super || k.Mods == ui.Ctrl)
}

// canSend says the composer has something to send: text, or files waiting.
func (st *ComposerState) canSend() bool { return hasText(st.Text) || len(st.Files) > 0 }
