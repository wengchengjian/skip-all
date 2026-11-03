//go:build windows

package wgc

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32dll          = syscall.NewLazyDLL("user32.dll")
	procGetWindowRect  = user32dll.NewProc("GetWindowRect")
	procGetClientRect  = user32dll.NewProc("GetClientRect")
	procClientToScreen = user32dll.NewProc("ClientToScreen")
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

// ClientArea 返回窗口客户区在"整窗画面"里的偏移与尺寸。
//
// WGC 的 CreateForWindow 截的是整个窗口（含标题栏和边框），而项目里所有
// 画面坐标都基于客户区。不做这一步裁剪，整幅画面会被压扁移位，
// 导致后续所有识别（队伍槽位、协奏环、目标模板）全部读错位置。
func ClientArea(hwnd uintptr) (x, y, w, h int, err error) {
	var wr, cr winRect
	if r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); r == 0 {
		return 0, 0, 0, 0, fmt.Errorf("GetWindowRect 失败")
	}
	if r, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr))); r == 0 {
		return 0, 0, 0, 0, fmt.Errorf("GetClientRect 失败")
	}
	// ClientToScreen 把客户区左上角换算成屏幕坐标。
	pt := [2]int32{0, 0}
	if r, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt[0]))); r == 0 {
		return 0, 0, 0, 0, fmt.Errorf("ClientToScreen 失败")
	}
	return int(pt[0] - wr.Left), int(pt[1] - wr.Top), int(cr.Right - cr.Left), int(cr.Bottom - cr.Top), nil
}
