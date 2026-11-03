//go:build windows

package wgc

import (
	"fmt"
	"syscall"
	"testing"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")

// findWindow 按标题找一个可见窗口，返回 HWND 与客户区尺寸。
// 测试不 import internal/platform/windows，否则会与它 import wgc 形成环。
func findWindow(titles ...string) (uintptr, int, int, bool) {
	procFindWindowW := user32.NewProc("FindWindowW")
	procGetClientRect := user32.NewProc("GetClientRect")
	for _, title := range titles {
		p, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			continue
		}
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(p)))
		if hwnd == 0 {
			continue
		}
		var rect [4]int32
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect[0])))
		w, h := int(rect[2]-rect[0]), int(rect[3]-rect[1])
		if w > 0 && h > 0 {
			return hwnd, w, h, true
		}
	}
	return 0, 0, 0, false
}

// TestCreateForWindow 验证纯 Go 能通过 WinRT 给真实游戏窗口创建捕获项。
//
// 这是整条 WGC 路径的第一道关卡：RoGetActivationFactory ->
// IGraphicsCaptureItemInterop::CreateForWindow -> IInspectable::GetRuntimeClassName。
func TestCreateForWindow(t *testing.T) {
	if err := RoInitialize(); err != nil {
		t.Fatalf("RoInitialize: %v", err)
	}
	defer RoUninitialize()

	hwnd, cw, ch, ok := findWindow("鸣潮", "Wuthering Waves", "任务管理器")
	if !ok {
		t.Skip("没有可用窗口")
	}
	t.Logf("目标窗口 hwnd=0x%X 客户区 %dx%d", hwnd, cw, ch)

	item, err := captureItem(hwnd)
	if err != nil {
		t.Fatalf("创建捕获项失败: %v", err)
	}
	defer item.release()

	// 用 GetRuntimeClassName（IInspectable 的 4 号槽）验证 vtable 布局：
	// 它能返回一个非空 HSTRING，说明 0..5 的布局与预期一致，
	// 后面的 CreateForWindow 结果也确实是个 WinRT 对象。
	var hstr HSTRING
	if _, err := item.call(4, uintptr(unsafe.Pointer(&hstr))); err != nil {
		t.Fatalf("GetRuntimeClassName: %v", err)
	}
	if hstr == 0 {
		t.Fatal("GetRuntimeClassName 返回空 HSTRING")
	}
	deleteHString(hstr)

	fmt.Printf("WGC|stage=create_for_window|hwnd=0x%X|w=%d|h=%d|runtimeclass_ok=1\n", hwnd, cw, ch)
}
