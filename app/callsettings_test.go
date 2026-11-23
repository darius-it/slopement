package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The call settings are real: no "Soon", and they are kept with the others.
func TestCallSettingsAreReal(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 1200)
	a.view, a.settings = viewSettings, "Calls"
	settle(tt)
	for _, want := range []string{"Microphone off when joining", "Camera on when joining", "Ask before joining"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q on the Calls page", want)
		}
	}
	if tt.HasText("Soon") {
		t.Fatal("the Calls page still says Soon")
	}
	a.prefs.callMicOff, a.prefs.callCamOn, a.prefs.callLobby, a.prefs.callsAccepted = true, true, true, true
	if got := a.prefs.state().prefs(); !got.callMicOff || !got.callCamOn || !got.callLobby || !got.callsAccepted {
		t.Fatalf("the call settings were not kept: %+v", got)
	}
	if d := defaultPrefs(); d.callMicOff || d.callCamOn || d.callLobby || d.callsAccepted {
		t.Fatal("by default a call joins with the microphone on, the camera off, no lobby, and nothing is agreed to")
	}
}

// People are told, before the first call and in Settings, that the call page
// comes from Element's server, and what it learns.
func TestCallsNoteSaysWhereThePageComesFrom(t *testing.T) {
	for _, want := range []string{"call.element.io", "run by Element", "IP address", "user and device ID", "sign-in token", "your own homeserver"} {
		if !strings.Contains(callsNote, want) {
			t.Errorf("the note does not say %q", want)
		}
	}
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 1400)
	a.view, a.settings = viewSettings, "Calls"
	settle(tt)
	if !tt.HasText("Load the call page from call.element.io") {
		t.Fatal("the switch that takes the agreement back is not on the Calls page")
	}
}
