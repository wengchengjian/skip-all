//go:build windows

package windows

import (
	"image"
	"testing"
	"time"
)

// BenchmarkConvertBGRA 量 1080p 一帧 BGRA->RGBA 的转换代价。
// 这段是纯 CPU 循环，每帧都要跑一次，是引擎单帧固定成本的一部分。
func BenchmarkConvertBGRA(b *testing.B) {
	width, height := 1920, 1080
	src := make([]byte, width*height*4)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		convertBGRA(dst, src, width, height)
	}
}

// TestCaptureFrameCost 量一次完整截图（含 BGRA 转换）的代价。
// 引擎每帧都做一次，是决策拍的下限。
func TestCaptureFrameCost(t *testing.T) {
	width, height := 1920, 1080
	src := make([]byte, width*height*4)
	for i := 0; i < len(src); i += 4 {
		src[i] = 40
		src[i+1] = 40
		src[i+2] = 40
		src[i+3] = 255
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	// 预热
	convertBGRA(dst, src, width, height)
	var total time.Duration
	const n = 10
	for i := 0; i < n; i++ {
		start := time.Now()
		convertBGRA(dst, src, width, height)
		total += time.Since(start)
	}
	t.Logf("1080p BGRA->RGBA 平均 %v", total/n)
}
