package vision

import (
	"image"
	"math"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// RingCoverage 统计环形区域里落在任一颜色范围内的像素占比，对应上游 count_rings：
// 环半径按区域高度的比例给出（上游协奏值环用 0.35119 与 0.42261）。
func RingCoverage(img *image.RGBA, rect geom.Rect, colors [][2][3]uint8, inner, outer float64) float64 {
	if img == nil || len(colors) == 0 {
		return 0
	}
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(img.Bounds())
	if r.Empty() {
		return 0
	}
	h := float64(r.Dy())
	cx := float64(r.Dx()) / 2
	cy := h / 2
	r1 := inner * h
	r2 := outer * h
	if r2 <= r1 {
		return 0
	}
	hit, total := 0, 0
	for y := 0; y < r.Dy(); y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < r.Dx(); x++ {
			dx := float64(x) + 0.5 - cx
			d := math.Hypot(dx, dy)
			if d < r1 || d > r2 {
				continue
			}
			total++
			px := img.RGBAAt(r.Min.X+x, r.Min.Y+y)
			for _, c := range colors {
				if px.R >= c[0][0] && px.R <= c[1][0] &&
					px.G >= c[0][1] && px.G <= c[1][1] &&
					px.B >= c[0][2] && px.B <= c[1][2] {
					hit++
					break
				}
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total)
}

// RingCoverages 一次扫描算出每种颜色的环内占比（对应上游 _ensure_ring_index 的逐色比较）。
// 返回的切片长度与 colors 一致。
func RingCoverages(img *image.RGBA, rect geom.Rect, colors [][2][3]uint8, inner, outer float64) []float64 {
	out := make([]float64, len(colors))
	if img == nil || len(colors) == 0 {
		return out
	}
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(img.Bounds())
	if r.Empty() {
		return out
	}
	h := float64(r.Dy())
	cx := float64(r.Dx()) / 2
	cy := h / 2
	r1 := inner * h
	r2 := outer * h
	if r2 <= r1 {
		return out
	}
	hits := make([]int, len(colors))
	total := 0
	for y := 0; y < r.Dy(); y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < r.Dx(); x++ {
			dx := float64(x) + 0.5 - cx
			d := math.Hypot(dx, dy)
			if d < r1 || d > r2 {
				continue
			}
			total++
			px := img.RGBAAt(r.Min.X+x, r.Min.Y+y)
			for i, c := range colors {
				if px.R >= c[0][0] && px.R <= c[1][0] &&
					px.G >= c[0][1] && px.G <= c[1][1] &&
					px.B >= c[0][2] && px.B <= c[1][2] {
					hits[i]++
				}
			}
		}
	}
	if total == 0 {
		return out
	}
	for i := range out {
		out[i] = float64(hits[i]) / float64(total)
	}
	return out
}
