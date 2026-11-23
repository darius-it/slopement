package osmmap

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// tileColor is the colour a fake tile (z,x,y) is filled with: different for
// every tile, so the test can tell which tile each pixel came from.
func tileColor(x, y int) color.RGBA { return color.RGBA{uint8(40 + x*7), uint8(40 + y*7), 200, 255} }

func fakeTiles(t *testing.T, fetched *atomic.Int32, ua *atomic.Value) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched.Add(1)
		ua.Store(r.Header.Get("User-Agent"))
		var z, x, y int
		if _, err := fmt.Sscanf(r.URL.Path, "/%d/%d/%d.png", &z, &x, &y); err != nil {
			http.NotFound(w, r)
			return
		}
		img := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
		c := tileColor(x, y)
		for i := 0; i < tileSize*tileSize; i++ {
			img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = c.R, c.G, c.B, 255
		}
		_ = png.Encode(w, img)
	}))
}

// The map is the tiles around the place, joined at the right offsets, with the
// pin on the middle, fetched with the app's User-Agent and few tiles.
func TestRenderJoinsTheTilesAroundThePlace(t *testing.T) {
	var fetched atomic.Int32
	var ua atomic.Value
	srv := fakeTiles(t, &fetched, &ua)
	defer srv.Close()
	old := TileURL
	TileURL = srv.URL + "/%d/%d/%d.png"
	defer func() { TileURL = old }()

	const lat, lon, zoom, w, h = 52.5200, 13.4050, 15, 240, 160
	data, err := Render(context.Background(), HTTPFetch(nil), lat, lon, zoom, w, h)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Fatalf("map is %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
	if n := fetched.Load(); n < 1 || n > 4 {
		t.Fatalf("fetched %d tiles, want 1 to 4", n)
	}
	if got, _ := ua.Load().(string); got != UserAgent {
		t.Fatalf("User-Agent %q", got)
	}

	// Each corner has the colour of the tile that holds that spot.
	cx, cy := project(lat, lon, zoom)
	for _, pt := range [][2]int{{2, 2}, {w - 3, 2}, {2, h - 3}, {w - 3, h - 3}} {
		px, py := cx-float64(w)/2+float64(pt[0]), cy-float64(h)/2+float64(pt[1])
		want := tileColor(int(px)/tileSize, int(py)/tileSize)
		r, g, b, _ := img.At(pt[0], pt[1]).RGBA()
		if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
			t.Fatalf("pixel %v is %v,%v,%v; tile colour is %v", pt, r>>8, g>>8, b>>8, want)
		}
	}
	// The pin: red pixels above the middle, none far from it.
	red := func(x, y int) bool { r, g, b, _ := img.At(x, y).RGBA(); return r>>8 > 200 && g>>8 < 90 && b>>8 < 90 }
	if !red(w/2+6, h/2-15) && !red(w/2-6, h/2-15) && !red(w/2, h/2-24) {
		t.Fatal("no pin above the middle")
	}
	if red(5, 5) || red(w-5, h-5) {
		t.Fatal("red at the corners")
	}
}

// Bad sizes and zooms are refused, a failing tile server is an error, and the
// edges of the world (the poles, the date line) do not ask for tiles that
// cannot exist.
func TestRenderRefusesAndSurvivesTheEdges(t *testing.T) {
	ok := func(context.Context, string) ([]byte, error) {
		var b bytes.Buffer
		_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, tileSize, tileSize)))
		return b.Bytes(), nil
	}
	ctx := context.Background()
	if _, err := Render(ctx, ok, 0, 0, 15, 300, 100); err == nil {
		t.Fatal("a map wider than a tile was drawn")
	}
	if _, err := Render(ctx, ok, 0, 0, 25, 100, 100); err == nil {
		t.Fatal("zoom 25 was drawn")
	}
	if _, err := Render(ctx, func(context.Context, string) ([]byte, error) { return nil, fmt.Errorf("down") }, 1, 1, 10, 100, 100); err == nil {
		t.Fatal("a failing fetch was not an error")
	}
	if _, err := Render(ctx, func(context.Context, string) ([]byte, error) { return []byte("junk"), nil }, 1, 1, 10, 100, 100); err == nil {
		t.Fatal("a junk tile was not an error")
	}
	for _, c := range [][2]float64{{90, 0}, {-90, 0}, {0, 180}, {0, -180}, {85, 179.99}} {
		var asked []string
		fetch := func(ctx context.Context, url string) ([]byte, error) { asked = append(asked, url); return ok(ctx, url) }
		if _, err := Render(ctx, fetch, c[0], c[1], 3, 200, 200); err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		for _, u := range asked {
			var z, x, y int
			fmt.Sscanf(u, "https://tile.openstreetmap.org/%d/%d/%d.png", &z, &x, &y)
			if x < 0 || y < 0 || x > 7 || y > 7 {
				t.Fatalf("%v asked for a tile that does not exist: %s", c, u)
			}
		}
	}
}
