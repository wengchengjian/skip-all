package vision

import (
	"image"
	"image/color"
	"math/rand"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestColorRatioAtLeastEquivalence 验证提前退出与"算完再比"完全等价。
//
// 提前退出用的是两条不等式，理论上不会改变结论；这个测试用随机画面兜底。
func TestColorRatioAtLeastEquivalence(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	dark := [2][3]uint8{{0, 0, 0}, {60, 60, 60}}
	for iter := 0; iter < 40; iter++ {
		w, h := 40+rng.Intn(120), 30+rng.Intn(90)
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		// 用一个可调的暗色比例，专门覆盖阈值附近。
		darkRatio := rng.Float64()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var v uint8
				if rng.Float64() < darkRatio {
					v = uint8(rng.Intn(61))
				} else {
					v = uint8(150 + rng.Intn(106))
				}
				img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
			}
		}
		rect := geom.Rect{X1: 0, Y1: 0, X2: w, Y2: h}
		if iter%3 == 0 && w > 20 && h > 20 {
			rect = geom.Rect{X1: 3, Y1: 2, X2: w - 4, Y2: h - 3}
		}
		for _, th := range []float64{0.3, 0.7, 0.85, 0.95} {
			want := ColorRatio(img, rect, dark) >= th
			got := ColorRatioAtLeast(img, rect, dark, th)
			if want != got {
				t.Fatalf("iter=%d th=%.2f 不一致: ColorRatio=%.4f want=%v got=%v",
					iter, th, ColorRatio(img, rect, dark), want, got)
			}
		}
	}
}
