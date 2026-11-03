//go:build windows

package wgc

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	d3d11dll = syscall.NewLazyDLL("d3d11.dll")

	procD3D11CreateDevice                    = d3d11dll.NewProc("D3D11CreateDevice")
	procCreateDirect3D11DeviceFromDXGIDevice = d3d11dll.NewProc("CreateDirect3D11DeviceFromDXGIDevice")
)

const (
	// D3D_DRIVER_TYPE_HARDWARE
	d3dDriverTypeHardware = 1
	// D3D11_CREATE_DEVICE_BGRA_SUPPORT：WGC 要求设备支持 BGRA。
	d3d11CreateDeviceBGRASupport = 0x20
	// D3D11_SDK_VERSION
	d3d11SDKVersion = 7

	// IID_IDXGIDevice
	iidDXGIDevice = "54EC77FA-1377-44E6-8C32-88FD5F44C84C"
	// IID_ID3D11Texture2D
	iidD3D11Texture2D = "6F15AAF2-D208-4E89-9AB4-489535D34F9C"
	// IID_IDirect3DDxgiInterfaceAccess（Windows.Graphics.DirectX.Direct3D11）
	iidDxgiInterfaceAccess = "A9B3D012-3DF2-4EE3-B8D1-8695F457D3C1"
)

// d3dDevice 持有 D3D11 设备与 WinRT 侧的 IDirect3DDevice。
type d3dDevice struct {
	// raw 是 ID3D11Device*
	raw comPtr
	// context 是 ID3D11DeviceContext*
	context comPtr
	// winrt 是 Windows.Graphics.DirectX.Direct3D11.IDirect3DDevice*
	winrt comPtr

	// staging 是读回用的复用纹理。每拍 CreateTexture2D 会让 WDDM 的
	// 提交内存随拍数上涨（现场实测 18 分钟涨到 10GB），所以按尺寸/格式缓存，
	// 只有窗口改分辨率时才重建。
	staging       comPtr
	stagingW      uint32
	stagingH      uint32
	stagingFormat uint32
	// stagingCreates 记录分配次数，供测试确认稳态零分配。
	stagingCreates int
}

// newD3DDevice 创建带 BGRA 支持的硬件 D3D11 设备，并包装成 WinRT 设备。
//
// WGC 的 Direct3D11CaptureFramePool.Create 需要一个 WinRT 的 IDirect3DDevice，
// 它由 CreateDirect3D11DeviceFromDXGIDevice 从 DXGI 设备转换而来。
func newD3DDevice() (*d3dDevice, error) {
	var device, context comPtr
	var featureLevel uint32
	hr, _, _ := procD3D11CreateDevice.Call(
		0, // 默认适配器
		uintptr(d3dDriverTypeHardware),
		0, // 无软件光栅器
		uintptr(d3d11CreateDeviceBGRASupport),
		0, 0, // 默认 feature level 列表
		uintptr(d3d11SDKVersion),
		uintptr(unsafe.Pointer(&device.p)),
		uintptr(unsafe.Pointer(&featureLevel)),
		uintptr(unsafe.Pointer(&context.p)),
	)
	if err := hresult(hr); err != nil {
		return nil, fmt.Errorf("D3D11CreateDevice: %w", err)
	}

	// QI 出 IDXGIDevice 才能转 WinRT 设备。
	dxgiIID, err := parseGUID(iidDXGIDevice)
	if err != nil {
		return nil, err
	}
	var dxgiDevice comPtr
	if _, err := device.call(0, uintptr(unsafe.Pointer(&dxgiIID)), uintptr(unsafe.Pointer(&dxgiDevice.p))); err != nil {
		return nil, fmt.Errorf("QueryInterface(IDXGIDevice): %w", err)
	}
	defer dxgiDevice.release()

	var winrtDev comPtr
	hr, _, _ = procCreateDirect3D11DeviceFromDXGIDevice.Call(
		uintptr(unsafe.Pointer(dxgiDevice.p)),
		uintptr(unsafe.Pointer(&winrtDev.p)),
	)
	if err := hresult(hr); err != nil {
		return nil, fmt.Errorf("CreateDirect3D11DeviceFromDXGIDevice: %w", err)
	}

	return &d3dDevice{raw: device, context: context, winrt: winrtDev}, nil
}

func (d *d3dDevice) release() {
	if d == nil {
		return
	}
	d.staging.release()
	d.winrt.release()
	d.context.release()
	d.raw.release()
}

// Device 是一个可跨会话复用的 D3D11 设备。
//
// 窗口改分辨率或句柄变化时只需要重建帧池，不必连设备一起重建；
// 每建一次会话就新建一个设备会让 WDDM 资源随重建次数累积。
//
// 只能在调用过 RoInitialize 的线程上使用，且同一时刻只供一个会话取帧
// （读回缓冲是设备级复用的）。
type Device struct{ dev *d3dDevice }

// NewDevice 创建带 BGRA 支持的 D3D11 设备。
func NewDevice() (*Device, error) {
	dev, err := newD3DDevice()
	if err != nil {
		return nil, err
	}
	return &Device{dev: dev}, nil
}

// Close 释放设备及其缓存的读回纹理。
func (d *Device) Close() {
	if d == nil || d.dev == nil {
		return
	}
	d.dev.release()
	d.dev = nil
}
