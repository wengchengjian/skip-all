//go:build !windows

// Package platform 是跨平台屏幕/输入层的门面，v0.1 只有 Windows 实现。
package platform

import (
	"errors"
	"image"

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
}

// Input 是输入接口。
type Input interface {
	Click(geom.Point) error
	MiddleClick(geom.Point) error
	ClickCurrent() error
	KeyTap(string) error
	ReleaseAll() error
	Close() error
	GamepadAvailable() bool
	GamepadError() error
}

// Find 在非 Windows 平台返回不支持错误。
func Find(titles, processNames []string) (Window, error) { return nil, ErrUnsupported }

// NewCapturer 在非 Windows 平台返回占位截图器。
func NewCapturer(w Window) Capturer { return unsupportedCapturer{} }

// NewInputWithOptions 在非 Windows 平台返回占位输入。
func NewInputWithOptions(w Window, opts InputOptions) Input { return unsupportedInput{} }

type unsupportedCapturer struct{}

func (unsupportedCapturer) Capture() (*image.RGBA, float64, error) { return nil, 0, ErrUnsupported }
func (unsupportedCapturer) IsActive() bool                         { return false }
func (unsupportedCapturer) Activate() error                        { return ErrUnsupported }
func (unsupportedCapturer) Close() error                           { return nil }

type unsupportedInput struct{}

func (unsupportedInput) Click(geom.Point) error { return ErrUnsupported }

func (unsupportedInput) MiddleClick(geom.Point) error { return ErrUnsupported }
func (unsupportedInput) ClickCurrent() error          { return ErrUnsupported }
func (unsupportedInput) KeyTap(string) error          { return ErrUnsupported }
func (unsupportedInput) ReleaseAll() error            { return nil }
func (unsupportedInput) Close() error                 { return nil }
func (unsupportedInput) GamepadAvailable() bool       { return false }
func (unsupportedInput) GamepadError() error          { return ErrUnsupported }
