package kit

import (
	"testing"
	"time"
)

func TestInitials(t *testing.T) {
	for in, want := range map[string]string{
		"Alex Rivera":        "AR",
		"mira":               "M",
		"@sam:matrix.org":    "SM",
		"Family 🏡":           "F",
		"🎉 Party":            "P",
		"":                   "?",
		"#design-crit":       "DC",
		"Noor الحسيني":       "Nا",
		"a-very-long-name-x": "AV",
	} {
		if got := Initials(in); got != want {
			t.Errorf("Initials(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFuzzyScore(t *testing.T) {
	if FuzzyScore("xyz", "design-crit") >= 0 {
		t.Error("non-match scored")
	}
	if FuzzyScore("dc", "design-crit") < 0 {
		t.Error("subsequence did not match")
	}
	if FuzzyScore("des", "design-crit") <= FuzzyScore("des", "codes") {
		t.Error("prefix should beat a later match")
	}
	if FuzzyScore("ALEX", "Alex Rivera") <= 0 {
		t.Error("case should not matter")
	}
}

func TestUserColors(t *testing.T) {
	var u UserColors
	h := HashIndex("@a:b")
	if u.UserIndex("@a:b") != h {
		t.Fatal("hash differs")
	}
	u.Set("@a:b", (h+1)%8)
	if u.UserIndex("@a:b") != (h+1)%8 {
		t.Fatal("override ignored")
	}
	u.Set("@a:b", -1)
	if u.UserIndex("@a:b") != h {
		t.Fatal("clearing did not restore the hash")
	}
}

func TestMutedUntilText(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		until time.Time
		want  string
	}{
		{time.Time{}, "Muted"},
		{now.Add(2 * time.Hour), "Muted until 14:00"},
		{now.Add(24 * time.Hour), "Muted until Sun 12:00"},
		{now.Add(7 * 24 * time.Hour), "Muted until 28 Nov"},
	} {
		if got := MutedUntilText(tc.until, now); got != tc.want {
			t.Errorf("MutedUntilText(%v) = %q, want %q", tc.until, got, tc.want)
		}
	}
}
