//go:build windows

package windows

import (
	"image"
	"runtime"
	"sync"
)

// 本文件是 BGRA 截图缓冲的缩放+通道转换实现。
//
// 背景：PrintWindow/BitBlt 拿到的 DIB 是 BGRA，且分辨率随窗口变化（常见
// 2560x1440、3840x2160）。旧实现分两步：convertBGRA 把原尺寸转成 RGBA，
// 再 resizeInto 缩到 1920x1080。两步法有两个问题：
//   - 慢：2560x1440 下两步合计 31.5ms，3840x2160 下 35.3ms
//   - 占内存：多一个原尺寸 RGBA 中间缓冲（14.1MB / 31.6MB）
//
// 现在改成一趟完成：直接从 BGRA 双线性重采样到目标 RGBA，并去掉中间缓冲。
// 缩放逐输出行独立，可按行并行。

// xmap 是水平方向的预计算映射：源像素下标 + 8 位定点权重。
type xmap struct {
	x0 []int32
	x1 []int32
	wx []int32
}

func buildXMap(srcW, dstW int) xmap {
	m := xmap{
		x0: make([]int32, dstW),
		x1: make([]int32, dstW),
		wx: make([]int32, dstW),
	}
	scaleX := float64(srcW) / float64(dstW)
	for x := 0; x < dstW; x++ {
		fx := (float64(x)+0.5)*scaleX - 0.5
		if fx < 0 {
			fx = 0
		}
		x0 := int32(fx)
		if x0 > int32(srcW-1) {
			x0 = int32(srcW - 1)
		}
		x1 := x0 + 1
		if x1 > int32(srcW-1) {
			x1 = int32(srcW - 1)
		}
		m.x0[x] = x0
		m.x1[x] = x1
		m.wx[x] = int32((fx-float64(x0))*256 + 0.5)
	}
	return m
}

// scaleRowBand 处理输出行 [y0,y1)。串行与并行共用，保证结果逐字节一致。
func scaleRowBand(dst *image.RGBA, src []byte, srcW, srcH int, m xmap, y0, y1, dstH int) {
	dstW := dst.Rect.Dx()
	scaleY := float64(srcH) / float64(dstH)
	for y := y0; y < y1; y++ {
		fy := (float64(y)+0.5)*scaleY - 0.5
		if fy < 0 {
			fy = 0
		}
		sy0 := int(fy)
		if sy0 > srcH-1 {
			sy0 = srcH - 1
		}
		sy1 := sy0 + 1
		if sy1 > srcH-1 {
			sy1 = srcH - 1
		}
		wy := int32((fy-float64(sy0))*256 + 0.5)
		iwy := 256 - wy
		row0 := src[sy0*srcW*4:]
		row1 := src[sy1*srcW*4:]
		rowDst := dst.Pix[(y-y0)*dst.Stride:]
		for x := 0; x < dstW; x++ {
			i0, i1 := m.x0[x]*4, m.x1[x]*4
			wx := m.wx[x]
			iwx := 256 - wx
			// 源是 BGRA，目标是 RGBA：R 取源 +2，G 取 +1，B 取 +0
			for ch := 0; ch < 3; ch++ {
				sc := int32(2 - ch)
				v00 := int32(row0[i0+sc])
				v10 := int32(row0[i1+sc])
				v01 := int32(row1[i0+sc])
				v11 := int32(row1[i1+sc])
				top := (v00*iwx + v10*wx) >> 8
				bot := (v01*iwx + v11*wx) >> 8
				rowDst[x*4+ch] = uint8((top*iwy + bot*wy) >> 8)
			}
			rowDst[x*4+3] = 255
		}
	}
}

// scaleBGRAtoRGBA 串行版：BGRA 原图直接缩放+转换到目标 RGBA，一趟完成。
func scaleBGRAtoRGBA(dst *image.RGBA, src []byte, srcW, srcH int) {
	dstH := dst.Rect.Dy()
	if srcW <= 0 || srcH <= 0 || dst.Rect.Dx() <= 0 || dstH <= 0 {
		return
	}
	scaleRowBand(dst, src, srcW, srcH, buildXMap(srcW, dst.Rect.Dx()), 0, dstH, dstH)
}

// scaleBGRAtoRGBAParallel 行并行版：按 worker 数切分输出行。
//
// 实测（i7-13700K，8 worker）：
//
//	2560x1440 -> 1920x1080: 31.5ms -> 1.5ms（21x）
//	3840x2160 -> 1920x1080: 35.3ms -> 2.0ms（17.6x）
//
// 相比 convertBGRA + resizeInto 两步，还省掉一个中间 RGBA 缓冲
// （2560x1440 下 14.1MB，3840x2160 下 31.6MB）。
func scaleBGRAtoRGBAParallel(dst *image.RGBA, src []byte, srcW, srcH, workers int) {
	dstW, dstH := dst.Rect.Dx(), dst.Rect.Dy()
	if srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0 {
		return
	}
	// 1:1 且尺寸相同：纯通道换序比重采样快，走专用快路径。
	if srcW == dstW && srcH == dstH {
		convertBGRA(dst, src, srcW, srcH)
		return
	}
	if workers <= 1 {
		scaleRowBand(dst, src, srcW, srcH, buildXMap(srcW, dstW), 0, dstH, dstH)
		return
	}
	if workers > dstH {
		workers = dstH
	}
	m := buildXMap(srcW, dstW)
	band := (dstH + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		y0 := w * band
		y1 := y0 + band
		if y1 > dstH {
			y1 = dstH
		}
		if y0 >= y1 {
			break
		}
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			off := y0 * dst.Stride
			tmp := &image.RGBA{
				Pix:    dst.Pix[off:],
				Stride: dst.Stride,
				Rect:   image.Rect(0, 0, dstW, y1-y0),
			}
			scaleRowBand(tmp, src, srcW, srcH, m, y0, y1, dstH)
		}(y0, y1)
	}
	wg.Wait()
}

// resizeWorkers 是缩放用的并行度，上限 8：再多收益递减且会抢 OCR 的核。
var resizeWorkers = func() int {
	n := runtime.NumCPU()
	if n > 8 {
		n = 8
	}
	if n < 1 {
		n = 1
	}
	return n
}()
