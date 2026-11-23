// Package licenses is the list of the open source software slopement is built
// from, for Settings → About, and the code that makes it (gen).
package licenses

import (
	"slices"
	"sort"
	"strings"
)

// Notice is one piece of software: a Go module in the build, its version, its
// license (an SPDX name, "A + B" when it has more than one) and the license's
// text as its authors wrote it.
type Notice struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	License string `json:"license"`
	URL     string `json:"url"`
	Text    string `json:"text"`
}

// Classify names the license a text is, as an SPDX identifier, or "Unknown".
func Classify(text string) string {
	t := strings.ToLower(strings.Join(strings.Fields(text), " "))
	has := func(s string) bool { return strings.Contains(t, s) }
	switch {
	// The MPL's own text names the GPL family (as "Secondary Licenses"), so it
	// is told first, by its title.
	case strings.HasPrefix(t, "mozilla public license") && has("2.0"):
		return "MPL-2.0"
	case strings.HasPrefix(t, "gnu affero general public license"):
		return "AGPL-3.0"
	case strings.HasPrefix(t, "gnu lesser general public license"):
		if has("version 2.1") {
			return "LGPL-2.1"
		}
		return "LGPL"
	case strings.HasPrefix(t, "gnu general public license"):
		return "GPL"
	case has("sqlite is public domain"):
		return "Public-Domain"
	case has("gnu affero general public license") && has("version 3"):
		return "AGPL-3.0"
	case has("gnu lesser general public license"):
		return "LGPL"
	case has("gnu general public license"):
		return "GPL"
	case has("apache license") && has("version 2.0"):
		return "Apache-2.0"
	case has("permission is hereby granted, free of charge"):
		return "MIT"
	case has("permission to use, copy, modify, and/or distribute this software for any purpose with or without fee") ||
		has("permission to use, copy, modify, and distribute this software for any purpose with or without fee"):
		if has("isc license") || has("copyright") {
			return "ISC"
		}
		return "ISC"
	case has("redistribution and use in source and binary forms"):
		if has("neither the name") || has("endorse or promote") {
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"
	case has("this is free and unencumbered software released into the public domain"):
		return "Unlicense"
	case has("creative commons") && has("cc0"):
		return "CC0-1.0"
	case has("zero-clause bsd") || has("0bsd"):
		return "0BSD"
	}
	return "Unknown"
}

// Combine joins the licenses a module's files say, without repeats ("MIT + BSD-3-Clause").
func Combine(kinds []string) string {
	var out []string
	for _, k := range kinds {
		if k != "" && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return "Unknown"
	}
	sort.Strings(out)
	return strings.Join(out, " + ")
}

// Copyleft says a license asks more of those who ship the software than a
// permissive one does (the generator points at these, to be looked at).
func Copyleft(license string) bool {
	for _, c := range []string{"GPL", "LGPL", "AGPL", "MPL"} {
		if strings.Contains(license, c) {
			return true
		}
	}
	return false
}
