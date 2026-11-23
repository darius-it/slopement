package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// centerOf is the middle of the element with this accessible name.
func centerOf(t *testing.T, tt *ui.Tester, name string) (float32, float32) {
	t.Helper()
	for _, acc := range tt.AccessAll() {
		if acc.Name == name && !acc.Offscreen {
			return acc.Bounds.X + acc.Bounds.W/2, acc.Bounds.Y + acc.Bounds.H/2
		}
	}
	t.Fatalf("no element named %q", name)
	return 0, 0
}

// A community dragged on the rail onto another takes its place, and the new
// order goes to the source.
func TestDragACommunityOnTheRail(t *testing.T) {
	a := testApp()
	ss := &spaceSource{Source: a.src}
	a.src = ss
	a.nav = kit.NavRail
	tt := ui.NewTester(a.View, 1300, 800)
	settle(tt)
	cms := a.communities()
	if len(cms) < 3 {
		t.Skip("needs three communities")
	}
	ids := a.communityIDs()
	x1, y1 := centerOf(t, tt, cms[0].Name)
	x2, y2 := centerOf(t, tt, cms[2].Name)
	tt.Press(x1, y1)
	for i := 1; i <= 8; i++ { // a drag: the pointer moves a few steps before the release
		f := float32(i) / 8
		tt.Move(x1+(x2-x1)*f, y1+(y2-y1)*f)
		tt.Frame()
	}
	tt.Release(x2, y2)
	settle(tt)
	want := append([]string{ids[1], ids[2], ids[0]}, ids[3:]...)
	if !reflect.DeepEqual(ss.order, want) {
		t.Fatalf("after the drag: %v, want %v", ss.order, want)
	}
}

// Dropped among the favourites (listed first), a community becomes one, or it
// would seem not to have moved; dropped among the rest, it stops being one.
func TestDropAmongFavouritesMakesAFavourite(t *testing.T) {
	a := testApp()
	fs := &favSource{Source: a.src, fav: map[string]bool{}}
	a.src = fs
	cms := a.communities()
	if len(cms) < 3 {
		t.Skip("needs three communities")
	}
	fs.fav[cms[0].ID] = true
	a.srcGen++ // the source says it changed (its lists are kept until it does)
	a.moveCommunity(cms[2].ID, cms[0].ID) // a plain community onto a favourite
	if !fs.fav[cms[2].ID] {
		t.Fatal("dropped among the favourites, it is not one")
	}
	a.srcGen++
	a.moveCommunity(cms[2].ID, cms[1].ID) // and back onto a plain one
	if fs.fav[cms[2].ID] {
		t.Fatal("dropped among the rest, it is still a favourite")
	}
}

// favSource keeps which communities are favourites.
type favSource struct {
	model.Source
	fav map[string]bool
}

func (s *favSource) Communities() []model.Community {
	cms := s.Source.Communities()
	for i := range cms {
		cms[i].Favourite = s.fav[cms[i].ID]
	}
	return cms
}

func (s *favSource) SetCommunityFavourite(id string, on bool) { s.fav[id] = on }
