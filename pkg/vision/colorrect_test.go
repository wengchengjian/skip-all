package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

var enemyRed = [2][3]uint8{{174, 55, 55}, {225, 85, 76}}

func drawRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func TestFindColorRectanglesFindsHealthBar(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	drawRect(img, image.Rect(800, 100, 920, 108), color.RGBA{200, 70, 60, 255})
	rects := FindColorRectangles(img, geom.Rect{X1: 0, Y1: 0, X2: 1920, Y2: 1080}, enemyRed, 50, 5, 14, 0.95)
	if len(rects) != 1 {
		t.Fatalf("应找到 1 条血条, got %+v", rects)
	}
	if rects[0].X1 != 800 || rects[0].Y1 != 100 || rects[0].X2 != 920 || rects[0].Y2 != 108 {
		t.Fatalf("血条位置 = %+v", rects[0])
	}
}

func TestFindColorRectanglesRejectsTooNarrow(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	drawRect(img, image.Rect(800, 100, 830, 108), color.RGBA{200, 70, 60, 255})
	if rects := FindColorRectangles(img, geom.Rect{X1: 0, Y1: 0, X2: 1920, Y2: 1080}, enemyRed, 50, 5, 14, 0.95); len(rects) != 0 {
		t.Fatalf("宽度不足不该算血条: %+v", rects)
	}
}

func TestFindColorRectanglesRejectsLowFill(t *testing.T) {
	// 上边一条实线加下面几根竖线：外接矩形 120x8 够大，但填充率只有三成左右，
	// 上游要求 >=0.95，这种涂鸦不该被当成血条。
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	drawRect(img, image.Rect(800, 100, 920, 101), color.RGBA{200, 70, 60, 255})
	for x := 800; x < 920; x += 4 {
		drawRect(img, image.Rect(x, 101, x+1, 108), color.RGBA{200, 70, 60, 255})
	}
	if rects := FindColorRectangles(img, geom.Rect{X1: 0, Y1: 0, X2: 1920, Y2: 1080}, enemyRed, 50, 5, 14, 0.95); len(rects) != 0 {
		t.Fatalf("填充率不足不该算血条: %+v", rects)
	}
}
