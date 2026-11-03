//go:build windows

package wuwa

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// TestProductionThreadsEndToEnd 用生产路径创建 OCR（不填 NumThreads），
// 对比改造前的单线程值，确认提速真实发生在真实截图上。
func TestProductionThreadsEndToEnd(t *testing.T) {
	t.Logf("本机 %d 核，自适应 %d 线程", runtime.NumCPU(), vision.DefaultNumThreads())
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	img, err := loadRealScreenshot("all_cd_1080p.png")
	if err != nil {
		t.Skipf("没有真实截图: %v", err)
	}
	rects := []geom.Rect{
		{X1: 1592, Y1: 938, X2: 1660, Y2: 995},
		{X1: 1685, Y1: 938, X2: 1753, Y2: 996},
		{X1: 1786, Y1: 938, X2: 1849, Y2: 995},
	}

	measure := func(label string, opts vision.PaddleOptions) time.Duration {
		engine, err := vision.NewPaddleOCRWithOptions(dir, opts)
		if err != nil {
			t.Fatalf("%s 创建失败: %v", label, err)
		}
		defer engine.Close()
		// 预热
		engine.Recognize(cropRegion(img, rects[0]), vision.OCROptions{})
		const rounds = 10
		const samples = 5
		var best time.Duration
		for s := 0; s < samples; s++ {
			start := time.Now()
			for i := 0; i < rounds; i++ {
				for _, r := range rects {
					engine.Recognize(cropRegion(img, r), vision.OCROptions{})
				}
			}
			c := time.Since(start) / rounds
			if s == 0 || c < best {
				best = c
			}
		}
		return best
	}

	old := measure("旧默认(1线程)", vision.PaddleOptions{NumThreads: 1})
	prod := measure("生产路径(零值)", vision.PaddleOptions{})
	t.Logf("旧默认 1 线程   %v", old)
	t.Logf("生产路径自适应  %v", prod)
	t.Logf("提速 %.2fx", float64(old)/float64(prod))
}
