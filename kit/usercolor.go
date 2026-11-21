package kit

import (
	"hash/fnv"

	"github.com/egoist/mygo/ui"
)

// UserColors resolves the color of a Matrix user: an override the user
// chose (their own, from account data, or locally for others), else a
// stable color of the palette hashed from the MXID.
type UserColors struct {
	// Overrides maps an MXID to an index of Palette.Users.
	Overrides map[string]int
}

// UserIndex returns the palette index of mxid: its override, or its hash.
func (u *UserColors) UserIndex(mxid string) int {
	if u != nil {
		if i, ok := u.Overrides[mxid]; ok && i >= 0 && i < len(Light.Users) {
			return i
		}
	}
	return HashIndex(mxid)
}

// HashIndex is the default palette index of mxid.
func HashIndex(mxid string) int {
	h := fnv.New32a()
	h.Write([]byte(mxid))
	return int(h.Sum32() % uint32(len(Light.Users)))
}

// Color returns mxid's color in p.
func (u *UserColors) Color(p *Palette, mxid string) ui.Color {
	return p.Users[u.UserIndex(mxid)]
}

// Set overrides mxid's color with palette index i; -1 clears it.
func (u *UserColors) Set(mxid string, i int) {
	if u.Overrides == nil {
		u.Overrides = map[string]int{}
	}
	if i < 0 {
		delete(u.Overrides, mxid)
		return
	}
	u.Overrides[mxid] = i
}
