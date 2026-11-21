package matrix

import (
	"context"
	"errors"
	"time"

	"slopement/model"
)

var errNotConnected = errors.New("not connected")

// LinkPreview asks the homeserver what a page says about itself (its
// OpenGraph title, description and picture). The homeserver fetches the page,
// not this device, so the site never sees the user's address; the link does
// go to the homeserver, which is why previews in encrypted rooms are the
// user's choice.
func (s *Source) LinkPreview(url string, done func(model.Preview, error)) {
	cli := s.cli
	if cli == nil {
		done(model.Preview{}, errNotConnected)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		lp, err := cli.GetURLPreview(ctx, url)
		if err != nil || lp == nil {
			done(model.Preview{}, err)
			return
		}
		p := model.Preview{URL: url, Site: lp.SiteName, Title: lp.Title, Description: lp.Description,
			Image: string(lp.ImageURL), W: int(lp.ImageWidth), H: int(lp.ImageHeight)}
		if lp.CanonicalURL != "" {
			p.URL = lp.CanonicalURL
		}
		done(p, nil)
	}()
}

var _ model.Previewer = (*Source)(nil)
