package vision

import (
	"image"
	"math"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// 协奏环判据的几个常量，对应上游 count_rings。
const (
	// ringSectors 是把环按角度切成的扇区数（每 5 度一格）。
	ringSectors = 72
	// ringSectorMin 是判定"环闭合"所需的扇区覆盖率。
	// 上游要求轮廓闭合；留一格容差给环本身的起始缺口。
	ringSectorMin = 0.9
	// ringMinAreaRatio 对应上游 min_area = 1500/(3840*2160)*W*H。
	ringMinAreaRatio = 1500.0 / (3840.0 * 2160.0)
)

// RingFull 判断环形区域里的颜色是否构成一个完整的环（协奏值满）。
//
// 不能沿用"命中像素 / 环内像素总数"这种覆盖率：环只有 4~5 px 宽、实际描边更细，
// 满了也只有 0.5~0.7，永远到不了 0.95。上游 count_rings 的做法是
// "掩码 -> 3x3 闭运算 -> 连通域 -> 包围盒面积够大且轮廓闭合"，
// 这里用角度扇区覆盖近似"轮廓闭合"：完整环每个角度都有颜色，断开的弧会留空。
func RingFull(img *image.RGBA, rect geom.Rect, colors [][2][3]uint8, inner, outer, minSectorRatio float64) bool {
	if img == nil || len(colors) == 0 {
		return false
	}
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(img.Bounds())
	if r.Empty() {
		return false
	}
	h := float64(r.Dy())
	cx := float64(r.Dx()) / 2
	cy := h / 2
	r1 := inner * h
	r2 := outer * h
	if r2 <= r1 || r2 < 2 {
		return false
	}

	// 掩码 + 3x3 膨胀，把环上 1px 的断点补上（对应上游的闭运算）。
	w, hh := r.Dx(), r.Dy()
	mask := make([]bool, w*hh)
	for y := 0; y < hh; y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < w; x++ {
			dx := float64(x) + 0.5 - cx
			d := math.Hypot(dx, dy)
			if d < r1 || d > r2 {
				continue
			}
			px := img.RGBAAt(r.Min.X+x, r.Min.Y+y)
			for _, c := range colors {
				if px.R >= c[0][0] && px.R <= c[1][0] &&
					px.G >= c[0][1] && px.G <= c[1][1] &&
					px.B >= c[0][2] && px.B <= c[1][2] {
					mask[y*w+x] = true
					break
				}
			}
		}
	}
	dilated := make([]bool, len(mask))
	for y := 0; y < hh; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < w && ny >= 0 && ny < hh {
						dilated[ny*w+nx] = true
					}
				}
			}
		}
	}

	// 角度扇区覆盖。
	var sectors [ringSectors]bool
	hits := 0
	minX, minY, maxX, maxY := w, hh, -1, -1
	for y := 0; y < hh; y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < w; x++ {
			if !dilated[y*w+x] {
				continue
			}
			dx := float64(x) + 0.5 - cx
			d := math.Hypot(dx, dy)
			if d < r1 || d > r2 {
				continue
			}
			hits++
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
			ang := math.Atan2(dy, dx) + math.Pi // 0..2pi
			idx := int(ang / (2 * math.Pi) * ringSectors)
			if idx >= ringSectors {
				idx = ringSectors - 1
			}
			sectors[idx] = true
		}
	}
	if hits == 0 {
		return false
	}
	// 包围盒面积门槛对应上游 min_area（按屏幕像素比例缩放）。
	minArea := ringMinAreaRatio * float64(img.Bounds().Dx()*img.Bounds().Dy())
	if (maxX-minX+1)*(maxY-minY+1) < int(minArea) {
		return false
	}
	covered := 0
	for _, ok := range sectors {
		if ok {
			covered++
		}
	}
	if minSectorRatio <= 0 {
		minSectorRatio = ringSectorMin
	}
	return float64(covered)/float64(ringSectors) >= minSectorRatio
}

// RingCoverageRatio 是旧的"命中像素占比"，仅用于诊断对比，不参与判定。
func RingCoverageRatio(img *image.RGBA, rect geom.Rect, colors [][2][3]uint8, inner, outer float64) float64 {
	best := 0.0
	for _, v := range RingCoverages(img, rect, colors, inner, outer) {
		if v > best {
			best = v
		}
	}
	return best
}
