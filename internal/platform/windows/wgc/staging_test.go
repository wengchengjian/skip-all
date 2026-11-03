//go:build windows

package wgc

import "testing"

// TestStagingTextureReuse 钉住读回纹理的复用：同尺寸/格式连续取帧时只分配一次。
//
// 每拍 CreateTexture2D 会让 WDDM 的提交内存随拍数上涨（现场：18 分钟涨到 10GB，
// 触发 onnxruntime bad allocation 和系统级低内存）。这条测试防止退回每帧新建。
func TestStagingTextureReuse(t *testing.T) {
	if err := RoInitialize(); err != nil {
		t.Fatalf("RoInitialize: %v", err)
	}
	defer RoUninitialize()

	dev, err := newD3DDevice()
	if err != nil {
		t.Fatalf("创建 D3D11 设备失败: %v", err)
	}
	defer dev.release()

	desc := texture2DDesc{Width: 64, Height: 64, Format: pixelFormatBGRA8}
	first, err := dev.stagingTexture(desc)
	if err != nil {
		t.Fatalf("第一次取 staging: %v", err)
	}
	for i := 0; i < 10; i++ {
		again, err := dev.stagingTexture(desc)
		if err != nil {
			t.Fatalf("第 %d 次取 staging: %v", i+2, err)
		}
		if again.p != first.p {
			t.Fatal("相同尺寸/格式返回了不同的 staging 纹理")
		}
	}
	if dev.stagingCreates != 1 {
		t.Fatalf("staging 分配次数 = %d，期望 1", dev.stagingCreates)
	}

	// 尺寸变了必须重建，否则读回会按旧纹理的跨度解释新帧。
	changed := desc
	changed.Width = 128
	if _, err := dev.stagingTexture(changed); err != nil {
		t.Fatalf("尺寸变化后取 staging: %v", err)
	}
	if dev.stagingCreates != 2 {
		t.Fatalf("尺寸变化后 staging 分配次数 = %d，期望 2", dev.stagingCreates)
	}
}
