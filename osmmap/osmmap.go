// Package osmmap draws a small map of a place from OpenStreetMap's standard
// tiles: the few 256-pixel tiles around the place, joined, cropped to the size
// asked for, with a pin on the place.
//
// Privacy: fetching a tile tells OpenStreetMap's servers the user's IP address
// and the area looked at. The app therefore does this only when the user turned
// it on (Settings → Privacy), and never for anything else.
//
// OpenStreetMap's tile usage policy is followed: a User-Agent that names the
// app, tiles cached by the caller, few tiles per map (4 at most here), and the
// attribution "© OpenStreetMap contributors" shown with every map.
package osmmap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
)

// TileURL is the standard tile layer; tests point it at a server of their own.
var TileURL = "https://tile.openstreetmap.org/%d/%d/%d.png"

// UserAgent names the app to the tile servers, as their policy asks. It carries
// nothing about the user.
const UserAgent = "slopement (Matrix client; tile fetch from the user's device, opt-in)"

const tileSize = 256

// Fetch gets one tile's bytes.
type Fetch func(ctx context.Context, url string) ([]byte, error)

// HTTPFetch is a Fetch over HTTP with the app's User-Agent. It refuses a reply
// over 512 KB (a tile is about 10 KB).
func HTTPFetch(client *http.Client) Fetch {
	if client == nil {
		client = http.DefaultClient
	}
	return func(ctx context.Context, url string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("osmmap: tile server answered %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	}
}

// project is a place's position in the tile grid at a zoom, in pixels from the
// grid's corner (Web Mercator, as the tiles are).
func project(lat, lon float64, zoom int) (x, y float64) {
	n := math.Exp2(float64(zoom)) * tileSize
	lat = math.Max(-85.05112878, math.Min(85.05112878, lat))
	rad := lat * math.Pi / 180
	x = (lon + 180) / 360 * n
	y = (1 - math.Log(math.Tan(rad)+1/math.Cos(rad))/math.Pi) / 2 * n
	return x, y
}

// Render draws a w×h map centred on the place, as PNG bytes. w and h are at most
// 256 pixels each in the sense that at most 2×2 tiles are fetched: a map of up to
// 256 pixels each way needs four tiles at worst.
func Render(ctx context.Context, fetch Fetch, lat, lon float64, zoom, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 || w > tileSize || h > tileSize {
		return nil, errors.New("osmmap: a map is 1 to 256 pixels each way")
	}
	if zoom < 1 || zoom > 19 {
		return nil, errors.New("osmmap: zoom is 1 to 19")
	}
	cx, cy := project(lat, lon, zoom)
	left, top := cx-float64(w)/2, cy-float64(h)/2
	maxTile := 1<<zoom - 1
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{0xE8, 0xE8, 0xE8, 255}), image.Point{}, draw.Src)

	tx0, tx1 := int(math.Floor(left/tileSize)), int(math.Floor((left+float64(w)-1)/tileSize))
	ty0, ty1 := int(math.Floor(top/tileSize)), int(math.Floor((top+float64(h)-1)/tileSize))
	got := 0
	for ty := ty0; ty <= ty1; ty++ {
		for tx := tx0; tx <= tx1; tx++ {
			if ty < 0 || ty > maxTile {
				continue // above or below the world
			}
			wx := ((tx % (maxTile + 1)) + maxTile + 1) % (maxTile + 1) // the world repeats sideways
			data, err := fetch(ctx, fmt.Sprintf(TileURL, zoom, wx, ty))
			if err != nil {
				return nil, err
			}
			tile, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("osmmap: bad tile: %w", err)
			}
			dx, dy := int(math.Round(float64(tx*tileSize)-left)), int(math.Round(float64(ty*tileSize)-top))
			draw.Draw(canvas, image.Rect(dx, dy, dx+tileSize, dy+tileSize).Intersect(canvas.Bounds()), tile,
				image.Pt(max(-dx, 0), max(-dy, 0)), draw.Src)
			got++
		}
	}
	if got == 0 {
		return nil, errors.New("osmmap: no tiles for that place")
	}
	pin(canvas, w/2, h/2)
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// pin draws a marker whose tip is at (x, y): a filled circle on a point, with a
// white edge so it reads on any map.
func pin(img *image.RGBA, x, y int) {
	red := color.RGBA{0xD9, 0x3A, 0x3A, 255}
	white := color.RGBA{255, 255, 255, 255}
	const r = 9
	cy := y - r - 6 // the circle sits above the place
	for dy := -r - 2; dy <= r+2; dy++ {
		for dx := -r - 2; dx <= r+2; dx++ {
			d := dx*dx + dy*dy
			switch {
			case d <= r*r:
				if d <= 9 { // the dot in the middle
					img.Set(x+dx, cy+dy, white)
				} else {
					img.Set(x+dx, cy+dy, red)
				}
			case d <= (r+2)*(r+2):
				img.Set(x+dx, cy+dy, white)
			}
		}
	}
	for i := 0; i <= 8; i++ { // the point down to the place
		half := (8 - i) / 2
		for dx := -half; dx <= half; dx++ {
			img.Set(x+dx, cy+r+i, red)
		}
	}
}
