package vision

import "image"

// pixelReader 是逐像素读 RGB 的快速访问器。
//
// image.Image.At() 每次返回 color.Color 接口：对 *image.RGBA 来说，
// 4 字节的 color.RGBA 会被装箱到堆上，实测**每个像素一次分配**
// （1100x300 的区域 33 万次、每帧 1.3MB）。这些函数都在每帧路径上，
// 分配量直接变成 GC 压力。
//
// 这里对最常见的 *image.RGBA 直接读 Pix，其它实现回退到 At。
type pixelReader struct {
	rgba *image.RGBA
	img  image.Image
}

func newPixelReader(img image.Image) pixelReader {
	if r, ok := img.(*image.RGBA); ok {
		return pixelReader{rgba: r}
	}
	return pixelReader{img: img}
}

// at 返回 (x,y) 的 8 位 RGB。调用方需保证坐标在图像边界内
// （各调用点都先用 Intersect 裁剪过）。
func (p pixelReader) at(x, y int) (uint8, uint8, uint8) {
	if p.rgba != nil {
		i := (y-p.rgba.Rect.Min.Y)*p.rgba.Stride + (x-p.rgba.Rect.Min.X)*4
		pix := p.rgba.Pix
		return pix[i], pix[i+1], pix[i+2]
	}
	r, g, b, _ := p.img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}
