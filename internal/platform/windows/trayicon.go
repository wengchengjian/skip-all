//go:build windows

package windows

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
)

// TrayIcon 生成一枚纯色圆形托盘图标（ICO 字节流），用于按运行状态换色。
// 用 DIB（32 位 BMP + AND 掩码）而不是 PNG 内嵌，兼容性最好。
func TrayIcon(c color.RGBA, size int) []byte {
	if size <= 0 || size > 256 {
		size = 32
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	radius := center - 1.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			dist := math.Hypot(dx, dy)
			// 边缘 1px 做抗锯齿，外圈描边略暗，缩小尺寸时仍有轮廓。
			alpha := 0.0
			switch {
			case dist <= radius-0.5:
				alpha = 1.0
			case dist <= radius+0.5:
				alpha = radius + 0.5 - dist
			}
			if alpha <= 0 {
				continue
			}
			fill := c
			if dist > radius-1.2 {
				fill = darker(c, 0.75)
			}
			img.SetRGBA(x, y, color.RGBA{R: fill.R, G: fill.G, B: fill.B, A: uint8(float64(fill.A) * alpha)})
		}
	}
	return encodeICO(img)
}

// darker 把颜色按比例压暗，用于描边。
func darker(c color.RGBA, scale float64) color.RGBA {
	return color.RGBA{R: uint8(float64(c.R) * scale), G: uint8(float64(c.G) * scale), B: uint8(float64(c.B) * scale), A: c.A}
}

// encodeICO 把单张 RGBA 图编码成单图像 ICO（32 位 DIB 条目）。
func encodeICO(img *image.RGBA) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	// XOR 位图：32bpp，行序自下而上。
	xor := make([]byte, 0, w*h*4)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			px := img.RGBAAt(x, y)
			// 预乘 alpha：BMP 的 32bpp 用 BGRA，且部分系统不做预乘会出现白边。
			premul := func(v uint8) uint8 {
				return uint8(uint16(v) * uint16(px.A) / 255)
			}
			xor = append(xor, premul(px.B), premul(px.G), premul(px.R), px.A)
		}
	}
	// AND 掩码：1bpp，全 0 表示不透明（透明度交给 alpha 通道），行按 4 字节对齐。
	maskRow := ((w + 31) / 32) * 4
	mask := make([]byte, maskRow*h)

	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)                // biSize
	binary.LittleEndian.PutUint32(header[4:], uint32(w))         // biWidth
	binary.LittleEndian.PutUint32(header[8:], uint32(h*2))       // biHeight（XOR + AND）
	binary.LittleEndian.PutUint16(header[12:], 1)                // biPlanes
	binary.LittleEndian.PutUint16(header[14:], 32)               // biBitCount
	binary.LittleEndian.PutUint32(header[20:], uint32(len(xor))) // biSizeImage

	image := append(append(append([]byte{}, header...), xor...), mask...)

	// ICONDIR + 单个 ICONDIRENTRY。
	var dir bytes.Buffer
	binary.Write(&dir, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&dir, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&dir, binary.LittleEndian, uint16(1)) // count
	dir.WriteByte(byte(w % 256))
	dir.WriteByte(byte(h % 256))
	dir.WriteByte(0)                                    // colorCount
	dir.WriteByte(0)                                    // reserved
	binary.Write(&dir, binary.LittleEndian, uint16(1))  // planes
	binary.Write(&dir, binary.LittleEndian, uint16(32)) // bitCount
	binary.Write(&dir, binary.LittleEndian, uint32(len(image)))
	binary.Write(&dir, binary.LittleEndian, uint32(6+16)) // 图像数据偏移

	out := append(dir.Bytes(), image...)
	return out
}
