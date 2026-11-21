// Package dotenv loads KEY=value lines of a .env file into the process's
// environment, without overriding variables already set. It is for local
// development secrets such as KLIPY_API_KEY; release builds take them from
// their configuration instead.
package dotenv

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Load reads the first .env found in the working directory or beside the
// executable. A missing file is not an error.
func Load() error {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()
		return parse(f)
	}
	return nil
}

func parse(f *os.File) error {
	s := bufio.NewScanner(f)
	for s.Scan() {
		k, v, ok := Line(s.Text())
		if !ok {
			continue
		}
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
	return s.Err()
}

// Line parses one line: KEY=value, with optional "export ", quotes around
// the value, and # comments. It reports false for blank and comment lines.
func Line(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	k, v, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}
	k, v = strings.TrimSpace(k), strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	} else if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return k, v, k != ""
}
