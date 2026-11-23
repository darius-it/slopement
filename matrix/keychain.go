package matrix

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// Starting the keychain. On Linux the keychain is a Secret Service on the
// session's D-Bus (GNOME Keyring, KDE's ksecretd/KWallet, KeePassXC…). When the
// app asks and nothing answers, other programs that keep secrets (the Minecraft
// launcher, browsers) get it started: D-Bus can launch a service on demand when
// the distribution installed its activation file, and a keyring daemon can be
// started by hand. This does both, once, and asks again — so a sign-in works on
// a desktop where the keyring was not running yet. It cannot make a keyring
// where none is installed.

// startKeychainOnce is how often it tries: not again within this long.
const startKeychainEvery = 20 * time.Second

var (
	keychainMu    sync.Mutex
	keychainTried time.Time

	// canStartKeychain: only Linux has a keychain that may not be running.
	// (A variable: a test turns it on elsewhere.)
	canStartKeychain = runtime.GOOS == "linux"

	// The seams: the commands it runs, how it finds a program, how it waits.
	keychainRun   = runCommand
	keychainFind  = exec.LookPath
	keychainSleep = time.Sleep
)

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// withKeychain runs a keychain operation; if it fails because the keychain is
// not there (not because the secret is missing), the keychain is started once
// and the operation is tried again.
func withKeychain(op func() error) error {
	err := op()
	if err == nil || errors.Is(err, mygo.ErrSecretNotFound) || !tryStartKeychain() {
		return err
	}
	return op()
}

// tryStartKeychain starts the keychain if it is not running, and says whether
// it is now: false when it was running already (so the failure is something
// else, a locked keyring, a refusal), when it cannot be started here, or when
// it was tried a moment ago.
func tryStartKeychain() bool {
	if !canStartKeychain {
		return false
	}
	keychainMu.Lock()
	defer keychainMu.Unlock()
	if !keychainTried.IsZero() && time.Since(keychainTried) < startKeychainEvery {
		return false
	}
	keychainTried = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return startKeychain(ctx)
}

const secretService = "org.freedesktop.secrets"

// secretServiceRunning says something owns the Secret Service's name on the
// session bus.
func secretServiceRunning(ctx context.Context) bool {
	if _, err := keychainFind("busctl"); err == nil {
		out, err := keychainRun(ctx, "busctl", "--user", "--no-pager", "status", secretService)
		return err == nil && len(strings.TrimSpace(string(out))) > 0
	}
	if _, err := keychainFind("dbus-send"); err == nil {
		out, err := keychainRun(ctx, "dbus-send", "--session", "--print-reply", "--dest=org.freedesktop.DBus",
			"/org/freedesktop/DBus", "org.freedesktop.DBus.NameHasOwner", "string:"+secretService)
		return err == nil && strings.Contains(string(out), "boolean true")
	}
	return false
}

// waitForSecretService polls for the service's name to appear.
func waitForSecretService(ctx context.Context) bool {
	for i := 0; i < 20; i++ {
		if secretServiceRunning(ctx) {
			return true
		}
		keychainSleep(200 * time.Millisecond)
	}
	return false
}

// startKeychain: first as other programs do, by asking D-Bus to launch the
// service (it can when an activation file is installed); failing that, by
// starting a keyring daemon that is installed.
func startKeychain(ctx context.Context) bool {
	if secretServiceRunning(ctx) {
		return false
	}
	// Ask the bus to start it.
	if _, err := keychainFind("busctl"); err == nil {
		_, _ = keychainRun(ctx, "busctl", "--user", "call", "org.freedesktop.DBus", "/org/freedesktop/DBus",
			"org.freedesktop.DBus", "StartServiceByName", "su", secretService, "0")
	} else if _, err := keychainFind("dbus-send"); err == nil {
		_, _ = keychainRun(ctx, "dbus-send", "--session", "--print-reply", "--dest=org.freedesktop.DBus",
			"/org/freedesktop/DBus", "org.freedesktop.DBus.StartServiceByName", "string:"+secretService, "uint32:0")
	}
	if waitForSecretService(ctx) {
		debugf("keychain: started by D-Bus activation")
		return true
	}
	// Start a keyring daemon that is here, the desktop's own first.
	for _, d := range keychainDaemons(keychainEnv("XDG_CURRENT_DESKTOP")) {
		if _, err := keychainFind(d.name); err != nil {
			continue
		}
		if d.stays {
			// It stays running: start it, do not wait for it to end.
			if err := keychainStart(d.name, d.args...); err != nil {
				continue
			}
		} else if _, err := keychainRun(ctx, d.name, d.args...); err != nil {
			continue
		}
		if waitForSecretService(ctx) {
			debugf("keychain: started %s", d.name)
			return true
		}
	}
	return false
}

// A daemon that can offer the Secret Service. stays: it keeps running (it is
// started and left), unlike one that starts a background process and exits.
type keychainDaemon struct {
	name  string
	args  []string
	stays bool
}

// keychainDaemons are the programs to try, for the desktop (XDG_CURRENT_DESKTOP):
// on KDE its wallet first — Plasma 6's ksecretd, then kwalletd6 and kwalletd5,
// which offer the Secret Service when KWallet's setting for it is on — then GNOME
// Keyring; elsewhere GNOME Keyring first.
func keychainDaemons(desktop string) []keychainDaemon {
	gnome := keychainDaemon{"gnome-keyring-daemon", []string{"--start", "--components=secrets"}, false}
	kde := []keychainDaemon{{"ksecretd", nil, true}, {"kwalletd6", nil, true}, {"kwalletd5", nil, true}}
	if strings.Contains(strings.ToUpper(desktop), "KDE") {
		return append(kde, gnome)
	}
	return append([]keychainDaemon{gnome}, kde...)
}

// keychainEnv reads the environment; keychainStart starts a program and leaves
// it running. Seams, for tests.
var (
	keychainEnv   = os.Getenv
	keychainStart = func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
)

// KeychainHelp says what to do about a keychain that is not there, for this
// desktop, in a sentence for the sign-in's error. On KDE it also notices that
// KWallet is set not to offer the Secret Service (kwalletrc,
// [org.freedesktop.secrets] apiEnabled=false), which is the usual reason it is
// running and still no use to the app.
func KeychainHelp() string {
	if strings.Contains(strings.ToUpper(keychainEnv("XDG_CURRENT_DESKTOP")), "KDE") {
		if kwalletSecretServiceOff() {
			return "KDE Wallet is set not to offer the Secret Service: in System Settings → KDE Wallet, turn on “Use KWallet for the Secret Service interface”, then try again."
		}
		return "On KDE, KDE Wallet has to offer the Secret Service: in System Settings → KDE Wallet turn on “Use KWallet for the Secret Service interface” (the kwallet package provides ksecretd), or install gnome-keyring."
	}
	return "Install and start a Secret Service: GNOME Keyring (gnome-keyring), or KDE Wallet with its Secret Service interface on."
}

// kwalletSecretServiceOff reads kwalletrc for the Secret Service being turned off.
func kwalletSecretServiceOff() bool {
	dir := keychainEnv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		dir = filepath.Join(home, ".config")
	}
	b, err := os.ReadFile(filepath.Join(dir, "kwalletrc"))
	if err != nil {
		return false
	}
	return secretServiceOffIn(string(b))
}

// secretServiceOffIn says a kwalletrc has apiEnabled=false in its
// [org.freedesktop.secrets] section.
func secretServiceOffIn(rc string) bool {
	in := false
	for _, line := range strings.Split(rc, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			in = line == "[org.freedesktop.secrets]"
		case in && strings.EqualFold(strings.ReplaceAll(line, " ", ""), "apiEnabled=false"):
			return true
		}
	}
	return false
}
