// Package app is slopement itself: the shell (rail, lists, the open room,
// the side panel), People, Settings and every overlay, on a model.Source.
// It is built from package kit alone; the Matrix layer (to come) and the
// mock (model/mock) feed it the same way.
package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/picker"
	"slopement/platform"
)

// view is what the main area shows.
type view int

const (
	viewRoom view = iota
	viewPeople
	viewSettings
)

// panel is the right panel beside the room; it stays open across rooms.
type panel int

const (
	panelNone panel = iota
	panelInfo
	panelMembers
)

// App is the app's UI state. The data is the Source's.
type App struct {
	src    model.Source
	pick   *picker.Controller
	now    time.Time
	colors kit.UserColors

	// Appearance.
	theme int // 0 the system's, 1 light, 2 dark
	nav   kit.NavLayout

	// Where the user is.
	view      view
	room      string // the open room ("" none)
	community string // the community open (Rail: its rail item; Tabs: opened)
	tab       int    // Tabs: 0 Chats, 1 Communities
	filter    int    // One inbox: All, People, Communities, Unread
	expanded  map[string]bool
	phoneTab  int  // compact: Chats, Communities, People, Settings
	pushed    bool // compact: the room is pushed over the lists
	panel     panel
	sidebar   bool // the list column is shown (desktop)
	settings  string
	lastRooms []string // most recent first, for the switcher

	// Per-room.
	drafts   map[string]*kit.ComposerState
	lists    map[string]*ui.ListState
	scrolls  map[string]*ui.ScrollState
	reveals  kit.Reveals
	docs     map[string]parsed
	roomMenu map[string]bool

	// The state file (uistate.go): its path, what was last written, a
	// write already scheduled, and the room and community of the last run
	// that are not in the Source yet.
	statePath        string
	stateLast        string
	statePending     bool
	rs               receiptState // the open room's, taken once a frame
	receiptsApplied  bool         // the source was told the setting (receipts.go)
	receiptsVal      bool
	restoreRoom      string
	restoreCommunity string
	restorePushed    bool

	// Attachments: decoded pictures by mxc URL (bounded, oldest out),
	// and the ones being fetched.
	stills     map[string]ui.ImageSource
	stillOrder []string
	stillBusy  map[string]bool

	// Overlays.
	switcher     kit.SwitcherState
	statusDraft  kit.StatusDraft
	pollSheet    kit.SheetState
	pollDraft    kit.PollDraft
	pollRoom     string
	profileOpen  string // People/header: the person whose card is open
	profileSheet kit.SheetState
	profileMsg   string // the message whose sender's card is open
	leave        kit.SheetState
	leaving      string
	picker       kit.PickerState
	pickerRoom   string
	viewer       bool
	viewerItems  []kit.MediaSpec
	viewerAt     int
	people       kit.PeopleState
	call         kit.CallState
	inCall       string // the room whose call the user is in
	verify       kit.SheetState
	verifyStep   int    // 0 the flow; 1 the Security Key entry
	verifyKey    string // the key being typed
	verifier     model.Verify
	verifyAsked  bool // the once-a-run prompt has shown
	toast        string
	reactTo      string            // the message the picker reacts to
	replyTo      map[string]string // per room: the event replied to
	roomInfo     kit.SheetState    // phones: room info
	roomSheet    kit.SheetState    // phones: a room's actions
	roomSheetID  string
	prefs        settingsPrefs

	win *mygo.Window
	// MenuChanged, set by main, rebuilds the menu bar when what it checks
	// changes; OpenGallery opens the design kit's gallery (Debug menu).
	MenuChanged func()
	OpenGallery func()
	menuNav     kit.NavLayout
	menuTheme   int
}

// New returns the app on src; pick is the GIF picker's controller (nil: no
// picker, as in tests).
func New(src model.Source, pick *picker.Controller) *App {
	a := &App{
		src:       src,
		pick:      pick,
		verifier:  src.Verify(),
		now:       time.Now(),
		nav:       kit.DefaultNavLayout(),
		sidebar:   true,
		settings:  "Account",
		expanded:  map[string]bool{},
		drafts:    map[string]*kit.ComposerState{},
		lists:     map[string]*ui.ListState{},
		scrolls:   map[string]*ui.ScrollState{},
		reveals:   kit.Reveals{},
		docs:      map[string]parsed{},
		roomMenu:  map[string]bool{},
		stills:    map[string]ui.ImageSource{},
		stillBusy: map[string]bool{},
		call:      kit.NewCall(),
		replyTo:   map[string]string{},
		prefs:     defaultPrefs(),
	}
	a.pollDraft = kit.NewPollDraft()
	a.menuNav = a.nav
	// Open the most recent chat, as a messenger does.
	for _, r := range src.Rooms() {
		if r.Community == "" {
			a.open(r.ID)
			break
		}
	}
	return a
}

// Attach connects the app to its window: the source's and picker's
// background work comes back through it.
func (a *App) Attach(w *mygo.Window) {
	a.win = w
	a.src.SetUpdate(w.Update)
	if a.pick != nil {
		a.pick.SetUpdate(w.Update)
	}
}

// open opens a room: it becomes the open room, is marked read, and leads
// the switcher's recents.
func (a *App) open(id string) {
	if _, ok := a.src.Room(id); !ok {
		return
	}
	a.room, a.view, a.pushed = id, viewRoom, true
	a.restoreRoom = "" // the user chose; the last run's room no longer applies
	a.profileMsg = ""
	a.src.MarkRead(id)
	recent := []string{id}
	for _, r := range a.lastRooms {
		if r != id && len(recent) < 20 {
			recent = append(recent, r)
		}
	}
	a.lastRooms = recent
}

// View is the window's content.
func (a *App) View(c *ui.Context) {
	a.now = time.Now()
	a.receiptsSync()
	switch a.theme {
	case 1:
		kit.ApplyTheme(c, false)
	case 2:
		kit.ApplyTheme(c, true)
	default:
		kit.Apply(c)
	}
	kit.SwitcherShortcut(c, &a.switcher)
	if a.glass(c) {
		kit.WindowChrome(c) // the bar and sidebars show the window's material
	}
	root := ui.Column(c).Fill()
	root.Children(func() {
		// The window's controls sit in one thin bar across the top.
		if bar := c.TitleBar(); bar.Height > 0 || bar.Bottom > 0 {
			a.topBar(c)
		}
		if kit.Compact(c) {
			a.phone(c)
		} else {
			a.desktop(c)
		}
	})
	a.overlays(c)
	kit.Toasts(c)
	if a.toast != "" {
		c.AddToast(ui.Toast{Title: a.toast})
		a.toast = ""
	}
	// The menu bar checks the layout and theme: rebuild it when they change.
	if (a.nav != a.menuNav || a.theme != a.menuTheme) && a.MenuChanged != nil {
		a.menuNav, a.menuTheme = a.nav, a.theme
		a.MenuChanged()
	}
	a.stateChanged()
}

// Actions are the menu bar's actions.
func (a *App) Actions() platform.Actions {
	open := func() (model.Room, bool) { return a.src.Room(a.room) }
	return platform.Actions{
		QuickSwitcher: func() { a.switcher.Toggle() },
		NewMessage:    func() { a.view, a.phoneTab = viewPeople, 2 },
		NewRoom:       func() { a.toast = "Creating rooms comes with the Matrix layer" },
		JoinRoom:      func() { a.toast = "Joining rooms comes with the Matrix layer" },
		Settings:      func() { a.view, a.phoneTab = viewSettings, 3 },
		ToggleSidebar: func() { a.sidebar = !a.sidebar },
		Appearance:    func(mode int) { a.theme = mode },
		AppearanceNow: a.theme,
		Layout:        func(l kit.NavLayout) { a.nav = l },
		LayoutNow:     a.nav,
		MarkRead:      func() { a.src.MarkRead(a.room) },
		Favourite: func() {
			if r, ok := open(); ok {
				a.src.SetFavourite(r.ID, !r.Favourite)
			}
		},
		Mute: func(d kit.MuteFor) { a.roomAction(a.room, kit.RoomAction{Mute: &d}) },
		RoomSettings: func() {
			a.panel = panelInfo
		},
		Leave: func() {
			if _, ok := open(); ok {
				a.leaving, a.leave.Open = a.room, true
			}
		},
		Gallery: a.OpenGallery,
	}
}

// roomAction applies a room menu's choice.
func (a *App) roomAction(id string, act kit.RoomAction) {
	r, ok := a.src.Room(id)
	if !ok {
		return
	}
	switch {
	case act.Mute != nil:
		until := time.Time{}
		if *act.Mute != 0 {
			until = a.now.Add(time.Duration(*act.Mute))
		}
		a.src.Mute(id, until)
		a.toast = r.Name + ": " + kit.MutedUntilText(until, a.now)
	case act.Unmute:
		a.src.Unmute(id)
	case act.ToggleFav:
		a.src.SetFavourite(id, !r.Favourite)
	case act.ToggleLow:
		a.src.SetLowPriority(id, !r.LowPriority)
	case act.MarkRead:
		a.src.MarkRead(id)
	case act.MarkUnread:
		a.src.MarkUnread(id)
	case act.CopyLink:
		a.toast = "Link copied"
	case act.Settings:
		a.open(id)
		a.panel = panelInfo
	case act.Leave:
		a.leaving, a.leave.Open = id, true
	}
}
