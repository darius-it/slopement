package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// keysSource is a source with key storage, an identity to reset and logs.
type keysSource struct {
	model.Source
	mu       sync.Mutex
	status   model.KeyStatus
	asked    int
	exported []string // passphrases
	imported [][2]string
	resetReq *model.IdentityReset
	resetKey string
	needPass bool
	logs     []bool
	problem  string
}

func (k *keysSource) KeyStatus() model.KeyStatus { return k.status }
func (k *keysSource) RefreshKeyStatus()          { k.asked++ }
func (k *keysSource) ExportRoomKeys(pass string, done func([]byte, int, string)) {
	k.exported = append(k.exported, pass)
	if k.problem != "" {
		done(nil, 0, k.problem)
		return
	}
	done([]byte("-----BEGIN MEGOLM SESSION DATA-----\nabc\n-----END MEGOLM SESSION DATA-----"), 3, "")
}
func (k *keysSource) ImportRoomKeys(pass string, data []byte, done func(int, int, string)) {
	k.imported = append(k.imported, [2]string{pass, string(data)})
	if k.problem != "" {
		done(0, 0, k.problem)
		return
	}
	done(2, 3, "")
}
func (k *keysSource) NeedsPasswordToReset() bool { return k.needPass }
func (k *keysSource) ResetIdentity(req model.IdentityReset, done func(string, string)) {
	k.resetReq = &req
	go func() {
		if req.Approve != nil && !k.needPass {
			if !req.Approve("https://account.example.org/reset") {
				done("", "Your homeserver did not accept it: the approval was not given.")
				return
			}
		}
		done(k.resetKey, "")
	}()
}
func (k *keysSource) ExportLogs(include bool, done func([]byte, string, string)) {
	k.logs = append(k.logs, include)
	done([]byte("PK-zip"), "slopement-logs-2026-11-22.zip", "")
}

func keysApp(t *testing.T) (*App, *keysSource, *ui.Tester) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	src := &keysSource{Source: m, resetKey: "EsTc WLwb Z5P1 aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii",
		status: model.KeyStatus{Asked: true, BackupVersion: "7", BackupKeys: 42, HaveBackupKey: true, Verified: true, SecretStorage: true}}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }
	a.view, a.settings = viewSettings, "Security"
	tt := ui.NewTester(a.View, 1000, 800)
	tt.Frame()
	tt.Frame()
	return a, src, tt
}

func frames(tt *ui.Tester) {
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
}

// The Security page says where the keys stand and has rows for each thing;
// "Key storage" opens the whole picture, and offers the restore when the
// account has a backup this device cannot read.
func TestKeyStorageRowAndDialog(t *testing.T) {
	a, src, tt := keysApp(t)
	for _, want := range []string{"Key storage", "Backed up on your homeserver", "Export room keys", "Import room keys", "Reset your identity"} {
		if !tt.HasText(want) {
			t.Fatalf("the Security page lacks %q", want)
		}
	}
	if src.asked != 1 {
		t.Fatalf("the status was asked %d times", src.asked)
	}
	snap(t, "keys-security", tt)
	if err := tt.Click("Key storage"); err != nil {
		t.Fatal(err)
	}
	frames(tt)
	if !tt.HasText("Key backup on your homeserver (version 7, 42 keys)") || tt.HasText("Restore with your Security Key") {
		t.Fatal("the dialog does not say the backup is here, or offers a restore it has no use for")
	}

	// A backup this device cannot read: the restore opens the Security Key step.
	src.status.HaveBackupKey = false
	a.keys.Sheet.Open = false
	frames(tt)
	a.openKeys(keysStatus)
	frames(tt)
	if !tt.HasText("this device does not have its key yet") {
		t.Fatal("the dialog does not say the key is missing")
	}
	snap(t, "keys-status-dialog", tt)
	if err := tt.Click("Restore with your Security Key"); err != nil {
		t.Fatal(err)
	}
	frames(tt)
	if !a.verify.Open || a.verifyStep != 1 || a.keys.Sheet.Open {
		t.Fatalf("verify open %v step %d, keys open %v", a.verify.Open, a.verifyStep, a.keys.Sheet.Open)
	}

	// No backup: it says so and says what to do.
	a.verify.Open = false
	src.status = model.KeyStatus{Asked: true}
	a.openKeys(keysStatus)
	frames(tt)
	if !tt.HasText("No key backup on your homeserver") || !tt.HasText("Secure Backup") {
		t.Fatal("the no-backup state is not explained")
	}
}

func TestKeyStatusLines(t *testing.T) {
	for st, want := range map[model.KeyStatus]string{
		{}: "Checking…",
		{Asked: true, BackupVersion: "1", HaveBackupKey: true}: "this device has the key",
		{Asked: true, BackupVersion: "1"}:                      "can't read it yet",
		{Asked: true}:                                          "No key backup",
		{Asked: true, Problem: "offline"}:                      "offline",
	} {
		if got := keyStatusLine(st); !strings.Contains(got, want) {
			t.Errorf("%+v: %q lacks %q", st, got, want)
		}
	}
}

// Exporting asks for the passphrase twice, will not go with an empty or a
// mismatched one, and writes the file where the user chose.
func TestExportRoomKeysDialog(t *testing.T) {
	a, src, tt := keysApp(t)
	dir := t.TempDir()
	var saved string
	a.saveFileFn = func(title, name string, done func(string)) { saved = filepath.Join(dir, name); done(saved) }
	a.openKeys(keysExport)
	frames(tt)

	if err := tt.Click("Export…"); err != nil {
		t.Fatal(err)
	}
	frames(tt)
	if len(src.exported) != 0 || !tt.HasText("Choose a passphrase") {
		t.Fatalf("exported with an empty passphrase: %v", src.exported)
	}
	a.keys.Pass, a.keys.Pass2 = "correct horse", "correct hors"
	tt.Click("Export…")
	frames(tt)
	if len(src.exported) != 0 || !tt.HasText("not the same") {
		t.Fatalf("exported with passphrases that differ: %v", src.exported)
	}
	a.keys.Pass2 = "correct horse"
	tt.Click("Export…")
	frames(tt)
	if len(src.exported) != 1 || src.exported[0] != "correct horse" {
		t.Fatalf("exports = %v", src.exported)
	}
	b, err := os.ReadFile(saved)
	if err != nil || !strings.HasPrefix(string(b), "-----BEGIN MEGOLM SESSION DATA-----") {
		t.Fatalf("saved file: %v %q", err, b)
	}
	if info, _ := os.Stat(saved); info.Mode().Perm() != 0o600 {
		t.Fatalf("the key file is readable by others: %v", info.Mode())
	}
	if !tt.HasText("Saved 3 keys to slopement-room-keys.txt") || a.keys.Pass != "" {
		t.Fatalf("done: %q, passphrase kept %q", a.keys.Done, a.keys.Pass)
	}

	// A refusal stays in the dialog.
	src.problem = "This device holds no room keys yet: there is nothing to export."
	a.openKeys(keysExport)
	a.keys.Pass, a.keys.Pass2 = "x", "x"
	frames(tt)
	tt.Click("Export…")
	frames(tt)
	if !tt.HasText("nothing to export") || a.keys.Done != "" {
		t.Fatalf("err %q done %q", a.keys.Err, a.keys.Done)
	}
}

// Importing reads the file chosen, asks for its passphrase, and says how many
// keys it took; a wrong passphrase is said and nothing is claimed.
func TestImportRoomKeysDialog(t *testing.T) {
	a, src, tt := keysApp(t)
	path := filepath.Join(t.TempDir(), "keys.txt")
	if err := os.WriteFile(path, []byte("THE FILE"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.pickFileFn = func(title, ext string, done func(string)) { done(path) }
	a.openKeys(keysImport)
	frames(tt)

	tt.Click("Import")
	frames(tt)
	if len(src.imported) != 0 || !tt.HasText("Choose the file first") {
		t.Fatal("imported with no file")
	}
	if err := tt.Click("Choose a file…"); err != nil {
		t.Fatal(err)
	}
	frames(tt)
	if a.keys.FileName != "keys.txt" || !tt.HasText("keys.txt") {
		t.Fatalf("file name %q", a.keys.FileName)
	}
	tt.Click("Import")
	frames(tt)
	if len(src.imported) != 0 || !tt.HasText("Enter the passphrase") {
		t.Fatal("imported with no passphrase")
	}
	src.problem = "That passphrase does not open this file."
	a.keys.Pass = "wrong"
	tt.Click("Import")
	frames(tt)
	if !tt.HasText("does not open this file") || a.keys.Done != "" {
		t.Fatalf("err %q done %q", a.keys.Err, a.keys.Done)
	}
	src.problem = ""
	a.keys.Pass = "right"
	tt.Click("Import")
	frames(tt)
	if len(src.imported) != 2 || src.imported[1] != [2]string{"right", "THE FILE"} || !tt.HasText("Read 2 of 3 keys") {
		t.Fatalf("imports %v, done %q", src.imported, a.keys.Done)
	}
}

// Resetting the identity: it says what it does first; where the server wants
// an approval in the browser, the page opens and the dialog waits for "I
// approved it"; then the new Security Key is shown, once.
func TestResetIdentityDialog(t *testing.T) {
	a, src, tt := keysApp(t)
	a.openKeys(keysReset)
	frames(tt)
	for _, want := range []string{"new encryption identity", "old Security Key stops working", "approve this in your browser"} {
		if !tt.HasText(want) {
			t.Fatalf("the warning lacks %q", want)
		}
	}
	settle(tt) // the dialog's entrance has played
	snap(t, "keys-reset-dialog", tt)
	if err := tt.Click("Reset identity"); err != nil {
		t.Fatal(err)
	}
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		for i := 0; i < 200; i++ {
			frames(tt)
			if ok() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", what)
	}
	waitFor("the approval step", func() bool { return a.keys.Step == 1 })
	if a.openURL != "https://account.example.org/reset" && a.keys.ApprovalURL != "https://account.example.org/reset" {
		t.Fatalf("the page was not opened: %q %q", a.openURL, a.keys.ApprovalURL)
	}
	if !tt.HasText("Approve it in your browser") {
		t.Fatal("the approval step is not shown")
	}
	if err := tt.Click("I approved it"); err != nil {
		t.Fatal(err)
	}
	waitFor("the new key", func() bool { return a.keys.Step == 2 })
	if !tt.HasText("Your new Security Key") || !tt.HasText("EsTc WLwb Z5P1") {
		t.Fatal("the new key is not shown")
	}
	if src.resetReq == nil || src.resetReq.Password != "" {
		t.Fatalf("request = %+v", src.resetReq)
	}
	if err := tt.Click("I've saved it"); err != nil {
		t.Fatal(err)
	}
	frames(tt)
	if a.keys.Sheet.Open || a.keys.RecoveryKey != "" {
		t.Fatal("the key is kept after the dialog closed")
	}

	// Giving up in the browser step.
	a.openKeys(keysReset)
	frames(tt)
	tt.Click("Reset identity")
	waitFor("the approval step again", func() bool { return a.keys.Step == 1 })
	tt.Click("Cancel")
	waitFor("the failure", func() bool { return a.keys.Err != "" })
	if !strings.Contains(a.keys.Err, "approval was not given") || a.keys.Step != 0 || a.keys.RecoveryKey != "" {
		t.Fatalf("err %q step %d key %q", a.keys.Err, a.keys.Step, a.keys.RecoveryKey)
	}
}

// A password account is asked for its password first, and it goes with the
// request; without one nothing starts.
func TestResetIdentityNeedsPassword(t *testing.T) {
	a, src, tt := keysApp(t)
	src.needPass = true
	a.openKeys(keysReset)
	frames(tt)
	if !tt.HasText("Your account password") {
		t.Fatal("no password field")
	}
	tt.Click("Reset identity")
	frames(tt)
	if src.resetReq != nil || !tt.HasText("Enter your account password") {
		t.Fatal("started with no password")
	}
	a.keys.Password = "hunter2"
	tt.Click("Reset identity")
	for i := 0; i < 100 && a.keys.Step != 2; i++ {
		frames(tt)
		time.Sleep(10 * time.Millisecond)
	}
	if src.resetReq == nil || src.resetReq.Password != "hunter2" || a.keys.Step != 2 || a.keys.Password != "" {
		t.Fatalf("request %+v, step %d, password kept %q", src.resetReq, a.keys.Step, a.keys.Password)
	}
}

// Export logs asks what to keep, makes the zip and saves it where asked.
func TestExportLogs(t *testing.T) {
	a, src, _ := keysApp(t)
	dir := t.TempDir()
	a.saveFileFn = func(title, name string, done func(string)) { done(filepath.Join(dir, name)) }
	for _, include := range []bool{false, true} {
		a.confirmLogs = func(answer func(include, ok bool)) { answer(include, true) }
		a.exportLogs()
	}
	if len(src.logs) != 2 || src.logs[0] || !src.logs[1] {
		t.Fatalf("exports asked with room IDs: %v", src.logs)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "slopement-logs-2026-11-22.zip")); err != nil || string(b) != "PK-zip" {
		t.Fatalf("saved zip: %v %q", err, b)
	}
	if !strings.HasPrefix(a.toast, "Saved slopement-logs") {
		t.Fatalf("toast %q", a.toast)
	}
	a.confirmLogs = func(answer func(include, ok bool)) { answer(false, false) } // cancelled
	a.exportLogs()
	if len(src.logs) != 2 {
		t.Fatal("exported though the user cancelled")
	}
}

// The identity reset in the verify dialog's start opens the same dialog.
func TestVerifyDialogResetOpensTheResetDialog(t *testing.T) {
	a, _, tt := keysApp(t)
	a.verify.Open, a.verifyStep = true, 0
	frames(tt)
	if err := tt.Click("Can't verify?"); err != nil {
		t.Fatal(err)
	}
	frames(tt)
	if a.verify.Open || !a.keys.Sheet.Open || a.keys.Mode != keysReset {
		t.Fatalf("verify open %v, keys open %v mode %v", a.verify.Open, a.keys.Sheet.Open, a.keys.Mode)
	}
}
