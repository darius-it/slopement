package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// LoadMedia downloads an attachment from the homeserver's media repo and
// decrypts it when it is an encrypted one, off the UI thread; done gets
// the plain bytes.
func (s *Source) LoadMedia(m model.Media, done func([]byte, error)) {
	cli := s.cli
	if cli == nil {
		done(nil, errors.New("not connected"))
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		uri, err := id.ParseContentURI(m.URL)
		if err != nil {
			done(nil, fmt.Errorf("media address: %w", err))
			return
		}
		var data []byte
		if m.Square > 0 {
			data = thumbnailBytes(ctx, cli, uri, m.Square) // nil: the server would not; take the whole file
		}
		if data == nil {
			if data, err = cli.DownloadBytes(ctx, uri); err != nil {
				done(nil, fmt.Errorf("downloading: %w", err))
				return
			}
		}
		if m.Secret != "" {
			var info event.EncryptedFileInfo
			if err := json.Unmarshal([]byte(m.Secret), &info); err != nil {
				done(nil, fmt.Errorf("attachment key: %w", err))
				return
			}
			if err := info.DecryptInPlace(data); err != nil {
				done(nil, fmt.Errorf("decrypting: %w", err))
				return
			}
		}
		done(data, nil)
	}()
}

// thumbnailBytes is a square, cropped thumbnail from the media repo, or
// nil when it cannot be had.
func thumbnailBytes(ctx context.Context, cli *mautrix.Client, uri id.ContentURI, side int) []byte {
	resp, err := cli.DownloadThumbnail(ctx, uri, side, side, mautrix.DownloadThumbnailExtra{Method: "crop"})
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

var _ model.MediaLoader = (*Source)(nil)
