package vision

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// 用真实鸣潮协奏环的参数：63x63 的框，内外半径按框高比例。
var (
	testRingRect   = geom.Rect{X1: 100, Y1: 100, X2: 163, Y2: 163}
	testRingColors = [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}}
)

// drawRing 画一个半径为 radius、线宽 width 的环，覆盖角度 [from, to]。
func drawRing(img *image.RGBA, radius, width float64, from, to float64) {
	r := testRingRect
	h := float64(r.Y2 - r.Y1)
	cx := float64(r.X2-r.X1)/2 + float64(r.X1)
	cy := h/2 + float64(r.Y1)
	inner := radius - width/2
	outer := radius + width/2
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy)
			if d < inner || d > outer {
				continue
			}
			ang := math.Atan2(dy, dx) + math.Pi
			if ang < from || ang > to {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{220, 205, 110, 255})
		}
	}
}

// TestRingFullThinStroke 是这次线上问题的回归用例。
//
// 游戏里的协奏环是细描边（约 2px），不是把整个环带填满。
// 旧的"命中像素 / 环内像素"判据在这种画法下只有 0.4 左右，永远判不满，
// 表现就是"协奏满了也不切人"。新的判据看环是否闭合，应当判满。
func TestRingFullThinStroke(t *testing.T) {
	r := testRingRect
	h := float64(r.Y2 - r.Y1)
	radius := (0.35119 + 0.42261) / 2 * h // 环带中线

	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	drawRing(img, radius, 2.0, 0, 2*math.Pi)

	ratio := RingCoverageRatio(img, r, testRingColors, 0.35119, 0.42261)
	full := RingFull(img, r, testRingColors, 0.35119, 0.42261, 0.95)
	t.Logf("细描边满环：旧占比=%.3f，新判据=%v", ratio, full)

	if ratio >= 0.95 {
		t.Fatalf("旧占比 %.3f 不该 >= 0.95，说明这个用例没复现出问题", ratio)
	}
	if !full {
		t.Fatalf("细描边的满环应当判为满（旧占比 %.3f）", ratio)
	}
}

// TestRingFullPartialArc 验证断开一大段的弧不算满。
func TestRingFullPartialArc(t *testing.T) {
	r := testRingRect
	h := float64(r.Y2 - r.Y1)
	radius := (0.35119 + 0.42261) / 2 * h

	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	// 只画 3/4 圈，留 90 度缺口（约 18 个扇区）。
	drawRing(img, radius, 2.0, 0, 1.5*math.Pi)

	if RingFull(img, r, testRingColors, 0.35119, 0.42261, 0.95) {
		t.Fatal("缺 90 度的弧不该判为满环")
	}
}

// TestRingFullEmpty 验证没有环时判为不满。
func TestRingFullEmpty(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	if RingFull(img, testRingRect, testRingColors, 0.35119, 0.42261, 0.95) {
		t.Fatal("空画面不该判为满环")
	}
}

// TestRingFullSolidAnnulus 验证把整个环带填满时也判为满（旧测试的画法）。
func TestRingFullSolidAnnulus(t *testing.T) {
	r := testRingRect
	h := float64(r.Y2 - r.Y1)
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			dx := float64(x) + 0.5 - (float64(r.X2-r.X1)/2 + float64(r.X1))
			dy := float64(y) + 0.5 - (h/2 + float64(r.Y1))
			d := math.Hypot(dx, dy)
			if d < 0.35119*h || d > 0.42261*h {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{220, 205, 110, 255})
		}
	}
	if !RingFull(img, r, testRingColors, 0.35119, 0.42261, 0.95) {
		t.Fatal("填满整个环带应当判为满")
	}
}
