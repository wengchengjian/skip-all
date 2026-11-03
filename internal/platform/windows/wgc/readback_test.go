//go:build windows

package wgc

import (
	"fmt"
	"testing"

	gamewin "github.com/wengchengjian/skip-all/internal/platform/windows"
)

// TestReadBack 取一帧并读回 CPU，检查尺寸与像素内容是否合理。
func TestReadBack(t *testing.T) {
	if err := RoInitialize(); err != nil {
		t.Fatal(err)
	}
	defer RoUninitialize()

	w, err := gamewin.Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到鸣潮窗口: %v", err)
	}
	rect, _ := w.ClientRect()

	item, err := captureItem(w.Handle())
	if err != nil {
		t.Fatalf("captureItem: %v", err)
	}
	defer item.release()
	dev, err := newD3DDevice()
	if err != nil {
		t.Fatalf("d3dDevice: %v", err)
	}
	defer dev.release()
	pool, err := newFramePool(dev, item, int32(rect.Width()), int32(rect.Height()))
	if err != nil {
		t.Fatalf("framePool: %v", err)
	}
	defer pool.release()

	frame, tex, err := pool.nextFrame(3e9)
	if err != nil {
		t.Fatalf("nextFrame: %v", err)
	}
	defer frame.release()
	defer tex.release()

	d, err := getDesc(tex)
	if err != nil {
		t.Fatalf("getDesc: %v", err)
	}
	fmt.Printf("DESC|w=%d|h=%d|mip=%d|array=%d|format=%d|sample=%d/%d|usage=%d|bind=0x%X|cpu=0x%X|misc=0x%X\n",
		d.Width, d.Height, d.MipLevels, d.ArraySize, d.Format, d.SampleCount, d.SampleQuality, d.Usage, d.BindFlags, d.CPUAccessFlags, d.MiscFlags)

	pix, desc, err := readBack(dev, tex)
	if err != nil {
		t.Fatalf("readBack: %v", err)
	}
	// 统计非零字节，确认不是全黑。
	nonzero := 0
	for _, b := range pix {
		if b != 0 {
			nonzero++
		}
	}
	fmt.Printf("READBACK|w=%d|h=%d|format=%d|bytes=%d|nonzero_pct=%.1f\n",
		desc.Width, desc.Height, desc.Format, len(pix), float64(nonzero)*100/float64(len(pix)))
	if desc.Width == 0 || desc.Height == 0 {
		t.Fatal("纹理尺寸为 0")
	}
	if nonzero == 0 {
		t.Fatal("读回的像素全为 0")
	}
}
