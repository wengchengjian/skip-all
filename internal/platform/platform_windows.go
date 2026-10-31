//go:build windows

// Package platform 是跨平台屏幕/输入层的门面，v0.1 只有 Windows 实现。
package platform

import (
	"errors"
	"image"

	win "github.com/wengchengjian/skip-all/internal/platform/windows"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// ErrUnsupported 表示当前平台暂不支持屏幕/输入自动化。
var ErrUnsupported = errors.New("当前平台暂不支持屏幕/输入自动化")

// InputOptions 控制输入方式。
type InputOptions struct {
	Background bool
	Gamepad    bool
	GamepadDLL string
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
}

// Input 是输入接口。
type Input interface {
	Click(geom.Point) error
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
	})
}

type unsupportedCapturer struct{}

func (unsupportedCapturer) Capture() (*image.RGBA, float64, error) { return nil, 0, ErrUnsupported }
func (unsupportedCapturer) IsActive() bool                         { return false }
func (unsupportedCapturer) Activate() error                        { return ErrUnsupported }
func (unsupportedCapturer) Close() error                           { return nil }

type unsupportedInput struct{ err error }

func (u unsupportedInput) Click(geom.Point) error { return u.err }
func (u unsupportedInput) ClickCurrent() error    { return u.err }
func (u unsupportedInput) KeyTap(string) error    { return u.err }
func (u unsupportedInput) ReleaseAll() error      { return nil }
func (u unsupportedInput) Close() error           { return nil }
func (u unsupportedInput) GamepadAvailable() bool { return false }
func (u unsupportedInput) GamepadError() error    { return u.err }
