package app

import (
	"bytes"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// Animated profile pictures. The avatars everywhere are the homeserver's
// thumbnails, which are still. A picture that is a GIF animates where it is
// asked to (Settings → Appearance → Animate profile pictures: always;
// otherwise on hover and in the profile card), from the original file, which
// is fetched the first time it would play, once per picture, kept for a few.

const (
	avatarAnimMax   = 24      // players kept (each holds its frames)
	avatarAnimBytes = 6 << 20 // an original larger than this stays a still
)

// wantAvatarAnim fetches the original of an avatar the first time it should
// play. A picture that is not a GIF is noted as such and not asked for again.
func (a *App) wantAvatarAnim(url string) {
	if _, known := a.avatarAnims[url]; known || a.avatarAnimBusy[url] {
		return
	}
	loader, ok := a.src.(model.MediaLoader)
	if !ok || a.runUI == nil {
		return
	}
	a.avatarAnimBusy[url] = true
	runUI := a.runUI
	loader.LoadMedia(model.Media{URL: url, Kind: model.MediaImage}, func(data []byte, err error) {
		var player *ui.VideoPlayer
		if err == nil && len(data) <= avatarAnimBytes && bytes.HasPrefix(data, []byte("GIF8")) {
			if d, derr := ui.NewGIFDecoder(data); derr == nil {
				if p, perr := ui.NewVideoPlayer(d); perr == nil {
					p.SetLoop(true).SetMuted(true)
					player = p
				}
			}
		}
		runUI(func() {
			delete(a.avatarAnimBusy, url)
			if err != nil {
				return // asked again the next time it would play
			}
			a.avatarAnims[url] = player // nil: not a GIF, so a still
			a.avatarAnimOrder = append(a.avatarAnimOrder, url)
			for len(a.avatarAnimOrder) > avatarAnimMax {
				old := a.avatarAnimOrder[0]
				a.avatarAnimOrder = a.avatarAnimOrder[1:]
				if p := a.avatarAnims[old]; p != nil {
					p.Close()
				}
				delete(a.avatarAnims, old)
			}
		})
	})
}
