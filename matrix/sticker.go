package matrix

import (
	"context"
	"net/http"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// SendSticker sends a sticker as an m.sticker event. A picture's bytes are
// uploaded first (unencrypted, as clients send stickers: it is a public
// picture, and m.sticker carries a plain url); one that is already an mxc://
// (a pack's) is sent as it is. It shows at once, pending, and a failure marks
// it.
func (s *Source) SendSticker(room string, st model.Sticker, replyTo string) {
	r, ok := s.rooms[room]
	if s.cli == nil || !ok || (len(st.Data) == 0 && st.URL == "") {
		return
	}
	mimeType := st.Mime
	if i := strings.Index(mimeType, ";"); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	if mimeType == "" && len(st.Data) > 0 {
		mimeType = http.DetectContentType(st.Data)
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	body := st.Body
	if body == "" {
		body = "Sticker"
	}
	media := model.Media{Kind: model.MediaImage, Name: body, Mime: mimeType, W: st.W, H: st.H, Sticker: true, URL: st.URL}
	if mimeType == "image/gif" {
		media.Kind = model.MediaGIF
	}
	if len(st.Data) > 0 && len(st.Data) <= 12<<20 {
		media.Local = st.Data
	}
	_ = r
	txn := s.cli.TxnID()
	local := localPrefix + txn
	s.appendLocal(room, model.Event{ID: local, Room: room, Sender: s.me.ID, Time: time.Now(), Kind: model.EventMedia,
		Body: body, Media: []model.Media{media}, ReplyTo: replyTo, Pending: true})

	go func() {
		fail := func(why string) { s.post(func() { s.roomOf(room).failLocal(local, why) }) }
		if len(st.Data) > maxUpload {
			fail("Too large to send (over 50 MB)")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		url := id.ContentURIString(st.URL)
		if url == "" {
			resp, err := s.cli.UploadBytes(ctx, st.Data, mimeType)
			if err != nil {
				s.post(func() { s.noteConnection(err) })
				fail(notSent(err))
				s.post(func() { s.rememberRetry(room, local, func() { s.SendSticker(room, st, replyTo) }) })
				return
			}
			url = id.ContentURIString(resp.ContentURI.String())
		}
		content := &event.MessageEventContent{Body: body, URL: url,
			Info: &event.FileInfo{MimeType: mimeType, Width: st.W, Height: st.H, Size: len(st.Data)}}
		if replyTo != "" {
			content.RelatesTo = (&event.RelatesTo{}).SetReplyTo(id.EventID(replyTo))
		}
		sent, err := s.cli.SendMessageEvent(ctx, id.RoomID(room), event.EventSticker, content, mautrix.ReqSendEvent{TransactionID: txn})
		s.post(func() {
			s.noteConnection(err)
			if err != nil {
				s.roomOf(room).failLocal(local, notSent(err))
				s.rememberRetry(room, local, func() { s.SendSticker(room, st, replyTo) })
			} else {
				s.roomOf(room).settleLocal(local, sent.EventID.String())
			}
		})
	}()
}

var _ model.StickerSender = (*Source)(nil)
