package app

import (
	"context"
	"time"

	"github.com/egoist/mygo/plugins/imagepicker"
)

// The + menu's photo library and camera on phones: the system's own pickers
// (imagepicker), which ask for no permission to read the library. What
// comes back is copies in the app's temporary directory; they join the
// files waiting in the composer, as any chosen file does.

// The pickers, replaced by tests.
var (
	pickPhotosFn = func(ctx context.Context) ([]string, error) {
		return imagepicker.Pick(ctx, imagepicker.Options{Kind: imagepicker.Any, Limit: -1})
	}
	capturePhotoFn = imagepicker.Capture
)

// pickPhotos shows the photo library; the picture or video chosen waits in
// the room's composer. The picker waits for the user, so it runs off the view.
func (a *App) pickPhotos(room string) {
	if a.runUI == nil {
		return // no window (tests)
	}
	go func() {
		paths, err := pickPhotosFn(context.Background())
		if err != nil || len(paths) == 0 {
			if err != nil {
				a.runUI(func() { a.toast = "Couldn't open your photos" })
			}
			return
		}
		a.addPaths(room, paths)
	}()
}

// takePhoto shows the camera; the photo waits in the room's composer.
func (a *App) takePhoto(room string) {
	if a.runUI == nil {
		return
	}
	go func() {
		// A person takes a while; the camera is the system's, and ends when they do.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		path, err := capturePhotoFn(ctx)
		if err != nil {
			a.runUI(func() { a.toast = "Couldn't open the camera" })
			return
		}
		if path == "" {
			return // they went back
		}
		a.addPaths(room, []string{path})
	}()
}
