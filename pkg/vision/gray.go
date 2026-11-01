package vision

import (
	"image"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// GrayRunLength 统计一行/一列里落在灰度区间 [lower, upper] 的连续像素长度。
// 用来读绝区零能量条这类"一条实心横条"的填充长度（上游 FOREGROUND_GRAY_RANGE_LENGTH）。
func GrayRunLength(img *image.RGBA, rect geom.Rect, lower, upper uint8) int {
	if img == nil || rect.X2 <= rect.X1 || rect.Y2 <= rect.Y1 {
		return 0
	}
	bounds := img.Bounds()
	if rect.X1 < bounds.Min.X || rect.Y1 < bounds.Min.Y || rect.X2 > bounds.Max.X || rect.Y2 > bounds.Max.Y {
		return 0
	}
	best := 0
	run := 0
	for y := rect.Y1; y < rect.Y2; y++ {
		run = 0
		for x := rect.X1; x < rect.X2; x++ {
			c := img.RGBAAt(x, y)
			if c.R >= lower && c.R <= upper && c.G >= lower && c.G <= upper && c.B >= lower && c.B <= upper {
				run++
				if run > best {
					best = run
				}
			} else {
				run = 0
			}
		}
	}
	return best
}
