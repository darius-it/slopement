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
	"slopement/linkpreview"
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
	viewSearch // message search (search.go)
)

// panel is the right panel beside the room; it stays open across rooms.
type panel int

const (
	panelNone panel = iota
	panelInfo
	panelMembers
	panelPins
	panelThread // the open thread (threads.go); not restored on a launch
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
	statePath    string
	stateLast    string
	statePending bool
	// Tests' file dialogs (dialogs.go), and the key storage dialog (keys.go).
	pickFileFn       func(title, ext string, done func(path string))
	saveFileFn       func(title, name string, done func(path string))
	keys             keysDialog
	source           eventSource                         // "View source" (source.go)
	testRowHook      func(*model.Event)                  // tests: runs inside each timeline row (to make one panic)
	rowPanics        []string                            // what a timeline row panicked with (a row shows as broken, not the view)
	previewFetcher   *linkpreview.Client                 // fetches previews from this device (previews.go)
	previewsDirect   bool                                // which source previews were asked from
	keysAsked        bool                                // the key status was asked for since the Security page opened
	confirmLogs      func(answer func(include, ok bool)) // tests' answer to the export logs dialog
	fc               frameCache                          // what this frame built from the lists (framecache.go)
	sc               srcCache                            // the source's lists, kept while it does not change
	srcGen           uint64                              // counts the source's changes
	scope            string                              // the list shown last frame (scope.go)
	scopeInit        bool                                // scope has been set
	scopeRooms       map[string]string                   // each list's last open room
	membersLS        ui.ListState                        // the Members panel's scroll (panels.go)
	membersRoom      string                              // whose it is
	inboxLS          ui.ListState                        // One inbox's scroll
	chatsLS          ui.ListState                        // the Chats list's scroll (lists.go)
	noteEd           noteEditor                          // a person's private nickname and note (notes.go)
	updCheck         bool                                // the switch "Check for updates at startup"
	updInit          bool                                // updCheck was read from the saved choice
	upd              updateDialog                        // an update found (updates.go)
	fwd              forwardDialog                       // "Forward" (forward.go)
	trail            navHistory                          // back and forward (nav.go)
	peopleAll        bool                                // People also lists other servers and bridges (convert.go)
	focusRoom        string                              // a room just opened: its composer takes the focus (room.go)
	klipyDraft       string                              // the KLIPY key being typed (gifsettings.go)
	gifFolders       []string                            // folders of pictures the GIF picker also offers (gifsettings.go)
	files            map[string][]model.File             // per room, waiting in the composer (files.go)
	previews         map[string]*previewState            // link previews by URL (previews.go)
	noPreview        map[string]bool                     // messages whose preview the user dismissed
	msgSheet         kit.SheetState                      // phones: a message's menu (actions.go)
	msgRoom, msgID   string
	readMarked       map[string]string // per room, the last event marked read while open (readmark.go)
	jumpRoom, jumpID string            // a jump to a message asked for (jump.go)
	jumpSince        time.Time         // when that jump was asked for (it pages history in until found)
	jumpAt           time.Time         // the target's time when known (zero if not)
	jumpLooking      bool              // the jump is paging history in, looking for it
	flash            string            // the message just jumped to, in its blink's color
	flashOut         string            // the one whose blink is fading out
	rs               receiptState      // the open room's, taken once a frame
	receiptsApplied  bool              // the source was told the setting (receipts.go)
	receiptsVal      bool
	restoreRoom      string
	restoreCommunity string
	restorePushed    bool

	// Attachments: decoded pictures by mxc URL (bounded, oldest out),
	// and the ones being fetched.
	stills     map[string]ui.ImageSource
	stillOrder []string
	stillBusy  map[string]bool
	// fulls are the viewer's pictures at full resolution (a few: each is
	// big), fetched when one is opened.
	fulls     map[string]*ui.Bitmap
	fullOrder []string
	fullBusy  map[string]bool
	// runUI runs a function on the UI thread (the window's Update; a test
	// runs it at once): where work done off-thread comes back.
	runUI       func(func())
	gifs        map[string]*ui.VideoPlayer // GIF attachments' looping players (media.go)
	gifOrder    []string
	gifBusy     map[string]bool
	avatars     map[string]*ui.Bitmap // profile pictures by mxc URL (media.go)
	avatarOrder []string
	avatarBusy  map[string]bool
	// The animations of GIF profile pictures by mxc URL, nil for one that is
	// not a GIF (avataranim.go).
	avatarAnims     map[string]*ui.VideoPlayer
	avatarAnimBusy  map[string]bool
	avatarAnimOrder []string

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
	// The accounts of this device (nil on the mock) and the removal being
	// confirmed.
	accounts *Accounts
	detached bool // the window moved on to another account's app
	fileSeq  int  // numbers the files waiting in composers (their thumbnails)
	// The display name being edited in Settings → Account, and whether it
	// has been taken from the profile yet.
	nameDraft  string
	nameLoaded bool
	focused    bool // the window is in front (notifications stay quiet for the room in view)
	srch       searchState
	find       findState              // ⌘F: find in this chat (find.go)
	typing     map[string]*typingRoom // typing notices told to rooms (typing.go)
	// narrow: the window is phone-sized this frame (no side panels);
	// thread is the open thread, inThread says one is being drawn.
	narrow   bool
	thread   threadState
	inThread bool
	clips    map[string]*clip // audio messages played or being loaded, by event (audio.go)
	// Join / create / explore, and inviting (rooms.go); a room to open as
	// soon as the sync has it.
	re            roomEdit              // editing a room or community (roomedit.go)
	hier          map[string]*hierState // what each community's hierarchy offers (spaces.go)
	justJoined    map[string]bool       // joined from a hierarchy list before the sync lands
	rd            roomsDialog
	inv           inviteDialog
	pendingOpen   string
	pendingOpenBy time.Time
	sessionsAsked bool   // the devices were asked for this visit to Security
	openURL       string // a page for the next frame to open in the browser
	removing      string
	removeAccount kit.SheetState
	picker        kit.PickerState
	pickerRoom    string
	viewer        bool
	viewerItems   []kit.MediaSpec
	viewerAt      int
	people        kit.PeopleState
	call          kit.CallState
	inCall        string // the room whose call the user is in
	callSess      *callSession // the real call, when the source can run one (callsession.go)
	verify        kit.SheetState
	verifyStep    int    // 0 the flow; 1 the Security Key entry
	verifyKey     string // the key being typed
	verifier      model.Verify
	verifyAsked   bool // the once-a-run prompt has shown
	toast         string
	reactTo       string            // the message the picker reacts to
	replyTo       map[string]string // per room: the event replied to
	roomInfo      kit.SheetState    // phones: room info
	roomSheet     kit.SheetState    // phones: a room's actions
	panelSheet    kit.SheetState    // phones: the pins or a thread, where wide windows have a side panel
	panelShown    panel             // the panel the sheet was opened for
	roomSheetID   string
	prefs         settingsPrefs

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
		src:        src,
		pick:       pick,
		verifier:   src.Verify(),
		now:        time.Now(),
		nav:        kit.DefaultNavLayout(),
		sidebar:    true,
		settings:   "Account",
		expanded:   map[string]bool{},
		drafts:     map[string]*kit.ComposerState{},
		lists:      map[string]*ui.ListState{},
		scrolls:    map[string]*ui.ScrollState{},
		reveals:    kit.Reveals{},
		docs:       map[string]parsed{},
		roomMenu:   map[string]bool{},
		stills:     map[string]ui.ImageSource{},
		stillBusy:  map[string]bool{},
		fulls:      map[string]*ui.Bitmap{},
		fullBusy:   map[string]bool{},
		readMarked: map[string]string{},
		files:      map[string][]model.File{},
		previews:   map[string]*previewState{},
		noPreview:  map[string]bool{},
		gifs:       map[string]*ui.VideoPlayer{},
		gifBusy:    map[string]bool{},
		avatars:    map[string]*ui.Bitmap{},
		avatarBusy: map[string]bool{},
		call:       kit.NewCall(),

		avatarAnims:    map[string]*ui.VideoPlayer{},
		avatarAnimBusy: map[string]bool{},
		replyTo:        map[string]string{},
		prefs:          defaultPrefs(),
	}
	a.pollDraft = kit.NewPollDraft()
	src.OnChange(func() { a.srcGen++ }) // the source's lists are kept until it changes (framecache.go)
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
	a.detached = false
	a.runUI = w.Update
	kit.ClipboardText = mygo.Clipboard.ReadText // the composer pastes plain text itself
	a.src.SetUpdate(w.Update)
	if a.pick != nil {
		a.pick.SetUpdate(w.Update)
	}
	// Coming back to the window is a frame: the room open there is read
	// (readStep), whatever arrived while the user was away.
	a.focused = true // opened in front
	w.OnFocus(func() { w.Update(func() { a.focused = true }) })
	w.OnBlur(func() { w.Update(func() { a.focused = false }) })
	a.listenForNotices()
	// Files dropped where no element takes them (over the composer's field,
	// the side panel, a gap) still go to the open room's composer: the
	// window takes files anywhere once it has a listener.
	w.OnFileDrop(func(e *mygo.FileDropEvent) {
		paths := e.Paths
		w.Update(func() {
			if !a.detached && a.room != "" && a.view == viewRoom && len(paths) > 0 {
				a.addPaths(a.room, paths)
			}
		})
	})
}

// Detach ends the app's hold on the window: its handlers do nothing from
// here on. The host calls it when it swaps the window to another account's
// app.
func (a *App) Detach() {
	a.detached = true
	a.closeClips()
}

// open opens a room: it becomes the open room, is marked read, and leads
// the switcher's recents.
func (a *App) open(id string) {
	if _, ok := a.src.Room(id); !ok {
		return
	}
	if a.room != id {
		a.stopTyping(a.room) // they left the room they were typing in
	}
	a.room, a.view, a.pushed = id, viewRoom, true
	a.focusRoom = id   // the composer takes the keyboard when the room is built (not on phones)
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
	a.frameStart()
	defer a.frameEnd()
	a.now = time.Now()
	a.updatesAtStart() // only if the user turned it on (Settings → Privacy)
	a.typingTick()
	// Appearance settings: the tint, the timeline's density, the size.
	kit.SetAccent(a.prefs.accent)
	kit.Dense = a.prefs.dense
	kit.EmojiImage = a.emojiPicture // custom emoji in messages (packs.go)
	if i := a.prefs.uiSize; i >= 0 && i < len(uiSizes) {
		c.SetZoom(uiSizes[i].Zoom)
	}
	a.narrow = kit.Compact(c)
	a.receiptsSync()
	a.syncColors()
	a.pendingStep()
	if a.openURL != "" { // a page to show in the browser, asked for off the view
		c.Services().OpenURL(a.openURL)
		a.openURL = ""
	}
	switch a.theme {
	case 1:
		kit.ApplyTheme(c, false)
	case 2:
		kit.ApplyTheme(c, true)
	default:
		kit.Apply(c)
	}
	kit.SwitcherShortcut(c, &a.switcher)
	a.navKeys(c)
	a.scopeSync()
	a.navTrack()
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF) { // ⌘⇧F: search every chat
		a.openSearch("")
	} else if c.Shortcut(ui.Cmd, ui.KeyF) { // ⌘F: find in this chat
		a.openFind()
	}
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
		NewRoom:       func() { a.openRooms(1, 0) },
		JoinRoom:      func() { a.openRooms(0, 0) },
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
	case act.SetLevel:
		if nl, ok := a.src.(model.NotifyLevels); ok {
			nl.SetNotifyLevel(id, model.NotifyLevel(act.Level))
			a.toast = r.Name + ": notifications set to " + kit.NotifyLevelNames[act.Level]
		} else {
			a.toast = "This account can't change a room's notifications here"
		}
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
		target := r.Alias // the room's address, else its ID
		if target == "" {
			target = r.ID
		}
		a.copyToClipboard("https://matrix.to/#/"+target, "Link copied")
	case act.Settings:
		a.open(id)
		a.panel = panelInfo
	case act.Leave:
		a.leaving, a.leave.Open = id, true
	}
}
