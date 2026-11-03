//go:build windows

package windows

import (
	"image"
	"math"
	"testing"
	"time"
)

// TestParallelEquivalence 验证行并行版本与串行版本逐字节一致。
func TestParallelEquivalence(t *testing.T) {
	cases := []struct{ sw, sh int }{
		{2560, 1440}, {3840, 2160}, {1920, 1080}, {1600, 900}, {1280, 720},
	}
	for _, c := range cases {
		src := make([]byte, c.sw*c.sh*4)
		for y := 0; y < c.sh; y++ {
			for x := 0; x < c.sw; x++ {
				i := (y*c.sw + x) * 4
				src[i+0] = byte(x % 256)
				src[i+1] = byte(y % 256)
				src[i+2] = byte((x * 3) % 256)
				src[i+3] = 255
			}
		}
		serial := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		par := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		scaleBGRAtoRGBA(serial, src, c.sw, c.sh)
		scaleBGRAtoRGBAParallel(par, src, c.sw, c.sh, 8)
		diff := 0
		for i := range serial.Pix {
			if serial.Pix[i] != par.Pix[i] {
				diff++
			}
		}
		t.Logf("%-11s 并行与串行差异字节 %d", fmtRes(c.sw, c.sh), diff)
		if diff != 0 {
			t.Errorf("%dx%d 并行与串行不一致（%d 字节）", c.sw, c.sh, diff)
		}
	}
}

// TestResizeCostByResolution 量各分辨率下的提速（旧两步 vs 新一趟并行）。
func TestResizeCostByResolution(t *testing.T) {
	for _, c := range []struct{ sw, sh int }{
		{1920, 1080}, {2560, 1440}, {3840, 2160},
	} {
		src := make([]byte, c.sw*c.sh*4)
		raw := image.NewRGBA(image.Rect(0, 0, c.sw, c.sh))
		dst := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		bestOld, bestNew := time.Hour, time.Hour
		for i := 0; i < 8; i++ {
			s := time.Now()
			if c.sw == 1920 && c.sh == 1080 {
				convertBGRA(dst, src, c.sw, c.sh)
			} else {
				convertBGRA(raw, src, c.sw, c.sh)
				legacyResizeInto(dst, raw)
			}
			if d := time.Since(s); d < bestOld {
				bestOld = d
			}
			s = time.Now()
			scaleBGRAtoRGBAParallel(dst, src, c.sw, c.sh, resizeWorkers)
			if d := time.Since(s); d < bestNew {
				bestNew = d
			}
		}
		t.Logf("%-11s 旧 %-12v 新 %-12v 提速 %.1fx", fmtRes(c.sw, c.sh), bestOld, bestNew,
			float64(bestOld)/float64(bestNew))
	}
}

func fmtRes(w, h int) string {
	return itoa2(w) + "x" + itoa2(h)
}

func itoa2(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// legacyResizeInto 是优化前的实现，仅用于对照测试。
func legacyResizeInto(dst, src *image.RGBA) {
	dstWidth, dstHeight := dst.Rect.Dx(), dst.Rect.Dy()
	srcWidth, srcHeight := src.Rect.Dx(), src.Rect.Dy()
	if srcWidth <= 0 || srcHeight <= 0 || dstWidth <= 0 || dstHeight <= 0 {
		return
	}
	scaleX := float64(srcWidth) / float64(dstWidth)
	scaleY := float64(srcHeight) / float64(dstHeight)
	for y := 0; y < dstHeight; y++ {
		fy := (float64(y)+0.5)*scaleY - 0.5
		if fy < 0 {
			fy = 0
		}
		y0 := int(fy)
		if y0 > srcHeight-1 {
			y0 = srcHeight - 1
		}
		y1 := y0 + 1
		if y1 > srcHeight-1 {
			y1 = srcHeight - 1
		}
		wy := fy - float64(y0)
		if wy < 0 {
			wy = 0
		}
		rowDst := dst.Pix[y*dst.Stride:]
		row0 := src.Pix[y0*src.Stride:]
		row1 := src.Pix[y1*src.Stride:]
		for x := 0; x < dstWidth; x++ {
			fx := (float64(x)+0.5)*scaleX - 0.5
			if fx < 0 {
				fx = 0
			}
			x0 := int(fx)
			if x0 > srcWidth-1 {
				x0 = srcWidth - 1
			}
			x1 := x0 + 1
			if x1 > srcWidth-1 {
				x1 = srcWidth - 1
			}
			wx := fx - float64(x0)
			if wx < 0 {
				wx = 0
			}
			i00, i10 := x0*4, x1*4
			for ch := 0; ch < 4; ch++ {
				v00 := float64(row0[i00+ch])
				v10 := float64(row0[i10+ch])
				v01 := float64(row1[i00+ch])
				v11 := float64(row1[i10+ch])
				top := v00 + (v10-v00)*wx
				bottom := v01 + (v11-v01)*wx
				value := top + (bottom-top)*wy
				rowDst[x*4+ch] = uint8(math.Round(value))
			}
		}
	}
}
