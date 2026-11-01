//go:build windows

package windows

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")
)

var (
	procScreenToClient   = user32.NewProc("ScreenToClient")
	procGetWindowRect    = user32.NewProc("GetWindowRect")
	procEnumChildWindows = user32.NewProc("EnumChildWindows")
	procIsWindowVisible  = user32.NewProc("IsWindowVisible")
	procMapVirtualKey    = user32.NewProc("MapVirtualKeyW")
)

// winRect 对应 Win32 RECT。
type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// winPoint 对应 Win32 POINT。
type winPoint struct {
	X int32
	Y int32
}

// lastErr 把 Win32 的 GetLastError 转成可读错误。
func lastErr(name string) error {
	err := windows.GetLastError()
	if err != nil && !errors.Is(err, windows.ERROR_SUCCESS) {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("%s 调用失败", name)
}
