//go:build windows

package windows

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"time"

	"github.com/wengchengjian/skip-all/internal/platform/windows/wgc"
	"github.com/wengchengjian/skip-all/pkg/assist"
)

// wgcRetryAfter 是 WGC 失败后回退到 GDI 的冷却时间。
//
// 窗口被遮挡或最小化时 WGC 拿不到帧，这期间全走 GDI；过一段时间再试一次，
// 免得窗口一直不可见还每帧都白试一遍。
const wgcRetryAfter = 3 * time.Second

// wgcFrameTimeout 是单次取帧最多等多久。产品 10 FPS，帧通常早就堆在池子里，
// 这里只是给"窗口刚被遮挡"这种情况一个上限。
const wgcFrameTimeout = 30 * time.Millisecond

// WGCDevice 是"WGC 优先、GDI 回退"的截图设备。
//
// Windows Graphics Capture 走合成器输出的 GPU 纹理，1080p 一帧约 3.5ms；
// 但要求窗口显示在屏幕上，被遮挡或最小化时拿不到帧，那时回退 GDI（约 29ms）。
//
// WGC 的 COM 调用必须在初始化过 WinRT 的线程上，所以内部固定一个专用线程。
type WGCDevice struct {
	win  *Window
	gdi  *Capturer
	pool *BufferPool

	req       chan wgcRequest
	quit      chan struct{}
	closeOnce sync.Once

	mu            sync.Mutex
	disabledUntil time.Time
	lastErr       error
}

type wgcRequest struct {
	hwnd   uintptr
	width  int
	height int
	dst    *image.RGBA
	resp   chan error
}

// 编译期断言：WGCDevice 可直接作为 engine.Device + FrameDevice 使用。
var _ interface {
	Capture() (*image.RGBA, float64, error)
	CaptureFrame() (assist.Frame, error)
	IsActive() bool
	Activate() error
} = (*WGCDevice)(nil)

// NewWGCDevice 创建优先使用 WGC 的截图设备。
func NewWGCDevice(w *Window) *WGCDevice {
	d := &WGCDevice{
		win:  w,
		gdi:  NewCapturer(w),
		req:  make(chan wgcRequest),
		quit: make(chan struct{}),
	}
	go d.run()
	return d
}

// SetBufferPool 让截图器从池里取输出缓冲。
func (d *WGCDevice) SetBufferPool(pool *BufferPool) {
	d.pool = pool
	d.gdi.SetBufferPool(pool)
}

// Close 释放 WGC 会话与 GDI 资源。
func (d *WGCDevice) Close() error {
	d.closeOnce.Do(func() {
		close(d.quit)
		_ = d.gdi.Close()
	})
	return nil
}

// run 是 WGC 专用线程：固定 OS 线程、初始化 WinRT、持有会话。
func (d *WGCDevice) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := wgc.RoInitialize(); err != nil {
		// 初始化失败就永久回退：把后续请求全部当失败处理。
		d.setDisabled(err)
		for {
			select {
			case <-d.quit:
				return
			case req := <-d.req:
				req.resp <- err
			}
		}
	}
	defer wgc.RoUninitialize()

	var (
		sess    *wgc.Session
		sessKey uintptr
		sessW   int
		sessH   int
		scratch []byte
	)
	defer func() {
		if sess != nil {
			sess.Close()
		}
	}()

	for {
		select {
		case <-d.quit:
			return
		case req := <-d.req:
			// 窗口句柄或客户区尺寸变了都要重建会话：帧池是按尺寸建的，
			// 改过分辨率（比如切全屏）之后旧帧池给出的帧尺寸对不上。
			if sess != nil && (sessKey != req.hwnd || sessW != req.width || sessH != req.height) {
				sess.Close()
				sess = nil
			}
			if sess == nil {
				s, err := wgc.OpenSession(req.hwnd, req.width, req.height)
				if err != nil {
					req.resp <- err
					continue
				}
				sess, sessKey, sessW, sessH = s, req.hwnd, req.width, req.height
			}
			// WGC 截的是整窗（含标题栏/边框），只取客户区。
			// 不裁剪会把整幅画面压扁移位，后续所有识别都会读错位置。
			cropX, cropY, cropW, cropH, cerr := wgc.ClientArea(req.hwnd)
			if cerr != nil || cropW <= 0 || cropH <= 0 {
				cropX, cropY, cropW, cropH = 0, 0, req.width, req.height
			}
			need := cropW * cropH * 4
			if len(scratch) < need {
				scratch = make([]byte, need)
			}
			fw, fh, err := sess.Capture(scratch, cropX, cropY, cropW, cropH, wgcFrameTimeout)
			if err != nil {
				// 会话可能已失效（窗口重建），下次重新建。
				sess.Close()
				sess = nil
				req.resp <- err
				continue
			}
			if fw*fh*4 > len(scratch) {
				sess.Close()
				sess = nil
				req.resp <- fmt.Errorf("帧尺寸 %dx%d 超出缓冲", fw, fh)
				continue
			}
			scaleBGRAtoRGBAParallel(req.dst, scratch, fw, fh, resizeWorkers)
			req.resp <- nil
		}
	}
}

func (d *WGCDevice) setDisabled(err error) {
	d.mu.Lock()
	d.disabledUntil = time.Now().Add(wgcRetryAfter)
	d.lastErr = err
	d.mu.Unlock()
}

func (d *WGCDevice) disabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return time.Now().Before(d.disabledUntil)
}

// CaptureFrame 截取一帧：优先 WGC，失败或冷却中回退 GDI。
func (d *WGCDevice) CaptureFrame() (assist.Frame, error) {
	if d.win == nil || d.disabled() {
		return d.gdi.CaptureFrame()
	}
	rect, err := d.win.ClientRect()
	if err != nil || rect.Width() <= 0 || rect.Height() <= 0 {
		return d.gdi.CaptureFrame()
	}
	hwnd := uintptr(d.win.Handle())
	if hwnd == 0 {
		return d.gdi.CaptureFrame()
	}

	var buf *Buffer
	var dst *image.RGBA
	if d.pool != nil {
		buf = d.pool.Acquire()
		dst = buf.Image()
	} else {
		dst = image.NewRGBA(image.Rect(0, 0, standardWidth, standardHeight))
	}

	resp := make(chan error, 1)
	select {
	case d.req <- wgcRequest{hwnd: hwnd, width: rect.Width(), height: rect.Height(), dst: dst, resp: resp}:
	case <-d.quit:
		if buf != nil {
			buf.Release()
		}
		return d.gdi.CaptureFrame()
	}
	if err := <-resp; err != nil {
		d.setDisabled(err)
		if buf != nil {
			buf.Release()
		}
		return d.gdi.CaptureFrame()
	}

	scale := float64(rect.Width()) / standardWidth
	frame := assist.Frame{Image: dst, At: time.Now(), Scale: scale}
	if buf != nil {
		frame.Retain = buf.Retain
		frame.Release = buf.Release
	}
	return frame, nil
}

// Capture 返回 1920x1080 的 RGBA 图（engine.Device 接口）。
func (d *WGCDevice) Capture() (*image.RGBA, float64, error) {
	frame, err := d.CaptureFrame()
	if err != nil {
		return nil, 0, err
	}
	return frame.Image, frame.Scale, nil
}

// IsActive 报告窗口是否位于前台。
func (d *WGCDevice) IsActive() bool {
	return d != nil && d.win != nil && d.win.IsActive()
}

// Activate 把窗口带到前台。
func (d *WGCDevice) Activate() error {
	if d == nil || d.win == nil {
		return ErrWindowNotFound
	}
	return d.win.Activate()
}
