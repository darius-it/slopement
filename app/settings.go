package app

import (
	"strings"
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
	{"GIFs", kit.IconFolder},
	{"Notifications", kit.IconBell},
	{"Privacy", kit.IconLock},
	{"Security", kit.IconShield},
	{"Calls", kit.IconPhone},
	{"Advanced", kit.IconSettings},
	{"About", kit.IconHelp},
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
	// accent is the tint (an index of kit.AccentPresets), uiSize the size of
	// the interface (an index of uiSizes), dense the closer timeline.
	accent, uiSize int
	dense          bool
	// usernames: people are shown by username ("ada"), not display name
	// (names.go); the default, display names, is what other clients show.
	usernames bool
	// jumpLast: switching to Chats or a community reopens the chat last open
	// there (scope.go); off, the pane says "Select a chat".
	jumpLast bool
	// What a call starts with (callsession.go): the microphone off, the camera
	// on, and the page's own lobby (to choose them there) instead of joining at once.
	callMicOff, callCamOn, callLobby bool
	// callsAccepted: the user agreed to the call page loading from Element's
	// server (asked once, before the first call; Settings → Calls takes it back).
	callsAccepted bool
	// statusBubble: in the Rail layout the user's status shows beside their
	// picture as a bubble with its words, always, and a press edits it.
	statusBubble bool
}

// uiSizes are the interface sizes to choose from: the window's zoom.
var uiSizes = []struct {
	Name string
	Zoom float32
}{{"Default", 1}, {"Small", 0.9}, {"Large", 1.12}, {"Larger", 1.25}}

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
	if a.settings != "Account" {
		a.nameLoaded = false // the name field takes the profile's again next visit
	}
	if a.settings != "Security" {
		a.sessionsAsked = false // and the devices are asked for again
		a.keysAsked = false     // and where the keys stand
	}
	switch a.settings {
	case "Account":
		group(c, "Profile", "", func() {
			ui.Row(c).Padding(kit.S3).Gap(kit.S4).AlignItems(ui.Center).Children(func() {
				if kit.AvatarEditor(c, a.avatar(me, 64)).Clicked() {
					a.pickAvatar()
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
			// The name, and the picture's removal: only where the account can change them.
			if _, ok := a.src.(model.Profile); ok {
				if !a.nameLoaded {
					a.nameDraft, a.nameLoaded = me.Name, true
				}
				ui.Column(c).Padding(0, kit.S3, kit.S3, kit.S3).Gap(kit.S2).Children(func() {
					in := kit.TextField(c.Key("myname"), &a.nameDraft, "Display name", me.Name)
					ui.Row(c).Gap(kit.S2).Children(func() {
						changed := strings.TrimSpace(a.nameDraft) != "" && strings.TrimSpace(a.nameDraft) != me.Name
						if (kit.Button(c, kit.Primary, "Save name", nil).Clicked() || in.Submitted()) && changed {
							a.saveName()
						}
						if me.Avatar != "" && kit.Button(c, kit.Ghost, "Remove picture", nil).Clicked() {
							a.removeAvatar()
						}
					})
				})
			}
		})
		group(c, "Your name's colour", "Saved with your account. You can recolour anyone for yourself from their profile.", func() {
			ui.Column(c).Padding(kit.S3).Children(func() {
				sel := a.colorOf(me.ID)
				was := sel
				kit.SwatchPicker(c, &sel, me.Name, me.ID)
				if sel != was {
					a.chooseColor(me.ID, sel)
				}
			})
		})
		group(c, "", "", func() {
			if a.accounts != nil && a.accounts.Remove != nil {
				if kit.ChoiceRow(c, kit.IconLogOut, "Log out", "Keeps your keys backed up if key storage is on").Clicked() {
					a.removing, a.removeAccount.Open = me.ID, true
				}
			}
		})
	case "GIFs":
		a.gifsPage(c)
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
		group(c, "Accent", "The one tint the app uses for selection, buttons and mentions.", func() {
			ui.Row(c).Padding(kit.S3).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				for i, pr := range kit.AccentPresets {
					if accentSwatch(c, pr, i == a.prefs.accent).Clicked() {
						a.prefs.accent = i
					}
				}
			})
		})
		group(c, "Size", "Makes everything in the window larger or smaller. Compact timeline brings messages closer together.", func() {
			ui.Column(c).Padding(kit.S2).Children(func() {
				names := make([]string, len(uiSizes))
				for i, s := range uiSizes {
					names[i] = s.Name
				}
				kit.Tabs(c.Key("uisize"), &a.prefs.uiSize, names...)
			})
			switchRow(c, "Compact timeline", "Less space between messages", &a.prefs.dense)
		})
		group(c, "Layout", "How Chats and Communities are laid out. At phone widths the rail shows as tabs.", func() {
			ui.Column(c).Padding(kit.S3).Children(func() { kit.LayoutPicker(c, &a.nav) })
		})
		// The small switches of how chats and people look and behave: one group,
		// a line under each, no notes between them.
		group(c, "Chats and people", "", func() {
			switchRow(c, "Show usernames", "Instead of display names", &a.prefs.usernames)
			switchRow(c, "Status on my picture", "Rail layout: show its words in a small cloud, not just the emoji", &a.prefs.statusBubble)
			switchRow(c, "Jump to last chat", "When switching between Chats and communities", &a.prefs.jumpLast)
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
			switchRow(c, "Animate profile pictures", "Off: a GIF picture plays only while the pointer is on it, and in a profile card. On: always.", &a.prefs.animatedAvatars)
			switchRow(c, "Hide joins and leaves", "Lines like “Mira joined” and “Sam left” are not shown in chats", &a.prefs.hideJoins)
		})
	case "Notifications":
		group(c, "This device", "", func() {
			switchRow(c, "Desktop notifications", "", &a.prefs.notifyDesktop)
			switchRow(c, "Sound", "", &a.prefs.notifySound)
		})
		group(c, "Notify me for", "Chats with a person always notify; in rooms, a mention of you does. A room muted from its menu stays quiet.", func() {
			switchRow(c, "Mentions of my name", "In rooms", &a.prefs.notifyMentions)
		})
	case "Privacy":
		group(c, "Activity", "", func() {
			switchRow(c, "Read receipts", "Show who has read your messages, and let others see what you've read. Off: nothing is shown, and your receipts stay private.", &a.prefs.receipts)
			switchRow(c, "Send typing notifications", "Others in a chat see \"… is typing\" while you write", &a.prefs.typing)
		})
		a.ignoredGroup(c)
		a.updatesPrivacyGroup(c)
		group(c, "Link previews", "Previews in encrypted rooms send the link to your homeserver; fetching them from this device shows your IP address to the site.", func() {
			switchRow(c, "Previews in encrypted rooms", "", &a.prefs.previewsE2EE)
			switchRow(c, "Fetch previews from this device", "Off: your homeserver fetches them and sites never see you. On: this device does, no link goes to the homeserver, and every site linked sees your IP address. Addresses on your own network are never fetched.", &a.prefs.previewsDirect)
		})
	case "Security":
		group(c, "This session", "", func() {
			if kit.ChoiceRow(c, kit.IconShield, "Verify this session", "With another device (emoji) or your Security Key").Clicked() {
				a.verify.Open, a.verifyStep = true, 0
			}
			a.keyRows(c)
		})
		a.resetIdentityGroup(c)
		// The devices are asked for once each time the page opens.
		if sm, ok := a.src.(model.SessionManager); ok && !a.sessionsAsked {
			a.sessionsAsked = true
			sm.RefreshSessions()
		}
		a.sessions(c)
	case "Calls":
		group(c, "Call system", callsNote, func() {
			if _, ok := a.src.(model.CallHoster); ok {
				ui.Text(c, "Your account can make calls where the server offers them.").FontSize(kit.SizeSmall).TextColor(kit.Of(c).TextMuted).Padding(kit.S2)
			} else {
				ui.Text(c, "This account cannot make calls.").FontSize(kit.SizeSmall).TextColor(kit.Of(c).TextMuted).Padding(kit.S2)
			}
			switchRow(c, "Load the call page from call.element.io", "Off: the next call asks first", &a.prefs.callsAccepted)
		})
		group(c, "Starting calls", "Applied when you join. The call page's own buttons change them as you go.", func() {
			switchRow(c, "Microphone off when joining", "Others do not hear you until you turn it on", &a.prefs.callMicOff)
			switchRow(c, "Camera on when joining", "Off: you join with the camera off and turn it on in the call", &a.prefs.callCamOn)
			switchRow(c, "Ask before joining", "Show the call page's lobby, to choose a microphone and camera first. It replaces the two above.", &a.prefs.callLobby)
		})
	case "Advanced":
		a.updatesAdvancedGroup(c)
		group(c, "Troubleshooting", "Resetting the cache never touches your encryption keys.", func() {
			if _, ok := a.src.(model.LogExporter); ok {
				if kit.ChoiceRow(c, kit.IconDownload, "Export logs", "A zip for bug reports, without message contents, names or tokens").Clicked() {
					a.exportLogs()
				}
			} else if kit.ChoiceRowSoon(c, kit.IconDownload, "Export logs", "Without message contents or tokens").Clicked() {
				a.toast = soonMsg
			}
			if cr, ok := a.src.(model.CacheResetter); ok {
				if kit.ChoiceRow(c, kit.IconTrash, "Reset local cache", "Syncs everything again the next time the app starts").Clicked() {
					cr.ResetCache(func() { a.toast = "Cache cleared. Restart the app to sync everything again." })
				}
			} else if kit.ChoiceRowSoon(c, kit.IconTrash, "Reset local cache", "Syncs everything again").Clicked() {
				a.toast = soonMsg
			}
		})
		if a.OpenGallery != nil {
			group(c, "Debug", "", func() {
				if kit.ChoiceRow(c, kit.IconEye, "Component gallery", "The design kit, every component in its states").Clicked() {
					a.OpenGallery()
				}
			})
		}
	case "About":
		a.aboutPage(c)
	}
}

// accentSwatch is one tint to choose: a disc of its color, ringed while it is
// the one in use. Its color is the tint's in the appearance shown.
func accentSwatch(c *ui.Context, pr kit.AccentPreset, selected bool) ui.Element {
	p := kit.Of(c)
	col := ui.Oklch(0.575+pr.Light, 0.165, pr.Hue).SRGB()
	if p.Dark {
		col = ui.Oklch(0.78+pr.Dark, 0.115, pr.Hue).SRGB()
	}
	b := ui.ButtonBase(c.Key("accent:"+pr.Name)).Size(34, 34).Radius(kit.RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
		Label(pr.Name + " accent").Tooltip(pr.Name).AccessSelected(selected)
	ring := ui.Transparent
	switch {
	case selected:
		ring = p.Text
	case b.Hovered():
		ring = p.BorderStrong
	}
	b.Border(2, b.AnimateColor("ring", ring, kit.Color))
	kit.Pressable(b)
	b.Children(func() { ui.Box(c).Size(24, 24).Radius(kit.RadiusFull).Background(col) })
	return b
}

// ignoredGroup lists the people ignored, each with a way to stop (nothing
// where the account cannot ignore, or no one is).
func (a *App) ignoredGroup(c *ui.Context) {
	ig, ok := a.src.(model.Ignorer)
	if !ok {
		return
	}
	p := kit.Of(c)
	list := ig.IgnoredUsers()
	if len(list) == 0 {
		return
	}
	group(c, "Ignored people", "Their messages and invitations are hidden. Ignore someone from their profile.", func() {
		for _, id := range list {
			id := id
			pe, _ := a.src.Person(id)
			ui.Row(c.Key("ign:"+id)).Padding(kit.S2, kit.S3).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
				kit.Avatar(c, a.avatar(pe, 28))
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, pe.Name).FontSize(kit.SizeBody).SingleLine()
					ui.Text(c, id).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
				})
				if kit.Button(c, kit.Ghost, "Unignore", nil).Label("Unignore " + pe.Name).Clicked() {
					a.setIgnore(id, false)
				}
			})
		}
	})
}

// sessionLine is a device's second line: what is known of it, joined.
func sessionLine(s model.Session, now time.Time) string {
	var parts []string
	for _, p := range []string{s.Client, s.IP} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if s.Current {
		parts = append(parts, "this device")
	} else if !s.LastSeen.IsZero() {
		parts = append(parts, "last seen "+timeLabel(s.LastSeen, now))
	}
	return strings.Join(parts, " · ")
}

// signOutSession signs another device out. A homeserver that wants it
// confirmed (as one with an account page does) is sent to, in the browser.
func (a *App) signOutSession(s model.Session) {
	sm, ok := a.src.(model.SessionManager)
	if !ok {
		a.toast = "This account can't sign sessions out here"
		return
	}
	sm.SignOutSession(s.ID, func(err string, needsBrowser bool) {
		switch {
		case err != "":
			a.toast = err
		case needsBrowser:
			page := sm.AccountPage()
			if page == "" {
				a.toast = "Your homeserver wants you to confirm this elsewhere: sign that session out from the app it runs in"
				return
			}
			a.toast = "Your homeserver asks you to confirm this on its account page"
			a.openURL = page // opened by the next frame, which has the services
		default:
			a.toast = "Signed out " + s.Name
		}
	})
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
						ui.Text(c, sessionLine(s, a.now)).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
					})
					if !s.Current && kit.Button(c, kit.Ghost, "Sign out", nil).Clicked() {
						a.signOutSession(s)
					}
				})
			}
		})
	}
}
