// Command gen makes every app icon from the two logo SVGs in icons/.
//
//	go run ./internal/icons/gen
//
// It needs rsvg-convert (librsvg), which knows the blend modes and masks the
// logo uses. -dark and -light take a new logo's SVGs and keep them in icons/.
//
// What it writes, on the app's own backgrounds (kit/tokens.go Bg):
//
//	resources/icon.png              Windows, Linux, the tray, mygo dev: a rounded tile (dark)
//	icons/ios.png                   iOS: full bleed and opaque, the system rounds and glazes it
//	icons/android-foreground.png    Android's adaptive icon, inside its safe zone
//	icons/AppIcon.icon/Assets/*.png macOS 26's Liquid Glass layers, light and dark
//	kit/assets/logo-*.png           the mark inside the app (kit.Logo)
//
// icons/AppIcon.icon/icon.json (the glass settings) is written by hand.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const size = 1024

// How big the logo's canvas is rendered for each icon, in pixels of the 1024
// square; the mark itself is about three quarters as wide.
const (
	tileLogo    = 880  // on the 824 tile: the mark 79% of its width
	fullLogo    = 1070 // iOS and the glass layers: 78% of the icon
	androidLogo = 760  // 81% of what launchers show, inside the safe circle
)

var darkBg = color.NRGBA{0x12, 0x11, 0x16, 0xFF} // kit's dark Bg

// pillarMark is the transform every pillar rect of the logo has, and nothing
// else does: it tells the two layers apart.
const pillarMark = "matrix(0.276194"

func main() {
	dark := flag.String("dark", "", "a new logo for dark backgrounds (SVG), kept as icons/logo-dark.svg")
	light := flag.String("light", "", "a new logo for light backgrounds (SVG), kept as icons/logo-light.svg")
	flag.Parse()
	for src, name := range map[string]string{*dark: "icons/logo-dark.svg", *light: "icons/logo-light.svg"} {
		if src == "" {
			continue
		}
		b, err := os.ReadFile(src)
		check(err)
		write(name, b)
	}

	darkSVG := read("icons/logo-dark.svg")
	lightSVG := read("icons/logo-light.svg")

	// The desktop tile: Apple's grid (824 of 1024, corners of 185), which
	// Windows and Linux icons sit well in too.
	tile := render([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024">`+
		`<rect x="100" y="100" width="824" height="824" rx="185" fill="#121116"/></svg>`), size)
	draw.Draw(tile, tile.Bounds(), centred(darkSVG, tileLogo), image.Point{}, draw.Over)
	writePNG("resources/icon.png", tile)

	ios := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(ios, ios.Bounds(), image.NewUniform(darkBg), image.Point{}, draw.Src)
	draw.Draw(ios, ios.Bounds(), centred(darkSVG, fullLogo), image.Point{}, draw.Over)
	writePNG("icons/ios.png", ios)

	// Launchers crop an adaptive icon to its middle two thirds and may mask it
	// to a circle: the mark stays inside the 66/108 safe circle.
	writePNG("icons/android-foreground.png", centred(darkSVG, androidLogo))

	for name, svg := range map[string][]byte{"dark": darkSVG, "light": lightSVG} {
		writePNG("kit/assets/logo-"+name+".png", trimmed(svg, 256))
		roof, pillars := layers(svg, fullLogo)
		writePNG("icons/AppIcon.icon/Assets/roof-"+name+".png", roof)
		writePNG("icons/AppIcon.icon/Assets/pillars-"+name+".png", pillars)
	}
}

// centred renders the logo n pixels across, its mark (not its canvas) in the
// middle of a transparent 1024 square.
func centred(svg []byte, n int) *image.NRGBA {
	img := render(svg, n)
	return place(img, offset(img))
}

// trimmed is the mark alone, centred in a transparent square of side pixels
// it nearly fills across: the logo as the app itself draws it.
func trimmed(svg []byte, side int) *image.NRGBA {
	img := render(svg, side*4/3) // the mark is about three quarters of its canvas
	at := offset(img).Sub(image.Pt((size-side)/2, (size-side)/2))
	out := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(out, img.Bounds().Add(at), img, image.Point{}, draw.Src)
	return out
}

// layers splits the logo into the roof and the pillars over it, placed alike:
// stacked, they are the logo. The roof is the logo without its pillars; the
// pillars are the whole logo cut to their shapes, so their colors stay as
// drawn, the roof showing through them included.
func layers(svg []byte, n int) (roof, pillars *image.NRGBA) {
	var without, shapes []string
	lines := strings.Split(string(svg), "\n")
	shapes = append(shapes, lines[0]) // the <svg> tag
	for _, l := range lines {
		if !strings.Contains(l, pillarMark) {
			without = append(without, l)
		} else if strings.Contains(l, `fill="#`) {
			shapes = append(shapes, l)
		}
	}
	shapes = append(shapes, "</svg>")
	if len(shapes) < 3 {
		log.Fatal("no pillars found: the logo changed, update pillarMark")
	}
	full := render(svg, n)
	mask := render([]byte(strings.Join(shapes, "\n")), n)
	cut := image.NewNRGBA(full.Bounds())
	draw.DrawMask(cut, cut.Bounds(), full, image.Point{}, mask, image.Point{}, draw.Src)
	at := offset(full)
	return place(render([]byte(strings.Join(without, "\n")), n), at), place(cut, at)
}

// offset is where an n-pixel rendering goes so its mark is centred.
func offset(img *image.NRGBA) image.Point {
	b := img.Bounds()
	box := image.Rectangle{Min: b.Max, Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A > 8 {
				box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
				box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
			}
		}
	}
	return image.Pt((size-box.Dx())/2-box.Min.X, (size-box.Dy())/2-box.Min.Y)
}

func place(img *image.NRGBA, at image.Point) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(out, img.Bounds().Add(at), img, image.Point{}, draw.Src)
	return out
}

func render(svg []byte, n int) *image.NRGBA {
	cmd := exec.Command("rsvg-convert", "-w", strconv.Itoa(n), "-h", strconv.Itoa(n))
	cmd.Stdin = bytes.NewReader(svg)
	var errs bytes.Buffer
	cmd.Stderr = &errs
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(fmt.Errorf("rsvg-convert: %w: %s", err, errs.String()))
	}
	img, err := png.Decode(bytes.NewReader(out))
	check(err)
	n2 := image.NewNRGBA(img.Bounds())
	draw.Draw(n2, n2.Bounds(), img, img.Bounds().Min, draw.Src)
	return n2
}

func read(name string) []byte {
	b, err := os.ReadFile(name)
	check(err)
	return b
}

func write(name string, b []byte) {
	check(os.MkdirAll(filepath.Dir(name), 0o755))
	check(os.WriteFile(name, b, 0o644))
	fmt.Println("wrote", name)
}

func writePNG(name string, img image.Image) {
	var b bytes.Buffer
	check(png.Encode(&b, img))
	write(name, b.Bytes())
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
