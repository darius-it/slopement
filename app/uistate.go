package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"slopement/kit"
)

// The app's own state across launches: the layout and theme, the
// settings, where the user was, and the messages half-typed. It lives in
// one small JSON file next to the account's other data; the Matrix data
// is the Source's own cache.

// uiState is what is written. Only what means something after a restart:
// not the open dialogs, scroll offsets or the person's card that was open.
type uiState struct {
	Theme      int
	Nav        int
	Sidebar    bool
	Panel      int
	Room       string
	Community  string
	Pushed     bool
	Tab        int
	Filter     int
	PhoneTab   int
	Settings   string
	Expanded   map[string]bool
	LastRooms  []string
	Drafts     map[string]string
	Prefs      prefsState
	GifFolders []string
}

// prefsState is settingsPrefs with its fields exported, for JSON.
type prefsState struct {
	NotifyDesktop, NotifySound, NotifyMentions, NotifyKeywords bool
	Receipts, Typing, PreviewsE2EE, PreviewsDirect             bool
	GifAutoplay, AnimatedAvatars, HideJoins                    bool
	LinkPreviews, VideoPreviews, GifPreviews                   bool
	MyColor                                                    int
	PeopleSub                                                  int
	Glass                                                      int
	GifFiles                                                   bool
	Accent, UISize                                             int
	Dense                                                      bool
	Usernames                                                  bool
	JumpLast                                                   bool
	CallMicOff, CallCamOn, CallLobby                           bool
	CallsAccepted                                              bool
	StatusBubble                                               bool
	MinimizeToTray                                             bool
	CloseMode                                                  int
	ShowMaps                                                   bool
}

func (p settingsPrefs) state() prefsState {
	return prefsState{p.notifyDesktop, p.notifySound, p.notifyMentions, p.notifyKeywords,
		p.receipts, p.typing, p.previewsE2EE, p.previewsDirect,
		p.gifAutoplay, p.animatedAvatars, p.hideJoins,
		p.linkPreviews, p.videoPreviews, p.gifPreviews, p.myColor, p.peopleSub, p.glass, p.gifFiles,
		p.accent, p.uiSize, p.dense, p.usernames, p.jumpLast, p.callMicOff, p.callCamOn, p.callLobby, p.callsAccepted, p.statusBubble, p.minimizeToTray, p.closeMode, p.showMaps}
}

func (p prefsState) prefs() settingsPrefs {
	return settingsPrefs{p.NotifyDesktop, p.NotifySound, p.NotifyMentions, p.NotifyKeywords,
		p.Receipts, p.Typing, p.PreviewsE2EE, p.PreviewsDirect,
		p.GifAutoplay, p.AnimatedAvatars, p.HideJoins,
		p.LinkPreviews, p.VideoPreviews, p.GifPreviews, p.MyColor, p.PeopleSub, p.Glass, p.GifFiles,
		p.Accent, p.UISize, p.Dense, p.Usernames, p.JumpLast, p.CallMicOff, p.CallCamOn, p.CallLobby, p.CallsAccepted, p.StatusBubble, p.MinimizeToTray, p.CloseMode, p.ShowMaps}
}

// stateWriter writes the file one write at a time, newest wins.
var stateWriter sync.Mutex

// SetStatePath names the file the app's state lives in and loads it. Call
// it once, before the first frame. An unreadable file is ignored — the
// app starts with its defaults, and the next save replaces it.
func (a *App) SetStatePath(path string) {
	a.statePath = path
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var st uiState
	if json.Unmarshal(b, &st) != nil {
		return
	}
	a.applyState(st)
	a.stateLast = string(b)
}

func (a *App) applyState(st uiState) {
	if st.Theme >= 0 && st.Theme <= 2 {
		a.theme = st.Theme
	}
	if st.Nav >= 0 && st.Nav < len(kit.NavLayouts) {
		a.nav = kit.NavLayout(st.Nav)
		a.menuNav = a.nav
	}
	a.menuTheme = a.theme
	a.sidebar = st.Sidebar
	if p := panel(st.Panel); p >= panelNone && p <= panelPins {
		a.panel = p
	}
	a.tab, a.filter, a.phoneTab = st.Tab, st.Filter, st.PhoneTab
	if st.Settings != "" {
		a.settings = st.Settings
	}
	for k, v := range st.Expanded {
		a.expanded[k] = v
	}
	a.lastRooms = st.LastRooms
	for room, text := range st.Drafts {
		if text != "" {
			a.drafts[room] = &kit.ComposerState{Text: text}
		}
	}
	a.prefs = st.Prefs.prefs()
	a.gifFolders = st.GifFolders
	if a.pick != nil {
		a.pick.SetFolders(a.gifFolders)
	}
	// Where the user was is applied when that room is there: from the
	// cache it is at once, on a first sync it arrives with the rooms.
	a.restoreRoom, a.restoreCommunity, a.restorePushed = st.Room, st.Community, st.Pushed
	a.restoreStep()
}

// restoreStep puts the user back in the room and community they were in,
// as soon as the Source has them; until then the app shows what New chose.
func (a *App) restoreStep() {
	if a.restoreCommunity != "" {
		if _, ok := a.communityOf(a.restoreCommunity); ok {
			a.community = a.restoreCommunity
			a.restoreCommunity = ""
		}
	}
	if a.restoreRoom != "" {
		if _, ok := a.src.Room(a.restoreRoom); ok {
			a.open(a.restoreRoom)
			a.pushed = a.restorePushed
			a.restoreRoom = ""
		}
	}
}

func (a *App) state() uiState {
	st := uiState{
		Theme: a.theme, Nav: int(a.nav), Sidebar: a.sidebar, Panel: int(a.panel),
		Room: a.room, Community: a.community, Pushed: a.pushed,
		Tab: a.tab, Filter: a.filter, PhoneTab: a.phoneTab, Settings: a.settings,
		Expanded: map[string]bool{}, LastRooms: a.lastRooms, Drafts: map[string]string{},
		Prefs: a.prefs.state(), GifFolders: a.gifFolders,
	}
	// A room or community not restored yet stays what it was.
	if a.restoreRoom != "" {
		st.Room, st.Pushed = a.restoreRoom, a.restorePushed
	}
	if a.restoreCommunity != "" {
		st.Community = a.restoreCommunity
	}
	for k, v := range a.expanded {
		if v {
			st.Expanded[k] = true
		}
	}
	for room, d := range a.drafts {
		if d.Text != "" {
			st.Drafts[room] = d.Text
		}
	}
	return st
}

// saveState writes the state if it changed.
func (a *App) saveState() {
	if a.statePath == "" {
		return
	}
	b, err := json.Marshal(a.state())
	if err != nil || string(b) == a.stateLast {
		return
	}
	a.stateLast = string(b)
	path := a.statePath
	go writeState(path, b)
}

// writeState writes through a temporary file, so a crash never leaves a
// half-written state.
func writeState(path string, b []byte) {
	stateWriter.Lock()
	defer stateWriter.Unlock()
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// stateChanged is called at the end of every frame: a change is written
// a moment later (so typing is not a write per key), once.
func (a *App) stateChanged() {
	if a.statePath == "" || a.win == nil || a.statePending {
		return
	}
	if a.restoreRoom != "" || a.restoreCommunity != "" {
		room, community := a.restoreRoom, a.restoreCommunity
		a.restoreStep()
		if room != a.restoreRoom || community != a.restoreCommunity {
			a.win.Update(func() {}) // draw the place the user is back in
		}
	}
	if b, err := json.Marshal(a.state()); err != nil || string(b) == a.stateLast {
		return
	}
	a.statePending = true
	win := a.win
	time.AfterFunc(800*time.Millisecond, func() {
		win.Update(func() {
			a.statePending = false
			a.saveState()
		})
	})
}

// SaveState writes the state now: the window is closing.
func (a *App) SaveState() {
	a.saveState()
	stateWriter.Lock()
	stateWriter.Unlock() // wait for a write that is running
}
