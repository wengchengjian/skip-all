//go:build !windows

// Package platform 是跨平台屏幕/输入层的门面，v0.1 只有 Windows 实现。
package platform

import (
	"errors"
	"image"
	"image/color"

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
	SetBufferPool(*BufferPool)
	CaptureFrame() (assist.Frame, error)
}

// BufferPool 在非 Windows 平台是空实现。
type BufferPool struct{}

// NewBufferPool 在非 Windows 平台返回空池。
func NewBufferPool(int, int) *BufferPool { return &BufferPool{} }

// Input 是输入接口。
type Input interface {
	Click(geom.Point) error
	MiddleClick(geom.Point) error
	ClickHold(geom.Point, time.Duration) error
	Press(geom.Point) error
	Release() error
	ClickCurrent() error
	KeyTap(string) error
	ReleaseAll() error
	Close() error
	GamepadAvailable() bool
	GamepadError() error
}

// Find 在非 Windows 平台返回不支持错误。
func Find(titles, processNames []string) (Window, error) { return nil, ErrUnsupported }

// ProcessAlive 在非 Windows 平台不做进程门控。
func ProcessAlive([]string) bool { return true }

// ProcessUsage 在非 Windows 平台不统计性能占用。
func ProcessUsage() (cpuPercent, memMB float64) { return 0, 0 }

// ProcessPrivateMB 在非 Windows 平台不统计私有内存。
func ProcessPrivateMB() float64 { return 0 }

// TotalPhysicalMB 在非 Windows 平台不统计物理内存。
func TotalPhysicalMB() float64 { return 0 }

// Hotkey 是一条快捷键绑定（非 Windows 平台为空实现）。
type Hotkey struct {
	Accelerator string
	Callback    func()
}

// HotkeyBinding 是解析后的快捷键（非 Windows 平台不使用）。
type HotkeyBinding struct{ Source string }

// HotkeyManager 在非 Windows 平台是空实现。
type HotkeyManager struct{}

// NewHotkeyManager 在非 Windows 平台返回空实现。
func NewHotkeyManager(func() bool) *HotkeyManager { return &HotkeyManager{} }

// Start 在非 Windows 平台不做任何事。
func (*HotkeyManager) Start() {}

// Stop 在非 Windows 平台不做任何事。
func (*HotkeyManager) Stop() {}

// Set 在非 Windows 平台忽略全部绑定。
func (*HotkeyManager) Set([]Hotkey) []error { return nil }

// ParseAccelerator 在非 Windows 平台不解析快捷键。
func ParseAccelerator(s string) (*HotkeyBinding, error) { return &HotkeyBinding{Source: s}, nil }

// ForegroundProcessName 在非 Windows 平台返回空串。
func ForegroundProcessName() string { return "" }

// StatusDot 在非 Windows 平台返回空图标。
func StatusDot(from, to color.RGBA, size int) []byte { return nil }

// 提示音类型（非 Windows 平台不使用）。
const (
	BeepInfo    uint32 = 0
	BeepWarning uint32 = 0
	BeepError   uint32 = 0
)

// Beep 在非 Windows 平台不做任何事。
func Beep(uint32) {}

// NewCapturer 在非 Windows 平台返回占位截图器。
func NewCapturer(w Window) Capturer { return unsupportedCapturer{} }

// NewDevice 在非 Windows 平台同样返回占位截图器。
func NewDevice(w Window) Capturer { return unsupportedCapturer{} }

// NewInputWithOptions 在非 Windows 平台返回占位输入。
func NewInputWithOptions(w Window, opts InputOptions) Input { return unsupportedInput{} }

type unsupportedCapturer struct{}

func (unsupportedCapturer) Capture() (*image.RGBA, float64, error) { return nil, 0, ErrUnsupported }
func (unsupportedCapturer) IsActive() bool                         { return false }
func (unsupportedCapturer) Activate() error                        { return ErrUnsupported }
func (unsupportedCapturer) Close() error                           { return nil }
func (unsupportedCapturer) SetBufferPool(*BufferPool)              {}
func (unsupportedCapturer) CaptureFrame() (assist.Frame, error) {
	return assist.Frame{}, ErrUnsupported
}

type unsupportedInput struct{}

func (unsupportedInput) Click(geom.Point) error { return ErrUnsupported }
func (unsupportedInput) Press(geom.Point) error { return ErrUnsupported }
func (unsupportedInput) Release() error         { return ErrUnsupported }

func (unsupportedInput) MiddleClick(geom.Point) error { return ErrUnsupported }
func (unsupportedInput) ClickCurrent() error          { return ErrUnsupported }
func (unsupportedInput) KeyTap(string) error          { return ErrUnsupported }
func (unsupportedInput) ReleaseAll() error            { return nil }
func (unsupportedInput) Close() error                 { return nil }
func (unsupportedInput) GamepadAvailable() bool       { return false }
func (unsupportedInput) GamepadError() error          { return ErrUnsupported }
