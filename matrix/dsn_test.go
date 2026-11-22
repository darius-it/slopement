package matrix

import (
	"net/url"
	"strings"
	"testing"
)

// A Windows path must not become "file://C:/…" (SQLite reads C: as a host:
// "invalid uri authority: C:"); it is "file:///C:/…", and the name survives
// the round trip through the URI.
func TestDSNOfAWindowsPath(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{`C:\Users\me\AppData\Roaming\slopement\accounts\ab\state.db`, "file:///C:/Users/me/AppData/Roaming/slopement/accounts/ab/state.db"},
		{`c:/Users/me/state.db`, "file:///c:/Users/me/state.db"},
		{`D:\My Stuff\état.db`, "file:///D:/My%20Stuff/%C3%A9tat.db"},
		{"/home/me/.config/slopement/state.db", "file:///home/me/.config/slopement/state.db"},
		{"/Users/me/Library/Application Support/slopement/state.db", "file:///Users/me/Library/Application%20Support/slopement/state.db"},
	} {
		dsn := dbDSN(tc.path)
		base, _, _ := strings.Cut(dsn, "?")
		if base != tc.want {
			t.Errorf("dbDSN(%q) = %q, want %q", tc.path, base, tc.want)
		}
		u, err := url.Parse(dsn)
		if err != nil || u.Host != "" {
			t.Errorf("dbDSN(%q): host %q, err %v", tc.path, u.Host, err)
		}
		if !strings.Contains(dsn, "journal_mode") {
			t.Errorf("dbDSN(%q) lost its pragmas: %s", tc.path, dsn)
		}
	}
}
