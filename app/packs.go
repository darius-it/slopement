package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Custom emoji and sticker packs (MSC2545) in the picker: what the source has
// on offer in the room becomes sections of the emoji and sticker tabs, and
// what is chosen goes into the message, out as a sticker, or into the user's
// own pack.

// fillPacks gives the picker's model the packs on offer in room.
func (a *App) fillPacks(room string, m *kit.PickerModel) {
	ps, ok := a.src.(model.Packs)
	if !ok {
		return
	}
	m.EmojiPacks, m.StickerPacks = nil, nil
	for _, p := range ps.EmojiPacks(room) {
		em := kit.PickerPack{ID: p.ID, Name: p.Name, Mine: p.Mine}
		st := em
		for _, e := range p.Emoji {
			t := kit.PackTile{ID: e.Shortcode, Label: e.Body, Image: a.packImage(e)}
			if e.Emoticon {
				em.Tiles = append(em.Tiles, t)
			}
			if e.Sticker {
				st.Tiles = append(st.Tiles, t)
			}
		}
		if len(em.Tiles) > 0 {
			m.EmojiPacks = append(m.EmojiPacks, em)
		}
		if len(st.Tiles) > 0 {
			m.StickerPacks = append(m.StickerPacks, st)
		}
	}
}

// packImage is a pack image's picture, fetched from the media repo when
// the tile first shows.
func (a *App) packImage(e model.PackEmoji) func() ui.ImageSource {
	media := model.Media{Kind: model.MediaImage, URL: e.URL, Mime: e.Mime, W: e.W, H: e.H, Name: e.Shortcode}
	return func() ui.ImageSource { return a.still(media) }
}

// emojiPicture is a custom emoji's picture in a message, by its mxc:// url.
func (a *App) emojiPicture(mxc string) ui.ImageSource {
	return a.still(model.Media{Kind: model.MediaImage, URL: mxc})
}

// findPackEmoji is the image a pick names, from the packs on offer in room.
func (a *App) findPackEmoji(room string, pk *kit.PackPick) (model.PackEmoji, bool) {
	ps, ok := a.src.(model.Packs)
	if !ok || pk == nil {
		return model.PackEmoji{}, false
	}
	for _, p := range ps.EmojiPacks(room) {
		if p.ID != pk.Pack {
			continue
		}
		for _, e := range p.Emoji {
			if e.Shortcode == pk.Tile {
				return e, true
			}
		}
	}
	return model.PackEmoji{}, false
}

// packAct does what the picker's pack results ask: a custom emoji is typed
// into the message as its ":shortcode:" (the source makes it a picture on
// sending), a sticker is sent, an image is added to the user's own pack.
func (a *App) packAct(room string, st *kit.ComposerState, r kit.PickerResult) {
	switch {
	case r.AddToPack != nil:
		if e, ok := a.findPackEmoji(room, r.AddToPack); ok {
			a.addToPack(e)
		}
	case r.CustomEmoji != nil:
		if e, ok := a.findPackEmoji(room, r.CustomEmoji); ok {
			st.Text += ":" + e.Shortcode + ":"
		}
	case r.PackSticker != nil:
		if e, ok := a.findPackEmoji(room, r.PackSticker); ok {
			a.sendPackSticker(room, e)
			a.picker.Open = false
		}
	}
}

// sendPackSticker sends a pack's sticker as an m.sticker: it is on the
// media repo already, so nothing is uploaded.
func (a *App) sendPackSticker(room string, e model.PackEmoji) {
	ss, ok := a.src.(model.StickerSender)
	if !ok {
		a.toast = "This account cannot send stickers"
		return
	}
	body := e.Body
	if body == "" {
		body = e.Shortcode
	}
	ss.SendSticker(room, model.Sticker{Body: body, Mime: e.Mime, W: e.W, H: e.H, URL: e.URL}, "")
}

// addToPack puts an image in the user's own pack and says how it went.
func (a *App) addToPack(e model.PackEmoji) {
	ps, ok := a.src.(model.Packs)
	if !ok {
		return
	}
	ps.AddToPack(e, func(problem string) {
		if problem != "" {
			a.toast = problem
		} else {
			a.toast = "Added to your emoji"
		}
	})
}
