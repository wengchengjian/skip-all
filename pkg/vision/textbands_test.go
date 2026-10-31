package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func TestFindTextBands(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	paint := func(r image.Rectangle) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	paint(image.Rect(20, 20, 120, 40))
	paint(image.Rect(20, 60, 80, 80))
	bands := FindTextBands(img, geom.Rect{X1: 0, Y1: 0, X2: 200, Y2: 100}, TextBandOptions{})
	if len(bands) != 2 {
		t.Fatalf("bands = %+v", bands)
	}
	if bands[0] != (geom.Rect{X1: 20, Y1: 20, X2: 120, Y2: 40}) {
		t.Fatalf("band[0] = %+v", bands[0])
	}
	if bands[1] != (geom.Rect{X1: 20, Y1: 60, X2: 80, Y2: 80}) {
		t.Fatalf("band[1] = %+v", bands[1])
	}
}
