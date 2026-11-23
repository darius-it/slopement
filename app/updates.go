package app

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/format"
	"slopement/kit"
)

// Updates. The app never updates on its own and never looks for updates unless
// the user turned that on (Settings → About): then it asks GitHub once, when
// the app starts, whether a newer version is published. A newer one is offered
// in a dialog; nothing is downloaded until the user presses Install. The
// signatures are checked by mygo's updater against the public key built into
// the app. "Check for updates now" (Settings → About) asks once, on request.

// updateOffer is a newer version found.
type updateOffer struct {
	Version string
	Notes   string // Markdown
	Date    time.Time
	Install func(ctx context.Context, progress func(done, total int64)) error
}

// updater is what the app asks of the updating machinery; tests replace it.
type updater interface {
	Enabled() bool
	Check(ctx context.Context) (*updateOffer, error)
}

// mygoUpdater is mygo.Updater: it checks the feed the app was built with
// (the GitHub releases of the repository, see mygo.config.ts).
type mygoUpdater struct{}

func (mygoUpdater) Enabled() bool { return mygo.Updater.Enabled() }

func (mygoUpdater) Check(ctx context.Context) (*updateOffer, error) {
	mygo.Updater.SetPrereleases(true) // every release of the alpha is a pre-release
	up, err := mygo.Updater.Check(ctx)
	if err != nil || up == nil {
		return nil, err
	}
	return &updateOffer{Version: up.Version, Notes: up.Notes, Date: up.Date, Install: up.Install}, nil
}

// theUpdater and relaunchApp are the seams of tests.
var (
	theUpdater  updater = mygoUpdater{}
	relaunchApp         = func() { mygo.App.Relaunch() }
)

// updatePrefs are the user's two choices about updates. They belong to the
// device, not to an account: one file, whichever account is signed in.
type updatePrefs struct {
	Check bool   `json:"check"` // look for an update once, at startup
	Skip  string `json:"skip"`  // a version not to be offered again at startup
}

var upd struct {
	mu      sync.Mutex
	loaded  bool
	path    string // "" is the default place; tests give another
	prefs   updatePrefs
	started bool // the startup check was made (once per run)
}

func updatePrefsPath() string {
	if upd.path != "" {
		return upd.path
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "slopement", "updates.json")
}

func getUpdatePrefs() updatePrefs {
	upd.mu.Lock()
	defer upd.mu.Unlock()
	if !upd.loaded {
		upd.loaded = true
		if p := updatePrefsPath(); p != "" {
			if b, err := os.ReadFile(p); err == nil {
				_ = json.Unmarshal(b, &upd.prefs)
			}
		}
	}
	return upd.prefs
}

func setUpdatePrefs(p updatePrefs) {
	getUpdatePrefs() // loaded first
	upd.mu.Lock()
	defer upd.mu.Unlock()
	upd.prefs = p
	path := updatePrefsPath()
	if path == "" {
		return
	}
	if b, err := json.MarshalIndent(p, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o700)
		_ = os.WriteFile(path, b, 0o600)
	}
}

// updatePhase is where the update dialog is.
type updatePhase int

const (
	updChecking updatePhase = iota
	updUpToDate
	updAvailable
	updInstalling
	updFailed
)

// updateDialog is the dialog of an update: checking, the news, the progress.
type updateDialog struct {
	Sheet       kit.SheetState
	Phase       updatePhase
	Offer       *updateOffer
	Err         string
	Done, Total int64
	gen         int // which check or install the answers belong to
}

// updatesAtStart asks for an update once per run, at startup, when the user
// allowed it. Call it each frame; it does the work once.
func (a *App) updatesAtStart() {
	upd.mu.Lock()
	started := upd.started
	upd.mu.Unlock()
	if started || a.runUI == nil {
		return
	}
	if !getUpdatePrefs().Check || !theUpdater.Enabled() {
		return
	}
	upd.mu.Lock()
	upd.started = true
	upd.mu.Unlock()
	a.checkUpdates(false)
}

// checkUpdates looks for an update. Manual: the user asked, so the dialog
// shows what came of it; otherwise (startup) only a newer version, not skipped,
// says anything.
func (a *App) checkUpdates(manual bool) {
	d := &a.upd
	d.gen++
	gen := d.gen
	if manual {
		*d = updateDialog{Phase: updChecking, gen: gen}
		d.Sheet.Open = true
	}
	runUI := a.runUI
	if runUI == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		offer, err := theUpdater.Check(ctx)
		runUI(func() {
			d := &a.upd
			if d.gen != gen {
				return
			}
			switch {
			case err != nil:
				log.Printf("updates: check failed: %v", err)
				if manual {
					d.Phase, d.Err = updFailed, "Couldn't check for updates: "+err.Error()
				}
			case offer == nil:
				if manual {
					d.Phase = updUpToDate
				}
			case !manual && getUpdatePrefs().Skip == offer.Version:
				// Skipped by the user: not offered again at startup.
			default:
				d.Phase, d.Offer, d.Err = updAvailable, offer, ""
				d.Sheet.Open = true
			}
		})
	}()
}

// installUpdate downloads the offered version, checks it and replaces the
// app, then restarts it.
func (a *App) installUpdate() {
	d := &a.upd
	offer := d.Offer
	if offer == nil || offer.Install == nil || a.runUI == nil {
		return
	}
	d.gen++
	gen := d.gen
	d.Phase, d.Err, d.Done, d.Total = updInstalling, "", 0, 0
	runUI := a.runUI
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		err := offer.Install(ctx, func(done, total int64) {
			runUI(func() {
				if a.upd.gen == gen {
					a.upd.Done, a.upd.Total = done, total
				}
			})
		})
		runUI(func() {
			if a.upd.gen != gen {
				return
			}
			if err != nil {
				log.Printf("updates: install failed: %v", err)
				a.upd.Phase, a.upd.Err = updFailed, "The update could not be installed: "+err.Error()
				return
			}
			a.saveState() // the app's own state is on disk before it restarts
			relaunchApp()
		})
	}()
}

// skipUpdate is "Skip this version": it is not offered again at startup (a
// check the user asks for still shows it).
func (a *App) skipUpdate() {
	if o := a.upd.Offer; o != nil {
		p := getUpdatePrefs()
		p.Skip = o.Version
		setUpdatePrefs(p)
	}
	a.upd.Sheet.Open = false
}

// updateDialogView draws the dialog.
func (a *App) updateDialogView(c *ui.Context) {
	d := &a.upd
	if !d.Sheet.Open {
		return
	}
	p := kit.Of(c)
	title := "Software update"
	body := func() {
		ui.Column(c).Gap(kit.S3).Children(func() {
			switch d.Phase {
			case updChecking:
				ui.Text(c, "Checking for updates…").FontSize(kit.SizeBody).TextColor(p.TextMuted)
			case updUpToDate:
				ui.Text(c, "slopement is up to date.").FontSize(kit.SizeBody)
				if kit.Button(c, kit.Secondary, "Close", nil).AlignSelf(ui.End).Clicked() {
					d.Sheet.Open = false
				}
			case updAvailable:
				a.updateAvailableBody(c)
			case updInstalling:
				ui.Text(c, "Downloading version "+d.Offer.Version+"…").FontSize(kit.SizeBody)
				frac := 0.0
				if d.Total > 0 {
					frac = float64(d.Done) / float64(d.Total)
				}
				ui.Progress(c, frac)
				ui.Text(c, "slopement restarts when it is done.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			case updFailed:
				ui.Text(c, d.Err).FontSize(kit.SizeBody).TextColor(p.Danger)
				ui.Row(c).Gap(kit.S2).Justify(ui.End).Children(func() {
					if d.Offer != nil && kit.Button(c, kit.Secondary, "Try again", nil).Clicked() {
						a.installUpdate()
					}
					if kit.Button(c, kit.Primary, "Close", nil).Clicked() {
						d.Sheet.Open = false
					}
				})
			}
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, title, body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, title, 460, body)
	}
}

func (a *App) updateAvailableBody(c *ui.Context) {
	p := kit.Of(c)
	d := &a.upd
	o := d.Offer
	ui.Text(c, "Version "+o.Version+" is available").FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
	ui.Text(c, "You have "+mygo.App.Version()+".").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	if o.Notes != "" {
		// A scroll area, not a clipped column: release notes run long, and the
		// wheel must move them.
		ui.Scroll(c.Key("update-notes")).MaxHeight(260).Padding(kit.S3).Background(p.BgInset).Radius(kit.RadiusSm + 2).Children(func() {
			kit.MessageBody(c, "update-notes", format.Parse(o.Notes), a.reveals, kit.BodyStyle{Size: kit.SizeSmall + 1})
		})
	}
	ui.Text(c, "Nothing is downloaded until you press Install. The update is checked against the key built into this app.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	ui.Row(c).Gap(kit.S2).Justify(ui.End).Children(func() {
		if kit.Button(c, kit.Ghost, "Skip this version", nil).Clicked() {
			a.skipUpdate()
		}
		if kit.Button(c, kit.Secondary, "Later", nil).Clicked() {
			d.Sheet.Open = false
		}
		if kit.Button(c, kit.Primary, "Install and restart", nil).Clicked() {
			a.installUpdate()
		}
	})
}

// updatesGroup is everything about updates, in Settings → About: whether to
// ask at startup (off unless the user turns it on), a check on request, and why
// there is none where the app cannot update itself.
func (a *App) updatesGroup(c *ui.Context) {
	p := kit.Of(c)
	group(c, "Updates", "slopement never updates by itself. With the switch on, it asks GitHub once when it starts whether a newer version is out: GitHub sees your IP address and that you asked for slopement's releases for your system, and nothing from your account. You are asked before anything is downloaded.", func() {
		saved := getUpdatePrefs()
		if !a.updInit {
			a.updCheck, a.updInit = saved.Check, true // the switch's own state, kept between frames
		}
		switchRow(c, "Check for updates at startup", "", &a.updCheck)
		if a.updCheck != saved.Check {
			saved.Check = a.updCheck
			setUpdatePrefs(saved)
		}
		if !theUpdater.Enabled() {
			ui.Text(c, "This copy updates through where it was installed from (a package manager), or is a development build.").
				FontSize(kit.SizeSmall).TextColor(p.TextMuted).Padding(kit.S1, kit.S3, kit.S2, kit.S3)
			return
		}
		if kit.ChoiceRow(c, kit.IconDownload, "Check for updates now", "Asks GitHub once; nothing is downloaded until you press Install").Clicked() {
			a.checkUpdates(true)
		}
	})
}
