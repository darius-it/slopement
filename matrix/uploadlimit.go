package matrix

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// The server's upload limit. A homeserver (or the proxy in front of it) refuses
// files over its limit, and says what it is at /media/config (m.upload.size). The
// app asks once, after sign-in, so a file that is too large is turned away when it
// is added to the message box, not after it has been uploaded.

// UploadLimit is the largest file the server takes, in bytes: 0 while it is
// unknown (not asked yet, or the server does not say). UI thread.
func (s *Source) UploadLimit() int64 { return s.uploadLimit }

// fetchUploadLimit asks the server for it, trying the authenticated endpoint,
// then the older unauthenticated one.
func (s *Source) fetchUploadLimit(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var size int64
	if cfg, err := s.cli.GetMediaConfig(ctx); err == nil && cfg != nil {
		size = cfg.UploadSize
	} else {
		var old mautrix.RespMediaConfig
		if _, err := s.cli.MakeRequest(ctx, http.MethodGet, s.cli.BuildURL(mautrix.MediaURLPath{"v3", "config"}), nil, &old); err == nil {
			size = old.UploadSize
		}
	}
	if size > 0 {
		s.post(func() { s.uploadLimit = size })
	}
}

// sendLimit is the largest file this device will send: the server's limit when it
// has said one, never more than the app's own. UI thread.
func (s *Source) sendLimit() int64 {
	if s.uploadLimit > 0 && s.uploadLimit < maxUpload {
		return s.uploadLimit
	}
	return maxUpload
}

// tooLargeText is the message of a file over the limit.
func tooLargeText(limit int64) string {
	return fmt.Sprintf("Too large to send (over %s)", megabytes(limit))
}

// megabytes writes a size as "25 MB" (a whole number where it is one).
func megabytes(n int64) string {
	mb := float64(n) / (1 << 20)
	if mb == float64(int64(mb)) {
		return fmt.Sprintf("%d MB", int64(mb))
	}
	return fmt.Sprintf("%.1f MB", mb)
}

var _ model.UploadLimiter = (*Source)(nil)
