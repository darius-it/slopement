package diag

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Nothing secret or written by a person survives a line being written.
func TestRedact(t *testing.T) {
	for name, tc := range map[string]struct{ in, gone string }{
		"access token":    {`{"level":"info","msg":"sync","token":"syt_dGVzdA_abcdefghijklmnop_0abcd"}`, "syt_dGVzdA"},
		"bearer header":   {`Authorization: Bearer abc123.def456-ghi`, "abc123"},
		"query token":     {`GET /_matrix/client/v3/sync?access_token=SECRETVALUE&timeout=30000`, "SECRETVALUE"},
		"password":        {`{"type":"m.login.password","password":"hunter2hunter2"}`, "hunter2"},
		"passphrase":      {`{"passphrase":"correct horse"}`, "correct horse"},
		"message body":    {`{"msgtype":"m.text","body":"meet me at 5, it's a secret"}`, "secret"},
		"formatted body":  {`{"formatted_body":"<b>private</b> plans"}`, "private"},
		"room name":       {`{"type":"m.room.name","content":{"name":"Surprise party"}}`, "Surprise"},
		"topic":           {`{"topic":"Where we hide the gift"}`, "hide the gift"},
		"display name":    {`{"displayname":"Kai Fireborn"}`, "Fireborn"},
		"escaped quote":   {`{"body":"she said \"hi\" and left"}`, "left"},
		"recovery key":    {`recovery EsTc LW2K PGiF wKEA 3As5 g5c4 BXwk qeeJ ZJV8 Q9fu gUMn`, "EsTc"},
		"oauth code":      {`redirect?code=AUTHCODE123&state=xyz`, "AUTHCODE123"},
		"private key":     {`{"private_key":"AAAA1111BBBB"}`, "AAAA1111"},
		"avatar":          {`{"avatar_url":"mxc://example.org/abcd"}`, "abcd"},
		"two on one line": {`{"body":"one","name":"two"} {"body":"three"}`, "three"},
	} {
		got := string(Redact([]byte(tc.in)))
		if strings.Contains(got, tc.gone) {
			t.Errorf("%s: %q survives in %q", name, tc.gone, got)
		}
	}
	// Ordinary lines are left alone.
	line := `{"level":"warn","time":"2026-11-22T10:00:00Z","message":"sync failed","status":502}`
	if got := string(Redact([]byte(line))); got != line {
		t.Errorf("an ordinary line changed: %s", got)
	}
}

func TestHashIDs(t *testing.T) {
	h := hasher{salt: []byte("salt")}
	in := []byte(`room !abcDEF123:example.org event $Zm9vYmFyYmF6cXV4 user @kai:example.org alias #lobby:example.org host matrix.example.org`)
	got := string(h.hash(in))
	for _, gone := range []string{"abcDEF123", "Zm9vYmFyYmF6cXV4", "kai", "lobby"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q survives: %s", gone, got)
		}
	}
	for _, kept := range []string{":example.org", "matrix.example.org"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q was lost: %s", kept, got)
		}
	}
	if again := string(h.hash(in)); again != got {
		t.Error("the same IDs hash differently")
	}
	if string(hasher{salt: []byte("other")}.hash(in)) == got {
		t.Error("another install hashes the same")
	}
	// Room and event IDs kept; users still hashed.
	kept := string(hasher{salt: []byte("salt"), rooms: true}.hash(in))
	if !strings.Contains(kept, "!abcDEF123:example.org") || !strings.Contains(kept, "$Zm9vYmFyYmF6cXV4") || strings.Contains(kept, "@kai:") {
		t.Errorf("room IDs included: %s", kept)
	}
}

// One file a day; files older than 7 days go; when the files are over the
// cap the oldest go first and today's stays.
func TestRotationAndPruning(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ { // ten old days of files
		d := day.AddDate(0, 0, -i-1)
		if err := os.WriteFile(filepath.Join(dir, filePrefix+d.Format(dayLayout)+fileSuffix), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := day
	l, err := open(dir, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.Write([]byte("today\n"))
	names := func() []string {
		var out []string
		for _, f := range l.files() {
			out = append(out, f.name)
		}
		return out
	}
	if n := names(); len(n) != 8 { // today and the 7 days before
		t.Fatalf("files after opening: %v", n)
	}
	now = now.AddDate(0, 0, 1)
	l.Write([]byte("tomorrow\n"))
	n := names()
	if len(n) != 8 || n[len(n)-1] != filePrefix+"2026-11-02"+fileSuffix {
		t.Fatalf("after the day turned: %v", n)
	}
	for _, f := range n {
		if f == filePrefix+"2026-10-24"+fileSuffix {
			t.Fatalf("a file older than %d days is kept: %v", Keep, n)
		}
	}
	b, _ := os.ReadFile(filepath.Join(dir, filePrefix+"2026-11-02"+fileSuffix))
	if string(b) != "tomorrow\n" {
		t.Fatalf("the new day's file = %q", b)
	}

	// The cap: a big old file goes though it is recent, today's stays.
	big := filepath.Join(dir, filePrefix+"2026-11-01"+fileSuffix)
	if err := os.WriteFile(big, bytes.Repeat([]byte("x"), MaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	now = now.AddDate(0, 0, 1)
	l.Write([]byte("later\n"))
	for _, f := range names() {
		if f == filePrefix+"2026-11-01"+fileSuffix {
			t.Fatalf("over the cap and still there: %v", names())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, filePrefix+"2026-11-03"+fileSuffix)); err != nil {
		t.Fatal("today's file is gone")
	}
}

func unzip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

// An export is a zip of the logs and an info file; secrets are not in it (they
// never were written), IDs are hashed, and room IDs stay when asked.
func TestExport(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.Write([]byte(`{"level":"warn","message":"send failed","room_id":"!room1:example.org","user_id":"@kai:example.org","body":"my private text","access_token":"syt_abcdefghijklmnopqrstu_x"}` + "\n"))

	var buf bytes.Buffer
	if err := l.Export(&buf, ExportOptions{Info: map[string]string{"server": "Synapse 1.2"}}); err != nil {
		t.Fatal(err)
	}
	files := unzip(t, buf.Bytes())
	info := files["info.txt"]
	for _, want := range []string{"slopement ", "server: Synapse 1.2", "room IDs: hashed"} {
		if !strings.Contains(info, want) {
			t.Errorf("info.txt lacks %q:\n%s", want, info)
		}
	}
	var logText string
	for name, body := range files {
		if strings.HasPrefix(name, filePrefix) {
			logText = body
		}
	}
	for _, gone := range []string{"private text", "syt_", "kai", "room1"} {
		if strings.Contains(logText, gone) {
			t.Errorf("%q is in the export: %s", gone, logText)
		}
	}
	if !strings.Contains(logText, "send failed") || !strings.Contains(logText, ":example.org") {
		t.Errorf("the useful part was lost: %s", logText)
	}

	buf.Reset()
	if err := l.Export(&buf, ExportOptions{IncludeRoomIDs: true}); err != nil {
		t.Fatal(err)
	}
	files = unzip(t, buf.Bytes())
	var withRooms string
	for name, body := range files {
		if strings.HasPrefix(name, filePrefix) {
			withRooms = body
		}
	}
	if !strings.Contains(withRooms, "!room1:example.org") || strings.Contains(withRooms, "@kai:") || !strings.Contains(files["info.txt"], "room IDs: included") {
		t.Errorf("with room IDs: %s", withRooms)
	}
	// What is on disk keeps the IDs (it is local) but never the secrets.
	disk, _ := os.ReadFile(filepath.Join(dir, filePrefix+time.Now().Format(dayLayout)+fileSuffix))
	if strings.Contains(string(disk), "syt_") || strings.Contains(string(disk), "private text") {
		t.Errorf("a secret is on disk: %s", disk)
	}
}
