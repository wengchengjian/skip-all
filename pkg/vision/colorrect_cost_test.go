//go:build windows

package vision

import (
	"image"
	"image/color"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

var benchRed = [2][3]uint8{{200, 40, 40}, {255, 110, 110}}

func benchImg() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 100; y < 108; y++ {
		for x := 800; x < 1000; x++ {
			img.SetRGBA(x, y, color.RGBA{220, 60, 60, 255})
		}
	}
	return img
}

func fullRect() geom.Rect {
	return geom.Rect{X1: 0, Y1: 0, X2: 1920, Y2: 1080}
}

// BenchmarkFindColorRectanglesFullScreen 量全屏扫描的代价（inCombat 每帧都调）。
func BenchmarkFindColorRectanglesFullScreen(b *testing.B) {
	img := benchImg()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FindColorRectangles(img, fullRect(), benchRed, 50, 5, 14, 0.95)
	}
}

// TestFindColorRectanglesCost 量单次调用耗时与分配量。
func TestFindColorRectanglesCost(t *testing.T) {
	img := benchImg()
	_ = FindColorRectangles(img, fullRect(), benchRed, 50, 5, 14, 0.95)

	best := time.Hour
	var allocBytes uint64
	for i := 0; i < 10; i++ {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		_ = FindColorRectangles(img, fullRect(), benchRed, 50, 5, 14, 0.95)
		d := time.Since(start)
		runtime.ReadMemStats(&after)
		if d < best {
			best = d
		}
		allocBytes = after.TotalAlloc - before.TotalAlloc
	}
	t.Logf("全屏 FindColorRectangles：%v/次，分配 %.1fMB/次", best, float64(allocBytes)/1048576)
}

// refFindColorRectangles 是优化前的参考实现，用于逐例对照。
func refFindColorRectangles(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8, minWidth, minHeight, maxHeight int, threshold float64) []geom.Rect {
	if img == nil {
		return nil
	}
	if threshold <= 0 {
		threshold = 0.95
	}
	bounds := img.Bounds()
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(bounds)
	if r.Empty() {
		return nil
	}
	w, h := r.Dx(), r.Dy()
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		row := img.Pix[(r.Min.Y+y-img.Rect.Min.Y)*img.Stride:]
		for x := 0; x < w; x++ {
			px := row[(r.Min.X+x-img.Rect.Min.X)*4:]
			if px[0] >= colorRange[0][0] && px[0] <= colorRange[1][0] &&
				px[1] >= colorRange[0][1] && px[1] <= colorRange[1][1] &&
				px[2] >= colorRange[0][2] && px[2] <= colorRange[1][2] {
				mask[y*w+x] = true
			}
		}
	}
	var out []geom.Rect
	seen := make([]bool, w*h)
	queue := make([]int, 0, 64)
	for start := 0; start < len(mask); start++ {
		if !mask[start] || seen[start] {
			continue
		}
		queue = queue[:0]
		queue = append(queue, start)
		seen[start] = true
		minX, minY, maxX, maxY, count := w, h, -1, -1, 0
		for len(queue) > 0 {
			idx := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			cx, cy := idx%w, idx/w
			count++
			if cx < minX {
				minX = cx
			}
			if cy < minY {
				minY = cy
			}
			if cx > maxX {
				maxX = cx
			}
			if cy > maxY {
				maxY = cy
			}
			if cx > 0 && mask[idx-1] && !seen[idx-1] {
				seen[idx-1] = true
				queue = append(queue, idx-1)
			}
			if cx < w-1 && mask[idx+1] && !seen[idx+1] {
				seen[idx+1] = true
				queue = append(queue, idx+1)
			}
			if cy > 0 && mask[idx-w] && !seen[idx-w] {
				seen[idx-w] = true
				queue = append(queue, idx-w)
			}
			if cy < h-1 && mask[idx+w] && !seen[idx+w] {
				seen[idx+w] = true
				queue = append(queue, idx+w)
			}
		}
		bw, bh := maxX-minX+1, maxY-minY+1
		if bw < minWidth || bh < minHeight {
			continue
		}
		if maxHeight > 0 && bh > maxHeight {
			continue
		}
		if float64(count)/float64(bw*bh) < threshold {
			continue
		}
		out = append(out, geom.Rect{X1: r.Min.X + minX, Y1: r.Min.Y + minY, X2: r.Min.X + maxX + 1, Y2: r.Min.Y + maxY + 1})
	}
	return out
}

// TestFindColorRectanglesEquivalence 用随机画面逐例对照优化前后结果。
// 这是"行为没变"的守卫：掩膜写入方式与代际标记都不该改变输出。
func TestFindColorRectanglesEquivalence(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	colors := []struct {
		name string
		rng  [2][3]uint8
	}{
		{"红", [2][3]uint8{{200, 40, 40}, {255, 110, 110}}},
		{"白", [2][3]uint8{{240, 240, 240}, {255, 255, 255}}},
	}
	for iter := 0; iter < 12; iter++ {
		w, h := 320+rng.Intn(200), 200+rng.Intn(150)
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		// 填基色
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i] = uint8(rng.Intn(60))
			img.Pix[i+1] = uint8(rng.Intn(60))
			img.Pix[i+2] = uint8(rng.Intn(60))
			img.Pix[i+3] = 255
		}
		// 随机撒几块实心矩形
		blocks := 1 + rng.Intn(5)
		for b := 0; b < blocks; b++ {
			bw := 5 + rng.Intn(120)
			bh := 3 + rng.Intn(40)
			x0 := rng.Intn(maxInt(1, w-bw))
			y0 := rng.Intn(maxInt(1, h-bh))
			c := colors[rng.Intn(len(colors))].rng[0]
			for y := y0; y < y0+bh && y < h; y++ {
				for x := x0; x < x0+bw && x < w; x++ {
					img.SetRGBA(x, y, color.RGBA{c[0], c[1], c[2], 255})
				}
			}
		}
		area := geom.Rect{X1: 0, Y1: 0, X2: w, Y2: h}
		for _, col := range colors {
			want := refFindColorRectangles(img, area, col.rng, 4, 3, 0, 0.9)
			got := FindColorRectangles(img, area, col.rng, 4, 3, 0, 0.9)
			if len(want) != len(got) {
				t.Fatalf("iter=%d 颜色=%s 个数 %d vs %d（want %v got %v）",
					iter, col.name, len(want), len(got), want, got)
			}
			for i := range want {
				if want[i] != got[i] {
					t.Fatalf("iter=%d 颜色=%s 第 %d 个不一致 %v vs %v", iter, col.name, i, want[i], got[i])
				}
			}
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
