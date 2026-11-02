package vision

import (
	"image"
	"image/color"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestRealOCRCost 量真实 PP-OCR 的耗时，作为"OCR 能不能放进决策拍"的依据。
// 不是断言，只打印读数；缺运行库时跳过。
func TestRealOCRCost(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewPaddleOCR(dir)
	if err != nil {
		t.Skipf("加载 OCR 失败（可能缺运行库）: %v", err)
	}
	defer engine.Close()

	img := image.NewRGBA(image.Rect(0, 0, 300, 80))
	for i := 0; i < 8; i++ {
		x := 10 + i*35
		for y := 20; y < 60; y++ {
			for dx := 0; dx < 20; dx++ {
				img.SetRGBA(x+dx, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	_, _ = engine.Recognize(img, OCROptions{})
	for i := 0; i < 5; i++ {
		start := time.Now()
		_, _ = engine.Recognize(img, OCROptions{})
		t.Logf("300x80 识别第 %d 次: %v", i+1, time.Since(start))
	}

	// 并发 2 个 worker 同时识别，看是否线性加速。
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.Recognize(img, OCROptions{})
		}()
	}
	wg.Wait()
	t.Logf("4 次串行调用总耗时（含并发排队）: %v", time.Since(start))
}
