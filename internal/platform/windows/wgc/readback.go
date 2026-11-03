//go:build windows

package wgc

import (
	"fmt"
	"unsafe"
)

const (
	// D3D11_USAGE_STAGING
	usageStaging = 3
	// D3D11_CPU_ACCESS_READ
	cpuAccessRead = 0x20000
	// D3D11_MAP_READ
	mapRead = 1

	// ID3D11Texture2D 的 vtable：IUnknown(0..2) + ID3D11DeviceChild(3..6)
	// + ID3D11Resource(7..9) + GetDesc(10)
	idxTexture2DGetDesc = 10
	// ID3D11Device 的 vtable：IUnknown(0..2) + CreateBuffer(3) + CreateTexture1D(4)
	// + CreateTexture2D(5)
	idxDeviceCreateTexture2D = 5
	// ID3D11DeviceContext 的 vtable：IUnknown(0..2) + ID3D11DeviceChild(3..6)
	// + VS/PSSet*(7..13) + Map(14) + Unmap(15) ... + CopyResource(47)
	idxContextMap          = 14
	idxContextUnmap        = 15
	idxContextCopyResource = 47
)

// texture2DDesc 对应 D3D11_TEXTURE2D_DESC（40 字节）。
type texture2DDesc struct {
	Width          uint32
	Height         uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32
	SampleCount    uint32
	SampleQuality  uint32
	Usage          uint32
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

// mappedSubresource 对应 D3D11_MAPPED_SUBRESOURCE（16 字节）。
type mappedSubresource struct {
	Data       unsafe.Pointer
	RowPitch   uint32
	DepthPitch uint32
}

// getDesc 读纹理描述。
func getDesc(tex comPtr) (texture2DDesc, error) {
	var d texture2DDesc
	if _, err := tex.call(idxTexture2DGetDesc, uintptr(unsafe.Pointer(&d))); err != nil {
		return d, fmt.Errorf("ID3D11Texture2D::GetDesc: %w", err)
	}
	return d, nil
}

// copyTo 把 GPU 纹理读回并写进 dst（紧凑 BGRA，行跨度 = 宽*4）。
//
// 与 readBack 的区别是复用调用方的缓冲，不在每帧分配 8MB。
func (d *d3dDevice) copyTo(dst []byte, tex comPtr, cropX, cropY, cropW, cropH int) (int, int, error) {
	desc, err := getDesc(tex)
	if err != nil {
		return 0, 0, err
	}
	// 只取客户区：WGC 截的是整窗，标题栏和边框要去掉。
	if cropW <= 0 || cropH <= 0 {
		cropX, cropY, cropW, cropH = 0, 0, int(desc.Width), int(desc.Height)
	}
	if cropX < 0 || cropY < 0 || cropX+cropW > int(desc.Width) || cropY+cropH > int(desc.Height) {
		return 0, 0, fmt.Errorf("裁剪区越界: %d,%d %dx%d 超出纹理 %dx%d",
			cropX, cropY, cropW, cropH, desc.Width, desc.Height)
	}
	need := cropW * cropH * 4
	if len(dst) < need {
		return 0, 0, fmt.Errorf("目标缓冲太小: 需要 %d，实际 %d", need, len(dst))
	}
	staging := texture2DDesc{
		Width:          desc.Width,
		Height:         desc.Height,
		MipLevels:      1,
		ArraySize:      1,
		Format:         desc.Format,
		SampleCount:    1,
		Usage:          usageStaging,
		CPUAccessFlags: cpuAccessRead,
	}
	var stagingTex comPtr
	if _, err := d.raw.call(idxDeviceCreateTexture2D,
		uintptr(unsafe.Pointer(&staging)),
		0,
		uintptr(unsafe.Pointer(&stagingTex.p)),
	); err != nil {
		return 0, 0, fmt.Errorf("CreateTexture2D(staging): %w", err)
	}
	defer stagingTex.release()

	if _, err := d.context.call(idxContextCopyResource,
		uintptr(unsafe.Pointer(stagingTex.p)),
		uintptr(unsafe.Pointer(tex.p)),
	); err != nil {
		return 0, 0, fmt.Errorf("CopyResource: %w", err)
	}

	var mapped mappedSubresource
	if _, err := d.context.call(idxContextMap,
		uintptr(unsafe.Pointer(stagingTex.p)),
		0,
		uintptr(mapRead),
		0,
		uintptr(unsafe.Pointer(&mapped)),
	); err != nil {
		return 0, 0, fmt.Errorf("Map: %w", err)
	}
	defer d.context.call(idxContextUnmap, uintptr(unsafe.Pointer(stagingTex.p)), 0)

	rowBytes := cropW * 4
	pitch := int(mapped.RowPitch)
	src := unsafe.Slice((*byte)(mapped.Data), pitch*int(desc.Height))
	for y := 0; y < cropH; y++ {
		start := (cropY+y)*pitch + cropX*4
		copy(dst[y*rowBytes:(y+1)*rowBytes], src[start:start+rowBytes])
	}
	return cropW, cropH, nil
}

// readBack 把 GPU 纹理读回 CPU，返回 BGRA 字节与行跨度。
//
// 路径：建同尺寸 staging 纹理 -> CopyResource -> Map -> 拷贝 -> Unmap。
func readBack(dev *d3dDevice, tex comPtr) ([]byte, texture2DDesc, error) {
	desc, err := getDesc(tex)
	if err != nil {
		return nil, desc, err
	}
	// SampleCount 必须 >= 1；写 0 会让 CreateTexture2D 返回 E_INVALIDARG。
	staging := texture2DDesc{
		Width:          desc.Width,
		Height:         desc.Height,
		MipLevels:      1,
		ArraySize:      1,
		Format:         desc.Format,
		SampleCount:    1,
		Usage:          usageStaging,
		CPUAccessFlags: cpuAccessRead,
	}
	var stagingTex comPtr
	if _, err := dev.raw.call(idxDeviceCreateTexture2D,
		uintptr(unsafe.Pointer(&staging)),
		0,
		uintptr(unsafe.Pointer(&stagingTex.p)),
	); err != nil {
		return nil, desc, fmt.Errorf("CreateTexture2D(staging): %w", err)
	}
	defer stagingTex.release()

	if _, err := dev.context.call(idxContextCopyResource,
		uintptr(unsafe.Pointer(stagingTex.p)),
		uintptr(unsafe.Pointer(tex.p)),
	); err != nil {
		return nil, desc, fmt.Errorf("CopyResource: %w", err)
	}

	var mapped mappedSubresource
	if _, err := dev.context.call(idxContextMap,
		uintptr(unsafe.Pointer(stagingTex.p)),
		0,
		uintptr(mapRead),
		0,
		uintptr(unsafe.Pointer(&mapped)),
	); err != nil {
		return nil, desc, fmt.Errorf("Map: %w", err)
	}
	defer dev.context.call(idxContextUnmap, uintptr(unsafe.Pointer(stagingTex.p)), 0)

	rowBytes := int(desc.Width) * 4
	pix := make([]byte, rowBytes*int(desc.Height))
	src := unsafe.Slice((*byte)(mapped.Data), int(mapped.RowPitch)*int(desc.Height))
	for y := 0; y < int(desc.Height); y++ {
		copy(pix[y*rowBytes:(y+1)*rowBytes], src[y*int(mapped.RowPitch):])
	}
	return pix, desc, nil
}
