package matrix

import "testing"

func TestParseGeo(t *testing.T) {
	for _, tc := range []struct {
		in       string
		lat, lon float64
		ok       bool
	}{
		{"geo:52.5,13.4", 52.5, 13.4, true},
		{"geo:52.5,13.4;u=30", 52.5, 13.4, true},
		{"geo:52.5,13.4,100.0;crs=wgs84", 52.5, 13.4, true},
		{"geo:-33.86,151.2", -33.86, 151.2, true},
		{"geo:91,0", 0, 0, false},
		{"geo:1,181", 0, 0, false},
		{"geo:a,b", 0, 0, false},
		{"geo:5", 0, 0, false},
		{"https://example.org", 0, 0, false},
		{"", 0, 0, false},
	} {
		lat, lon, ok := parseGeo(tc.in)
		if ok != tc.ok || (ok && (lat != tc.lat || lon != tc.lon)) {
			t.Errorf("parseGeo(%q) = %v, %v, %v", tc.in, lat, lon, ok)
		}
	}
}

func TestLocationWords(t *testing.T) {
	for in, want := range map[string]string{
		"Location":                     "",
		"Location: 48.1,11.5":          "",
		"Location: Brandenburger Tor":  "Brandenburger Tor",
		"48.1, 11.5":                   "",
		"geo:48.1,11.5":                "",
		"Our meeting point":            "Our meeting point",
		"locations of the old library": "locations of the old library", // only "Location:" is the fallback
	} {
		if got := locationWords(in); got != want {
			t.Errorf("locationWords(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripReplyFallback(t *testing.T) {
	for in, want := range map[string]string{
		"> <@a:x> q\n\nanswer":     "answer",
		"> <@a:x> q\n> more\n\nok": "ok",
		"plain":                    "plain",
		"> only a quote":           "> only a quote",
		"> q\n\nline1\nline2":      "line1\nline2",
	} {
		if got := stripReplyFallback(in); got != want {
			t.Errorf("stripReplyFallback(%q) = %q, want %q", in, got, want)
		}
	}
}
