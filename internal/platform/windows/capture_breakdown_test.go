//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"os"
	"testing"
	"time"
)

// findBenchWindow 找一个真实窗口用于截图测量；没有就跳过。
//
// 截图路径（PrintWindow + 缩放）是整条帧管线里唯一没被基线覆盖的一段，
// 它只能对着真实 HWND 量。优先用环境变量 CAPTURE_WINDOW 指定进程名。
func findBenchWindow(tb testing.TB) *Window {
	tb.Helper()
	names := []string{"chrome.exe", "explorer.exe", "Taskmgr.exe", "wegame.exe"}
	if env := os.Getenv("CAPTURE_WINDOW"); env != "" {
		names = []string{env}
	}
	for _, n := range names {
		w, err := Find(nil, []string{n})
		if err == nil {
			if rect, err := w.ClientRect(); err == nil && rect.Width() > 0 && rect.Height() > 0 {
				return w
			}
		}
		if err != nil && !errors.Is(err, ErrWindowNotFound) {
			tb.Logf("%s: %v", n, err)
		}
	}
	tb.Skip("本机没有可用于截图测量的窗口")
	return nil
}

// TestCaptureBreakdown 把截图拆成 PrintWindow / BitBlt / 缩放三段分别计时。
//
// 目的是回答"截图到底贵在哪"：缩放已经做过并行优化（1080p 约 2ms），
// 但 PrintWindow 本身一直没量过。
func TestCaptureBreakdown(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑")
	}
	w := findBenchWindow(t)
	rect, err := w.ClientRect()
	if err != nil {
		t.Fatalf("ClientRect: %v", err)
	}
	c := NewCapturer(w)
	defer c.Close()

	dst := image.NewRGBA(image.Rect(0, 0, standardWidth, standardHeight))
	// 先跑一次把资源（DIB/DC）建好，否则第一次会把初始化算进去。
	if _, _, err := c.Capture(); err != nil {
		t.Fatalf("预热截图失败: %v", err)
	}

	width, height := rect.Width(), rect.Height()
	const n = 30
	best := func(fn func()) time.Duration {
		bestD := time.Hour
		for i := 0; i < n; i++ {
			start := time.Now()
			fn()
			if d := time.Since(start); d < bestD {
				bestD = d
			}
		}
		return bestD
	}

	c.mu.Lock()
	printOK := c.printWindow()
	black := isBlackFrame(c.pixels())
	c.mu.Unlock()

	printD := best(func() { c.mu.Lock(); c.printWindow(); c.mu.Unlock() })
	bltD := best(func() { c.mu.Lock(); _ = c.bitBlt(rect); c.mu.Unlock() })
	fillD := best(func() { c.fillRGBA(dst, width, height) })
	// 生产路径：engine 走池化的 CaptureFrame，不是每次新建 8.3MB 的 Capture。
	pool := NewBufferPool(standardWidth, standardHeight)
	c.SetBufferPool(pool)
	frameD := best(func() {
		f, err := c.CaptureFrame()
		if err != nil {
			t.Errorf("CaptureFrame: %v", err)
			return
		}
		f.Drop()
	})
	unpooledD := best(func() { _, _, _ = c.Capture() })

	t.Logf("窗口 %dx%d（scale %.2f）PrintWindow 可用=%v 首帧全黑=%v",
		width, height, float64(width)/standardWidth, printOK, black)
	fmt.Printf("CAPTURE|stage=printwindow|w=%d|h=%d|us=%d\n", width, height, printD.Microseconds())
	fmt.Printf("CAPTURE|stage=bitblt|w=%d|h=%d|us=%d\n", width, height, bltD.Microseconds())
	fmt.Printf("CAPTURE|stage=scale|w=%d|h=%d|us=%d\n", width, height, fillD.Microseconds())
	fmt.Printf("CAPTURE|stage=pooled_frame|w=%d|h=%d|us=%d\n", width, height, frameD.Microseconds())
	fmt.Printf("CAPTURE|stage=unpooled_capture|w=%d|h=%d|us=%d\n", width, height, unpooledD.Microseconds())
}
