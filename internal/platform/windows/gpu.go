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

// comCall 调 COM 接口的第 index 个方法。
// obj 用 unsafe.Pointer 承载而不是 uintptr：这样全程不出现 uintptr -> unsafe.Pointer 的转换，
// go vet 的 unsafeptr 检查才过得去。
// comObject 只用于读取 COM 对象首字段的 vtable 指针。
type comObject struct {
	vtable *[512]uintptr
}

func comCall(obj unsafe.Pointer, index int, args ...uintptr) uintptr {
	// COM 对象首字段是指向 vtable 的指针，不是 vtable 本身。先取 vtable，
	// 再按槽位取函数指针；否则读到的槽位全是对象数据，极易调用空指针。
	fn := (*comObject)(obj).vtable[index]
	all := append([]uintptr{uintptr(obj)}, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

// DiscreteGPUIndex 返回独立显卡在 DXGI 枚举中的序号和名称；没有独显时返回 0。
// ONNX Runtime DirectML 的 device_id 使用同一个 DXGI adapter 枚举顺序。
func DiscreteGPUIndex() (int, string) {
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	defer windows.CoUninitialize()

	var factory unsafe.Pointer
	hr, _, _ := procCreateDXGIFactory1.Call(uintptr(unsafe.Pointer(&iidIDXGIFactory1)), uintptr(unsafe.Pointer(&factory)))
	if hr != 0 || factory == nil {
		return 0, ""
	}
	defer comCall(factory, 2)

	bestIndex, bestMemory := -1, uint64(0)
	bestName := ""
	for index := 0; index < 16; index++ {
		var adapter unsafe.Pointer
		// IDXGIFactory::EnumerateAdapters 在 vtable 的第 7 个槽位（IUnknown 0-2，
		// IDXGIObject 3-6 之后）。此前误用了 12，会调用到错误函数指针并崩溃。
		hr := comCall(factory, 7, uintptr(index), uintptr(unsafe.Pointer(&adapter)))
		if hr == dxgiErrorNotFound || adapter == nil {
			break
		}
		var desc dxgiAdapterDesc1
		// IDXGIAdapter1::GetDesc1 在 vtable 的第 9 个槽位；第 10 个是较新的
		// IDXGIAdapter2::GetDesc2，不是所有适配器都支持。
		hr = comCall(adapter, 9, uintptr(unsafe.Pointer(&desc)))
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
