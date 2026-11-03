//go:build windows

package storyskip

import (
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// staleFreshRuntime 模拟异步运行时：每次 OCR 都返回"更早一帧"的结果。
type staleFreshRuntime struct {
	*fakeRuntime
	mu    sync.Mutex
	calls int
	// staleBy 表示结果基于多久之前的帧。
	staleBy time.Duration
}

func (r *staleFreshRuntime) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *staleFreshRuntime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	blocks, _, err := r.OCRFresh(frame, area, opts)
	return blocks, err
}

func (r *staleFreshRuntime) OCRFresh(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, time.Time, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	blocks, err := r.fakeRuntime.OCR(frame, area, opts)
	// 返回"基于更早一帧"的时间，模拟异步滞后。
	return blocks, frame.At.Add(-r.staleBy), err
}

// TestOcrAreaDoesNotCacheStaleResult 验证 ocrArea 不会把"基于旧画面的结果"
// 记成"当前画面的结论"。
//
// 缓存判据是 areaHash(当前帧)。异步返回的 blocks 来自更早一帧，
// 若照样用当前帧哈希记账，画面变了也会命中缓存——拿着旧结论当新结果用。
func TestOcrAreaDoesNotCacheStaleResult(t *testing.T) {
	base := &fakeRuntime{screens: testScreens()}
	// 滞后 300ms，超过 staleResultTolerance(150ms)，属于"明显滞后"。
	rt := &staleFreshRuntime{fakeRuntime: base, staleBy: 300 * time.Millisecond}
	raw, err := NewFeature(testSpec()).NewSession(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := raw.(*Session)
	defer session.Close()

	area := screen.Area{Name: "测试区", Rect: geom.Rect{X1: 100, Y1: 100, X2: 400, Y2: 200}}
	now := time.Now()
	ttl := storyButtonCacheTTL

	// 第一帧：画面 A。
	imgA := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	fillRect(imgA, area.Rect, color.RGBA{255, 255, 255, 255})
	if _, err := session.ocrArea(assist.Frame{Image: imgA, At: now}, area, vision.OCROptions{}, ttl); err != nil {
		t.Fatal(err)
	}
	if got := rt.count(); got != 1 {
		t.Fatalf("首帧识别 %d 次，应当 1 次", got)
	}

	// 第二帧：同一画面 A，立刻再来。缓存允许命中（画面没变）。
	if _, err := session.ocrArea(assist.Frame{Image: imgA, At: now.Add(50 * time.Millisecond)}, area, vision.OCROptions{}, ttl); err != nil {
		t.Fatal(err)
	}
	callsAfterSame := rt.count()

	// 第三帧：画面变成 B（内容完全不同）。必须重新识别。
	imgB := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	fillRect(imgB, area.Rect, color.RGBA{10, 10, 10, 255})
	if _, err := session.ocrArea(assist.Frame{Image: imgB, At: now.Add(100 * time.Millisecond)}, area, vision.OCROptions{}, ttl); err != nil {
		t.Fatal(err)
	}
	callsAfterChange := rt.count()

	t.Logf("首帧后 %d 次；同画面重复后 %d 次；画面变了之后 %d 次",
		1, callsAfterSame, callsAfterChange)
	if callsAfterChange <= callsAfterSame {
		t.Fatalf("画面内容变了却没有重新识别（识别次数停在 %d），"+
			"说明旧画面的结论被当成新画面的结果缓存下来了", callsAfterChange)
	}
}

func fillRect(img *image.RGBA, r geom.Rect, c color.RGBA) {
	for y := r.Y1; y < r.Y2; y++ {
		for x := r.X1; x < r.X2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
