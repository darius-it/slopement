package matrix

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// fakeDesktop is a session where the Secret Service is not running until the
// right thing is done: an activation file (the bus starts it), or a daemon.
type fakeDesktop struct {
	programs map[string]bool // what is installed
	activate bool            // D-Bus can start the service by itself
	starts   []string        // what was started, in order
	running  atomic.Bool     // someone owns the Secret Service name
	// daemon is the program whose start brings the service up.
	daemon string
}

func (d *fakeDesktop) install(t *testing.T, desktop string) {
	t.Helper()
	oldRun, oldFind, oldSleep, oldEnv, oldStart, oldCan, oldTried := keychainRun, keychainFind, keychainSleep, keychainEnv, keychainStart, canStartKeychain, keychainTried
	t.Cleanup(func() {
		keychainRun, keychainFind, keychainSleep, keychainEnv, keychainStart, canStartKeychain, keychainTried = oldRun, oldFind, oldSleep, oldEnv, oldStart, oldCan, oldTried
	})
	canStartKeychain, keychainTried = true, time.Time{}
	keychainSleep = func(time.Duration) {}
	keychainEnv = func(k string) string {
		if k == "XDG_CURRENT_DESKTOP" {
			return desktop
		}
		return ""
	}
	keychainFind = func(name string) (string, error) {
		if d.programs[name] {
			return "/usr/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
	keychainRun = func(_ context.Context, name string, args ...string) ([]byte, error) {
		has := func(s string) bool {
			for _, a := range args {
				if a == s {
					return true
				}
			}
			return false
		}
		switch {
		case name == "busctl" && has("status"):
			if d.running.Load() {
				return []byte("Name=org.freedesktop.secrets\n"), nil
			}
			return nil, errors.New("exit status 1")
		case name == "busctl" && has("call"):
			d.starts = append(d.starts, "activate")
			if d.activate {
				d.running.Store(true)
			}
			return nil, nil
		}
		d.starts = append(d.starts, name)
		if name == d.daemon {
			d.running.Store(true)
		}
		return nil, nil
	}
	keychainStart = func(name string, args ...string) error {
		d.starts = append(d.starts, name)
		if name == d.daemon {
			d.running.Store(true)
		}
		return nil
	}
}

// With no Secret Service running, the keychain is started — by D-Bus when it
// can, else by the desktop's own wallet — and the operation tried again.
func TestKeychainIsStartedWhenItIsNotRunning(t *testing.T) {
	// On KDE: D-Bus cannot (no activation file), so its wallet is started.
	d := &fakeDesktop{programs: map[string]bool{"busctl": true, "ksecretd": true, "gnome-keyring-daemon": true}, daemon: "ksecretd"}
	d.install(t, "KDE")
	var tries int
	err := withKeychain(func() error {
		tries++
		if !d.running.Load() {
			return errors.New("The name org.freedesktop.secrets was not provided by any .service files")
		}
		return nil
	})
	if err != nil || tries != 2 {
		t.Fatalf("err %v after %d tries", err, tries)
	}
	if got := strings.Join(d.starts, ","); got != "activate,ksecretd" {
		t.Fatalf("started: %s (KDE: ask D-Bus, then KWallet's ksecretd — not GNOME Keyring)", got)
	}
}

func TestKeychainByDBusActivationNeedsNoDaemon(t *testing.T) {
	d := &fakeDesktop{programs: map[string]bool{"busctl": true, "ksecretd": true}, activate: true}
	d.install(t, "KDE")
	if !tryStartKeychain() {
		t.Fatal("not started by D-Bus")
	}
	if got := strings.Join(d.starts, ","); got != "activate" {
		t.Fatalf("started: %s", got)
	}
}

// Elsewhere GNOME Keyring first; on KDE the order is KWallet's programs; what is
// not installed is skipped.
func TestKeychainDaemonsByDesktop(t *testing.T) {
	names := func(desktop string) string {
		var out []string
		for _, d := range keychainDaemons(desktop) {
			out = append(out, d.name)
		}
		return strings.Join(out, ",")
	}
	if got := names("KDE"); got != "ksecretd,kwalletd6,kwalletd5,gnome-keyring-daemon" {
		t.Fatalf("KDE: %s", got)
	}
	if got := names("GNOME"); got != "gnome-keyring-daemon,ksecretd,kwalletd6,kwalletd5" {
		t.Fatalf("GNOME: %s", got)
	}
	// Only the older KWallet is installed: that one.
	d := &fakeDesktop{programs: map[string]bool{"busctl": true, "kwalletd5": true}, daemon: "kwalletd5"}
	d.install(t, "KDE")
	if !tryStartKeychain() || strings.Join(d.starts, ",") != "activate,kwalletd5" {
		t.Fatalf("started: %v", d.starts)
	}
}

// A running Secret Service is left alone (the failure is a lock or a refusal),
// nothing installed means no start, a missing secret is not a keychain failure,
// and it does not try again within the minute.
func TestKeychainIsLeftAloneWhenItShouldBe(t *testing.T) {
	d := &fakeDesktop{programs: map[string]bool{"busctl": true, "ksecretd": true}, daemon: "ksecretd"}
	d.install(t, "KDE")
	d.running.Store(true)
	if tryStartKeychain() || len(d.starts) != 0 {
		t.Fatalf("it started something while the service runs: %v", d.starts)
	}

	d = &fakeDesktop{programs: map[string]bool{"busctl": true}}
	d.install(t, "KDE")
	if tryStartKeychain() {
		t.Fatal("it started a keychain that is not installed")
	}
	if tryStartKeychain() { // a second time at once: not asked again
		t.Fatal("it tried again within the minute")
	}

	d = &fakeDesktop{programs: map[string]bool{"busctl": true, "ksecretd": true}, daemon: "ksecretd"}
	d.install(t, "KDE")
	tries := 0
	if err := withKeychain(func() error { tries++; return mygo.ErrSecretNotFound }); !errors.Is(err, mygo.ErrSecretNotFound) || tries != 1 || len(d.starts) != 0 {
		t.Fatalf("a missing secret started the keychain: %v, %d tries, %v", err, tries, d.starts)
	}
	canStartKeychain = false // macOS, Windows
	if tryStartKeychain() {
		t.Fatal("it tried to start a keychain where there is nothing to start")
	}
}

// KDE's own advice, and the case where KWallet is set to keep the Secret
// Service off.
func TestKeychainHelpForKDE(t *testing.T) {
	d := &fakeDesktop{}
	d.install(t, "KDE")
	if got := KeychainHelp(); !strings.Contains(got, "KDE Wallet") || !strings.Contains(got, "Secret Service interface") {
		t.Fatalf("KDE help: %s", got)
	}
	if got := KeychainHelp(); strings.Contains(got, "set not to offer") {
		t.Fatalf("it says the setting is off with no kwalletrc: %s", got)
	}
	d.install(t, "GNOME")
	if got := KeychainHelp(); !strings.Contains(got, "gnome-keyring") {
		t.Fatalf("other help: %s", got)
	}
	for rc, want := range map[string]bool{
		"[org.freedesktop.secrets]\napiEnabled=false\n":           true,
		"[org.freedesktop.secrets]\napiEnabled = false\n":         true,
		"[org.freedesktop.secrets]\napiEnabled=true\n":            false,
		"[Wallet]\napiEnabled=false\n":                            false,
		"[Wallet]\nEnabled=true\n[org.freedesktop.secrets]\n":     false,
		"[Wallet]\n[org.freedesktop.secrets]\napiEnabled=false\n": true,
	} {
		if secretServiceOffIn(rc) != want {
			t.Errorf("secretServiceOffIn(%q) != %v", rc, want)
		}
	}
}
