//go:build windows

package windows

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dxgi                   = windows.NewLazySystemDLL("dxgi.dll")
	procCreateDXGIFactory1 = dxgi.NewProc("CreateDXGIFactory1")
	iidIDXGIFactory1       = windows.GUID{Data1: 0x770aae78, Data2: 0xf26f, Data3: 0x4dba, Data4: [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}
	dxgiErrorNotFound      = uintptr(0x887A0002)
)

type dxgiAdapterDesc1 struct {
	Description           [128]uint16
	VendorID              uint32
	DeviceID              uint32
	SubSysID              uint32
	Revision              uint32
	DedicatedVideoMemory  uint64
	DedicatedSystemMemory uint64
	SharedSystemMemory    uint64
	AdapterLUID           windows.LUID
	Flags                 uint32
}

func comCall(obj uintptr, index int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	all := append([]uintptr{obj}, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

// DiscreteGPUIndex 返回独立显卡在 DXGI 枚举中的序号和名称；没有独显时返回 0。
// ONNX Runtime DirectML 的 device_id 使用同一个 DXGI adapter 枚举顺序。
func DiscreteGPUIndex() (int, string) {
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	defer windows.CoUninitialize()

	var factory uintptr
	hr, _, _ := procCreateDXGIFactory1.Call(uintptr(unsafe.Pointer(&iidIDXGIFactory1)), uintptr(unsafe.Pointer(&factory)))
	if hr != 0 || factory == 0 {
		return 0, ""
	}
	defer comCall(factory, 2)

	bestIndex, bestMemory := -1, uint64(0)
	bestName := ""
	for index := 0; index < 16; index++ {
		var adapter uintptr
		hr := comCall(factory, 12, uintptr(index), uintptr(unsafe.Pointer(&adapter)))
		if hr == dxgiErrorNotFound || adapter == 0 {
			break
		}
		var desc dxgiAdapterDesc1
		hr = comCall(adapter, 10, uintptr(unsafe.Pointer(&desc)))
		comCall(adapter, 2)
		if hr != 0 {
			continue
		}
		if desc.VendorID == 0x8086 || desc.DedicatedVideoMemory == 0 {
			continue
		}
		if int64(desc.DedicatedVideoMemory) > int64(bestMemory) {
			bestMemory = desc.DedicatedVideoMemory
			bestIndex = index
			bestName = windows.UTF16ToString(desc.Description[:])
		}
	}
	if bestIndex < 0 {
		return 0, ""
	}
	return bestIndex, fmt.Sprintf("%s (%d MB)", bestName, bestMemory/(1024*1024))
}
