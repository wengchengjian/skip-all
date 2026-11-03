//go:build windows

package wgc

import (
	"fmt"
	"testing"
	"time"

	gamewin "github.com/wengchengjian/skip-all/internal/platform/windows"
)

// TestWGCLatestFrame 模拟真实用法：产品跑 10 FPS，每拍取"当前最新帧"而不是等新帧。
//
// nextFrame 在紧循环里会阻塞到下一帧到达（受游戏自身帧率限制），
// 但产品 100ms 才取一次，帧早就堆在池子里了，取帧本身应当是零等待。
func TestWGCLatestFrame(t *testing.T) {
	w, err := gamewin.Find([]string{"鸣潮", "Wuthering Waves"}, []string{"Client-Win64-Shipping.exe"})
	if err != nil {
		t.Skipf("找不到鸣潮窗口: %v", err)
	}
	rect, _ := w.ClientRect()

	if err := RoInitialize(); err != nil {
		t.Fatal(err)
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

	var total, read []time.Duration
	miss := 0
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond) // 产品节拍：10 FPS
		start := time.Now()
		frame, tex, err := pool.nextFrame(50 * time.Millisecond)
		if err != nil {
			miss++
			continue
		}
		readStart := time.Now()
		if _, _, err := readBack(dev, tex); err != nil {
			frame.release()
			tex.release()
			t.Fatalf("readBack: %v", err)
		}
		read = append(read, time.Since(readStart))
		frame.release()
		tex.release()
		total = append(total, time.Since(start))
	}
	if len(total) == 0 {
		t.Fatalf("一次都没取到帧（miss=%d）", miss)
	}
	tMin, tP50, _ := summarize(total)
	rMin, rP50, _ := summarize(read)
	fmt.Printf("LATEST|n=%d|miss=%d|total_min_us=%d|total_p50_us=%d|read_min_us=%d|read_p50_us=%d\n",
		len(total), miss, tMin.Microseconds(), tP50.Microseconds(), rMin.Microseconds(), rP50.Microseconds())
}
