// Package diag keeps the app's logs and exports them for a bug report.
//
// Logs are written to one file a day under the config directory, kept 7 days
// and at most 20 MB in all. What is written never holds what must not leave
// the device: tokens, keys, passwords, message bodies, names and topics are
// removed as a line is written (redact.go). Room, user and event IDs stay in
// the files, which are local; an export hashes them, unless the user asks to
// include the room IDs, so a report can be matched to a room by whoever
// is helping.
package diag

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// Keep is how many days of logs are kept.
	Keep = 7
	// MaxBytes is the most all the log files hold together.
	MaxBytes = 20 << 20

	filePrefix = "slopement-"
	fileSuffix = ".log"
	dayLayout  = "2006-01-02"
)

// Log writes redacted lines to the day's file. It is an io.Writer, safe for
// concurrent use (zerolog and the standard log package write to it).
type Log struct {
	dir string
	now func() time.Time

	mu   sync.Mutex
	day  string
	file *os.File
	salt []byte
}

// Open starts logging into dir (created if need be) and trims old files.
func Open(dir string) (*Log, error) {
	return open(dir, time.Now)
}

func open(dir string, now func() time.Time) (*Log, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	l := &Log{dir: dir, now: now}
	l.salt = l.loadSalt()
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.rotate(); err != nil {
		return nil, err
	}
	return l, nil
}

// loadSalt is the install's own random salt, kept beside the logs, which
// hashed IDs are made with: the same ID hashes the same way in every export of
// the install, and no one else can find out which ID a hash is of.
func (l *Log) loadSalt() []byte {
	path := filepath.Join(l.dir, "salt")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 16 {
		return b
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	_ = os.WriteFile(path, b, 0o600)
	return b
}

// Write redacts p (a line, or several) and appends it to today's file.
func (l *Log) Write(p []byte) (int, error) {
	out := Redact(p)
	l.mu.Lock()
	defer l.mu.Unlock()
	if day := l.now().Format(dayLayout); day != l.day || l.file == nil {
		if err := l.rotate(); err != nil {
			return len(p), nil // logging never fails what it logs
		}
	}
	if _, err := l.file.Write(out); err != nil {
		return len(p), nil
	}
	return len(p), nil
}

// rotate opens the file of today and trims the old ones. Callers hold mu.
func (l *Log) rotate() error {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	day := l.now().Format(dayLayout)
	f, err := os.OpenFile(filepath.Join(l.dir, filePrefix+day+fileSuffix), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	l.file, l.day = f, day
	l.prune()
	return nil
}

// logFile is a log file on disk.
type logFile struct {
	name string
	day  time.Time
	size int64
}

func (l *Log) files() []logFile {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil
	}
	var out []logFile
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, filePrefix) || !strings.HasSuffix(n, fileSuffix) {
			continue
		}
		day, err := time.ParseInLocation(dayLayout, strings.TrimSuffix(strings.TrimPrefix(n, filePrefix), fileSuffix), l.now().Location())
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, logFile{n, day, info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].day.Before(out[j].day) })
	return out
}

// prune removes the files older than Keep days, then the oldest ones while
// the files hold more than MaxBytes (today's stays).
func (l *Log) prune() {
	files := l.files()
	n := l.now()
	cutoff := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, n.Location()).AddDate(0, 0, -Keep)
	var kept []logFile
	for _, f := range files {
		if f.day.Before(cutoff) && f.name != filePrefix+l.day+fileSuffix {
			_ = os.Remove(filepath.Join(l.dir, f.name))
			continue
		}
		kept = append(kept, f)
	}
	var total int64
	for _, f := range kept {
		total += f.size
	}
	for len(kept) > 1 && total > MaxBytes {
		oldest := kept[0]
		_ = os.Remove(filepath.Join(l.dir, oldest.name))
		total -= oldest.size
		kept = kept[1:]
	}
}

// Close closes the file.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// ExportOptions say what an export holds.
type ExportOptions struct {
	// IncludeRoomIDs keeps room and event IDs as they are; without it they
	// are hashed. User IDs are hashed either way.
	IncludeRoomIDs bool
	// Info are lines for info.txt: versions of the server and the like.
	Info map[string]string
}

// Export writes a zip of the logs and an info.txt to w: the app, Go and
// system versions, then Info, then what the zip holds. IDs are hashed
// as the options say.
func (l *Log) Export(w io.Writer, opt ExportOptions) error {
	l.mu.Lock()
	if l.file != nil {
		_ = l.file.Sync()
	}
	files := l.files()
	l.mu.Unlock()

	zw := zip.NewWriter(w)
	var info bytes.Buffer
	fmt.Fprintf(&info, "slopement %s\n", AppVersion())
	fmt.Fprintf(&info, "%s, %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&info, "exported %s\n", l.now().UTC().Format(time.RFC3339))
	keys := make([]string, 0, len(opt.Info))
	for k := range opt.Info {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&info, "%s: %s\n", k, opt.Info[k])
	}
	if opt.IncludeRoomIDs {
		info.WriteString("room IDs: included; user IDs hashed\n")
	} else {
		info.WriteString("room IDs: hashed; user IDs hashed\n")
	}
	info.WriteString("Message text, names, topics, tokens and keys are never logged.\n")
	for _, f := range files {
		fmt.Fprintf(&info, "file %s (%d bytes)\n", f.name, f.size)
	}
	h := l.hasher(opt.IncludeRoomIDs)
	iw, err := zw.Create("info.txt")
	if err != nil {
		return err
	}
	if _, err := iw.Write(info.Bytes()); err != nil {
		return err
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(l.dir, f.name))
		if err != nil {
			continue
		}
		fw, err := zw.Create(f.name)
		if err != nil {
			return err
		}
		if _, err := fw.Write(h.hash(data)); err != nil {
			return err
		}
	}
	return zw.Close()
}

// hasher makes the IDs of an export unreadable.
type hasher struct {
	salt  []byte
	rooms bool // room and event IDs are kept
}

func (l *Log) hasher(includeRooms bool) hasher { return hasher{salt: l.salt, rooms: includeRooms} }

func (h hasher) short(s string) string {
	sum := sha256.Sum256(append(append([]byte(nil), h.salt...), s...))
	return hex.EncodeToString(sum[:4])
}

// AppVersion is the app's version: the module's, or the commit it was built
// from, else "dev".
func AppVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 8 {
			return "dev-" + s.Value[:8]
		}
	}
	return "dev"
}
