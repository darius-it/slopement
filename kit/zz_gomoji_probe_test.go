package kit

import (
	"testing"

	"github.com/forPelevin/gomoji"
)

func TestProbeGomoji(t *testing.T) {
	all := gomoji.AllEmojis()
	for i, e := range all {
		if i < 12 || (i >= 1900 && i < 1906) {
			t.Logf("%d %q %q %q %q cp=%q", i, e.Character, e.UnicodeName, e.Group, e.SubGroup, e.CodePoint)
		}
	}
	a2 := gomoji.AllEmojis()
	same := true
	for i := range all {
		if all[i].Character != a2[i].Character {
			same = false
		}
	}
	t.Log("same order twice:", same)
	t.Logf("%+v", all[0])
}
