package vision

import (
	"image"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TextBandOptions 控制白色文字行的检测。
type TextBandOptions struct {
	ColorRange *[2][3]uint8
	MinPixels  int // 一行至少需要多少个当前颜色像素
	MinHeight  int // 文字行最小高度
	MaxRowGap  int // 行间允许的空隙
}

// FindTextBands 在区域内按行投影找白色文字行，返回每行的包围盒（整帧坐标）。
// 只做颜色和行投影，不跑 OCR，适合按钮、选项、确认框这类已知区域。
func FindTextBands(img image.Image, rect geom.Rect, opts TextBandOptions) []geom.Rect {
	if img == nil {
		return nil
	}
	colorRange := [2][3]uint8{{220, 220, 220}, {255, 255, 255}}
	if opts.ColorRange != nil {
		colorRange = *opts.ColorRange
	}
	if opts.MinPixels <= 0 {
		opts.MinPixels = 2
	}
	if opts.MinHeight <= 0 {
		opts.MinHeight = 6
	}
	if opts.MaxRowGap <= 0 {
		opts.MaxRowGap = 2
	}
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(img.Bounds())
	if r.Empty() {
		return nil
	}
	type span struct {
		y1, y2 int
		minX   int
		maxX   int
	}
	var bands []span
	var cur span
	gap := 0
	active := false
	for y := r.Min.Y; y < r.Max.Y; y++ {
		rowMin, rowMax, count := r.Max.X, r.Min.X, 0
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			rr, gg, bb := uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)
			if rr >= colorRange[0][0] && rr <= colorRange[1][0] &&
				gg >= colorRange[0][1] && gg <= colorRange[1][1] &&
				bb >= colorRange[0][2] && bb <= colorRange[1][2] {
				count++
				if x < rowMin {
					rowMin = x
				}
				if x > rowMax {
					rowMax = x
				}
			}
		}
		if count >= opts.MinPixels {
			if !active {
				cur = span{y1: y, y2: y + 1, minX: rowMin, maxX: rowMax}
				active = true
			} else {
				cur.y2 = y + 1
				if rowMin < cur.minX {
					cur.minX = rowMin
				}
				if rowMax > cur.maxX {
					cur.maxX = rowMax
				}
			}
			gap = 0
			continue
		}
		if active {
			gap++
			if gap > opts.MaxRowGap {
				if cur.y2-cur.y1 >= opts.MinHeight && cur.maxX-cur.minX >= 8 {
					bands = append(bands, cur)
				}
				cur = span{}
				active = false
				gap = 0
			}
		}
	}
	if active && cur.y2-cur.y1 >= opts.MinHeight && cur.maxX-cur.minX >= 8 {
		bands = append(bands, cur)
	}
	out := make([]geom.Rect, 0, len(bands))
	for _, b := range bands {
		// Rect 的 X2/Y2 是开区间；行扫描取到最后像素后要补 1。
		out = append(out, geom.Rect{X1: b.minX, Y1: b.y1, X2: b.maxX + 1, Y2: b.y2})
	}
	return out
}
