package gallery

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

func hexOf(col ui.Color) string {
	return fmt.Sprintf("#%02X%02X%02X", col.R, col.G, col.B)
}

func (a *App) colorsPage(c *ui.Context) {
	a.specimen(c, "Surfaces & text", "Neutral, with a 2% purple tint. Hierarchy comes from gray steps and weight, not boxes.", false, func() {
		p := kit.Of(c)
		swatches(c, []swatchSpec{
			{"Bg", p.Bg}, {"BgSubtle", p.BgSubtle}, {"BgInset", p.BgInset}, {"Raised", p.Raised},
			{"Border", p.Border}, {"BorderStrong", p.BorderStrong},
			{"Text", p.Text}, {"TextMuted", p.TextMuted}, {"TextFaint", p.TextFaint},
		})
	})
	a.specimen(c, "Accent: light purple", "One accent, used sparingly: the selection, the primary action, mentions, focus.", false, func() {
		p := kit.Of(c)
		swatches(c, []swatchSpec{
			{"Accent", p.Accent}, {"AccentHover", p.AccentHover}, {"AccentPressed", p.AccentPressed},
			{"AccentSubtle", p.AccentSubtle}, {"AccentStrong", p.AccentStrong}, {"OnAccent", p.OnAccent},
		})
	})
	a.specimen(c, "Status", "", false, func() {
		p := kit.Of(c)
		swatches(c, []swatchSpec{{"Danger", p.Danger}, {"Warning", p.Warning}, {"Success", p.Success}})
	})
	a.specimen(c, "Username palette", "Eight hues at one lightness, each AA on the timeline in its theme (tested in kit).", false, func() {
		p := kit.Of(c)
		var s []swatchSpec
		for i, u := range p.Users {
			s = append(s, swatchSpec{kit.UserColorNames[i], u})
		}
		swatches(c, s)
	})
}

type swatchSpec struct {
	name string
	col  ui.Color
}

func swatches(c *ui.Context, list []swatchSpec) {
	p := kit.Of(c)
	ui.Row(c).Wrap().Gap(kit.S3).Children(func() {
		for _, s := range list {
			ui.Column(c.Key(s.name)).Width(104).Gap(kit.S1).Children(func() {
				ui.Box(c).Height(48).Radius(kit.RadiusSm + 2).Background(s.col).Border(1, p.Border)
				ui.Text(c, s.name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).SingleLine()
				ui.Text(c, hexOf(s.col)).FontSize(kit.SizeCaption).Font("monospace").TextColor(p.TextMuted).Selectable()
			})
		}
	})
}

func (a *App) typePage(c *ui.Context) {
	a.specimen(c, "Type scale", "The system UI font (no font to embed or load). Weights 400 / 500 / 600 only; tabular digits for times and counts.", false, func() {
		p := kit.Of(c)
		for _, t := range []struct {
			name   string
			size   float32
			weight int
			sample string
		}{
			{"Display 28 / 600", kit.SizeDisplay, kit.WeightSemibold, "Welcome back"},
			{"Heading 22 / 600", kit.SizeHeading, kit.WeightSemibold, "Design Collective"},
			{"Title 18 / 600", kit.SizeTitle, kit.WeightSemibold, "design-crit"},
			{"Label 15 / 500", kit.SizeLabel, kit.WeightMedium, "Mute for 2 hours"},
			{"Body 14 / 400", kit.SizeBody, kit.WeightRegular, "Pushed the new rail spacing, have a look when you can."},
			{"Small 13 / 400", kit.SizeSmall, kit.WeightRegular, "Mira: pushed the new rail spacing"},
			{"Caption 11 / 500", kit.SizeCaption, kit.WeightMedium, "14:02 · 3 unread"},
		} {
			ui.Row(c.Key(t.name)).Gap(kit.S4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, t.name).FontSize(kit.SizeCaption).Font("monospace").TextColor(p.TextMuted).Width(140).Shrink(0)
				ui.Text(c, t.sample).FontSize(t.size).FontWeight(t.weight).SingleLine().Grow(1).MinWidth(0)
			})
		}
	})
}

func (a *App) spacingPage(c *ui.Context) {
	a.specimen(c, "Spacing", "A 4 DIP grid: 2 4 6 8 12 16 20 24 32.", false, func() {
		p := kit.Of(c)
		ui.Row(c).Gap(kit.S4).AlignItems(ui.End).Wrap().Children(func() {
			for _, s := range []float32{2, 4, 6, 8, 12, 16, 20, 24, 32} {
				ui.Column(c.Key(s)).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(s, s).Background(p.Accent)
					ui.Text(c, strconv.Itoa(int(s))).FontSize(kit.SizeCaption).TextColor(p.TextMuted)
				})
			}
		})
	})
	a.specimen(c, "Radius", "6 controls · 10 cards, popovers, menus · 14 sheets, dialogs · full for avatars, chips and badges.", false, func() {
		p := kit.Of(c)
		ui.Row(c).Gap(kit.S4).Wrap().Children(func() {
			for _, r := range []struct {
				n string
				v float32
			}{{"6", kit.RadiusSm}, {"10", kit.RadiusMd}, {"14", kit.RadiusLg}, {"full", 32}} {
				ui.Column(c.Key(r.n)).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(64, 64).Radius(r.v).Background(p.BgInset).Border(1, p.BorderStrong)
					ui.Text(c, r.n).FontSize(kit.SizeCaption).TextColor(p.TextMuted)
				})
			}
		})
	})
	a.specimen(c, "Elevation", "Only floating layers cast a shadow, one soft one.", false, func() {
		p := kit.Of(c)
		ui.Row(c).Gap(kit.S6).Padding(kit.S3).Children(func() {
			ui.Box(c).Size(140, 80).Radius(kit.RadiusMd).Background(p.Bg).Border(1, p.Border).AlignItems(ui.Center).Justify(ui.Center).
				Children(func() { ui.Text(c, "Flat").FontSize(kit.SizeSmall).TextColor(p.TextMuted) })
			ui.Box(c).Size(140, 80).Radius(kit.RadiusMd).Background(p.Raised).Border(1, p.Border).
				Shadow(0, 8, 24, -4, p.Shadow).Shadow(0, 1, 2, 0, p.Shadow).AlignItems(ui.Center).Justify(ui.Center).
				Children(func() { ui.Text(c, "Floating").FontSize(kit.SizeSmall).TextColor(p.TextMuted) })
		})
	})
}

var iconList = []struct {
	n string
	i *ui.SVG
}{
	{"hash", kit.IconHash}, {"search", kit.IconSearch}, {"plus", kit.IconPlus}, {"settings", kit.IconSettings},
	{"bell", kit.IconBell}, {"bell-off", kit.IconBellOff}, {"star", kit.IconStar}, {"phone", kit.IconPhone},
	{"phone-off", kit.IconPhoneOff}, {"video", kit.IconVideo}, {"video-off", kit.IconVideoOff}, {"mic", kit.IconMic},
	{"mic-off", kit.IconMicOff}, {"screen", kit.IconScreen}, {"paperclip", kit.IconPaperclip}, {"smile", kit.IconSmile},
	{"send", kit.IconSend}, {"x", kit.IconX}, {"check", kit.IconCheck}, {"chevron", kit.IconChevronR},
	{"more", kit.IconMore}, {"lock", kit.IconLock}, {"shield", kit.IconShield}, {"shield-x", kit.IconShieldX},
	{"home", kit.IconHome}, {"at", kit.IconAt}, {"reply", kit.IconReply}, {"image", kit.IconImage},
	{"file", kit.IconFile}, {"link", kit.IconLink}, {"calendar", kit.IconCalendar}, {"user", kit.IconUser},
	{"users", kit.IconUsers}, {"message", kit.IconMessage}, {"pencil", kit.IconPencil}, {"trash", kit.IconTrash},
	{"log-out", kit.IconLogOut}, {"arrow-down", kit.IconArrowDown}, {"filter", kit.IconFilter}, {"key", kit.IconKey},
	{"camera", kit.IconCamera}, {"globe", kit.IconGlobe}, {"sticker", kit.IconSticker}, {"gif", kit.IconGIF},
}

func (a *App) iconsPage(c *ui.Context) {
	a.specimen(c, "Icons", "Lucide, stroked at 1.75, shown at 20. In the text's color, muted until hovered in toolbars.", false, func() {
		p := kit.Of(c)
		ui.Row(c).Wrap().Gap(kit.S1).Children(func() {
			for _, ic := range iconList {
				ui.Column(c.Key(ic.n)).Width(76).Height(64).Gap(kit.S15).AlignItems(ui.Center).Justify(ui.Center).
					Radius(kit.RadiusSm).Children(func() {
					ui.Icon(c, ic.i).Size(20, 20).TextColor(p.Text)
					ui.Text(c, ic.n).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
				})
			}
		})
	})
}

func (a *App) motionPage(c *ui.Context) {
	p := kit.Of(c)
	a.specimen(c, "Rules", "", true, func() {
		for _, r := range []string{
			"Animate only to explain a change. Never on keyboard-driven, frequent UI: the ⌘K switcher, switching rooms, arrowing through lists.",
			"Enter with ease-out, exit faster (150 ms in, 100 ms out). Nothing grows from nothing: start at 0.97 and fade.",
			"Popovers and menus come from their trigger; dialogs stay centered.",
			"Buttons press to 0.97. Rows change color instead.",
			"Sheets follow the finger and keep its velocity; a flick closes them.",
			"New messages rise 8 DIPs as they arrive live, never when history loads.",
			"Reduce Motion keeps the fades and drops the movement.",
		} {
			ui.Row(c.Key(r)).Gap(kit.S2).Children(func() {
				ui.Text(c, "·").TextColor(p.Accent).FontWeight(kit.WeightSemibold)
				ui.Text(c, r).FontSize(kit.SizeBody).TextColor(p.Text).Grow(1)
			})
		}
	})
	a.specimen(c, "Try it", "", true, func() {
		ui.Row(c).Gap(kit.S2).Wrap().Children(func() {
			kit.Button(c, kit.Primary, "Press me", nil)
			more := kit.Button(c, kit.Secondary, "Menu from trigger", kit.IconChevronD)
			more.PopoverMenuButton(func(m *ui.Menu) {
				m.Item("Mark as read")
				m.Item("Copy link")
				m.Separator()
				m.Item("Leave").Danger()
			}, ui.PopoverMenuButtonOptions{Panel: kit.MenuStyle(c).Panel})
			if kit.Button(c, kit.Secondary, "Open sheet", nil).Clicked() {
				a.sheet.Open = true
			}
			if kit.Button(c, kit.Secondary, "Open dialog", nil).Clicked() {
				a.dialog = true
			}
		})
		a.overlays(c)
	})
}

// overlays builds the gallery's sheet and dialog, wherever they are opened.
func (a *App) overlays(c *ui.Context) {
	p := kit.Of(c)
	kit.Sheet(c, &a.sheet, "Notifications", func() {
		ui.Text(c, "Drag the grabber down, flick it, tap outside or press Escape.").FontSize(kit.SizeBody).TextColor(p.TextMuted)
		if d := kit.MuteList(c, a.now); d != nil {
			a.did = fmt.Sprint("Muted ", kit.MutedUntilText(muteEnd(a, *d), a.now))
			a.sheet.Open = false
		}
	})
	kit.Dialog(c, &a.dialog, "Leave design-crit?", 400, func() {
		ui.Text(c, "You can rejoin later if the room is public. Its history stays on the server.").FontSize(kit.SizeBody).TextColor(p.TextMuted)
		ui.Row(c).Gap(kit.S2).Justify(ui.End).Children(func() {
			if kit.Button(c, kit.Secondary, "Cancel", nil).Clicked() {
				a.dialog = false
			}
			if kit.Button(c, kit.Danger, "Leave", nil).Clicked() {
				a.dialog = false
				a.did = "Left room"
			}
		})
	})
}

func (a *App) buttonsPage(c *ui.Context) {
	a.specimen(c, "Buttons", "One primary per view. Secondary for the rest, ghost in toolbars, danger only to confirm.", false, func() {
		ui.Row(c).Gap(kit.S2).Wrap().AlignItems(ui.Center).Children(func() {
			kit.Button(c, kit.Primary, "Join room", nil)
			kit.Button(c, kit.Secondary, "Invite", kit.IconUser)
			kit.Button(c, kit.Ghost, "Cancel", nil)
			kit.Button(c, kit.Danger, "Leave", kit.IconLogOut)
			kit.Button(c, kit.Primary, "Disabled", nil).Disabled(true)
		})
		ui.Row(c).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
			kit.IconButton(c, kit.Ghost, kit.IconPhone, "Start call")
			kit.IconButton(c, kit.Ghost, kit.IconSearch, "Search")
			kit.IconButton(c, kit.Ghost, kit.IconMore, "More")
			kit.IconButton(c, kit.Secondary, kit.IconPlus, "New room")
			kit.IconButton(c, kit.Primary, kit.IconSend, "Send")
		})
	})
}

func (a *App) inputsPage(c *ui.Context) {
	a.specimen(c, "Fields", "Inset fill; the hairline turns purple with focus.", false, func() {
		kit.TextField(c.Key("name"), &a.name, "Display name", "How others see you").Width(320)
		kit.SearchField(c.Key("search"), &a.query, "Search messages", kit.CmdK()).Width(320)
		kit.SearchButton(c.Key("sb"), "Search", kit.CmdK()).Width(320)
	})
	a.specimen(c, "Filter chips & tokens", "Chips filter the room list; tokens are search filters (in:, from:, has:, before:).", false, func() {
		ui.Row(c).Gap(kit.S1 + 2).Wrap().Children(func() {
			for i, l := range []string{"All", "Unread", "Favourites", "People"} {
				if kit.Chip(c.Key(l), l, a.chip == i).Clicked() {
					a.chip = i
				}
			}
		})
		ui.Row(c).Gap(kit.S1 + 2).Wrap().Children(func() {
			for i := 0; i < len(a.tokens); i++ {
				t := a.tokens[i]
				k, v := t, ""
				for j := range t {
					if t[j] == ':' {
						k, v = t[:j], t[j+2:]
						break
					}
				}
				if kit.TokenChip(c.Key(t), k, v) {
					a.tokens = append(a.tokens[:i], a.tokens[i+1:]...)
				}
			}
		})
	})
	a.specimen(c, "Tabs", "", false, func() {
		kit.Tabs(c.Key("tabs"), &a.tab, kit.PickerTabs...).Width(400)
	})
}

func (a *App) avatarsPage(c *ui.Context) {
	a.specimen(c, "Avatars", "Pictures cropped round (spaces: rounded squares). Without one: initials on the user's color, tinted, not loud.", false, func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Wrap().Children(func() {
			for i, id := range people {
				s := []float32{24, 32, 36, 40, 64, 96}[i]
				kit.Avatar(c.Key(id), kit.AvatarSpec{Name: names[id], ID: id, Size: s, Colors: &a.colors})
			}
		})
		ui.Row(c).Gap(kit.S4).AlignItems(ui.Center).Wrap().Children(func() {
			kit.Avatar(c.Key("p1"), kit.AvatarSpec{Name: "Mira", ID: "@mira:matrix.org", Size: 40, Presence: kit.PresenceOnline})
			kit.Avatar(c.Key("p2"), kit.AvatarSpec{Name: "Sam", ID: "@sam:matrix.org", Size: 40, Presence: kit.PresenceAway})
			kit.Avatar(c.Key("p3"), kit.AvatarSpec{Name: "Alex Rivera", ID: "@alex:example.com", Size: 40, Trust: kit.TrustVerified})
			kit.Avatar(c.Key("p4"), kit.AvatarSpec{Name: "Jun", ID: "@jun:kde.org", Size: 40, Trust: kit.TrustWarning})
			kit.Avatar(c.Key("p5"), kit.AvatarSpec{Name: "Design Collective", ID: "!design", Size: 44, Square: true})
			kit.Avatar(c.Key("p6"), kit.AvatarSpec{Name: "🎉 Party", ID: "!party", Size: 44, Square: true})
			kit.Avatar(c.Key("p7"), kit.AvatarSpec{Name: "", ID: "!empty", Size: 40})
		})
	})
	a.specimen(c, "Your picture", "Hover shows the camera; the pencil badge always says it can change (Remote, CREME on Mobbin). Opens: Take photo · Choose · Remove.", true, func() {
		if kit.AvatarEditor(c, kit.AvatarSpec{Name: "You", ID: "@me:matrix.org", Size: 96}).Clicked() {
			a.did = "Change picture"
		}
	})
	a.specimen(c, "Badges", "Neutral for unread, purple for mentions; 999+ caps the count. A dot for unread in muted rooms.", false, func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			kit.Count(c, 1, false)
			kit.Count(c, 42, false)
			kit.Count(c, 3, true)
			kit.Count(c, 12000, true)
			kit.Dot(c, false)
			kit.Dot(c, true)
			kit.Pill(c, "Encrypted")
			kit.Pill(c, "Admin")
		})
	})
}

func (a *App) roomListPage(c *ui.Context) {
	a.specimen(c, "Rail & room list", "Spaces on the rail (an indicator grows from an unread pip to a bar when selected), the space's rooms in sections. Right-click a row, or ⋯, for its actions.", false, func() {
		p := kit.Of(c)
		ui.Row(c).Height(560).Radius(kit.RadiusMd).Border(1, p.Border).Clip().AlignItems(ui.Stretch).Children(func() {
			a.rail(c)
			a.roomList(c, kit.RoomListWidth)
		})
	})
}

func (a *App) timelinePage(c *ui.Context) {
	a.specimen(c, "Timeline", "Flat rows, grouped by sender: names in their colors, the time in the gutter on hover, a toolbar of actions on hover. Mentions are tinted with a purple edge.", false, func() {
		p := kit.Of(c)
		ui.Column(c).Radius(kit.RadiusMd).Border(1, p.Border).Background(p.Bg).Clip().PaddingY(kit.S3).Children(func() {
			a.timeline(c)
		})
	})
}

func (a *App) composerPage(c *ui.Context) {
	a.specimen(c, "Composer", "Enter sends, Shift+Enter breaks the line. Send shows once there's text. The GIF button opens the picker (a sheet on phones).", true, func() {
		a.composerBox(c)
	})
}

// composerBox is the composer with its picker, shared by the pages.
func (a *App) composerBox(c *ui.Context) {
	r := kit.Composer(c.Key("composer"), &a.composer, "Message #design-crit", &a.colors)
	if r.GIF || r.Emoji {
		a.picker.Open = !a.picker.Open
		if r.Emoji {
			a.picker.Tab = 0
		} else {
			a.picker.Tab = 1
		}
	}
	anchor := r.GIFButton
	if a.picker.Tab == 0 && r.EmojiButton.Valid() {
		anchor = r.EmojiButton
	}
	if pr := kit.Picker(c, anchor, &a.picker, gifs); pr.Emoji != "" {
		a.composer.Text += pr.Emoji
	} else if pr.GIF != nil {
		a.did = "Send " + pr.GIF.Label
	}
	if r.Send {
		a.sent = append(a.sent, kit.MessageData{ID: "s" + strconv.Itoa(len(a.sent)), Sender: "You", SenderID: "@me:matrix.org",
			Time: a.now.Format("15:04"), Body: a.composer.Text, Reply: a.composer.Reply, First: len(a.sent) == 0})
		a.composer.Text, a.composer.Reply = "", nil
	}
}

func (a *App) menusPage(c *ui.Context) {
	a.specimen(c, "Room menu", "Right-click (or ⋯) on desktop; a sheet on phones. Mute shows until when each duration lasts.", true, func() {
		st := a.roomStates["!r1"]
		ui.Row(c).Gap(kit.S2).Children(func() {
			b := kit.Button(c, kit.Secondary, "design-crit", kit.IconMore)
			b.PopoverMenuButton(func(m *ui.Menu) { a.applyRoom("!r1", kit.RoomMenu(m, *st, a.now)) },
				ui.PopoverMenuButtonOptions{Panel: kit.MenuStyle(c).Panel})
			if kit.Button(c, kit.Secondary, "Mute sheet (phone)", kit.IconBellOff).Clicked() {
				a.sheet.Open = true
			}
			if kit.Button(c, kit.Secondary, "Confirm dialog", nil).Clicked() {
				a.dialog = true
			}
		})
		p := kit.Of(c)
		label := "Not muted"
		if st.Muted {
			label = kit.MutedUntilText(st.MutedUntil, a.now)
		}
		ui.Text(c, label).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		a.overlays(c)
	})
}

func (a *App) switcherPage(c *ui.Context) {
	p := kit.Of(c)
	a.specimen(c, "Quick switcher", "Press "+kit.CmdK()+" anywhere. Fuzzy over rooms, people and spaces; mentions and unread rank first, then recent. ↑↓ choose, ↵ opens, Tab filters. No open/close animation.", true, func() {
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			if kit.Button(c, kit.Primary, "Open switcher", kit.IconSearch).Clicked() {
				a.switcher.Toggle()
			}
			kit.Kbd(c, kit.CmdK())
		})
		ui.Text(c, "References: Devin, StackAI, Claude (Mobbin): top-anchored panel, grouped results, key hints in a footer.").
			FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	})
}

func (a *App) verifyPage(c *ui.Context) {
	a.specimen(c, "Verify this device", "", false, func() {
		ui.Column(c).MaxWidth(420).Children(func() {
			if ch := kit.VerifyStart(c); ch != kit.VerifyNone {
				a.did = []string{"", "Verify with device", "Verify with key", "Reset"}[ch]
			}
		})
	})
	a.specimen(c, "Compare emoji", "", false, func() {
		ui.Column(c).MaxWidth(420).Children(func() {
			switch kit.VerifyEmoji(c, "Alex's phone", sas) {
			case kit.VerifyMatch:
				a.did = "Verified"
			case kit.VerifyMismatch:
				a.did = "Cancelled: emoji didn't match"
			}
		})
	})
}

func (a *App) callsPage(c *ui.Context) {
	a.specimen(c, "Call", "One kind of call: it starts with the camera off and the mic on. Jitsi runs underneath in a web view, its own toolbar hidden; these controls are native.", false, func() {
		p := kit.Of(c)
		kit.CallBanner(c, 3, false)
		ui.Column(c).Height(280).Radius(kit.RadiusMd).Background(p.BgSubtle).Padding(kit.S4).Gap(kit.S4).Children(func() {
			ui.Row(c).Grow(1).Gap(kit.S3).Children(func() {
				for _, id := range people[:3] {
					ui.Column(c.Key(id)).Grow(1).Basis(0).Radius(kit.RadiusMd).Background(p.BgInset).
						AlignItems(ui.Center).Justify(ui.Center).Gap(kit.S2).Children(func() {
						kit.Avatar(c, kit.AvatarSpec{Name: names[id], ID: id, Size: 56})
						ui.Text(c, names[id]).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium)
					})
				}
			})
			if kit.CallBar(c, &a.call).Leave {
				a.did = "Left call"
			}
		})
	})
}

func (a *App) colorsUserPage(c *ui.Context) {
	a.specimen(c, "Your name's color", "Auto hashes your ID; or pick one of eight that read well in both themes. Others' colors can be overridden locally from their profile.", false, func() {
		kit.SwatchPicker(c, &a.color, "You", "@me:matrix.org")
	})
	a.colors.Set("@me:matrix.org", a.color) // -1 (Auto) clears it
}
