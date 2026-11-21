package kit

import (
	"github.com/egoist/mygo/ui"
)

// LoginStep is where the sign-in is.
type LoginStep int

const (
	LoginServer   LoginStep = iota // choosing the homeserver
	LoginPassword                  // a username and a password form
	LoginBrowser                   // the server's browser flow (MAS/OIDC)
)

// SavedLogin is a login recorded on this device, offered first: the fast
// way back in.
type SavedLogin struct {
	User   string // the MXID
	Server string // what was typed to sign in
}

// LoginState is the login screen's model.
type LoginState struct {
	Step   LoginStep
	Field  string // the other server, being typed
	User   string
	Pass   string
	Server string // the server as probed, once past choosing
	Saved  []SavedLogin
	Err    string // one line under the form, from the last try
	Busy   bool
}

// LoginAction is what the user did this frame.
type LoginAction struct {
	Continue int    // a saved login's index, -1 none
	Probe    string // a server to resolve and classify, "" none
	SignIn   bool   // the password form, submitted
	Browser  bool   // open the server's browser flow
}

// Login is the sign-in: first the homeserver — a login recorded here,
// matrix.org, or another server found by discovery — then its way in:
// the browser (MAS, the first-class path) or a username and a password.
// It is keyboard-first: Enter submits every step.
//
// References (Mobbin, the slopement login collection): centered
// single-column sign-ins — [Nutmeg], [Centr], [Tana], [Plain] — one
// primary action per step, errors as one quiet line under the form.
//
// [Nutmeg]: https://mobbin.com/screens/2d0e03fb-6e8e-4232-a4c4-6cd1a0b06c0d
// [Centr]: https://mobbin.com/screens/bf438ccb-6402-47bf-8ef2-e3b9c6ff2ab7
// [Tana]: https://mobbin.com/screens/1fa0617c-01bc-4d19-91d7-cce7dd7ba445
// [Plain]: https://mobbin.com/screens/b9ff7441-defd-4155-a73e-ec088c14eff3
func Login(c *ui.Context, st *LoginState, act *LoginAction) {
	*act = LoginAction{Continue: -1}
	p := Of(c)
	ui.Column(c).Fill().Background(p.Bg).AlignItems(ui.Center).Justify(ui.Center).DragWindow().Children(func() {
		ui.Column(c).Width(360).MaxWidthPercent(90).Gap(S6).Children(func() {
			// The wordmark: the name and the accent's dot.
			ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "slopement").FontSize(SizeTitle + 2).FontWeight(WeightSemibold)
				ui.Box(c).Size(8, 8).Radius(RadiusFull).Background(p.Accent)
			})
			switch st.Step {
			case LoginServer:
				serverStep(c, st, act)
			case LoginPassword:
				passwordStep(c, st, act)
			case LoginBrowser:
				browserStep(c, st, act)
			}
		})
	})
}

// loginErr is the one quiet line under the form.
func loginErr(c *ui.Context, st *LoginState) {
	if st.Err != "" {
		ui.Text(c, st.Err).FontSize(SizeSmall).TextColor(Of(c).Danger)
	}
}

// serverHeader is the probed server over the later steps, with the way
// back to choosing another.
func serverHeader(c *ui.Context, st *LoginState) {
	p := Of(c)
	ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, IconGlobe).Size(16, 16).TextColor(p.TextMuted)
		ui.Text(c, st.Server).FontSize(SizeBody).FontWeight(WeightMedium).Grow(1).SingleLine()
		if kitGhostLink(c, "Change").Clicked() {
			st.Step, st.Err, st.Busy = LoginServer, "", false
		}
	})
}

// kitGhostLink is a small quiet text button, as headers' changes are.
func kitGhostLink(c *ui.Context, label string) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(label)).Height(28).PaddingX(S2).Radius(RadiusSm).AlignItems(ui.Center).Label(label)
	fg := p.TextMuted
	if b.Hovered() {
		fg = p.Text
		b.Background(p.Hover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(SizeSmall).TextColor(fg) })
	return b
}

// serverStep is the homeserver: recorded logins, matrix.org, or another
// server by its address.
func serverStep(c *ui.Context, st *LoginState, act *LoginAction) {
	p := Of(c)
	ui.Text(c, "Where is your account?").FontSize(SizeSmall).TextColor(p.TextMuted)

	// Logins recorded on this device, newest first.
	for i, s := range st.Saved {
		if i >= 3 {
			break
		}
		row := ChoiceRow(c, IconUser, s.User, s.Server)
		if row.Clicked() {
			act.Continue = i
		}
	}
	if ChoiceRow(c, IconGlobe, "matrix.org", "The public homeserver").Clicked() {
		act.Probe = "matrix.org"
	}

	ui.Column(c).Gap(S15).Children(func() {
		in := TextField(c.Key("server"), &st.Field, "Other server", "your-server.org")
		if in.Submitted() && st.Field != "" {
			act.Probe = st.Field
		}
		if st.Busy {
			ui.Text(c, "Checking the server…").FontSize(SizeSmall).TextColor(p.TextMuted)
		} else {
			if Button(c, Primary, "Continue", IconChevronR).Clicked() && st.Field != "" {
				act.Probe = st.Field
			}
		}
	})
	loginErr(c, st)
}

// passwordStep is the server's username and password form.
func passwordStep(c *ui.Context, st *LoginState, act *LoginAction) {
	p := Of(c)
	serverHeader(c, st)
	ui.Column(c).Gap(S15).Children(func() {
		TextField(c.Key("user"), &st.User, "Username", "ada").AutoFocus()
		var pass ui.Element
		ui.Column(c).Gap(S15).Children(func() {
			ui.Text(c, "Password").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted)
			box := ui.Row(c)
			box.Children(func() {
				pass = ui.TextInputBase(c.Key("pass"), &st.Pass).Grow(1).Password().
					Placeholder("••••••••").PlaceholderColor(p.TextMuted).Label("Password").FontSize(fieldSize(c))
			})
			field(c, box)
		})
		if pass.Submitted() {
			act.SignIn = true
		}
		if st.Busy {
			ui.Text(c, "Signing in…").FontSize(SizeSmall).TextColor(p.TextMuted)
		} else if Button(c, Primary, "Sign in", nil).Clicked() {
			act.SignIn = true
		}
	})
	loginErr(c, st)
}

// browserStep is the server's browser flow, MAS's way in: open, sign in
// there, and the window waits for the return.
func browserStep(c *ui.Context, st *LoginState, act *LoginAction) {
	p := Of(c)
	serverHeader(c, st)
	ui.Text(c, st.Server+" signs in through your browser.").FontSize(SizeBody).TextColor(p.Text)
	ui.Text(c, "This window is waiting for you to come back.").FontSize(SizeSmall).TextColor(p.TextMuted)
	if st.Busy {
		ui.Text(c, "Waiting for the browser…").FontSize(SizeSmall).TextColor(p.TextMuted)
	} else if Button(c, Primary, "Continue in the browser", IconGlobe).Clicked() {
		act.Browser = true
	}
	loginErr(c, st)
}
