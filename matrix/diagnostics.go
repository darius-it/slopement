package matrix

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"slopement/internal/diag"
	"slopement/model"
)

// The logs (internal/diag) are set once at start-up; every client made after
// writes its own log lines to them (mautrix's, at info level: failures and
// warnings, never message contents).
var (
	diagMu  sync.RWMutex
	diagLog *diag.Log
)

// SetDiagnostics sets the log that clients write to and that ExportLogs
// exports. Call it before signing in; nil turns logging off.
func SetDiagnostics(l *diag.Log) {
	diagMu.Lock()
	diagLog = l
	diagMu.Unlock()
}

func diagnostics() *diag.Log {
	diagMu.RLock()
	defer diagMu.RUnlock()
	return diagLog
}

// clientLogger is the logger a new client gets: the file log at info level,
// or none.
func clientLogger() zerolog.Logger {
	l := diagnostics()
	if l == nil {
		return zerolog.Nop()
	}
	return zerolog.New(l).With().Timestamp().Logger().Level(zerolog.InfoLevel)
}

// ExportLogs makes a zip of the logs for a bug report: IDs hashed (room IDs
// too unless includeRoomIDs), nothing secret in it, with the app's version and
// the homeserver's supported versions. done gets the bytes and a file name, on
// the UI thread, or why not.
func (s *Source) ExportLogs(includeRoomIDs bool, done func(data []byte, name string, problem string)) {
	l := diagnostics()
	if l == nil {
		done(nil, "", "Logging is off.")
		return
	}
	cli := s.cli
	go func() {
		info := map[string]string{}
		if cli != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if v, err := cli.Versions(ctx); err == nil {
				var vs []string
				for _, sv := range v.Versions {
					vs = append(vs, sv.String())
				}
				info["homeserver Matrix versions"] = strings.Join(vs, ", ")
			} else {
				info["homeserver Matrix versions"] = "unknown (" + plain(err) + ")"
			}
			cancel()
		}
		var buf bytes.Buffer
		err := l.Export(&buf, diag.ExportOptions{IncludeRoomIDs: includeRoomIDs, Info: info})
		name := "slopement-logs-" + time.Now().Format("2006-01-02") + ".zip"
		s.post(func() {
			if err != nil {
				done(nil, "", "Could not make the export: "+plain(err))
				return
			}
			done(buf.Bytes(), name, "")
		})
	}()
}

var _ model.LogExporter = (*Source)(nil)
