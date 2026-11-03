//go:build windows

package wgc

import (
	"fmt"
	"sort"
	"testing"
	"time"

	gamewin "github.com/wengchengjian/skip-all/internal/platform/windows"
)

func pct(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	i := int(float64(len(ds)-1) * p)
	return ds[i]
}

func summarize(ds []time.Duration) (min, p50, p95 time.Duration) {
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(a, b int) bool { return s[a] < s[b] })
	return s[0], pct(s, 0.5), pct(s, 0.95)
}

// TestWGCvsGDI 在同一窗口上对比 WGC 与 GDI 的整帧取图耗时。
//
// WGC 拆成两段量：等帧（受内容更新与合成节拍影响）与读回（GPU->CPU）。
func TestWGCvsGDI(t *testing.T) {
	w, err := gamewin.Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到鸣潮窗口: %v", err)
	}
	rect, _ := w.ClientRect()
	t.Logf("窗口客户区 %dx%d", rect.Width(), rect.Height())

	// ---- GDI ----
	gdi := gamewin.NewCapturer(w)
	defer gdi.Close()
	var gdiTimes []time.Duration
	for i := 0; i < 40; i++ {
		start := time.Now()
		img, _, err := gdi.Capture()
		if err != nil {
			t.Fatalf("GDI Capture: %v", err)
		}
		_ = img
		gdiTimes = append(gdiTimes, time.Since(start))
	}
	gMin, gP50, gP95 := summarize(gdiTimes)
	fmt.Printf("CMP|kind=gdi|min_us=%d|p50_us=%d|p95_us=%d\n", gMin.Microseconds(), gP50.Microseconds(), gP95.Microseconds())

	// ---- WGC ----
	if err := RoInitialize(); err != nil {
		t.Fatalf("RoInitialize: %v", err)
	}
	defer RoUninitialize()

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

	var waitTimes, readTimes, totalTimes []time.Duration
	for i := 0; i < 40; i++ {
		start := time.Now()
		frame, tex, err := pool.nextFrame(2 * time.Second)
		if err != nil {
			t.Logf("第 %d 帧取失败: %v", i, err)
			break
		}
		waited := time.Since(start)

		readStart := time.Now()
		_, _, err = readBack(dev, tex)
		if err != nil {
			frame.release()
			tex.release()
			t.Fatalf("readBack: %v", err)
		}
		read := time.Since(readStart)
		frame.release()
		tex.release()

		waitTimes = append(waitTimes, waited)
		readTimes = append(readTimes, read)
		totalTimes = append(totalTimes, time.Since(start))
	}
	if len(totalTimes) == 0 {
		t.Fatal("没有取到任何 WGC 帧")
	}
	wMin, wP50, wP95 := summarize(waitTimes)
	rMin, rP50, rP95 := summarize(readTimes)
	tMin, tP50, tP95 := summarize(totalTimes)
	fmt.Printf("CMP|kind=wgc_wait|n=%d|min_us=%d|p50_us=%d|p95_us=%d\n", len(waitTimes), wMin.Microseconds(), wP50.Microseconds(), wP95.Microseconds())
	fmt.Printf("CMP|kind=wgc_readback|n=%d|min_us=%d|p50_us=%d|p95_us=%d\n", len(readTimes), rMin.Microseconds(), rP50.Microseconds(), rP95.Microseconds())
	fmt.Printf("CMP|kind=wgc_total|n=%d|min_us=%d|p50_us=%d|p95_us=%d\n", len(totalTimes), tMin.Microseconds(), tP50.Microseconds(), tP95.Microseconds())
}
