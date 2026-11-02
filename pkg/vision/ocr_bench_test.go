//go:build windows

package vision

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkRealOCR 量真实 PP-OCR 的识别耗时（CPU）。用来判断 OCR 能不能放进 tick。
func BenchmarkRealOCR(b *testing.B) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		b.Fatal(err)
	}
	engine, err := NewPaddleOCR(dir)
	if err != nil {
		b.Skipf("加载 OCR 失败（可能缺运行库）: %v", err)
	}
	defer engine.Close()

	// 造一张 300x80 的图，画些白色矩形当"文字"。
	img := image.NewRGBA(image.Rect(0, 0, 300, 80))
	for i := 0; i < 8; i++ {
		x := 10 + i*35
		for y := 20; y < 60; y++ {
			for dx := 0; dx < 20; dx++ {
				img.SetRGBA(x+dx, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	// 预热一次，避免把模型加载算进读数。
	_, _ = engine.Recognize(img, OCROptions{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, _ = engine.Recognize(img, OCROptions{})
		b.Logf("第 %d 次: %v", i+1, time.Since(start))
	}
}
