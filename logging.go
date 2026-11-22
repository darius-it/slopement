package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"slopement/internal/diag"
	"slopement/matrix"
)

// startLogging writes the app's logs (and the standard library's) to the
// day's file under the config directory, redacted as they are written, for
// Settings → Advanced → Export logs. A directory that can't be written means
// no logs, not no app.
func startLogging() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	l, err := diag.Open(filepath.Join(dir, "slopement", "logs"))
	if err != nil {
		log.Printf("logs: %v", err)
		return
	}
	matrix.SetDiagnostics(l)
	log.SetOutput(io.MultiWriter(os.Stderr, l))
	log.Printf("slopement %s starting", diag.AppVersion())
}
