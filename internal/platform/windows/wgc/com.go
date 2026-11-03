//go:build windows

// Package wgc 用纯 Go（无 cgo）实现 Windows Graphics Capture 的窗口单帧抓取。
//
// 动机：GDI 的屏幕回读被显示器刷新率量化（见 docs/perf-audit.md），1080p 一次
// 要 14ms；WGC+D3D11 走合成器输出的 GPU 纹理，不需要 CPU 回读。现成的 Go 实现
// go2tv.app/screencast 是 cgo + cppwinrt，与本项目 CGO_ENABLED=0 的发布链路冲突，
// 所以这里手工搭 COM 调用。
package wgc

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	combase = syscall.NewLazyDLL("combase.dll")

	procRoInitialize           = combase.NewProc("RoInitialize")
	procRoUninitialize         = combase.NewProc("RoUninitialize")
	procRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString    = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = combase.NewProc("WindowsDeleteString")
	procCoTaskMemFree          = combase.NewProc("CoTaskMemFree")
)

// HSTRING 是 WinRT 的字符串句柄。
type HSTRING uintptr

func newHString(s string) (HSTRING, error) {
	utf16, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h HSTRING
	hr, _, _ := procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)-1),
		uintptr(unsafe.Pointer(&h)),
	)
	if err := hresult(hr); err != nil {
		return 0, fmt.Errorf("WindowsCreateString: %w", err)
	}
	return h, nil
}

func deleteHString(h HSTRING) {
	if h != 0 {
		procWindowsDeleteString.Call(uintptr(h))
	}
}

// guid 是 Windows GUID。
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func parseGUID(s string) (g guid, err error) {
	hexDigit := func(c byte) (uint8, bool) {
		switch {
		case c >= '0' && c <= '9':
			return c - '0', true
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10, true
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10, true
		}
		return 0, false
	}
	var vals []uint8
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			continue
		}
		v, ok := hexDigit(s[i])
		if !ok {
			return g, fmt.Errorf("GUID 非法字符 %q", string(s[i]))
		}
		vals = append(vals, v)
	}
	if len(vals) != 32 {
		return g, fmt.Errorf("GUID 长度不对: %q", s)
	}
	g.Data1 = uint32(vals[0])<<28 | uint32(vals[1])<<24 | uint32(vals[2])<<20 | uint32(vals[3])<<16 |
		uint32(vals[4])<<12 | uint32(vals[5])<<8 | uint32(vals[6])<<4 | uint32(vals[7])
	g.Data2 = uint16(vals[8])<<12 | uint16(vals[9])<<8 | uint16(vals[10])<<4 | uint16(vals[11])
	g.Data3 = uint16(vals[12])<<12 | uint16(vals[13])<<8 | uint16(vals[14])<<4 | uint16(vals[15])
	for i := 0; i < 8; i++ {
		g.Data4[i] = vals[16+i*2]<<4 | vals[16+i*2+1]
	}
	return g, nil
}

// comPtr 是一个 COM/WinRT 接口指针（指向对象，对象首字段是 vtable 指针）。
type comPtr struct{ p unsafe.Pointer }

func (p comPtr) valid() bool { return p.p != nil }

// call 按 vtable 下标调用方法，this 作为第一个参数。
//
// 参数个数用固定分支而不是切片：uintptr(unsafe.Pointer) 若经过变量中转，
// go vet 的 unsafeptr 检查会报 possible misuse，直接进 syscall 才是它认的形态。
func (p comPtr) call(idx int, args ...uintptr) (uintptr, error) {
	if p.p == nil {
		return 0, fmt.Errorf("接口指针为空")
	}
	vtbl := *(*unsafe.Pointer)(p.p)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	if fn == 0 {
		return 0, fmt.Errorf("vtable[%d] 为空", idx)
	}
	var r uintptr
	switch len(args) {
	case 0:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p))
	case 1:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0])
	case 2:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1])
	case 3:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1], args[2])
	case 4:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1], args[2], args[3])
	case 5:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1], args[2], args[3], args[4])
	case 6:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1], args[2], args[3], args[4], args[5])
	case 7:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1], args[2], args[3], args[4], args[5], args[6])
	case 8:
		r, _, _ = syscall.SyscallN(fn, uintptr(p.p), args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7])
	default:
		return 0, fmt.Errorf("参数过多: %d", len(args))
	}
	return r, hresult(r)
}

func (p comPtr) release() {
	if p.p != nil {
		_, _ = p.call(2) // IUnknown::Release
	}
}

// RoInitialize / RoUninitialize 管理 WinRT 运行时（多线程套间）。
func RoInitialize() error {
	hr, _, _ := procRoInitialize.Call(1) // RO_INIT_MULTITHREADED
	if err := hresult(hr); err != nil {
		return fmt.Errorf("RoInitialize: %w", err)
	}
	return nil
}

func RoUninitialize() { procRoUninitialize.Call() }

// activationFactory 取某个 WinRT 类的激活工厂，并 QI 到指定接口。
func activationFactory(className, iidStr string) (comPtr, error) {
	h, err := newHString(className)
	if err != nil {
		return comPtr{}, err
	}
	defer deleteHString(h)
	id, err := parseGUID(iidStr)
	if err != nil {
		return comPtr{}, err
	}
	var out comPtr
	hr, _, _ := procRoGetActivationFactory.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&id)),
		uintptr(unsafe.Pointer(&out.p)),
	)
	if err := hresult(hr); err != nil {
		return comPtr{}, fmt.Errorf("RoGetActivationFactory(%s): %w", className, err)
	}
	return out, nil
}

// hresult 把 HRESULT 转成 error。
func hresult(hr uintptr) error {
	if int32(hr) >= 0 {
		return nil
	}
	return fmt.Errorf("HRESULT 0x%08X", uint32(hr))
}
