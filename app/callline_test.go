package app

import (
	"testing"
	"time"

	"slopement/model"
)

func TestCallLineWords(t *testing.T) {
	ended := func(d time.Duration) model.Event {
		return model.Event{Body: "The call ended", Call: &model.CallLine{Ended: true, Duration: d}}
	}
	for d, want := range map[time.Duration]string{
		0:                               "The call ended",
		20 * time.Second:                "The call ended · under a minute",
		7*time.Minute + 20*time.Second:  "The call ended · 7 min",
		59*time.Minute + 40*time.Second: "The call ended · 1 h",
		time.Hour + 5*time.Minute:       "The call ended · 1 h 5 min",
		3 * time.Hour:                   "The call ended · 3 h",
	} {
		if got := callLineText(ended(d)); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
	started := model.Event{Body: "Kai started a call", Call: &model.CallLine{}}
	if got := callLineText(started); got != "Kai started a call" {
		t.Errorf("started: %q", got)
	}
}
