package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// uiTester is the app's tester at the standard size, settled.
func uiTester(a *App) *ui.Tester {
	tt := ui.NewTester(a.View, 1280, 900)
	return tt
}

// stubVerify is the crypto layer's verification as a test wants it.
type stubVerify struct {
	st model.VerifyState
}

func (v *stubVerify) State() model.VerifyState { return v.st }
func (v *stubVerify) BeginWithDevice()         {}
func (v *stubVerify) Accept()                  {}
func (v *stubVerify) Decline()                 {}
func (v *stubVerify) Confirm(match bool)       {}
func (v *stubVerify) UseKey(key string)        {}

// TestVerifyPromptOnce pins the once-a-run prompt: unverified with
// unreadable messages, the verify dialog opens by itself, once — even
// after the messages arrive while it is still open.
func TestVerifyPromptOnce(t *testing.T) {
	a := testApp()
	v := &stubVerify{st: model.VerifyState{Phase: model.VerifyIdle, Needed: true}}
	a.verifier = v
	tt := uiTester(a)
	tt.Frame()

	if !a.verify.Open || !a.verifyAsked {
		t.Fatal("the prompt did not open")
	}
	if !tt.HasText("Verify this device") {
		t.Fatal("the prompt did not show the choices")
	}

	// Declining it (closing) does not bring it back this run.
	a.verify.Open = false
	tt.Frame()
	if a.verify.Open {
		t.Fatal("the prompt opened again in the same run")
	}

	// A new run asks again.
	a.verifyAsked = false
	tt.Frame()
	if !a.verify.Open {
		t.Fatal("a new run did not prompt")
	}

	// Verified: no prompt, whatever came before.
	a.verify.Open, a.verifyAsked = false, false
	v.st = model.VerifyState{Phase: model.VerifyDone}
	tt.Frame()
	if a.verify.Open {
		t.Fatal("a verified session was prompted")
	}
}
