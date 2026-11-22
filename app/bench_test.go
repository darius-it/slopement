package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// BenchmarkTypingFrame is a frame while the user types in a heavy account (a
// few hundred chats and rooms, a long room open): what each keystroke costs.
func BenchmarkTypingFrame(b *testing.B) {
	b.Setenv("HOME", b.TempDir())
	b.Setenv("XDG_CONFIG_HOME", b.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	m.AddBulk(300, 10, 30, 300)
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!crit")
	tt := ui.NewTesterWith(a.View, 1440, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Type("a")
		tt.Frame()
	}
}
