package app

import (
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

var settingsPages = []struct {
	name string
	icon *ui.SVG
}{
	{"Account", kit.IconUser},
	{"Appearance", kit.IconEye},
	{"Notifications", kit.IconBell},
	{"Privacy", kit.IconLock},
	{"Security", kit.IconShield},
	{"Calls", kit.IconPhone},
	{"Advanced", kit.IconSettings},
}

// settingsPrefs are the scaffold's settings (persisted to account data by
// the Matrix layer).
type settingsPrefs struct {
	notifyDesktop, notifySound, notifyMentions, notifyKeywords bool
	receipts, typing, previewsE2EE, previewsDirect             bool
	gifAutoplay, animatedAvatars, hideJoins                    bool
	linkPreviews, videoPreviews, gifPreviews                   bool
	myColor                                                    int
	peopleSub                                                  int // under names in People: kit.PeopleSubStatus or PeopleSubLastChat
	glass                                                      int // glassOff, glassTinted, glassClear
	// gifFiles: GIFs from the picker go as uploaded files, which every
	// client animates, instead of as links (the default, which asks nothing
	// of the homeserver's storage).
	gifFiles bool
}

func defaultPrefs() settingsPrefs {
	return settingsPrefs{notifyDesktop: true, notifySound: true, notifyMentions: true, typing: true,
		gifAutoplay: true, linkPreviews: true, videoPreviews: true, gifPreviews: true, myColor: -1, glass: glassTinted}
}

// settingsView is Settings: its pages beside (desktop) or over (phone)
// their content.
func (a *App) settingsView(c *ui.Context, back bool) {
	p := kit.Of(c)
	compact := kit.Compact(c)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		ui.Row(c).Height(52).PaddingX(kit.S4).Gap(kit.S2).AlignItems(ui.Center).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
			if back && kit.IconButton(c, kit.Ghost, kit.IconChevronL, "Back").Clicked() {
				a.view = viewRoom
			}
			ui.Text(c, "Settings").FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold).Grow(1)
			if !compact && kit.IconButton(c, kit.Ghost, kit.IconX, "Close settings").Clicked() {
				a.view = viewRoom
			}
		})
		if compact {
			ui.Scroll(c).Row().Gap(kit.S1).Scrollbars(false).Padding(kit.S2, kit.S3).Shrink(0).Children(func() {
				for _, pg := range settingsPages {
					if kit.Chip(c.Key("sp:"+pg.name), pg.name, a.settings == pg.name).Clicked() {
						a.settings = pg.name
					}
				}
			})
			ui.Scroll(c.Key("set:"+a.settings)).Grow(1).Padding(kit.S3, kit.S4).Gap(kit.S4).Children(func() { a.settingsPage(c) })
			return
		}
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(220).Shrink(0).Padding(kit.S3, kit.S2).Gap(2).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
				for _, pg := range settingsPages {
					a.settingsNav(c, pg.name, pg.icon)
				}
			})
			ui.Scroll(c.Key("set:"+a.settings)).Grow(1).Padding(kit.S6, kit.S8).Children(func() {
				ui.Column(c).MaxWidth(620).Gap(kit.S4).Children(func() { a.settingsPage(c) })
			})
		})
	})
}

func (a *App) settingsNav(c *ui.Context, name string, icon *ui.SVG) {
	p := kit.Of(c)
	sel := a.settings == name
	b := ui.ButtonBase(c.Key("sn:" + name)).Height(34).PaddingX(kit.S2).Gap(kit.S2).Radius(kit.RadiusSm).AlignItems(ui.Center).
		Justify(ui.Start).Label(name).AccessSelected(sel)
	bg, fg := ui.Transparent, p.TextMuted
	switch {
	case sel:
		bg, fg = p.AccentSubtle, p.AccentStrong
	case b.Hovered():
		bg, fg = p.Hover, p.Text
	}
	b.Background(b.AnimateColor("bg", bg, kit.Color))
	b.Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(fg)
		ui.Text(c, name).FontSize(kit.SizeBody).TextColor(fg).SingleLine()
	})
	if b.Clicked() {
		a.settings = name
	}
}

// group is a titled card of settings rows.
func group(c *ui.Context, title, note string, fn func()) {
	p := kit.Of(c)
	ui.Column(c.Key("g:" + title)).Gap(kit.S2).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).PaddingStart(kit.S1)
		}
		ui.Column(c).Radius(kit.RadiusMd).Background(p.Raised).Border(1, p.Border).Padding(kit.S1).Children(fn)
		if note != "" {
			ui.Text(c, note).FontSize(kit.SizeSmall).TextColor(p.TextMuted).PaddingStart(kit.S1)
		}
	})
}

func (a *App) settingsPage(c *ui.Context) {
	p := kit.Of(c)
	me := a.src.Me()
	switch a.settings {
	case "Account":
		group(c, "Profile", "", func() {
			ui.Row(c).Padding(kit.S3).Gap(kit.S4).AlignItems(ui.Center).Children(func() {
				if kit.AvatarEditor(c, a.avatar(me, 64)).Clicked() {
					a.toast = "Changing your picture comes with the Matrix layer"
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, me.Name).FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold)
					ui.Text(c, me.ID).FontSize(kit.SizeSmall).TextColor(p.TextMuted).Selectable()
					if me.Status.Set(a.now) {
						kit.StatusLine(c, status(me.Status), a.now, true) // your own: with when it clears
					}
				})
				if kit.Button(c, kit.Secondary, "Set a status", nil).Clicked() {
					a.statusDraft.Edit(status(me.Status))
				}
			})
		})
		group(c, "Your name's colour", "Others see it only in apps that read it; you can recolour anyone for yourself from their profile.", func() {
			ui.Column(c).Padding(kit.S3).Children(func() {
				was := a.prefs.myColor
				kit.SwatchPicker(c, &a.prefs.myColor, me.Name, me.ID)
				if a.prefs.myColor != was {
					a.colors.Set(me.ID, a.prefs.myColor)
				}
			})
		})
		group(c, "", "", func() {
			if kit.ChoiceRow(c, kit.IconLogOut, "Sign out", "Keeps your keys backed up if key storage is on").Clicked() {
				a.toast = "Signing out comes with the Matrix layer"
			}
		})
	case "Appearance":
		group(c, "Theme", "", func() {
			ui.Column(c).Padding(kit.S2).Children(func() {
				kit.Tabs(c.Key("theme"), &a.theme, "Match system", "Light", "Dark")
			})
		})
		group(c, "Window glass", "The desktop shows through the sidebars. Tinted keeps their contrast steady over any wallpaper; Clear shows the system's material as it is. Only while the theme matches the system (macOS, Windows 11).", func() {
			ui.Column(c).Padding(kit.S2).Children(func() {
				kit.Tabs(c.Key("glass"), &a.prefs.glass, "Off", "Tinted", "Clear")
			})
		})
		group(c, "Layout", "How Chats and Communities are laid out. At phone widths the rail shows as tabs.", func() {
			ui.Column(c).Padding(kit.S3).Children(func() { kit.LayoutPicker(c, &a.nav) })
		})
		group(c, "People", "What shows under each name in People: their status (or how they are, when they have none), or when you last chatted.", func() {
			ui.Column(c).Padding(kit.S2).Children(func() {
				kit.Tabs(c.Key("peoplesub"), &a.prefs.peopleSub, "Status", "Last chat")
			})
		})
		group(c, "Timeline", "Previews apply in chats, communities and rooms alike.", func() {
			switchRow(c, "Link previews", "A card for YouTube and other links", &a.prefs.linkPreviews)
			switchRow(c, "Video previews", "Video links show as the video", &a.prefs.videoPreviews)
			switchRow(c, "GIF previews", "A message that is only a GIF link shows as the GIF", &a.prefs.gifPreviews)
			switchRow(c, "Animate GIFs", "Only while they're on screen", &a.prefs.gifAutoplay)
			switchRow(c, "Send GIFs as files", "Off: the picker sends a link, which costs your homeserver nothing, and shows as the GIF here and in clients that preview links. On: the GIF is uploaded, so every client animates it.", &a.prefs.gifFiles)
			switchRow(c, "Animate profile pictures", "Otherwise only on hover", &a.prefs.animatedAvatars)
			switchRow(c, "Hide joins and leaves", "In large rooms they're collapsed anyway", &a.prefs.hideJoins)
		})
	case "Notifications":
		group(c, "This device", "", func() {
			switchRow(c, "Desktop notifications", "", &a.prefs.notifyDesktop)
			switchRow(c, "Sound", "", &a.prefs.notifySound)
		})
		group(c, "Notify me for", "Rooms can override this from their menu: all messages, mentions only, or muted for a while.", func() {
			switchRow(c, "Mentions of my name", "", &a.prefs.notifyMentions)
			switchRow(c, "Keywords", "Add words that should notify you", &a.prefs.notifyKeywords)
		})
	case "Privacy":
		group(c, "Activity", "", func() {
			switchRow(c, "Read receipts", "Show who has read your messages, and let others see what you've read. Off: nothing is shown, and your receipts stay private.", &a.prefs.receipts)
			switchRow(c, "Send typing notifications", "", &a.prefs.typing)
		})
		group(c, "Link previews", "Previews in encrypted rooms send the link to your homeserver; fetching them from this device shows your IP address to the site.", func() {
			switchRow(c, "Previews in encrypted rooms", "", &a.prefs.previewsE2EE)
			switchRow(c, "Fetch previews from this device", "", &a.prefs.previewsDirect)
		})
	case "Security":
		group(c, "This session", "", func() {
			if kit.ChoiceRow(c, kit.IconShield, "Verify this session", "With another device (emoji) or your Security Key").Clicked() {
				a.verify.Open, a.verifyStep = true, 0
			}
			if kit.ChoiceRow(c, kit.IconKey, "Key storage", "Backed up · in sync").Clicked() {
				a.toast = "Key backup comes with the Matrix layer"
			}
			if kit.ChoiceRow(c, kit.IconDownload, "Export room keys", "A file encrypted with a passphrase").Clicked() {
				a.toast = "Key export comes with the Matrix layer"
			}
		})
		a.sessions(c)
	case "Calls":
		group(c, "Call system", "Calls use MatrixRTC (Element Call) when your homeserver supports it, and Jitsi when it doesn't or MatrixRTC fails. A setting to prefer Jitsi comes in v1.1.", func() {
			ui.Row(c).Padding(kit.S3).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, kit.IconPhone).Size(16, 16).TextColor(p.TextMuted)
				ui.Text(c, "MatrixRTC, with Jitsi as the fallback").FontSize(kit.SizeBody).Grow(1)
			})
		})
		group(c, "Starting calls", "", func() {
			on := true
			switchRow(c, "Camera off when joining", "Turn it on in the call", &on)
		})
	case "Advanced":
		group(c, "Troubleshooting", "Resetting the cache never touches your encryption keys.", func() {
			if kit.ChoiceRow(c, kit.IconDownload, "Export logs", "Without message contents or tokens").Clicked() {
				a.toast = "Log export comes with the Matrix layer"
			}
			if kit.ChoiceRow(c, kit.IconTrash, "Reset local cache", "Syncs everything again").Clicked() {
				a.toast = "Cache reset comes with the Matrix layer"
			}
		})
		if a.OpenGallery != nil {
			group(c, "Debug", "", func() {
				if kit.ChoiceRow(c, kit.IconEye, "Component gallery", "The design kit, every component in its states").Clicked() {
					a.OpenGallery()
				}
			})
		}
		ui.Text(c, "slopement 0.1 · mock data").FontSize(kit.SizeSmall).TextColor(p.TextFaint).PaddingStart(kit.S1)
	}
}

// sessions lists the user's devices: current, verified, unverified,
// inactive (90 days).
func (a *App) sessions(c *ui.Context) {
	p := kit.Of(c)
	groups := map[string][]model.Session{}
	order := []string{"Current", "Verified", "Unverified", "Inactive"}
	for _, s := range a.src.Sessions() {
		switch {
		case s.Current:
			groups["Current"] = append(groups["Current"], s)
		case a.now.Sub(s.LastSeen) > 90*24*time.Hour:
			groups["Inactive"] = append(groups["Inactive"], s)
		case s.Verified:
			groups["Verified"] = append(groups["Verified"], s)
		default:
			groups["Unverified"] = append(groups["Unverified"], s)
		}
	}
	for _, g := range order {
		list := groups[g]
		if len(list) == 0 {
			continue
		}
		group(c, g+" sessions", "", func() {
			for _, s := range list {
				ui.Row(c.Key("s:"+s.ID)).Padding(kit.S2, kit.S3).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
					ic, col := kit.IconShield, p.Success
					if !s.Verified {
						ic, col = kit.IconShieldX, p.Warning
					}
					ui.Icon(c, ic).Size(18, 18).TextColor(col)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, s.Name).FontSize(kit.SizeBody).SingleLine()
						ui.Text(c, s.Client+" · "+s.IP+" · last seen "+timeLabel(s.LastSeen, a.now)).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
					})
					if !s.Current && kit.Button(c, kit.Ghost, "Sign out", nil).Clicked() {
						a.toast = "Signing out sessions comes with the Matrix layer"
					}
				})
			}
		})
	}
}
