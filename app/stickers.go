package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"slopement/klipy"
	"slopement/model"
	"slopement/picker"
)

// stickerSize is the most a sticker in the timeline is a side, in DIPs.
const stickerSize = 176

// sendPicked sends what the picker chose: a GIF as its link (or file, by
// the setting), a sticker as an m.sticker.
func (a *App) sendPicked(room string, it klipy.Item) {
	if picker.IsLocalURL(it.Full.URL) {
		a.sendLocalFile(room, picker.LocalPath(it.Full.URL))
		return
	}
	if it.Kind == klipy.Stickers {
		a.sendSticker(room, it)
		return
	}
	a.sendGIF(room, it.Full.URL)
}

// sendLocalFile sends a picture from one of the user's GIF folders: always
// uploaded as a file, whatever "Send GIFs as files" says, since a link to a
// file on this disk would be no use to anyone, and would say where it is.
func (a *App) sendLocalFile(room, path string) {
	up, ok := a.src.(model.Uploader)
	runUI := a.runUI
	if !ok || runUI == nil {
		a.toast = "This account cannot send files"
		return
	}
	go func() {
		data, err := os.ReadFile(path)
		runUI(func() {
			if err != nil || len(data) == 0 {
				a.toast = "Could not read " + filepath.Base(path)
				return
			}
			up.SendFile(room, model.File{Name: filepath.Base(path), Data: data}, "")
		})
	}()
}

// sendSticker fetches a KLIPY sticker and sends it as an m.sticker, uploaded
// to the homeserver (not hotlinked). Where the account can't send stickers it
// goes as the link, as a GIF does.
func (a *App) sendSticker(room string, it klipy.Item) {
	ss, ok := a.src.(model.StickerSender)
	runUI := a.runUI
	if !ok || runUI == nil {
		a.src.Send(room, it.Full.URL, "")
		return
	}
	go func() {
		data, mime, err := fetchPicture(it.Full.URL, 20<<20)
		runUI(func() {
			if err != nil {
				a.toast = "Could not load the sticker"
				a.src.Send(room, it.Full.URL, "") // the link still shows as the sticker
				return
			}
			body := it.Title
			if body == "" {
				body = "Sticker"
			}
			ss.SendSticker(room, model.Sticker{Body: body, Mime: mime, W: it.Full.Width, H: it.Full.Height, Data: data}, "")
		})
	}()
}

// fetchPicture downloads a picture of at most limit bytes and says its media
// type.
func fetchPicture(url string, limit int64) ([]byte, string, error) {
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > limit {
		return nil, "", fmt.Errorf("over %d bytes", limit)
	}
	mime := strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	return data, mime, nil
}
