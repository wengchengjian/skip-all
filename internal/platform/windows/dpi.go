//go:build windows

package windows

import (
	"errors"
	"sync"

	"golang.org/x/sys/windows"
)

const (
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 等于 (DPI_AWARENESS_CONTEXT)-4。
	dpiAwarenessPerMonitorV2 = ^uintptr(3)
	defaultDPI               = 96
)

var (
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")

	dpiOnce sync.Once
	dpiErr  error
)

// EnableDPIAwareness 启用 Per-Monitor V2 DPI 感知，保证客户区坐标与物理像素一致。
// 会在第一次 Find 时自动调用；进程已设置过 DPI 感知时视为成功。
func EnableDPIAwareness() error {
	dpiOnce.Do(func() {
		if err := procSetProcessDpiAwarenessContext.Find(); err == nil {
			r, _, callErr := procSetProcessDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2)
			if r != 0 || errors.Is(callErr, windows.ERROR_ACCESS_DENIED) {
				return
			}
		}
		if err := procSetProcessDPIAware.Find(); err == nil {
			if r, _, _ := procSetProcessDPIAware.Call(); r != 0 {
				return
			}
		}
		dpiErr = errors.New("启用 DPI 感知失败")
	})
	return dpiErr
}

// DPIForWindow 返回窗口所在显示器的 DPI，读取失败时回退为 96。
func DPIForWindow(w *Window) uint32 {
	if w == nil {
		return defaultDPI
	}
	hwnd := w.Handle()
	if hwnd == 0 {
		return defaultDPI
	}
	r, _, _ := procGetDpiForWindow.Call(hwnd)
	if r == 0 {
		return defaultDPI
	}
	return uint32(r)
}

// ScaleFactor 返回窗口的 DPI 缩放比例，96 DPI 时为 1。
func ScaleFactor(w *Window) float64 {
	return float64(DPIForWindow(w)) / defaultDPI
}
