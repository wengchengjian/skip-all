//go:build windows

package wgc

import (
	"fmt"
	"testing"
	"unsafe"
)

// TestD3DDevice 验证能创建带 BGRA 支持的 D3D11 设备，并转成 WinRT 设备。
func TestD3DDevice(t *testing.T) {
	if err := RoInitialize(); err != nil {
		t.Fatalf("RoInitialize: %v", err)
	}
	defer RoUninitialize()

	dev, err := newD3DDevice()
	if err != nil {
		t.Fatalf("创建 D3D11 设备失败: %v", err)
	}
	defer dev.release()

	// 用 IInspectable::GetRuntimeClassName（4 号槽）确认 WinRT 设备对象有效。
	var hstr HSTRING
	if _, err := dev.winrt.call(4, uintptr(unsafe.Pointer(&hstr))); err != nil {
		t.Fatalf("WinRT 设备 GetRuntimeClassName: %v", err)
	}
	if hstr == 0 {
		t.Fatal("WinRT 设备 GetRuntimeClassName 返回空")
	}
	deleteHString(hstr)

	fmt.Printf("D3D|stage=device_ok|runtimeclass_ok=1\n")
}
