//go:build windows

// Package platform 是跨平台屏幕/输入层的门面，v0.1 只有 Windows 实现。
package platform

import (
	"errors"
	"image"
	"image/color"
	"time"

	win "github.com/wengchengjian/skip-all/internal/platform/windows"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// ErrUnsupported 表示当前平台暂不支持屏幕/输入自动化。
var ErrUnsupported = errors.New("当前平台暂不支持屏幕/输入自动化")

// InputOptions 控制输入方式。
type InputOptions struct {
	Background bool
	Gamepad    bool
	GamepadDLL string
	CursorMode string
}

// Window 是游戏窗口的最小接口。
type Window interface {
	Title() string
	ProcessName() string
	IsValid() bool
}

// Capturer 是截图器接口。
type Capturer interface {
	Capture() (*image.RGBA, float64, error)
	IsActive() bool
	Activate() error
	Close() error
	// SetBufferPool 让截图直接写进池缓冲，异步识别 worker 可跨拍持有。
	SetBufferPool(*BufferPool)
	// CaptureFrame 返回带引用计数的帧（实现 engine.FrameDevice）。
	CaptureFrame() (assist.Frame, error)
}

// BufferPool 是帧缓冲池的类型别名。
type BufferPool = win.BufferPool

// NewBufferPool 创建帧缓冲池。
func NewBufferPool(width, height int) *BufferPool { return win.NewBufferPool(width, height) }

// Input 是输入接口。
type Input interface {
	Click(geom.Point) error
	MiddleClick(geom.Point) error
	ClickHold(geom.Point, time.Duration) error
	// Press 按下左键并保持，Release 抬起；用于按住期间不阻塞调用方。
	Press(geom.Point) error
	Release() error
	ClickCurrent() error
	KeyTap(string) error
	ReleaseAll() error
	Close() error
	GamepadAvailable() bool
	GamepadError() error
}

// Find 查找游戏窗口。
func Find(titles, processNames []string) (Window, error) {
	return win.Find(titles, processNames)
}

// ProcessAlive 报告游戏进程是否仍在运行。
func ProcessAlive(names []string) bool { return win.ProcessAlive(names) }

// ProcessUsage 返回本进程的 CPU 占用率（%）与工作集（MB），供托盘显示性能占用。
func ProcessUsage() (cpuPercent, memMB float64) { return win.ProcessUsage() }

// Hotkey 是一条快捷键绑定。
type Hotkey = win.Hotkey

// HotkeyBinding 是解析后的快捷键。
type HotkeyBinding = win.HotkeyBinding

// HotkeyManager 用低级键盘钩子实现全局快捷键：只有前台是目标进程时才拦截按键。
type HotkeyManager = win.HotkeyManager

// NewHotkeyManager 创建快捷键管理器；gate 返回 false 时快捷键不生效，按键透传。
func NewHotkeyManager(gate func() bool) *HotkeyManager { return win.NewHotkeyManager(gate) }

// ParseAccelerator 解析快捷键字符串，供配置校验。
func ParseAccelerator(s string) (*HotkeyBinding, error) { return win.ParseAccelerator(s) }

// ForegroundProcessName 返回前台窗口所属进程的文件名（小写）。
func ForegroundProcessName() string { return win.ForegroundProcessName() }

// StatusDot 生成带渐变的圆形状态点（PNG 字节流），给右键菜单项当图标。
// from 是高光色、to 是暗部色。
func StatusDot(from, to color.RGBA, size int) []byte {
	return win.GradientDot(from, to, size)
}

// 提示音类型。
const (
	BeepInfo    = win.BeepInfo
	BeepWarning = win.BeepWarning
	BeepError   = win.BeepError
)

// Beep 播放一次系统提示音。
func Beep(kind uint32) { win.Beep(kind) }

// NewCapturer 创建截图器。
func NewCapturer(w Window) Capturer {
	ww, ok := w.(*win.Window)
	if !ok || ww == nil {
		return unsupportedCapturer{}
	}
	return win.NewCapturer(ww)
}

// NewInputWithOptions 创建输入控制器。
func NewInputWithOptions(w Window, opts InputOptions) Input {
	ww, ok := w.(*win.Window)
	if !ok || ww == nil {
		return unsupportedInput{err: ErrUnsupported}
	}
	return win.NewInputWithOptions(ww, win.InputOptions{
		Background: opts.Background,
		Gamepad:    opts.Gamepad,
		GamepadDLL: opts.GamepadDLL,
		CursorMode: opts.CursorMode,
	})
}

type unsupportedCapturer struct{}

func (unsupportedCapturer) Capture() (*image.RGBA, float64, error) { return nil, 0, ErrUnsupported }
func (unsupportedCapturer) IsActive() bool                         { return false }
func (unsupportedCapturer) Activate() error                        { return ErrUnsupported }
func (unsupportedCapturer) Close() error                           { return nil }
func (unsupportedCapturer) SetBufferPool(*BufferPool)              {}
func (unsupportedCapturer) CaptureFrame() (assist.Frame, error) {
	return assist.Frame{}, ErrUnsupported
}

type unsupportedInput struct{ err error }

func (u unsupportedInput) Click(geom.Point) error { return u.err }
func (u unsupportedInput) Press(geom.Point) error { return u.err }
func (u unsupportedInput) Release() error         { return u.err }

func (u unsupportedInput) MiddleClick(geom.Point) error { return u.err }

func (u unsupportedInput) ClickHold(geom.Point, time.Duration) error { return u.err }
func (u unsupportedInput) ClickCurrent() error                       { return u.err }
func (u unsupportedInput) KeyTap(string) error                       { return u.err }
func (u unsupportedInput) ReleaseAll() error                         { return nil }
func (u unsupportedInput) Close() error                              { return nil }
func (u unsupportedInput) GamepadAvailable() bool                    { return false }
func (u unsupportedInput) GamepadError() error                       { return u.err }
