//go:build ignore

// Run with: go run gen_icon.go
// Generates Icon.png required by: fyne package -os windows -name "CloudSave"

package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

func main() {
	const size = 256
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	bg := color.RGBA{R: 52, G: 101, B: 164, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Background
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// Cloud shape — overlapping circles
	for _, c := range []struct{ x, y, r float64 }{
		{128, 105, 45},
		{90, 118, 32},
		{166, 118, 32},
		{110, 85, 36},
		{148, 85, 36},
	} {
		fillCircle(img, c.x, c.y, c.r, white)
	}
	fillRect(img, 62, 118, 194, 145, white)

	// Down-arrow in bg colour (push/pull symbol)
	fillRect(img, 116, 148, 140, 185, bg)
	fillTriangle(img, 128, 210, 98, 182, 158, 182, bg)

	f, err := os.Create("Icon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func fillCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	b := img.Bounds()
	for y := int(cy - r); y <= int(cy+r)+1; y++ {
		for x := int(cx - r); x <= int(cx+r)+1; x++ {
			if x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y {
				dx, dy := float64(x)-cx, float64(y)-cy
				if dx*dx+dy*dy <= r*r {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	b := img.Bounds()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillTriangle(img *image.RGBA, x0, y0, x1, y1, x2, y2 int, c color.RGBA) {
	minX, maxX := min3(x0, x1, x2), max3(x0, x1, x2)
	minY, maxY := min3(y0, y1, y2), max3(y0, y1, y2)
	b := img.Bounds()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y {
				if inTriangle(x, y, x0, y0, x1, y1, x2, y2) {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
}

func inTriangle(px, py, x0, y0, x1, y1, x2, y2 int) bool {
	d1 := (px-x1)*(y0-y1) - (x0-x1)*(py-y1)
	d2 := (px-x2)*(y1-y2) - (x1-x2)*(py-y2)
	d3 := (px-x0)*(y2-y0) - (x2-x0)*(py-y0)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max3(a, b, c int) int {
	if a > b {
		if a > c {
			return a
		}
		return c
	}
	if b > c {
		return b
	}
	return c
}
