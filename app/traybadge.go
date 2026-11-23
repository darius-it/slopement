package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/egoist/mygo"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	xdraw "golang.org/x/image/draw"

	"slopement/kit"
)

// The unread count on the tray icon (a red dot with the number) and on the
// Dock or launcher icon. It is worked out when the app's data changes, not by
// timer: a frame only compares the count with the last one.

// unreadTotal is the number of unread messages in the rooms that notify: a muted
// room does not count.
func (a *App) unreadTotal() int {
	n := 0
	for _, r := range a.roomList() {
		if !r.Muted {
			n += r.Unread
		}
	}
	return n
}

// badgeText is the count as the badge writes it.
func badgeText(n int) string {
	if n > 99 {
		return "99"
	}
	return fmt.Sprint(n)
}

// trayBadgePNG is the tray icon with a red dot and the count in its corner
// (nothing added for zero).
func trayBadgePNG(base []byte, n int) []byte {
	if n <= 0 {
		return base
	}
	src, err := png.Decode(bytes.NewReader(base))
	if err != nil {
		return base
	}
	b := src.Bounds()
	img := image.NewRGBA(b)
	draw.Draw(img, b, src, b.Min, draw.Src)

	// The dot: the kit's danger color, in the lower right.
	d := kit.DarkPalette.Danger
	red := color.RGBA{d.R, d.G, d.B, 255}
	text := badgeText(n)
	// The digits are drawn at their own size, then doubled so they read at 64 px.
	face := basicfont.Face7x13
	w := font.MeasureString(face, text).Ceil()
	glyphs := image.NewRGBA(image.Rect(0, 0, w, 13))
	dr := font.Drawer{Dst: glyphs, Src: image.NewUniform(color.White), Face: face, Dot: fixed.P(0, 11)}
	dr.DrawString(text)
	scale := 2
	tw, th := w*scale, 13*scale
	diam := max(tw+6, th+4)
	r := diam / 2
	cx, cy := b.Max.X-r-1, b.Max.Y-r-1
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				img.SetRGBA(cx+x, cy+y, red)
			}
		}
	}
	dst := image.Rect(cx-tw/2, cy-th/2, cx-tw/2+tw, cy-th/2+th)
	xdraw.NearestNeighbor.Scale(img, dst, glyphs, glyphs.Bounds(), xdraw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return base
	}
	return out.Bytes()
}

// badgeSync keeps the counts shown as the unread number is: on the tray icon
// (while it exists) and on the app's Dock or launcher icon. It does its work
// only when the count has changed since the last frame. UI thread.
func (a *App) badgeSync() {
	if a.win == nil && !trayWithoutWindow {
		return
	}
	n := a.unreadTotal()
	if n == a.badge && a.badgeTray == a.tray {
		return
	}
	a.badge, a.badgeTray = n, a.tray
	setDockBadge(n)
	if a.tray != nil && len(TrayIcon) > 0 {
		_ = a.tray.SetIcon(trayBadgePNG(trayPNG(TrayIcon), n), false)
		tip := "slopement"
		if n > 0 {
			tip = fmt.Sprintf("slopement — %d unread", n)
		}
		a.tray.SetToolTip(tip)
	}
}

// setDockBadge puts the count on the Dock or launcher icon; a variable, so
// tests see it.
var setDockBadge = func(n int) { mygo.App.SetBadgeCount(n) }
