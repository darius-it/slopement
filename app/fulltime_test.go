package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestFullTime(t *testing.T) {
	now := time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 10, 7, 12, 2, 0, 0, time.Local), "Wednesday 7th October, 12:02"},
		{time.Date(2026, 3, 1, 8, 5, 0, 0, time.Local), "Sunday 1st March, 08:05"},
		{time.Date(2026, 3, 2, 23, 59, 0, 0, time.Local), "Monday 2nd March, 23:59"},
		{time.Date(2026, 3, 3, 0, 0, 0, 0, time.Local), "Tuesday 3rd March, 00:00"},
		{time.Date(2026, 3, 11, 10, 0, 0, 0, time.Local), "Wednesday 11th March, 10:00"},
		{time.Date(2026, 3, 12, 10, 0, 0, 0, time.Local), "Thursday 12th March, 10:00"},
		{time.Date(2026, 3, 13, 10, 0, 0, 0, time.Local), "Friday 13th March, 10:00"},
		{time.Date(2026, 3, 21, 10, 0, 0, 0, time.Local), "Saturday 21st March, 10:00"},
		{time.Date(2026, 3, 22, 10, 0, 0, 0, time.Local), "Sunday 22nd March, 10:00"},
		{time.Date(2026, 3, 23, 10, 0, 0, 0, time.Local), "Monday 23rd March, 10:00"},
		{time.Date(2025, 10, 7, 12, 2, 0, 0, time.Local), "Tuesday 7th October 2025, 12:02"}, // another year
	} {
		if got := fullTime(tc.at, now); got != tc.want {
			t.Errorf("fullTime(%v) = %q, want %q", tc.at, got, tc.want)
		}
	}
	if fullTime(time.Time{}, now) != "" {
		t.Error("no time, no tooltip")
	}
}

// Hovering a message's time shows it in full.
func TestHoverTimeShowsTheFullDate(t *testing.T) {
	a, ts, _ := thrApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	settle(tt)
	ev := ts.tl[3] // a message that starts a group: its time sits by its name
	label := ev.Time.Format("15:04")
	at, ok := tt.Find(label)
	if !ok {
		t.Fatalf("no time %q on screen", label)
	}
	tt.Move(at.X+at.W/2, at.Y+at.H/2)
	tt.Advance(2 * time.Second)
	tt.Frame()
	want := fullTime(ev.Time, a.now)
	if !tt.HasText(want) {
		t.Fatalf("hovering %q did not show %q", label, want)
	}
}
