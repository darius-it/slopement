package licenses

import (
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	for want, text := range map[string]string{
		"MIT":          "MIT License\n\nCopyright (c) 2024 X\n\nPermission is hereby granted, free of charge, to any person obtaining a copy",
		"Apache-2.0":   "                                 Apache License\n                           Version 2.0, January 2004",
		"ISC":          "Copyright (c) 2020 X\n\nPermission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted",
		"BSD-3-Clause": "Redistribution and use in source and binary forms, with or without modification, are permitted. Neither the name of the copyright holder nor",
		"BSD-2-Clause": "Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met: 1. Redistributions",
		"Unlicense":    "This is free and unencumbered software released into the public domain.",
		// The MPL's text names the GPL family; it is still the MPL.
		"MPL-2.0":       "Mozilla Public License Version 2.0\n==================================\n1.12. \"Secondary License\" means either the GNU General Public License, Version 2.0, the GNU Lesser General Public License, Version 2.1, the GNU Affero General Public License, Version 3.0",
		"LGPL-2.1":      "                  GNU LESSER GENERAL PUBLIC LICENSE\n                       Version 2.1, February 1999",
		"AGPL-3.0":      "                    GNU AFFERO GENERAL PUBLIC LICENSE\n                       Version 3, 19 November 2007",
		"GPL":           "                    GNU GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007",
		"Public-Domain": "SQLite Is Public Domain\n\nAll of the code and documentation in SQLite has been dedicated to the public domain",
		"Unknown":       "Do what you like, but call me.",
	} {
		if got := Classify(text); got != want {
			t.Errorf("%q…: %s, want %s", strings.Fields(text)[0], got, want)
		}
	}
}

func TestCombineAndCopyleft(t *testing.T) {
	if got := Combine([]string{"MIT", "BSD-3-Clause", "MIT", ""}); got != "BSD-3-Clause + MIT" {
		t.Fatalf("combined: %s", got)
	}
	if got := Combine(nil); got != "Unknown" {
		t.Fatalf("none: %s", got)
	}
	for l, want := range map[string]bool{"MIT": false, "Apache-2.0": false, "LGPL-2.1": true, "MPL-2.0": true, "AGPL-3.0": true, "BSD-3-Clause + LGPL-2.1": true} {
		if Copyleft(l) != want {
			t.Errorf("Copyleft(%s) != %v", l, want)
		}
	}
}
