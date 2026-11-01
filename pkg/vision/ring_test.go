package vision

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func TestRingCoverageDetectsFullRing(t *testing.T) {
	rect := geom.Rect{X1: 0, Y1: 0, X2: 63, Y2: 63}
	img := image.NewRGBA(image.Rect(0, 0, 63, 63))
	c := color.RGBA{220, 205, 110, 255}
	for y := 0; y < 63; y++ {
		for x := 0; x < 63; x++ {
			d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
			if d >= 22.1 && d <= 26.6 {
				img.SetRGBA(x, y, c)
			}
		}
	}
	colors := [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}}
	if coverage := RingCoverage(img, rect, colors, 0.35119, 0.42261); coverage < 0.9 {
		t.Fatalf("满环覆盖率 = %.3f", coverage)
	}
}

func TestRingCoverageEmptyWhenColorMismatch(t *testing.T) {
	rect := geom.Rect{X1: 0, Y1: 0, X2: 63, Y2: 63}
	img := image.NewRGBA(image.Rect(0, 0, 63, 63))
	gray := color.RGBA{40, 40, 40, 255}
	for y := 0; y < 63; y++ {
		for x := 0; x < 63; x++ {
			d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
			if d >= 22.1 && d <= 26.6 {
				img.SetRGBA(x, y, gray)
			}
		}
	}
	colors := [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}}
	if coverage := RingCoverage(img, rect, colors, 0.35119, 0.42261); coverage > 0.01 {
		t.Fatalf("颜色不符覆盖率 = %.3f", coverage)
	}
}
