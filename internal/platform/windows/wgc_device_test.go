//go:build windows

package windows

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// requirePerf 让真实窗口的测量只在显式开启时跑。
//
// 这些测试会开真实截图会话、占 CPU，混在 go test ./... 里会干扰别的计时测试。
func requirePerf(t *testing.T) {
	t.Helper()
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑真实截图测量")
	}
}

func p50of(ds []time.Duration) (min, p50, p95 time.Duration) {
	if len(ds) == 0 {
		return 0, 0, 0
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(a, b int) bool { return s[a] < s[b] })
	return s[0], s[len(s)/2], s[len(s)*95/100]
}

// TestWGCDeviceAt10FPS 按产品真实节拍（10 FPS）量整帧耗时。
//
// 紧循环每帧都要等游戏出新帧（约 60 FPS -> 16.7ms），那不是产品的用法；
// 产品 100ms 才取一次，帧早就堆在池子里，取帧本身应当接近零等待。
func TestWGCDeviceAt10FPS(t *testing.T) {
	requirePerf(t)
	w, err := Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到游戏窗口: %v", err)
	}
	dev := NewWGCDevice(w)
	dev.SetBufferPool(NewBufferPool(1920, 1080))
	defer dev.Close()
	if frame, err := dev.CaptureFrame(); err == nil {
		frame.Release()
	}

	var ds []time.Duration
	for i := 0; i < 15; i++ {
		time.Sleep(100 * time.Millisecond)
		start := time.Now()
		frame, err := dev.CaptureFrame()
		if err != nil {
			t.Fatalf("第 %d 帧失败: %v", i, err)
		}
		frame.Release()
		ds = append(ds, time.Since(start))
	}
	min, p50, p95 := p50of(ds)
	fmt.Printf("DEV10|kind=wgc_device|n=%d|min_us=%d|p50_us=%d|p95_us=%d\n", len(ds), min.Microseconds(), p50.Microseconds(), p95.Microseconds())
}

// TestWGCDeviceVsGDI 对比"WGC 优先 + GDI 回退"设备与纯 GDI 的整帧耗时。
//
// 需要游戏窗口可见；找不到就跳过。
func TestWGCDeviceVsGDI(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过")
	}
	w, err := Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到游戏窗口: %v", err)
	}
	rect, _ := w.ClientRect()
	t.Logf("窗口客户区 %dx%d", rect.Width(), rect.Height())

	run := func(name string, dev interface {
		CaptureFrame() (assist.Frame, error)
	}) {
		var ds []time.Duration
		for i := 0; i < 30; i++ {
			start := time.Now()
			frame, err := dev.CaptureFrame()
			if err != nil {
				t.Fatalf("%s 第 %d 帧失败: %v", name, i, err)
			}
			frame.Release()
			ds = append(ds, time.Since(start))
		}
		min, p50, p95 := p50of(ds)
		fmt.Printf("DEV|kind=%s|min_us=%d|p50_us=%d|p95_us=%d\n", name, min.Microseconds(), p50.Microseconds(), p95.Microseconds())
	}

	gdi := NewCapturer(w)
	gdi.SetBufferPool(NewBufferPool(1920, 1080))
	defer gdi.Close()
	run("gdi", gdi)

	dev := NewWGCDevice(w)
	dev.SetBufferPool(NewBufferPool(1920, 1080))
	defer dev.Close()
	// 第一帧建会话，不计入统计。
	if frame, err := dev.CaptureFrame(); err == nil {
		frame.Release()
	}
	run("wgc_device", dev)
}

// TestWGCFallbackToGDI 找一个 WGC 拿不到帧的窗口，验证会自动回退 GDI 且不报错。
//
// 最小化/隐藏窗口是 WGC 的典型失败场景，而 GDI 的 PrintWindow 恰好能截这种窗口，
// 这正是保留回退的意义。候选窗口里没有能触发失败的，就跳过。
func TestWGCFallbackToGDI(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过")
	}
	candidates := []string{"wegame.exe", "pythonw.exe", "QQ.exe", "Taskmgr.exe"}
	for _, proc := range candidates {
		w, err := Find(nil, []string{proc})
		if err != nil {
			continue
		}
		dev := NewWGCDevice(w)
		dev.SetBufferPool(NewBufferPool(1920, 1080))

		frame, err := dev.CaptureFrame()
		if err == nil {
			frame.Release()
		}
		disabled := dev.disabled()
		dev.Close()
		if !disabled {
			continue // 这个窗口 WGC 能截，换个候选
		}

		// WGC 已判定失败，后续必须靠 GDI 连续出帧。
		dev2 := NewWGCDevice(w)
		dev2.SetBufferPool(NewBufferPool(1920, 1080))
		for i := 0; i < 5; i++ {
			f, err := dev2.CaptureFrame()
			if err != nil {
				dev2.Close()
				t.Fatalf("%s 第 %d 帧失败（回退没生效）: %v", proc, i, err)
			}
			if f.Image == nil || f.Image.Bounds().Dx() != standardWidth {
				dev2.Close()
				t.Fatalf("%s 第 %d 帧尺寸不对", proc, i)
			}
			f.Release()
		}
		dev2.Close()
		fmt.Printf("FALLBACK|proc=%s|frames=5|ok=1\n", proc)
		return
	}
	t.Skip("没有找到 WGC 会失败的窗口")
}
