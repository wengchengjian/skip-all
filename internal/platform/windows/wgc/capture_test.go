//go:build windows

package wgc

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestCaptureFirstFrame 在真实游戏窗口上取到第一帧的 D3D11 纹理。
func TestCaptureFirstFrame(t *testing.T) {
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
	t.Logf("窗口客户区 %dx%d", cw, ch)

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

	start := time.Now()
	frame, tex, err := pool.nextFrame(3 * time.Second)
	if err != nil {
		t.Fatalf("nextFrame: %v", err)
	}
	defer frame.release()
	defer tex.release()

	fmt.Printf("WGC|stage=first_frame|wait_ms=%d|tex_ok=1\n", time.Since(start).Milliseconds())
}
