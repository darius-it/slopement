package matrix

import (
	"bytes"
	"context"
	"image"
	_ "image/gif" // decoders for the dimensions of what is sent
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/attachment"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// maxUpload is the largest file the app sends: it holds the file in memory
// (and, in an encrypted room, a second copy to encrypt).
const maxUpload = 50 << 20

// SendFile uploads a file to the media repo and sends it to a room as an
// image, video, audio or file message. In an encrypted room the file is
// encrypted first and its key travels in the (encrypted) message, as every
// client does. It shows at once, pending; the sync's echo lands on the same
// message, and a failure — too large, the server refusing — marks it.
func (s *Source) SendFile(room string, f model.File, replyTo string) {
	if s.cli == nil || len(f.Data) == 0 {
		return
	}
	mimeType := f.Mime
	if mimeType == "" {
		mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(f.Name)))
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(f.Data)
	}
	if i := strings.Index(mimeType, ";"); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	info := &event.FileInfo{MimeType: mimeType, Size: len(f.Data)}
	msgType := event.MsgFile
	media := model.Media{Kind: model.MediaFile, Name: f.Name, Size: int64(len(f.Data)), Mime: mimeType}
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		msgType, media.Kind = event.MsgImage, model.MediaImage
		if mimeType == "image/gif" {
			media.Kind = model.MediaGIF
		}
		if len(f.Data) <= 12<<20 {
			media.Local = f.Data // shown while it uploads
		}
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(f.Data)); err == nil {
			info.Width, info.Height = cfg.Width, cfg.Height
			media.W, media.H = cfg.Width, cfg.Height
		}
	case strings.HasPrefix(mimeType, "video/"):
		msgType, media.Kind = event.MsgVideo, model.MediaVideo
	case strings.HasPrefix(mimeType, "audio/"):
		msgType = event.MsgAudio
	}

	r, ok := s.rooms[room]
	if !ok {
		return
	}
	encrypted := r.encrypted
	txn := s.cli.TxnID()
	local := localPrefix + txn
	pending := model.Event{ID: local, Room: room, Sender: s.me.ID, Time: time.Now(), Kind: model.EventMedia,
		Body: f.Name, Media: []model.Media{media}, ReplyTo: replyTo, Pending: true}
	s.appendLocal(room, pending)

	go func() {
		fail := func(why string) { s.post(func() { s.roomOf(room).failLocal(local, why) }) }
		if len(f.Data) > maxUpload {
			fail("Too large to send (over 50 MB)")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		content := &event.MessageEventContent{MsgType: msgType, Body: f.Name, FileName: f.Name, Info: info}
		upload, uploadType := f.Data, mimeType
		var enc *attachment.EncryptedFile
		if encrypted {
			enc = attachment.NewEncryptedFile()
			upload, uploadType = enc.Encrypt(f.Data), "application/octet-stream"
		}
		resp, err := s.cli.UploadBytes(ctx, upload, uploadType)
		if err != nil {
			s.post(func() { s.noteConnection(err) })
			fail(notSent(err))
			return
		}
		if enc != nil {
			content.File = &event.EncryptedFileInfo{EncryptedFile: *enc, URL: id.ContentURIString(resp.ContentURI.String())}
		} else {
			content.URL = id.ContentURIString(resp.ContentURI.String())
		}
		if replyTo != "" {
			content.RelatesTo = (&event.RelatesTo{}).SetReplyTo(id.EventID(replyTo))
		}
		sent, err := s.cli.SendMessageEvent(ctx, id.RoomID(room), event.EventMessage, content, mautrix.ReqSendEvent{TransactionID: txn})
		s.post(func() {
			s.noteConnection(err)
			if err != nil {
				s.roomOf(room).failLocal(local, notSent(err))
			} else {
				s.roomOf(room).settleLocal(local, sent.EventID.String())
			}
		})
	}()
}

// appendLocal shows a message the user is sending, at once.
func (s *Source) appendLocal(room string, e model.Event) {
	s.post(func() { s.roomOf(room).appendEvent(e, "You", true) })
}

var _ model.Uploader = (*Source)(nil)
