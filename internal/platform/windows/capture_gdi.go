//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"unsafe"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"golang.org/x/sys/windows"
)

const (
	standardWidth  = 1920
	standardHeight = 1080

	pwClientOnly        = 0x00000001
	pwRenderFullContent = 0x00000002
	pwFlags             = pwClientOnly | pwRenderFullContent
	srcCopy             = 0x00CC0020
	dibRGBColors        = 0

	// 连续失败达到该次数后，错误信息会带上累计次数。
	maxCaptureFailures = 3
)

var (
	procPrintWindow        = user32.NewProc("PrintWindow")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
)

// bitmapInfoHeader 对应 Win32 BITMAPINFOHEADER。
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// Capturer 截取窗口客户区并标准化为 1920x1080 的 RGBA 图。
// 内部复用 DIB Section 与设备上下文；非并发安全。
type Capturer struct {
	mu  sync.Mutex
	win *Window

	hwnd   windows.HWND
	width  int
	height int
	memDC  windows.Handle
	bmp    windows.Handle
	oldBmp windows.Handle
	bits   unsafe.Pointer
	raw    *image.RGBA

	failures int
}

// 编译期断言：Capturer 可直接作为 engine.Device 使用。
var _ interface {
	Capture() (*image.RGBA, float64, error)
	IsActive() bool
	Activate() error
} = (*Capturer)(nil)

// NewCapturer 创建窗口截图器。
func NewCapturer(w *Window) *Capturer {
	return &Capturer{win: w}
}

// Capture 截取窗口客户区，返回 1920x1080 的 RGBA 图。
// scale 是实际客户区宽度 / 1920，用于把逻辑坐标换算成客户区物理坐标。
// 优先使用 PrintWindow(PW_CLIENTONLY|PW_RENDERFULLCONTENT)，失败或全黑时回退屏幕 BitBlt。
func (c *Capturer) Capture() (*image.RGBA, float64, error) {
	if c == nil || c.win == nil {
		return nil, 0, ErrWindowNotFound
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	rect, err := c.win.ClientRect()
	if err != nil {
		c.releaseLocked()
		c.failures++
		return nil, 0, fmt.Errorf("截图前读取客户区失败: %w", err)
	}
	width, height := rect.Width(), rect.Height()
	if width <= 0 || height <= 0 {
		c.failures++
		return nil, 0, fmt.Errorf("客户区尺寸无效: %dx%d", width, height)
	}
	hwnd := windows.HWND(c.win.Handle())
	if hwnd == 0 {
		c.releaseLocked()
		c.failures++
		return nil, 0, ErrWindowNotFound
	}
	if err := c.ensureResources(hwnd, width, height); err != nil {
		c.failures++
		return nil, 0, err
	}

	printOK := c.printWindow()
	if !printOK || isBlackFrame(c.pixels()) {
		if err := c.bitBlt(rect); err != nil {
			c.failures++
			if printOK {
				err = fmt.Errorf("PrintWindow 返回全黑且 BitBlt 回退失败: %w", err)
			} else {
				err = fmt.Errorf("PrintWindow 失败且 BitBlt 回退失败: %w", err)
			}
			if c.failures >= maxCaptureFailures {
				err = fmt.Errorf("截图连续失败 %d 次: %w", c.failures, err)
			}
			return nil, 0, err
		}
	}
	c.failures = 0

	scale := float64(width) / standardWidth
	return c.toRGBA(width, height), scale, nil
}

// IsActive 报告窗口是否位于前台。
func (c *Capturer) IsActive() bool {
	return c != nil && c.win != nil && c.win.IsActive()
}

// Activate 激活窗口。
func (c *Capturer) Activate() error {
	if c == nil || c.win == nil {
		return ErrWindowNotFound
	}
	return c.win.Activate()
}

// Close 释放缓存的 GDI 资源。
func (c *Capturer) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseLocked()
	return nil
}

func (c *Capturer) ensureResources(hwnd windows.HWND, width, height int) error {
	if c.memDC != 0 && c.bmp != 0 && c.hwnd == hwnd && c.width == width && c.height == height {
		return nil
	}
	c.releaseLocked()

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return errors.New("获取屏幕 DC 失败")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return errors.New("创建兼容 DC 失败")
	}

	bih := bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(width),
		Height:      int32(-height), // 负数表示自顶向下的 DIB
		Planes:      1,
		BitCount:    32,
		Compression: 0, // BI_RGB
	}
	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(
		screenDC,
		uintptr(unsafe.Pointer(&bih)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if bmp == 0 || bits == nil {
		procDeleteDC.Call(memDC)
		return fmt.Errorf("创建 DIB Section 失败 (w=%d, h=%d): %w", width, height, lastErr("CreateDIBSection"))
	}

	old, _, _ := procSelectObject.Call(memDC, bmp)
	c.memDC = windows.Handle(memDC)
	c.bmp = windows.Handle(bmp)
	if old != 0 && old != ^uintptr(0) {
		c.oldBmp = windows.Handle(old)
	}
	c.bits = bits
	c.hwnd = hwnd
	c.width = width
	c.height = height
	return nil
}

func (c *Capturer) releaseLocked() {
	if c.memDC != 0 {
		if c.oldBmp != 0 {
			procSelectObject.Call(uintptr(c.memDC), uintptr(c.oldBmp))
			c.oldBmp = 0
		}
		if c.bmp != 0 {
			procDeleteObject.Call(uintptr(c.bmp))
			c.bmp = 0
		}
		procDeleteDC.Call(uintptr(c.memDC))
		c.memDC = 0
	}
	c.bits = nil
	c.raw = nil
	c.hwnd = 0
	c.width = 0
	c.height = 0
}

func (c *Capturer) printWindow() bool {
	r, _, _ := procPrintWindow.Call(uintptr(c.hwnd), uintptr(c.memDC), pwFlags)
	return r != 0
}

func (c *Capturer) bitBlt(rect geom.Rect) error {
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return errors.New("获取屏幕 DC 失败")
	}
	defer procReleaseDC.Call(0, screenDC)
	r, _, _ := procBitBlt.Call(
		uintptr(c.memDC),
		0, 0,
		uintptr(c.width), uintptr(c.height),
		screenDC,
		uintptr(rect.X1), uintptr(rect.Y1),
		srcCopy,
	)
	if r == 0 {
		return lastErr("BitBlt")
	}
	return nil
}

// pixels 返回 DIB Section 的像素内存（BGRA，自顶向下）。
func (c *Capturer) pixels() []byte {
	if c.bits == nil || c.width <= 0 || c.height <= 0 {
		return nil
	}
	return unsafe.Slice((*byte)(c.bits), c.width*c.height*4)
}

// toRGBA 把 DIB 内容转换成 1920x1080 的 RGBA 图。
func (c *Capturer) toRGBA(width, height int) *image.RGBA {
	src := c.pixels()
	if width == standardWidth && height == standardHeight {
		dst := image.NewRGBA(image.Rect(0, 0, standardWidth, standardHeight))
		convertBGRA(dst, src, width, height)
		return dst
	}
	if c.raw == nil || c.raw.Rect.Dx() != width || c.raw.Rect.Dy() != height {
		c.raw = image.NewRGBA(image.Rect(0, 0, width, height))
	}
	convertBGRA(c.raw, src, width, height)
	return resizeRGBA(c.raw, standardWidth, standardHeight)
}

// convertBGRA 把 BGRA 字节流写入 RGBA 图，并把 alpha 置为不透明。
func convertBGRA(dst *image.RGBA, src []byte, width, height int) {
	for y := 0; y < height; y++ {
		row := src[y*width*4:]
		rowDst := dst.Pix[y*dst.Stride:]
		for x := 0; x < width; x++ {
			i := x * 4
			rowDst[i] = row[i+2]   // R
			rowDst[i+1] = row[i+1] // G
			rowDst[i+2] = row[i]   // B
			rowDst[i+3] = 255      // A
		}
	}
}

// isBlackFrame 判断一整帧是否近似全黑。PrintWindow 对部分游戏会返回全黑图。
func isBlackFrame(pix []byte) bool {
	if len(pix) < 4 {
		return true
	}
	// 逐像素检查 RGB 分量，1920x1080 下约 200 万次比较，开销可以接受。
	for i := 0; i+3 < len(pix); i += 4 {
		if pix[i] > 8 || pix[i+1] > 8 || pix[i+2] > 8 {
			return false
		}
	}
	last := len(pix) - 4
	return pix[last] <= 8 && pix[last+1] <= 8 && pix[last+2] <= 8
}

// resizeRGBA 用双线性插值缩放图像，避免截图分辨率变化影响 OCR。
func resizeRGBA(src *image.RGBA, dstWidth, dstHeight int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	srcWidth, srcHeight := src.Rect.Dx(), src.Rect.Dy()
	if srcWidth <= 0 || srcHeight <= 0 || dstWidth <= 0 || dstHeight <= 0 {
		return dst
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
	return dst
}
