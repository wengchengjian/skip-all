//go:build windows

package wgc

import (
	"fmt"
	"testing"
	"unsafe"

	gamewin "github.com/wengchengjian/skip-all/internal/platform/windows"
)

const (
	// IGraphicsCaptureItemInterop 派生自 IUnknown（不是 IInspectable），
	// 所以 CreateForWindow 在 vtable 的 3 号位。
	iidGraphicsCaptureItemInterop = "3628E81B-3CAC-4C60-B7F4-23CE0E0C3356"
	iidGraphicsCaptureItem        = "79C3F95B-31F7-4EC2-A464-632EF5D30760"
	classNameGraphicsCaptureItem  = "Windows.Graphics.Capture.GraphicsCaptureItem"
)

// captureItem 为一个窗口创建 WGC 捕获项，并返回它和窗口客户区尺寸。
//
// 注意：IGraphicsCaptureItem::get_Size 这条路走不通——实测该槽写回的是一个指针
// 而不是 SizeInt32（2026-11 在 1920x1080 的鸣潮窗口上验证）。尺寸改从
// GetClientRect 取，后面帧纹理本身也会带尺寸。
func captureItem(hwnd uintptr) (comPtr, error) {
	factory, err := activationFactory(classNameGraphicsCaptureItem, iidGraphicsCaptureItemInterop)
	if err != nil {
		return comPtr{}, err
	}
	defer factory.release()

	itemIID, err := parseGUID(iidGraphicsCaptureItem)
	if err != nil {
		return comPtr{}, err
	}
	var item comPtr
	if _, err := factory.call(3, hwnd, uintptr(unsafe.Pointer(&itemIID)), uintptr(unsafe.Pointer(&item.p))); err != nil {
		return comPtr{}, fmt.Errorf("CreateForWindow: %w", err)
	}
	return item, nil
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

	w, err := gamewin.Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		w, err = gamewin.Find(nil, []string{"chrome.exe", "Taskmgr.exe"})
	}
	if err != nil {
		t.Skipf("没有可用窗口: %v", err)
	}
	rect, _ := w.ClientRect()
	t.Logf("目标窗口 hwnd=0x%X 客户区 %dx%d", w.Handle(), rect.Width(), rect.Height())

	item, err := captureItem(w.Handle())
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

	fmt.Printf("WGC|stage=create_for_window|hwnd=0x%X|w=%d|h=%d|runtimeclass_ok=1\n",
		w.Handle(), rect.Width(), rect.Height())
}
