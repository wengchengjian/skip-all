//go:build windows

// Package windows 提供绝区零窗口发现、GDI 截图和 SendInput 输入。
// 仅支持 Windows，调用方需要运行在交互式桌面会话中。
package windows

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"golang.org/x/sys/windows"
)

// ErrWindowNotFound 表示没有找到匹配的游戏窗口。
var ErrWindowNotFound = errors.New("未找到游戏窗口")

const (
	swShow    = 5 // SW_SHOW
	swRestore = 9 // SW_RESTORE
)

var (
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetClientRect        = user32.NewProc("GetClientRect")
	procClientToScreen       = user32.NewProc("ClientToScreen")
	procIsIconic             = user32.NewProc("IsIconic")
	procShowWindow           = user32.NewProc("ShowWindow")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop     = user32.NewProc("BringWindowToTop")
	procAttachThreadInput    = user32.NewProc("AttachThreadInput")
)

// Window 是一个已定位的游戏窗口。请通过 Find 获取，零值不可用。
type Window struct {
	mu           sync.Mutex
	hwnd         windows.HWND
	titles       []string
	processNames []string
	title        string
	processName  string
}

// Find 按窗口标题精确匹配查找游戏窗口。
// titles 是候选标题（如“绝区零”“ZenlessZoneZero”），processNames 是可选进程名过滤。
// 标题匹配优先；标题不匹配但进程名匹配时也会返回（用于标题变化的情况）。
func Find(titles []string, processNames []string) (*Window, error) {
	_ = EnableDPIAwareness()
	hwnd, title, proc, err := locate(titles, processNames)
	if err != nil {
		return nil, err
	}
	return &Window{
		hwnd:         hwnd,
		titles:       append([]string(nil), titles...),
		processNames: append([]string(nil), processNames...),
		title:        title,
		processName:  proc,
	}, nil
}

// locate 枚举顶层窗口，返回评分最高的一个。
func locate(titles, processNames []string) (windows.HWND, string, string, error) {
	if len(titles) == 0 && len(processNames) == 0 {
		return 0, "", "", errors.New("窗口标题与进程名不能同时为空")
	}

	var best windows.HWND
	var bestTitle, bestProcess string
	bestScore := 0

	cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		h := windows.HWND(hwnd)
		if !windows.IsWindowVisible(h) {
			return 1
		}
		title := windowText(h)
		if title == "" && len(titles) > 0 {
			return 1
		}
		var proc string
		if len(processNames) > 0 {
			proc = windowProcessName(h)
		}
		titleMatch := containsString(titles, title)
		procMatch := len(processNames) > 0 && proc != "" && containsFold(processNames, filepath.Base(proc))
		procOK := len(processNames) == 0 || proc == "" || procMatch
		score := 0
		switch {
		case titleMatch && procOK:
			score = 2
		case procMatch:
			score = 1
		}
		if score > bestScore {
			bestScore = score
			best = h
			bestTitle = title
			bestProcess = proc
		}
		return 1
	})

	if err := windows.EnumWindows(cb, nil); err != nil {
		return 0, "", "", fmt.Errorf("枚举窗口失败: %w", err)
	}
	if best == 0 {
		return 0, "", "", fmt.Errorf("%w: 标题=%v 进程=%v", ErrWindowNotFound, titles, processNames)
	}
	return best, bestTitle, bestProcess, nil
}

// Handle 返回当前窗口句柄，窗口未定位时返回 0。
func (w *Window) Handle() uintptr {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return uintptr(w.hwnd)
}

// Title 返回最近一次匹配到的窗口标题。
func (w *Window) Title() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.title
}

// ProcessName 返回最近一次匹配到的进程可执行文件路径。
func (w *Window) ProcessName() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.processName
}

// Refresh 清空缓存的句柄并重新查找窗口。
func (w *Window) Refresh() error {
	if w == nil {
		return ErrWindowNotFound
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hwnd = 0
	return w.refreshLocked()
}

func (w *Window) refreshLocked() error {
	hwnd, title, proc, err := locate(w.titles, w.processNames)
	if err != nil {
		w.hwnd = 0
		return err
	}
	w.hwnd = hwnd
	w.title = title
	w.processName = proc
	return nil
}

func (w *Window) ensureLocked() error {
	if w.hwnd != 0 && windows.IsWindow(w.hwnd) {
		return nil
	}
	return w.refreshLocked()
}

// IsValid 报告窗口是否仍然有效，句柄失效时会清除缓存并重新查找。
func (w *Window) IsValid() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.hwnd != 0 && windows.IsWindow(w.hwnd) {
		return true
	}
	return w.refreshLocked() == nil
}

// IsActive 报告窗口是否位于前台。
func (w *Window) IsActive() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.hwnd == 0 || !windows.IsWindow(w.hwnd) {
		return false
	}
	return windows.GetForegroundWindow() == w.hwnd
}

// Activate 恢复并激活窗口到前台。
func (w *Window) Activate() error {
	if w == nil {
		return ErrWindowNotFound
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ensureLocked(); err != nil {
		return err
	}
	if windows.GetForegroundWindow() == w.hwnd {
		return nil
	}
	if isIconic(w.hwnd) {
		procShowWindow.Call(uintptr(w.hwnd), swRestore)
	}
	procShowWindow.Call(uintptr(w.hwnd), swShow)
	if r, _, _ := procSetForegroundWindow.Call(uintptr(w.hwnd)); r != 0 {
		return nil
	}

	// SetForegroundWindow 受前台锁定限制，附加到当前前台线程后重试。
	if fg := windows.GetForegroundWindow(); fg != 0 && fg != w.hwnd {
		fgThread, _ := windows.GetWindowThreadProcessId(fg, nil)
		curThread := windows.GetCurrentThreadId()
		if fgThread != 0 && fgThread != curThread {
			procAttachThreadInput.Call(uintptr(curThread), uintptr(fgThread), 1)
			procBringWindowToTop.Call(uintptr(w.hwnd))
			procSetForegroundWindow.Call(uintptr(w.hwnd))
			procAttachThreadInput.Call(uintptr(curThread), uintptr(fgThread), 0)
		}
	}
	if windows.GetForegroundWindow() == w.hwnd {
		return nil
	}
	return fmt.Errorf("激活窗口失败: %w", lastErr("SetForegroundWindow"))
}

// ClientRect 返回客户区在屏幕上的矩形（左上闭、右下开）。
// 窗口最小化时会先尝试恢复，再读取客户区大小。
func (w *Window) ClientRect() (geom.Rect, error) {
	if w == nil {
		return geom.Rect{}, ErrWindowNotFound
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ensureLocked(); err != nil {
		return geom.Rect{}, err
	}

	var rc winRect
	if !callGetClientRect(w.hwnd, &rc) {
		w.hwnd = 0
		return geom.Rect{}, fmt.Errorf("读取客户区失败: %w", lastErr("GetClientRect"))
	}
	if rc.Right <= 0 || rc.Bottom <= 0 {
		if isIconic(w.hwnd) {
			procShowWindow.Call(uintptr(w.hwnd), swRestore)
			callGetClientRect(w.hwnd, &rc)
		}
	}
	if rc.Right <= 0 || rc.Bottom <= 0 {
		return geom.Rect{}, fmt.Errorf("窗口客户区为空: %dx%d", rc.Right, rc.Bottom)
	}

	origin := winPoint{}
	if r, _, _ := procClientToScreen.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&origin))); r == 0 {
		w.hwnd = 0
		return geom.Rect{}, fmt.Errorf("客户区坐标转换失败: %w", lastErr("ClientToScreen"))
	}
	return geom.Rect{
		X1: int(origin.X),
		Y1: int(origin.Y),
		X2: int(origin.X) + int(rc.Right),
		Y2: int(origin.Y) + int(rc.Bottom),
	}, nil
}

func callGetClientRect(h windows.HWND, rc *winRect) bool {
	r, _, _ := procGetClientRect.Call(uintptr(h), uintptr(unsafe.Pointer(rc)))
	return r != 0
}

func isIconic(h windows.HWND) bool {
	r, _, _ := procIsIconic.Call(uintptr(h))
	return r != 0
}

// windowText 读取窗口标题。
func windowText(h windows.HWND) string {
	n, _, _ := procGetWindowTextLengthW.Call(uintptr(h))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

// windowProcessName 读取窗口所属进程的可执行文件路径。
func windowProcessName(h windows.HWND) string {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(h, &pid); err != nil || pid == 0 {
		return ""
	}
	return processNameOf(pid)
}

func processNameOf(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}
