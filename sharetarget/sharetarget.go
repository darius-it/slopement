// Package sharetarget makes slopement a share target on Android: what other apps
// share to it (words, a picture, several files) reaches OnShare. Elsewhere it
// does nothing (desktops use "Open with", see mygo.config.ts; iOS has no share
// extension yet, see docs/known-gaps.md).
package sharetarget

import (
	"context"
	"encoding/json"

	"github.com/egoist/mygo"
)

const pluginID = "sharetarget"

// File is one shared file, already copied into the app's cache: the app removes
// it when it has read it, or when the user cancels.
type File struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Mime string `json:"mime"`
}

// Share is what another app handed over.
type Share struct {
	Text  string `json:"text"`
	Files []File `json:"files"`
}

// OnShare calls fn on the main thread when something is shared to the app. It
// also delivers the share the app was started by, once the app is ready, so
// register it before Run.
func OnShare(fn func(Share)) (off func()) {
	off = mygo.Native(pluginID).On("share", func(data []byte) {
		var s Share
		if json.Unmarshal(data, &s) == nil && (s.Text != "" || len(s.Files) > 0) {
			fn(s)
		}
	})
	mygo.App.WhenReady(func() {
		go mygo.Native(pluginID).Call(context.Background(), "listen", nil, nil)
	})
	return off
}
