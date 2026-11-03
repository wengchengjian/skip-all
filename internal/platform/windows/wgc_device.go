//go:build windows

package windows

import (
	"fmt"
	"image"
	"log/slog"
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
	done      chan struct{}
	closeOnce sync.Once
	logger    *slog.Logger

	mu              sync.Mutex
	disabledUntil   time.Time
	lastErr         error
	lastDisabledLog time.Time
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
		win:    w,
		gdi:    NewCapturer(w),
		req:    make(chan wgcRequest),
		quit:   make(chan struct{}),
		done:   make(chan struct{}),
		logger: slog.Default(),
	}
	go d.run()
	return d
}

// SetLogger 设置日志出口；nil 时保持默认 logger。
func (d *WGCDevice) SetLogger(l *slog.Logger) {
	if l != nil {
		d.logger = l
	}
}

// SetBufferPool 让截图器从池里取输出缓冲。
func (d *WGCDevice) SetBufferPool(pool *BufferPool) {
	d.pool = pool
	d.gdi.SetBufferPool(pool)
}

// Close 释放 WGC 会话与 GDI 资源，并等专用线程收尾。
//
// 等待是必要的：调用方（engine 停止）紧接着可能启动新会话，
// 旧线程还持有设备/帧池时重开会短暂出现两份 native 资源。
func (d *WGCDevice) Close() error {
	d.closeOnce.Do(func() {
		close(d.quit)
		_ = d.gdi.Close()
	})
	<-d.done
	return nil
}

// run 是 WGC 专用线程：固定 OS 线程、初始化 WinRT、持有会话。
func (d *WGCDevice) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(d.done)

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

	// 设备复用：会话因窗口尺寸/句柄变化重建时只换帧池，不重建 D3D11 设备。
	dev, derr := wgc.NewDevice()
	if derr != nil {
		// 设备建不起来就永久回退，和 RoInitialize 失败同一处理。
		d.setDisabled(derr)
		for {
			select {
			case <-d.quit:
				return
			case req := <-d.req:
				req.resp <- derr
			}
		}
	}
	defer dev.Close()

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
				d.logger.Debug("WGC 会话重建",
					"old_hwnd", sessKey, "new_hwnd", req.hwnd,
					"old_w", sessW, "old_h", sessH,
					"new_w", req.width, "new_h", req.height)
				sess.Close()
				sess = nil
			}
			if sess == nil {
				s, err := wgc.OpenSessionWithDevice(dev, req.hwnd, req.width, req.height)
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
	// 取帧失败会每拍触发一次，日志节流到 30 秒一条，避免刷屏。
	logNow := time.Since(d.lastDisabledLog) >= 30*time.Second
	if logNow {
		d.lastDisabledLog = time.Now()
	}
	d.mu.Unlock()
	if logNow {
		d.logger.Warn("WGC 取帧失败，临时回退 GDI 截图", "err", err)
	}
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
