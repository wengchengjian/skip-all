package vision

import (
	"image"
	"image/color"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// GrayMaskOptions 是灰度遮罩判定参数。零值字段会使用默认值。
type GrayMaskOptions struct {
	// Threshold 是纯黑/纯白阈值：RGB 全部小于它或全部大于 255-它 视为黑白。
	Threshold int
	// RGBDiffThreshold 是 RGB 最大差值阈值，差值小于它视为灰色。
	RGBDiffThreshold int
	// PercentRate 是满足条件的像素占比阈值。
	PercentRate float64
}

// DefaultGrayMaskOptions 返回灰度遮罩的默认参数。
func DefaultGrayMaskOptions() GrayMaskOptions {
	return GrayMaskOptions{Threshold: 55, RGBDiffThreshold: 20, PercentRate: 0.9}
}

func (o GrayMaskOptions) withDefaults() GrayMaskOptions {
	d := DefaultGrayMaskOptions()
	if o.Threshold <= 0 {
		o.Threshold = d.Threshold
	}
	if o.Threshold > 255 {
		o.Threshold = 255
	}
	if o.RGBDiffThreshold <= 0 {
		o.RGBDiffThreshold = d.RGBDiffThreshold
	}
	if o.RGBDiffThreshold > 255 {
		o.RGBDiffThreshold = 255
	}
	if o.PercentRate <= 0 {
		o.PercentRate = d.PercentRate
	}
	return o
}

// ColorOptions 是彩色判定参数。零值字段会使用默认值。
type ColorOptions struct {
	// SaturationThreshold 是 HSV 饱和度均值阈值（0-255）。
	SaturationThreshold float64
	// ColorRatioThreshold 是高饱和像素占比阈值。
	ColorRatioThreshold float64
}

// DefaultColorOptions 返回彩色判定的默认参数。
func DefaultColorOptions() ColorOptions {
	return ColorOptions{SaturationThreshold: 1, ColorRatioThreshold: 0.01}
}

func (o ColorOptions) withDefaults() ColorOptions {
	d := DefaultColorOptions()
	if o.SaturationThreshold <= 0 {
		o.SaturationThreshold = d.SaturationThreshold
	}
	if o.ColorRatioThreshold <= 0 {
		o.ColorRatioThreshold = d.ColorRatioThreshold
	}
	return o
}

// IsInGrayMask 判断指定区域是否接近灰色遮罩（通常是对话框底部的半透明遮罩）。
func IsInGrayMask(img image.Image, rect geom.Rect, opts GrayMaskOptions) bool {
	opts = opts.withDefaults()
	stats, ok := analyzeColorWith(img, rect, opts)
	if !ok {
		return false
	}
	return stats.GrayMaskPercent > opts.PercentRate
}

// IsColorful 判断指定区域是否包含明显彩色像素。
func IsColorful(img image.Image, rect geom.Rect, opts ColorOptions) bool {
	opts = opts.withDefaults()
	stats, ok := analyzeColorWith(img, rect, DefaultGrayMaskOptions())
	if !ok {
		return false
	}
	return stats.MeanSaturation > opts.SaturationThreshold && stats.ColorfulPercent > opts.ColorRatioThreshold
}

// AnalyzeColor 统计指定区域的灰度遮罩占比、彩色像素占比和平均饱和度。
func AnalyzeColor(img image.Image, rect geom.Rect) ColorStats {
	stats, _ := analyzeColorWith(img, rect, DefaultGrayMaskOptions())
	return stats
}

func analyzeColorWith(img image.Image, rect geom.Rect, grayOpts GrayMaskOptions) (ColorStats, bool) {
	var stats ColorStats
	if img == nil {
		return stats, false
	}
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(img.Bounds())
	if r.Empty() {
		return stats, false
	}
	total := 0
	grayCount := 0
	colorCount := 0
	var saturationSum float64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb := rgb8(img.At(x, y))
			total++
			maxC := max(cr, max(cg, cb))
			minC := min(cr, min(cg, cb))
			if (cr < uint8(grayOpts.Threshold) && cg < uint8(grayOpts.Threshold) && cb < uint8(grayOpts.Threshold)) ||
				(cr > uint8(255-grayOpts.Threshold) && cg > uint8(255-grayOpts.Threshold) && cb > uint8(255-grayOpts.Threshold)) ||
				int(maxC)-int(minC) < grayOpts.RGBDiffThreshold {
				grayCount++
			}
			s := 0.0
			if maxC > 0 {
				s = float64(maxC-minC) / float64(maxC) * 255
			}
			saturationSum += s
			if s > DefaultColorOptions().SaturationThreshold {
				colorCount++
			}
		}
	}
	stats.GrayMaskPercent = float64(grayCount) / float64(total)
	stats.ColorfulPercent = float64(colorCount) / float64(total)
	stats.MeanSaturation = saturationSum / float64(total)
	return stats, true
}

// rgb8 把任意颜色转换为 8 位 RGB。
func rgb8(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// filterByColorRange 只保留颜色范围内的像素，输出黑白 RGB 图供 OCR 使用。
func filterByColorRange(img image.Image, colorRange [2][3]uint8) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b := rgb8(img.At(x, y))
			if inRange(r, g, b, colorRange) {
				out.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				out.SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
	}
	return out
}

func inRange(r, g, b uint8, colorRange [2][3]uint8) bool {
	lo, hi := colorRange[0], colorRange[1]
	return r >= lo[0] && r <= hi[0] && g >= lo[1] && g <= hi[1] && b >= lo[2] && b <= hi[2]
}

// ColorRatio 统计区域里落在颜色范围内的像素比例。
// 上游大量状态判定都是"颜色占比"（如闪光、技能可用高亮）。
func ColorRatio(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8) float64 {
	if img == nil {
		return 0
	}
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(img.Bounds())
	if r.Empty() {
		return 0
	}
	hit, total := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			total++
			c := img.RGBAAt(x, y)
			if inRange(c.R, c.G, c.B, colorRange) {
				hit++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total)
}
