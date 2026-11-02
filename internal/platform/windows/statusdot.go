//go:build windows

package windows

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// GradientDot 生成状态点图标（PNG 字节流）。
//
// 设计取向是"精密仪器"而不是"玻璃珠"：纯平涂的底色 + 极轻的纵向光照渐变
// （顶部略亮、底部略暗，幅度约 10%），顶部内缘一道几乎看不见的细高光作为
// 倒角，圆外不做暗圈。高饱和的糖果色、径向高光球、黑色描边都会让 16px 的
// 状态点在原生菜单里显得廉价，这里刻意都不用。
//
// 用 4×4 超采样抗锯齿，菜单项通过 SetMenuItemBitmaps 显示（PNG 是 Wails
// 唯一接受的菜单图标格式）。
func GradientDot(top, base color.RGBA, size int) []byte {
	if size <= 0 || size > 64 {
		size = 16
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	radius := center - 0.5

	const samples = 4
	step := 1.0 / samples

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var sumR, sumG, sumB, hits float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := float64(x) + step*(float64(sx)+0.5)
					py := float64(y) + step*(float64(sy)+0.5)
					dx := px - center
					dy := py - center
					dist := math.Hypot(dx, dy)
					if dist > radius {
						continue
					}

					// 纵向光照：只有 10% 的明度差，够出体积又不油。
					c := mix(top, base, clamp01(py/float64(size)))

					// 顶部内缘 1px 细高光，像金属件的倒角。
					if rim := radius - dist; rim < 1.0 && dy < 0 {
						c = lighten(c, 0.22*(1.0-rim))
					}

					sumR += float64(c.R)
					sumG += float64(c.G)
					sumB += float64(c.B)
					hits++
				}
			}
			if hits == 0 {
				continue
			}
			coverage := hits / float64(samples*samples)
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(sumR / hits),
				G: uint8(sumG / hits),
				B: uint8(sumB / hits),
				A: uint8(255 * coverage),
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// mix 按比例 t（0=from，1=to）混合两个颜色。
func mix(from, to color.RGBA, t float64) color.RGBA {
	t = clamp01(t)
	return color.RGBA{
		R: uint8(float64(from.R)*(1-t) + float64(to.R)*t),
		G: uint8(float64(from.G)*(1-t) + float64(to.G)*t),
		B: uint8(float64(from.B)*(1-t) + float64(to.B)*t),
		A: 255,
	}
}

// lighten 按比例把颜色往白色提。
func lighten(c color.RGBA, amount float64) color.RGBA {
	return mix(c, color.RGBA{R: 255, G: 255, B: 255, A: 255}, clamp01(amount))
}

// clamp01 把值限制在 [0,1]。
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
