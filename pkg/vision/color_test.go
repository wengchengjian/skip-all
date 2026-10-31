package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func TestIsInGrayMask(t *testing.T) {
	black := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			black.SetRGBA(x, y, color.RGBA{A: 255})
		}
	}
	if !IsInGrayMask(black, geom.Rect{X1: 0, Y1: 0, X2: 20, Y2: 20}, GrayMaskOptions{}) {
		t.Error("全黑图片应判定为灰度遮罩")
	}

	half := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if x < 10 {
				half.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			} else {
				half.SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
	}
	if IsInGrayMask(half, geom.Rect{X1: 0, Y1: 0, X2: 20, Y2: 20}, GrayMaskOptions{}) {
		t.Error("一半彩色不应判定为灰度遮罩")
	}

	stats := AnalyzeColor(half, geom.Rect{X1: 0, Y1: 0, X2: 20, Y2: 20})
	if stats.GrayMaskPercent != 0.5 {
		t.Errorf("灰度占比 = %v, want 0.5", stats.GrayMaskPercent)
	}
	if stats.ColorfulPercent != 0.5 {
		t.Errorf("彩色占比 = %v, want 0.5", stats.ColorfulPercent)
	}
}

func TestIsColorful(t *testing.T) {
	red := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			red.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	if !IsColorful(red, geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}, ColorOptions{}) {
		t.Error("纯红图片应判定为彩色")
	}

	gray := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			gray.SetRGBA(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	if IsColorful(gray, geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}, ColorOptions{}) {
		t.Error("灰色图片不应判定为彩色")
	}
}

func TestIsInGrayMaskCustomOptions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 100, G: 130, B: 160, A: 255})
		}
	}
	rect := geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}
	if IsInGrayMask(img, rect, GrayMaskOptions{}) {
		t.Error("默认阈值下不应判定为灰度遮罩")
	}
	if !IsInGrayMask(img, rect, GrayMaskOptions{Threshold: 200}) {
		t.Error("提高阈值后应判定为灰度遮罩")
	}
}

func TestAnalyzeColorEmptyRect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	stats := AnalyzeColor(img, geom.Rect{X1: 20, Y1: 20, X2: 30, Y2: 30})
	if stats != (ColorStats{}) {
		t.Errorf("越界区域应返回零值: %+v", stats)
	}
}
