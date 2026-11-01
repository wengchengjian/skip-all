//go:build windows

package windows

import (
	"syscall"
	"unsafe"
)

// clientToScreen 把窗口客户区坐标换算成屏幕坐标。
func clientToScreen(hwnd uintptr, p winPoint) (winPoint, error) {
	if r, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&p))); r == 0 {
		return winPoint{}, lastErr("ClientToScreen")
	}
	return p, nil
}

// screenToClient 把屏幕坐标换算成窗口客户区坐标。
func screenToClient(hwnd uintptr, p winPoint) (winPoint, error) {
	if r, _, _ := procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&p))); r == 0 {
		return winPoint{}, lastErr("ScreenToClient")
	}
	return p, nil
}

// targetWindowAt 返回屏幕坐标下最深的可见子窗口；没有子窗口覆盖该点时返回 top。
// 上游 ok-ww 的后台点击也是先按客户区换算屏幕坐标，再挑命中的子窗口投递消息，
// UE5 这类游戏真正收输入的是子窗口，投给顶层窗口会被忽略。
func targetWindowAt(top uintptr, screen winPoint) uintptr {
	target := top
	bestArea := int64(-1)
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
			return 1
		}
		var rect winRect
		if r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); r == 0 {
			return 1
		}
		if screen.X < rect.Left || screen.X >= rect.Right || screen.Y < rect.Top || screen.Y >= rect.Bottom {
			return 1
		}
		area := int64(rect.Right-rect.Left) * int64(rect.Bottom-rect.Top)
		if bestArea < 0 || area < bestArea {
			bestArea = area
			target = hwnd
		}
		return 1
	})
	_, _, _ = procEnumChildWindows.Call(top, callback, 0)
	return target
}

// keyLParam 按 Win32 要求填好扫描码与释放标志；缺扫描码时游戏往往直接忽略按键。
func keyLParam(vk uint16, up bool) uintptr {
	scan, _, _ := procMapVirtualKey.Call(uintptr(vk), 0) // MAPVK_VK_TO_VSC
	lparam := uintptr(1) | (scan&0xFF)<<16
	if scan&0xFF == 0 {
		lparam = 1
	}
	if up {
		lparam |= 1<<30 | 1<<31
	}
	return lparam
}
