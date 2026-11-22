package kit

import (
	"testing"
	"time"
)

// "Today" clears at the user's own midnight, whatever their zone; the clear
// times are moments, so a status set in one zone ends at the same instant in
// another's clock.
func TestClearTimeIsInTheUsersZone(t *testing.T) {
	for _, zone := range []string{"Europe/Berlin", "UTC", "America/New_York", "Asia/Tokyo", "Pacific/Auckland"} {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Skipf("no zone data for %s", zone)
		}
		now := time.Date(2026, 11, 22, 22, 36, 0, 0, loc)
		today := ClearTime(3, now)
		if want := time.Date(2026, 11, 23, 0, 0, 0, 0, loc); !today.Equal(want) {
			t.Errorf("%s: Today clears at %v, want %v", zone, today, want)
		}
		if !(Status{Emoji: "💬", Until: today}).Set(now) {
			t.Errorf("%s: a status set for Today is gone at %v", zone, now)
		}
		if want := now.Add(4 * time.Hour); !ClearTime(2, now).Equal(want) {
			t.Errorf("%s: 4 hours clears at %v", zone, ClearTime(2, now))
		}
		// The same instant read on another clock is still before the end.
		if !(Status{Emoji: "💬", Until: today}).Set(now.UTC()) {
			t.Errorf("%s: the status ends early when read in UTC", zone)
		}
	}
}
