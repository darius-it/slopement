package kit

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The app's logo, made by internal/icons/gen from icons/logo-*.svg (as
// bitmaps: it uses blend modes ui.SVG has not).
var (
	//go:embed assets/logo-dark.png
	logoDarkPNG []byte
	//go:embed assets/logo-light.png
	logoLightPNG []byte

	logoDark  = sync.OnceValues(func() (*ui.Bitmap, error) { return ui.DecodeBitmap(logoDarkPNG) })
	logoLight = sync.OnceValues(func() (*ui.Bitmap, error) { return ui.DecodeBitmap(logoLightPNG) })
)

// Logo is the app's mark, side DIPs square, the one drawn for the theme.
func Logo(c *ui.Context, side float32) {
	load := logoLight
	if Of(c).Dark {
		load = logoDark
	}
	if bmp, err := load(); err == nil {
		ui.Image(c, bmp).Size(side, side).Fit(ui.Contain).Shrink(0).Label("slopement logo")
	}
}
