//go:build windows

package wgc

import (
	"fmt"
	"os"
	"testing"
)

// TestReadBack 取一帧并读回 CPU，检查尺寸与像素内容是否合理。
func TestReadBack(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑真实截图测量")
	}
	if err := RoInitialize(); err != nil {
		t.Fatal(err)
	}
	defer RoUninitialize()

	hwnd, cw, ch, ok := findWindow("鸣潮", "Wuthering Waves", "任务管理器")
	if !ok {
		t.Skip("没有可用窗口")
	}

	item, err := captureItem(hwnd)
	if err != nil {
		t.Fatalf("captureItem: %v", err)
	}
	defer item.release()
	dev, err := newD3DDevice()
	if err != nil {
		t.Fatalf("d3dDevice: %v", err)
	}
	defer dev.release()
	pool, err := newFramePool(dev, item, int32(cw), int32(ch))
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

	pix, desc, err := readBack(dev, tex)
	if err != nil {
		t.Fatalf("readBack: %v", err)
	}
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
