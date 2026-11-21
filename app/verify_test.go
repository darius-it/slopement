package app

import (
	"testing"
	"time"

	"slopement/model"
	"slopement/model/mock"

	"github.com/egoist/mygo/ui"
)

// TestVerifyFlow walks the verify dialog on the mock's fake: another
// device's emoji, then the Security Key, and a mismatch declined.
func TestVerifyFlow(t *testing.T) {
	a := testApp()
	a.verify.Open = true
	tt := ui.NewTester(a.View, 1280, 900)
	tt.Frame()

	if !tt.HasText("Verify this device") {
		t.Fatal("the start screen did not show")
	}

	// With another device: the emoji, matching.
	if err := tt.Click("Use another device"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Compare emoji") || !tt.HasText("🐶") {
		t.Fatal("the emoji step did not show")
	}
	if err := tt.Click("They match"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("This session is verified") {
		t.Fatal("the done screen did not show")
	}
	if err := tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.verify.Open {
		t.Fatal("the dialog stayed open after Done")
	}
	if a.verifier.State().Phase != model.VerifyDone {
		t.Fatalf("phase = %d, want done", a.verifier.State().Phase)
	}

	// The Security Key: typed and submitted, from a fresh flow.
	a.verifier = mock.New(time.Now()).Verify()
	a.verify.Open, a.verifyStep = true, 0
	tt.Frame()
	if err := tt.Click("Use your Security Key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Security Key") || tt.HasText("Compare emoji") {
		t.Fatal("the key entry did not show")
	}
	for _, x := range tt.AccessAll() {
		if x.Role == ui.RoleTextField && x.Name == "Security Key" {
			tt.ClickAt(x.Bounds.X+x.Bounds.W/2, x.Bounds.Y+x.Bounds.H/2)
		}
	}
	tt.Type("EsTc WLwb Z5P1")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.verifyStep != 0 || a.verifyKey != "" {
		t.Fatalf("the key entry did not submit: step %d, key %q", a.verifyStep, a.verifyKey)
	}
	tt.Frame()
	if !tt.HasText("This session is verified") {
		t.Fatal("the key did not verify")
	}
	if err := tt.Click("Done"); err != nil {
		t.Fatal(err)
	}

	// A mismatch declines: the start screen returns with its line.
	a.verifier = mock.New(time.Now()).Verify()
	a.verify.Open, a.verifyStep = true, 0
	tt.Frame()
	if err := tt.Click("Use another device"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("They don't match"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.verifier.State().Phase != model.VerifyIdle {
		t.Fatalf("phase = %d, want idle after a mismatch", a.verifier.State().Phase)
	}
	if !tt.HasText("The emoji didn't match") {
		t.Fatal("the mismatch has no line")
	}
}
