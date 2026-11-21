package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// focusInput clicks the text input named label (its label text is a
// different element; the input is found by its role).
func focusInput(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleTextField && a.Name == label {
			tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
			return
		}
	}
	t.Fatalf("no text input named %q", label)
}

// runLogin renders the login and records the last action it reported,
// as the screen's real consumer (package login) does inside its own
// build — the next pass resets the action.
func runLogin(st *LoginState, last *LoginAction) func(*ui.Context) {
	return func(c *ui.Context) {
		Apply(c)
		var act LoginAction
		Login(c, st, &act)
		if act.Continue >= 0 || act.Probe != "" || act.SignIn || act.Browser {
			*last = act
		}
	}
}

// clickButton clicks the button named label — its center, not a text
// element with the same name inside another row.
func clickButton(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == label {
			tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
			return
		}
	}
	t.Fatalf("no button named %q", label)
}

func TestLoginServerStep(t *testing.T) {
	st := &LoginState{Saved: []SavedLogin{{User: "@ada:example.org", Server: "matrix.org"}}}
	var last LoginAction
	tt := ui.NewTester(runLogin(st, &last), 800, 600)
	tt.Frame()

	if !tt.HasText("slopement") || !tt.HasText("Where is your account?") {
		t.Fatal("the server step did not render")
	}
	if !tt.HasText("@ada:example.org") || !tt.HasText("matrix.org") {
		t.Fatal("the saved login or the matrix.org row is missing")
	}

	if err := tt.Click("@ada:example.org"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if last.Continue != 0 {
		t.Fatalf("continue = %d", last.Continue)
	}
	// The saved row's subtitle says "matrix.org" too: the button is
	// found by its role, so the click lands on the right row.
	clickButton(t, tt, "matrix.org")
	tt.Frame()
	if last.Probe != "matrix.org" {
		t.Fatalf("probe = %q", last.Probe)
	}

	// A typed server probes on Enter.
	focusInput(t, tt, "Other server")
	tt.Type("myserver.org")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if last.Probe != "myserver.org" {
		t.Fatalf("typed probe = %q", last.Probe)
	}
}

func TestLoginPasswordStep(t *testing.T) {
	st := &LoginState{Step: LoginPassword, Server: "myserver.org"}
	var last LoginAction
	tt := ui.NewTester(runLogin(st, &last), 800, 600)
	tt.Frame()

	if !tt.HasText("myserver.org") || !tt.HasText("Username") || !tt.HasText("Password") {
		t.Fatal("the password step did not render")
	}
	// The way back to the server step.
	if err := tt.Click("Change"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if st.Step != LoginServer {
		t.Fatalf("step = %d, want the server step", st.Step)
	}
	st.Step = LoginPassword
	tt.Frame()

	focusInput(t, tt, "Username")
	tt.Type("ada")
	focusInput(t, tt, "Password")
	tt.Type("hunter2")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !last.SignIn {
		t.Fatal("Enter in the password did not sign in")
	}
	// A wrong try shows one quiet line.
	st.Err, st.Busy = "Wrong username or password.", true
	tt.Frame()
	if !tt.HasText("Wrong username or password.") || !tt.HasText("Signing in…") {
		t.Fatal("the error or busy line is missing")
	}
}

func TestLoginBrowserStep(t *testing.T) {
	st := &LoginState{Step: LoginBrowser, Server: "matrix.org"}
	var last LoginAction
	tt := ui.NewTester(runLogin(st, &last), 800, 600)
	tt.Frame()

	if !tt.HasText("matrix.org signs in through your browser.") {
		t.Fatal("the browser step did not render")
	}
	if err := tt.Click("Continue in the browser"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !last.Browser {
		t.Fatal("the browser button did not report")
	}
	// While waiting, one quiet line instead of the button.
	st.Busy = true
	tt.Frame()
	if !tt.HasText("Waiting for the browser…") {
		t.Fatal("the waiting line is missing")
	}
}

// TestLoginSnapshots writes each step, wide and phone, light and dark;
// with SNAP_DIR it writes each.
func TestLoginSnapshots(t *testing.T) {
	steps := []struct {
		name string
		st   LoginState
	}{
		{"server", LoginState{Saved: []SavedLogin{{User: "@ada:example.org", Server: "matrix.org"}}}},
		{"password", LoginState{Step: LoginPassword, Server: "myserver.org", User: "ada", Pass: "hunter2"}},
		{"browser", LoginState{Step: LoginBrowser, Server: "matrix.org", Busy: true}},
	}
	dir := os.Getenv("SNAP_DIR")
	for _, dark := range []bool{false, true} {
		for _, compact := range []bool{false, true} {
			for _, s := range steps {
				w, h := 1280, 860
				tag := "wide"
				if compact {
					w, h, tag = 390, 700, "phone"
				}
				tt := ui.NewTesterWith(func(c *ui.Context) {
					Apply(c)
					var act LoginAction
					Login(c, &s.st, &act)
				}, w, h, ui.TesterOptions{Dark: dark})
				tt.Frame()
				if dir != "" {
					theme := "light"
					if dark {
						theme = "dark"
					}
					f, err := os.Create(filepath.Join(dir, "login-"+s.name+"-"+tag+"-"+theme+".png"))
					if err == nil {
						png.Encode(f, tt.Image())
						f.Close()
					}
				}
			}
		}
	}
}
