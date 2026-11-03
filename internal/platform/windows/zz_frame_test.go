//go:build windows

package windows

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestDumpFrame 抓一帧存盘，供人眼核对 UI 布局与识别区域是否对得上。
func TestDumpFrame(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑")
	}
	w, err := Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到游戏窗口: %v", err)
	}
	rect, _ := w.ClientRect()
	c := NewCapturer(w)
	c.SetBufferPool(NewBufferPool(1920, 1080))
	defer c.Close()
	frame, err := c.CaptureFrame()
	if err != nil {
		t.Fatalf("截图失败: %v", err)
	}
	defer frame.Release()
	dir := filepath.Join(os.TempDir(), "framedump")
	_ = os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, "frame.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	_ = png.Encode(f, frame.Image)
	f.Close()
	fmt.Printf("FRAME|client=%dx%d|png=%s\n", rect.Width(), rect.Height(), p)
}
