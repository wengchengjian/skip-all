//go:build windows

package wgc

import (
	"fmt"
	"unsafe"
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
