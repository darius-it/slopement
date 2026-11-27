package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Settings → Security: where the account's keys stand, a file of the room keys
// to take elsewhere (and the way back in), and starting over with a new
// identity. One dialog, in the step each asks for.

type keyMode int

const (
	keysStatus keyMode = iota
	keysExport
	keysImport
	keysReset
)

// keysDialog is the state of that dialog.
type keysDialog struct {
	Sheet kit.SheetState
	Mode  keyMode
	// Fields: the passphrase of a key file (and its repeat), the account
	// password for a reset.
	Pass, Pass2, Password string
	Busy                  bool
	Err, Done             string
	// Import: the file chosen.
	File     []byte
	FileName string
	// Reset: 0 what it does, 1 waiting for the approval in the browser, 2 the
	// new Security Key.
	Step        int
	ApprovalURL string
	approve     chan bool // the user's answer to the approval step
	RecoveryKey string
}

// openKeys opens the dialog in a mode, with nothing left from before.
func (a *App) openKeys(mode keyMode) {
	if a.keys.approve != nil { // an unanswered approval from a reset closed with the dialog
		select {
		case a.keys.approve <- false:
		default:
		}
	}
	a.keys = keysDialog{Mode: mode}
	a.keys.Sheet.Open = true
	if ks, ok := a.src.(model.KeyStorage); ok && mode == keysStatus {
		ks.RefreshKeyStatus()
	}
}

// keyStatusLine is the short line under "Key storage" in the settings.
func keyStatusLine(st model.KeyStatus) string {
	switch {
	case !st.Asked:
		return "Checking…"
	case st.Problem != "" && st.BackupVersion == "":
		return st.Problem
	case st.BackupVersion != "" && st.HaveBackupKey:
		return "Backed up on your homeserver · this device has the key"
	case st.BackupVersion != "":
		return "A backup exists · this device can't read it yet"
	default:
		return "No key backup on your homeserver yet"
	}
}

// keyStatusLines is the whole of it, a line a fact.
func keyStatusLines(st model.KeyStatus) []string {
	if !st.Asked {
		return []string{"Checking with your homeserver…"}
	}
	var out []string
	if st.Verified {
		out = append(out, "This session is verified.")
	} else if st.CrossSigning {
		out = append(out, "This session is not verified yet: verify it to read your older messages.")
	} else {
		out = append(out, "Your account has no cross-signing keys yet.")
	}
	switch {
	case st.BackupVersion == "":
		out = append(out, "No key backup on your homeserver: your message keys live only on your devices.")
	case st.HaveBackupKey:
		out = append(out, fmt.Sprintf("Key backup on your homeserver (version %s, %d keys): this device has its key and reads it on its own.", st.BackupVersion, st.BackupKeys))
	default:
		out = append(out, fmt.Sprintf("Key backup on your homeserver (version %s, %d keys): this device does not have its key yet.", st.BackupVersion, st.BackupKeys))
	}
	if st.SecretStorage {
		out = append(out, "Your Security Key is set up.")
	} else {
		out = append(out, "No Security Key is set up.")
	}
	if st.Problem != "" {
		out = append(out, st.Problem)
	}
	return out
}

// keysDialogView draws the dialog.
func (a *App) keysDialogView(c *ui.Context) {
	d := &a.keys
	if !d.Sheet.Open {
		return
	}
	p := kit.Of(c)
	title := map[keyMode]string{keysStatus: "Key storage", keysExport: "Export room keys",
		keysImport: "Import room keys", keysReset: "Reset your identity"}[d.Mode]
	body := func() {
		ui.Column(c).Gap(kit.S3).Children(func() {
			switch d.Mode {
			case keysStatus:
				a.keysStatusBody(c)
			case keysExport:
				a.keysExportBody(c)
			case keysImport:
				a.keysImportBody(c)
			case keysReset:
				a.keysResetBody(c)
			}
			if d.Err != "" {
				ui.Text(c, d.Err).FontSize(kit.SizeSmall).TextColor(p.Danger)
			}
			if d.Done != "" {
				ui.Text(c, d.Done).FontSize(kit.SizeSmall).TextColor(p.Success)
			}
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, title, body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, title, 480, body)
	}
}

func (a *App) keysStatusBody(c *ui.Context) {
	p := kit.Of(c)
	ks, ok := a.src.(model.KeyStorage)
	if !ok {
		return
	}
	st := ks.KeyStatus()
	for _, l := range keyStatusLines(st) {
		ui.Text(c, l).FontSize(kit.SizeBody)
	}
	if st.Asked && st.BackupVersion != "" && !st.HaveBackupKey {
		if kit.Button(c, kit.Primary, "Restore with your Security Key", kit.IconKey).Clicked() {
			a.keys.Sheet.Open = false
			a.verify.Open, a.verifyStep = true, 1
		}
	}
	if st.Asked && st.BackupVersion == "" {
		ui.Text(c, "Slopement can't create a key backup yet: make one in Element (Settings → Security & Privacy → Secure Backup), then restore it here with your Security Key. Until then, “Export room keys” keeps a copy of the keys on this device.").
			FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	}
	ui.Row(c).Gap(kit.S2).Children(func() {
		if kit.Button(c, kit.Secondary, "Check again", nil).Clicked() {
			ks.RefreshKeyStatus()
		}
		if kit.Button(c, kit.Ghost, "Close", nil).Clicked() {
			a.keys.Sheet.Open = false
		}
	})
}

func (a *App) keysExportBody(c *ui.Context) {
	d, p := &a.keys, kit.Of(c)
	ui.Text(c, "A file of the keys this device holds for your encrypted messages, locked with a passphrase. Element and other clients can read it. Anyone with the file and the passphrase can read those messages: keep both safe.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	pass := kit.PasswordField(c.Key("kx-pass"), &d.Pass, "Passphrase", "")
	again := kit.PasswordField(c.Key("kx-pass2"), &d.Pass2, "Passphrase again", "")
	go1 := kit.Button(c, kit.Primary, "Export…", kit.IconDownload).Clicked() || pass.Submitted() || again.Submitted()
	if go1 && !d.Busy && d.Done == "" {
		a.exportKeys()
	}
}

// exportKeys makes the file, then asks where to keep it.
func (a *App) exportKeys() {
	d := &a.keys
	ks, ok := a.src.(model.KeyStorage)
	switch {
	case !ok:
		return
	case strings.TrimSpace(d.Pass) == "":
		d.Err = "Choose a passphrase to lock the file with."
		return
	case d.Pass != d.Pass2:
		d.Err = "The two passphrases are not the same."
		return
	}
	d.Err, d.Busy = "", true
	ks.ExportRoomKeys(d.Pass, func(data []byte, count int, problem string) {
		if problem != "" {
			d.Busy, d.Err = false, problem
			return
		}
		a.saveFile("Save your room keys", "slopement-room-keys.txt", func(path string) {
			a.writeSaved(path, data, 0o600, func(name string, err error) {
				switch {
				case err != nil:
					d.Busy, d.Err = false, "Could not save the file: "+err.Error()
				case name != "":
					d.Busy, d.Pass, d.Pass2 = false, "", ""
					d.Done = fmt.Sprintf("Saved %d keys to %s.", count, name)
				}
			})
		})
		d.Busy = false // the dialog may be cancelled: nothing is waiting
	})
}

func (a *App) keysImportBody(c *ui.Context) {
	d, p := &a.keys, kit.Of(c)
	ui.Text(c, "Read a file of room keys (from Element, another device or an export from here) so messages that were waiting for those keys can be read.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	label := "Choose a file…"
	if d.FileName != "" {
		label = d.FileName
	}
	if kit.ChoiceRow(c, kit.IconDownload, label, "").Clicked() {
		a.pickFile("Choose a file of room keys", "", func(path string) { a.readKeyFile(path) })
	}
	pass := kit.PasswordField(c.Key("ki-pass"), &d.Pass, "Passphrase", "The one the file was locked with")
	if (kit.Button(c, kit.Primary, "Import", nil).Clicked() || pass.Submitted()) && !d.Busy && d.Done == "" {
		a.importKeys()
	}
}

// maxKeyFile is the largest key file read: they are a few MB at most.
const maxKeyFile = 64 << 20

func (a *App) readKeyFile(path string) {
	d := &a.keys
	info, err := os.Stat(path)
	if err == nil && info.Size() > maxKeyFile {
		d.Err = "That file is too big to be a file of room keys."
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		d.Err = "Could not read that file: " + err.Error()
		return
	}
	d.File, d.FileName, d.Err = data, filepath.Base(path), ""
}

func (a *App) importKeys() {
	d := &a.keys
	ks, ok := a.src.(model.KeyStorage)
	switch {
	case !ok:
		return
	case len(d.File) == 0:
		d.Err = "Choose the file first."
		return
	case d.Pass == "":
		d.Err = "Enter the passphrase the file was locked with."
		return
	}
	d.Err, d.Busy = "", true
	ks.ImportRoomKeys(d.Pass, d.File, func(imported, total int, problem string) {
		d.Busy = false
		if problem != "" {
			d.Err = problem
			return
		}
		d.Pass, d.File = "", nil
		d.Done = fmt.Sprintf("Read %d of %d keys. Messages that were waiting for them are being tried again.", imported, total)
	})
}

func (a *App) keysResetBody(c *ui.Context) {
	d, p := &a.keys, kit.Of(c)
	rs, ok := a.src.(model.IdentityResetter)
	if !ok {
		return
	}
	switch d.Step {
	case 0:
		ui.Text(c, "This replaces your account's encryption identity with a new one, with a new Security Key.").FontSize(kit.SizeBody)
		for _, line := range []string{
			"Your other devices and the people who verified you will see you as new, and must verify you again.",
			"Your old Security Key stops working. Keep the new one safe: it is the way to verify a device when you have no other.",
			"Messages already on this device stay readable. The key backup on your homeserver is not replaced.",
		} {
			ui.Text(c, "• "+line).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		}
		needPass := rs.NeedsPasswordToReset()
		var pass ui.Element
		if needPass {
			pass = kit.PasswordField(c.Key("kr-pass"), &d.Password, "Your account password", "")
		} else {
			ui.Text(c, "Your homeserver will ask you to approve this in your browser.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		}
		submitted := pass.Valid() && pass.Submitted()
		ui.Row(c).Gap(kit.S2).Children(func() {
			if kit.Button(c, kit.Danger, "Reset identity", kit.IconShield).Clicked() || submitted {
				a.resetIdentity(rs, needPass)
			}
			if kit.Button(c, kit.Ghost, "Cancel", nil).Clicked() {
				d.Sheet.Open = false
			}
		})
		if d.Busy {
			ui.Text(c, "Working…").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		}
	case 1:
		ui.Text(c, "Approve it in your browser").FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold)
		ui.Text(c, "A page of your homeserver's account service was opened. Approve the reset there, then come back and press the button.").
			FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		ui.Row(c).Gap(kit.S2).Children(func() {
			if kit.Button(c, kit.Primary, "I approved it", nil).Clicked() {
				a.answerApproval(true)
			}
			if kit.Button(c, kit.Secondary, "Open the page again", nil).Clicked() {
				a.openURL = d.ApprovalURL
			}
			if kit.Button(c, kit.Ghost, "Cancel", nil).Clicked() {
				a.answerApproval(false)
			}
		})
	case 2:
		ui.Text(c, "Your new Security Key").FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold)
		ui.Text(c, d.RecoveryKey).FontSize(kit.SizeBody).Font("monospace").Selectable()
		ui.Text(c, "Write it down or keep it in a password manager. It is shown only now.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		ui.Row(c).Gap(kit.S2).Children(func() {
			if kit.Button(c, kit.Secondary, "Copy", kit.IconCopy).Clicked() {
				a.copyToClipboard(d.RecoveryKey, "Security Key copied")
			}
			if kit.Button(c, kit.Primary, "I've saved it", nil).Clicked() {
				d.Sheet.Open, d.RecoveryKey = false, ""
			}
		})
	}
}

// resetIdentity starts the reset; the approval step, where a server has one,
// is asked of the user while it waits.
func (a *App) resetIdentity(rs model.IdentityResetter, needPass bool) {
	d := &a.keys
	if d.Busy {
		return
	}
	if needPass && d.Password == "" {
		d.Err = "Enter your account password."
		return
	}
	d.Err, d.Busy = "", true
	req := model.IdentityReset{Password: d.Password, Approve: func(url string) bool { return a.askApproval(url) }}
	rs.ResetIdentity(req, func(key, problem string) {
		d.Busy, d.Password, d.approve = false, "", nil
		if problem != "" {
			d.Err, d.Step = problem, 0
			return
		}
		d.RecoveryKey, d.Step = key, 2
	})
}

// askApproval is called from the reset (off the UI thread): the page opens
// in the browser, the dialog asks the user to come back, and this waits for
// their answer.
func (a *App) askApproval(url string) bool {
	ch := make(chan bool, 1)
	run := func() {
		a.keys.approve, a.keys.ApprovalURL, a.keys.Step = ch, url, 1
		a.openURL = url
	}
	if a.runUI != nil {
		a.runUI(run)
	} else {
		run()
	}
	return <-ch
}

func (a *App) answerApproval(ok bool) {
	d := &a.keys
	if d.approve != nil {
		select {
		case d.approve <- ok:
		default:
		}
		d.approve = nil
	}
	if !ok {
		d.Step = 0
	} else {
		d.Step = 0 // back to "working": the reset goes on
		d.Busy = true
	}
}
