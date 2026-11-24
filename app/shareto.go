package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Sharing into slopement. Another app hands over some words or files: a desktop's
// "Open with" and a file dropped on the Dock icon (mygo's OnOpenFile), Android's
// share sheet (the sharetarget plugin). The app collects what arrives in a short
// burst (one file per call is how a selection comes), shows "Send to…" with the
// chats, and sends it where the user picks. Nothing is sent without that pick.

// Share is what was handed over: some words, some files (by path), or both.
type Share struct {
	Text  string
	Paths []string
	// Temp: the paths are copies made for this share (Android's), removed once read.
	Temp bool
}

// shareSettle is how long a burst of arrivals is waited for before the question
// is asked: a selection of ten files comes as ten calls.
const shareSettle = 400 * time.Millisecond

// shareAfter waits out the burst: a variable, so tests need no clock.
var shareAfter = func(d time.Duration, f func()) { time.AfterFunc(d, f) }

// shareDialog is "Send to…".
type shareDialog struct {
	Sheet kit.SheetState
	Share Share
	Query string
	Err   string
	Busy  bool
	list  ui.ListState
}

// ReceiveShare takes what another app handed over. Call it on the UI thread. The
// question is asked once the burst has ended; a second share meanwhile joins it.
func (a *App) ReceiveShare(s Share) {
	if len(s.Paths) == 0 && strings.TrimSpace(s.Text) == "" {
		return
	}
	a.shareIn.Paths = append(a.shareIn.Paths, s.Paths...)
	a.shareIn.Temp = a.shareIn.Temp || s.Temp
	if t := strings.TrimSpace(s.Text); t != "" {
		if a.shareIn.Text != "" {
			a.shareIn.Text += "\n"
		}
		a.shareIn.Text += t
	}
	if a.shareWait || a.runUI == nil {
		return
	}
	a.shareWait = true
	runUI := a.runUI
	shareAfter(shareSettle, func() {
		runUI(func() {
			a.shareWait = false
			a.openShare()
		})
	})
}

// openShare shows "Send to…" for what has arrived.
func (a *App) openShare() {
	s := a.shareIn
	a.shareIn = Share{}
	if len(s.Paths) == 0 && s.Text == "" {
		return
	}
	a.shareDlg = shareDialog{Share: s}
	a.shareDlg.Sheet.Open = true
}

// shareSummary says what is being sent, in a line.
func shareSummary(s Share) string {
	var parts []string
	switch n := len(s.Paths); {
	case n == 1:
		parts = append(parts, filepath.Base(s.Paths[0]))
	case n > 1:
		parts = append(parts, fmt.Sprintf("%d files", n))
	}
	if t := strings.TrimSpace(s.Text); t != "" {
		if r := []rune(t); len(r) > 80 {
			t = string(r[:80]) + "…"
		}
		parts = append(parts, "“"+strings.ReplaceAll(t, "\n", " ")+"”")
	}
	return strings.Join(parts, " and ")
}

// sendShare sends the share to a chat: the files as files, the words as a
// message, in that order, and opens the chat. Reading the files is off the UI
// thread; the sends are on it.
func (a *App) sendShare(s Share, room, name string) {
	d := &a.shareDlg
	d.Busy, d.Err = true, ""
	runUI := a.runUI
	finish := func(files []model.File, err string) {
		d.Busy = false
		if err != "" {
			d.Err = err
			return
		}
		if len(files) > 0 {
			up, ok := a.src.(model.Uploader)
			if !ok {
				d.Err = "This account cannot send files"
				return
			}
			for _, f := range files {
				up.SendFile(room, f, "")
			}
		}
		if t := strings.TrimSpace(s.Text); t != "" {
			a.src.Send(room, t, "")
		}
		d.Sheet.Open = false
		a.community = ""
		a.open(room)
		a.toast = "Sent to " + name
	}
	check := a.sizeChecker() // the limit, read here on the UI thread
	if runUI == nil {
		files, err := readShared(s, check)
		finish(files, err)
		return
	}
	go func() {
		files, err := readShared(s, check)
		runUI(func() { finish(files, err) })
	}()
}

// readShared reads the files of a share, refusing one too large to send and
// removing the copies made for it.
func readShared(s Share, check func(name string, size int64) string) ([]model.File, string) {
	var out []model.File
	for _, p := range s.Paths {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return nil, "Couldn't read " + filepath.Base(p)
		}
		if why := check(filepath.Base(p), st.Size()); why != "" {
			return nil, why
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, "Couldn't read " + filepath.Base(p)
		}
		out = append(out, model.File{Name: filepath.Base(p), Data: b})
		if s.Temp {
			_ = os.Remove(p)
		}
	}
	return out, ""
}

func (a *App) shareDialogView(c *ui.Context) {
	d := &a.shareDlg
	if !d.Sheet.Open {
		// Closed without sending: the copies made for the share are not kept.
		if d.Share.Temp && len(d.Share.Paths) > 0 {
			for _, p := range d.Share.Paths {
				_ = os.Remove(p)
			}
		}
		d.Share = Share{}
		return
	}
	p := kit.Of(c)
	body := func() {
		targets := a.forwardTargets(d.Query)
		ui.Column(c).Gap(kit.S2).Children(func() {
			ui.Text(c, shareSummary(d.Share)).FontSize(kit.SizeSmall).TextColor(p.TextMuted).MaxLines(2)
			kit.SearchField(c.Key("share-q"), &d.Query, "Search chats", "").AutoFocus()
			if d.Err != "" {
				ui.Text(c, d.Err).FontSize(kit.SizeSmall).TextColor(p.Danger)
			}
			if len(targets) == 0 {
				emptyNote(c, "No chat matches.")
				return
			}
			ui.Column(c.Key("share-box")).Height(340).Children(func() {
				ui.List(c.Key("share-list"), &d.list, len(targets), func(i int) {
					r := targets[i]
					row := ui.ButtonBase(c.Key("share:" + r.ID)).Height(44).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusSm).
						AlignItems(ui.Center).Justify(ui.Start).Label("Send to " + r.Name)
					if row.Hovered() {
						row.Background(p.Hover)
					}
					row.Children(func() {
						kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Square: r.Kind != model.KindDM, Colors: &a.colors})
						ui.Text(c, r.Name).FontSize(kit.SizeBody).SingleLine().Grow(1)
					})
					if row.Clicked() && !d.Busy {
						a.sendShare(d.Share, r.ID, r.Name)
					}
				}).Grow(1)
			})
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, "Send to…", body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, "Send to…", 420, body)
	}
}
