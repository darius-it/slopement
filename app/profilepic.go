package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// openProfilePicture shows a person's profile picture enlarged, in the same
// viewer as a message's pictures: the small one first, the full-size one as it
// arrives. The card stays open under it.
func (a *App) openProfilePicture(person string) {
	p, _ := a.src.Person(person)
	if !strings.HasPrefix(p.Avatar, "mxc://") {
		return
	}
	media := model.Media{Kind: model.MediaImage, URL: p.Avatar, Name: p.Name}
	spec := kit.MediaSpec{ID: "avatar:" + p.ID, Kind: kit.MediaImage, W: 1, H: 1, Progress: -1,
		Label: p.Name + "'s profile picture", Full: func() ui.ImageSource { return a.fullPic(media) },
		Save: func() { a.saveMedia(media) }}
	if img := a.avatar(p, 0).Image; img != nil { // not a nil bitmap as a non-nil source
		spec.Still = img
	}
	a.viewerItems, a.viewerAt, a.viewer = []kit.MediaSpec{spec}, 0, true
}
