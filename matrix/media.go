package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
		data, err := cli.DownloadBytes(ctx, uri)
		if err != nil {
			done(nil, fmt.Errorf("downloading: %w", err))
			return
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

var _ model.MediaLoader = (*Source)(nil)
