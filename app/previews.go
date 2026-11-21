package app

import (
	"slopement/embeds"
	"slopement/format"
	"slopement/kit"
	"slopement/model"
)

// previewState is what the app knows of one link's preview.
type previewState struct {
	asked bool          // requested; asked stays true after a failure: one try per link
	got   bool          // the card is there
	card  model.Preview // the card
}

// linkCard is the preview card of a message's first link, from the
// homeserver, when the settings allow it and the page has something to say:
// a card of its site, title, description and picture. Nothing while it is
// asked for, or when it failed. YouTube has its own card (content), and
// links to media show as that media.
//
// Previews go to the homeserver (it fetches the page), so in an encrypted
// room they wait for the user's choice.
func (a *App) linkCard(ev model.Event, doc *format.Node) *kit.LinkCard {
	if !a.prefs.linkPreviews || doc == nil || a.noPreview[ev.ID] {
		return nil
	}
	if r, ok := a.src.Room(ev.Room); ok && r.Encrypted && !a.prefs.previewsE2EE {
		return nil
	}
	var link string
	for _, l := range embeds.Links(doc, 3) {
		if e := embeds.Classify(l); e.Kind == embeds.Generic {
			link = l
			break
		}
	}
	if link == "" {
		return nil
	}
	st := a.previews[link]
	if st == nil {
		st = &previewState{}
		a.previews[link] = st
	}
	if !st.asked {
		a.askPreview(link, st)
		return nil
	}
	if !st.got {
		return nil
	}
	p := st.card
	card := &kit.LinkCard{URL: link, Site: p.Site, Title: p.Title, Description: p.Description}
	if p.Image != "" {
		card.Image = kit.MediaSpec{ID: "link:" + link, Kind: kit.MediaImage, W: float32(p.W), H: float32(p.H), Progress: -1,
			Label: p.Title, Still: a.still(model.Media{URL: p.Image, Kind: model.MediaImage})}
		if card.Image.W == 0 || card.Image.H == 0 {
			card.Image.W, card.Image.H = 64, 64 // shown small, as the card's thumbnail
		}
	}
	return card
}

// askPreview requests a link's preview from the source, once; the answer
// comes back through the window.
func (a *App) askPreview(link string, st *previewState) {
	pv, ok := a.src.(model.Previewer)
	st.asked = true
	if !ok || a.runUI == nil {
		return
	}
	runUI := a.runUI
	pv.LinkPreview(link, func(p model.Preview, err error) {
		runUI(func() {
			// A page with nothing to say is no card.
			if err == nil && (p.Title != "" || p.Description != "") {
				st.card, st.got = p, true
			}
		})
	})
}
