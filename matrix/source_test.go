package matrix

import (
	"testing"

	"maunium.net/go/mautrix/id"
)

// TestSourceMe pins who the pre-sync source says is signed in: the MXID
// itself, named by its local part — never the server (that was a
// strings.Cut mix-up: the part after the colon).
func TestSourceMe(t *testing.T) {
	for _, tc := range []struct{ mxid, name string }{
		{"@ada:example.org", "ada"},
		{"@trobo:matrix.org", "trobo"},
		{"@long.name:server.net", "long.name"},
	} {
		s := NewSource(&Session{UserID: id.UserID(tc.mxid)})
		me := s.Me()
		if me.ID != tc.mxid {
			t.Errorf("%s: ID = %q", tc.mxid, me.ID)
		}
		if me.Name != tc.name {
			t.Errorf("%s: Name = %q, want %q", tc.mxid, me.Name, tc.name)
		}
	}
}
