//go:build windows

package wgc

import (
	"fmt"
	"testing"
	"time"

	gamewin "github.com/wengchengjian/skip-all/internal/platform/windows"
)

// TestCaptureFirstFrame 在真实鸣潮窗口上取到第一帧的 D3D11 纹理。
func TestCaptureFirstFrame(t *testing.T) {
	if err := RoInitialize(); err != nil {
		t.Fatal(err)
	}
	defer RoUninitialize()

	w, err := gamewin.Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到鸣潮窗口: %v", err)
	}
	rect, _ := w.ClientRect()
	t.Logf("窗口客户区 %dx%d", rect.Width(), rect.Height())

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

	start := time.Now()
	frame, tex, err := pool.nextFrame(3 * time.Second)
	if err != nil {
		t.Fatalf("nextFrame: %v", err)
	}
	defer frame.release()
	defer tex.release()

	fmt.Printf("WGC|stage=first_frame|wait_ms=%d|tex_ok=1\n", time.Since(start).Milliseconds())
}
