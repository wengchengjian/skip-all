//go:build windows

package wgc

import (
	"fmt"
	"time"
	"unsafe"
)

const (
	// IID_IDirect3D11CaptureFramePoolStatics（运行时 GetIids 查得）
	iidFramePoolStatics = "7784056A-67AA-4D53-AE54-1088D5A8CA21"
	// IGraphicsCaptureSession2，提供 put_IsBorderRequired
	iidGraphicsCaptureSession2 = "2C39AE40-7D2E-5044-804E-8B6799D4CF9E"
	// DirectXPixelFormat.B8G8R8A8UIntNormalized
	pixelFormatBGRA8 = 87
	// 帧池缓冲数
	framePoolBuffers = 2
)

// framePool 是 WGC 的帧池与捕获会话。
type framePool struct {
	pool    comPtr
	session comPtr
}

// newFramePool 为捕获项创建帧池并启动会话。
//
//	vtable（IDirect3D11CaptureFramePool，IInspectable 之后）：
//	  6 Recreate  7 TryGetNextFrame  8 add_FrameArrived  9 remove_FrameArrived
//	  10 CreateCaptureSession
//	IGraphicsCaptureSession：6 StartCapture
func newFramePool(dev *d3dDevice, item comPtr, width, height int32) (*framePool, error) {
	statics, err := activationFactory("Windows.Graphics.Capture.Direct3D11CaptureFramePool", iidFramePoolStatics)
	if err != nil {
		return nil, err
	}
	defer statics.release()

	// SizeInt32 是 8 字节结构体，x64 下按值占一个寄存器。
	size := uintptr(uint32(width)) | uintptr(uint32(height))<<32
	var pool comPtr
	if _, err := statics.call(6,
		uintptr(unsafe.Pointer(dev.winrt.p)),
		uintptr(pixelFormatBGRA8),
		uintptr(framePoolBuffers),
		size,
		uintptr(unsafe.Pointer(&pool.p)),
	); err != nil {
		return nil, fmt.Errorf("Direct3D11CaptureFramePool::Create: %w", err)
	}

	var session comPtr
	if _, err := pool.call(10,
		uintptr(unsafe.Pointer(item.p)),
		uintptr(unsafe.Pointer(&session.p)),
	); err != nil {
		pool.release()
		return nil, fmt.Errorf("CreateCaptureSession: %w", err)
	}
	// 关掉 WGC 默认画的黄色边框，否则用户会看到游戏窗口外一圈黄框。
	// 关不掉不是错误（旧系统没有 IGraphicsCaptureSession2），照常继续。
	disableBorder(session)

	// IGraphicsCaptureSession::StartCapture
	if _, err := session.call(6); err != nil {
		session.release()
		pool.release()
		return nil, fmt.Errorf("StartCapture: %w", err)
	}
	return &framePool{pool: pool, session: session}, nil
}

// disableBorder 关掉捕获时的黄色边框。
//
// IGraphicsCaptureSession2 派生自 IInspectable：
// 6 get_IsBorderRequired，7 put_IsBorderRequired。
func disableBorder(session comPtr) {
	id, err := parseGUID(iidGraphicsCaptureSession2)
	if err != nil {
		return
	}
	var s2 comPtr
	if _, err := session.call(0, uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&s2.p))); err != nil {
		return
	}
	defer s2.release()
	// VARIANT_BOOL：0 = false
	_, _ = s2.call(7, 0)
}

func (f *framePool) release() {
	if f == nil {
		return
	}
	f.session.release()
	f.pool.release()
}

// nextFrame 取下一帧（最多等 timeout）。返回帧对象与帧的纹理。
func (f *framePool) nextFrame(timeout time.Duration) (comPtr, comPtr, error) {
	deadline := time.Now().Add(timeout)
	for {
		var frame comPtr
		_, err := f.pool.call(7, uintptr(unsafe.Pointer(&frame.p))) // TryGetNextFrame
		if err != nil {
			return comPtr{}, comPtr{}, fmt.Errorf("TryGetNextFrame: %w", err)
		}
		if frame.valid() {
			// IDirect3D11CaptureFrame::get_Surface（6 号槽）
			var surface comPtr
			if _, err := frame.call(6, uintptr(unsafe.Pointer(&surface.p))); err != nil {
				frame.release()
				return comPtr{}, comPtr{}, fmt.Errorf("get_Surface: %w", err)
			}
			// IDirect3DSurface -> IDirect3DDxgiInterfaceAccess -> ID3D11Texture2D
			tex, err := dxgiTexture(surface)
			surface.release()
			if err != nil {
				frame.release()
				return comPtr{}, comPtr{}, err
			}
			return frame, tex, nil
		}
		if time.Now().After(deadline) {
			return comPtr{}, comPtr{}, fmt.Errorf("等待帧超时 %v", timeout)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// dxgiTexture 从 WinRT 的 IDirect3DSurface 取出底层 ID3D11Texture2D。
func dxgiTexture(surface comPtr) (comPtr, error) {
	accessIID, err := parseGUID(iidDxgiInterfaceAccess)
	if err != nil {
		return comPtr{}, err
	}
	var access comPtr
	if _, err := surface.call(0, uintptr(unsafe.Pointer(&accessIID)), uintptr(unsafe.Pointer(&access.p))); err != nil {
		return comPtr{}, fmt.Errorf("QueryInterface(IDirect3DDxgiInterfaceAccess): %w", err)
	}
	defer access.release()

	texIID, err := parseGUID(iidD3D11Texture2D)
	if err != nil {
		return comPtr{}, err
	}
	var tex comPtr
	if _, err := access.call(3, uintptr(unsafe.Pointer(&texIID)), uintptr(unsafe.Pointer(&tex.p))); err != nil {
		return comPtr{}, fmt.Errorf("GetInterface(ID3D11Texture2D): %w", err)
	}
	return tex, nil
}
