//go:build windows

package windows

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// StatusDot 生成一枚纯色圆形状态点的 PNG 字节流，用于右键菜单里的状态行。
// 菜单项用 SetMenuItemBitmaps 显示图片，PNG 是 Wails 唯一接受的格式。
func StatusDot(c color.RGBA, size int) []byte {
	if size <= 0 || size > 64 {
		size = 16
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	radius := center - 1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			dist := math.Hypot(dx, dy)
			alpha := 0.0
			switch {
			case dist <= radius-0.5:
				alpha = 1
			case dist <= radius+0.5:
				alpha = radius + 0.5 - dist
			}
			if alpha <= 0 {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: uint8(float64(c.A) * alpha)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
