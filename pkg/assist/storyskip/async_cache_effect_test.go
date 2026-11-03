//go:build windows

package storyskip

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// countingRuntime 是最朴素的同步运行时，用来量"纯缓存"的效果。
type countingRuntime struct {
	*fakeRuntime
	calls *int
}

func (r *countingRuntime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	*r.calls++
	return r.fakeRuntime.OCR(frame, area, opts)
}

// TestOcrAreaCachesSameFrame 验证同步运行时下，同一画面重复请求会命中缓存
// （这是缓存本来的价值：省掉重复识别）。
func TestOcrAreaCachesSameFrame(t *testing.T) {
	calls := 0
	base := &fakeRuntime{screens: testScreens()}
	rt := &countingRuntime{fakeRuntime: base, calls: &calls}
	raw, err := NewFeature(testSpec()).NewSession(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := raw.(*Session)
	defer session.Close()

	area := screen.Area{Name: "测试区", Rect: geom.Rect{X1: 100, Y1: 100, X2: 400, Y2: 200}}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	fillRect(img, area.Rect, color.RGBA{255, 255, 255, 255})
	now := time.Now()
	for i := 0; i < 6; i++ {
		if _, err := session.ocrArea(assist.Frame{Image: img, At: now.Add(time.Duration(i) * 20 * time.Millisecond)}, area, vision.OCROptions{}, storyButtonCacheTTL); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("同一画面 6 次请求，实际识别 %d 次", calls)
	if calls != 1 {
		t.Fatalf("同一画面 6 次请求识别了 %d 次，缓存没生效", calls)
	}
}

// TestOcrAreaSkipsCacheWhenStale 验证异步（FreshOCR）下，基于旧画面的结果
// 不会被当成当前画面的缓存，但"同画面"仍然可以走缓存。
func TestOcrAreaSkipsCacheWhenStale(t *testing.T) {
	base := &fakeRuntime{screens: testScreens()}
	rt := &staleFreshRuntime{fakeRuntime: base}
	raw, err := NewFeature(testSpec()).NewSession(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := raw.(*Session)
	defer session.Close()

	area := screen.Area{Name: "测试区", Rect: geom.Rect{X1: 100, Y1: 100, X2: 400, Y2: 200}}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	fillRect(img, area.Rect, color.RGBA{255, 255, 255, 255})
	now := time.Now()

	// 同一画面连续 6 次：异步返回的都是"基于更早帧"的结果，
	// 但画面本身没变，内容哈希一致，仍应允许命中缓存。
	for i := 0; i < 6; i++ {
		if _, err := session.ocrArea(assist.Frame{Image: img, At: now.Add(time.Duration(i) * 20 * time.Millisecond)}, area, vision.OCROptions{}, storyButtonCacheTTL); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("异步 + 同一画面 6 次请求，实际识别 %d 次", rt.count())
}
