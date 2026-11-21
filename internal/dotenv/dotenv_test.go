package dotenv

import "testing"

func TestLine(t *testing.T) {
	for _, tc := range []struct {
		in, k, v string
		ok       bool
	}{
		{"KLIPY_API_KEY=abc", "KLIPY_API_KEY", "abc", true},
		{"export A = \"x y\"", "A", "x y", true},
		{"B='q'", "B", "q", true},
		{"C=v # note", "C", "v", true},
		{"EMPTY=", "EMPTY", "", true},
		{"# comment", "", "", false},
		{"", "", "", false},
		{"novalue", "", "", false},
	} {
		k, v, ok := Line(tc.in)
		if k != tc.k || v != tc.v || ok != tc.ok {
			t.Errorf("Line(%q) = %q, %q, %v; want %q, %q, %v", tc.in, k, v, ok, tc.k, tc.v, tc.ok)
		}
	}
}
