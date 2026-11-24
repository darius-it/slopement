package main

import (
	"strings"
	"sync"

	"slopement/app"
)

// What the system hands the app arrives on any goroutine, possibly before an
// account is open (a file the app was opened with): it waits here until the
// window takes it. Files from Open with and the Dock, and the files and words of
// Android's share sheet (the sharetarget plugin).
var shared struct {
	mu sync.Mutex
	s  app.Share
}

func addShared(s app.Share) {
	shared.mu.Lock()
	defer shared.mu.Unlock()
next:
	for _, p := range s.Paths {
		for _, q := range shared.s.Paths {
			if p == q {
				continue next
			}
		}
		shared.s.Paths = append(shared.s.Paths, p)
	}
	if t := strings.TrimSpace(s.Text); t != "" {
		if shared.s.Text != "" {
			shared.s.Text += "\n"
		}
		shared.s.Text += t
	}
	shared.s.Temp = shared.s.Temp || s.Temp
}

func takeShared() app.Share {
	shared.mu.Lock()
	defer shared.mu.Unlock()
	out := shared.s
	shared.s = app.Share{}
	return out
}
