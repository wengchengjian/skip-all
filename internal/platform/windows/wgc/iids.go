//go:build windows

package wgc

import (
	"fmt"
	"unsafe"
)

// getIids 调 IInspectable::GetIids（3 号槽）列出对象实现的所有接口 IID。
//
// 用来在运行时发现 WinRT statics 接口的 IID，避免把 GUID 硬编码进来写错。
func getIids(p comPtr) ([]guid, error) {
	var count uint32
	var arr *guid
	if _, err := p.call(3, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&arr))); err != nil {
		return nil, err
	}
	if count == 0 || arr == nil {
		return nil, nil
	}
	out := make([]guid, 0, count)
	for i := uint32(0); i < count; i++ {
		out = append(out, *(*guid)(unsafe.Add(unsafe.Pointer(arr), uintptr(i)*unsafe.Sizeof(guid{}))))
	}
	// GetIids 分配的内存由 CoTaskMemFree 释放。
	procCoTaskMemFree.Call(uintptr(unsafe.Pointer(arr)))
	return out, nil
}

func (g guid) String() string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}
