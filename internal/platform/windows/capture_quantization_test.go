//go:build windows

package windows

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
	"unsafe"
)

// TestCaptureQuantization 证明屏幕回读的耗时被显示器刷新率量化。
//
// 做法：对 1x1 和 1920x1080 各做 120 次 BitBlt，看分布。
// 如果固定开销是"常数"，1x1 与 1080p 的 min 应该只差数据量；实测 1x1 的 p50
// 恰好等于一个刷新周期，说明每次回读都在等下一个合成帧。
//
// 这个测试的读数随机器变化（刷新率、显卡、是否核显），本身就是有用信息：
// 它解释了为什么"截图快不快"不能只看代码。
func TestCaptureQuantization(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑")
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		t.Fatal("GetDC 失败")
	}
	defer procReleaseDC.Call(0, screenDC)

	mk := func(w, h int) (uintptr, uintptr) {
		dc, _, _ := procCreateCompatibleDC.Call(screenDC)
		bih := bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(w), Height: int32(-h), Planes: 1, BitCount: 32}
		var bits unsafe.Pointer
		bmp, _, _ := procCreateDIBSection.Call(screenDC, uintptr(unsafe.Pointer(&bih)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
		procSelectObject.Call(dc, bmp)
		return dc, bmp
	}

	for _, sz := range [][2]int{{1, 1}, {1920, 1080}} {
		dc, bmp := mk(sz[0], sz[1])
		ds := make([]time.Duration, 0, 120)
		for i := 0; i < 120; i++ {
			start := time.Now()
			procBitBlt.Call(dc, 0, 0, uintptr(sz[0]), uintptr(sz[1]), screenDC, 0, 0, srcCopy)
			ds = append(ds, time.Since(start))
		}
		sort.Slice(ds, func(a, b int) bool { return ds[a] < ds[b] })
		fmt.Printf("QUANT|w=%d|h=%d|min_us=%d|p50_us=%d|p95_us=%d\n",
			sz[0], sz[1], ds[0].Microseconds(), ds[len(ds)/2].Microseconds(), ds[len(ds)*95/100].Microseconds())
		procDeleteObject.Call(bmp)
		procDeleteDC.Call(dc)
	}

	// 纯内存对照：同样的 8MB 在两块 DIB 之间搬，完全不碰屏幕。
	srcDC, srcBmp := mk(1920, 1080)
	defer procDeleteObject.Call(srcBmp)
	defer procDeleteDC.Call(srcDC)
	dstDC, dstBmp := mk(1920, 1080)
	defer procDeleteObject.Call(dstBmp)
	defer procDeleteDC.Call(dstDC)
	best := time.Hour
	for i := 0; i < 40; i++ {
		start := time.Now()
		procBitBlt.Call(dstDC, 0, 0, 1920, 1080, srcDC, 0, 0, srcCopy)
		if d := time.Since(start); d < best {
			best = d
		}
	}
	fmt.Printf("QUANT|w=1920|h=1080|mem_to_mem_us=%d\n", best.Microseconds())
}
