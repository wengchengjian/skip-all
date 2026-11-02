//go:build windows

package storyskip

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
	"golang.org/x/image/draw"
)

// realOCRRuntime 把剧情跳过的 OCR 换成真实 PP-OCR，其余识别仍走脚本。
type realOCRRuntime struct {
	*fakeRuntime
	ocr   vision.OCR
	calls int
}

func (r *realOCRRuntime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	r.calls++
	return vision.NewMatcher(r.ocr, nil).OCR(frame.Image, area, opts)
}

// TestStorySkipRealOCRCost 量剧情跳过接真实 OCR 后一帧的代价。
// 剧情跳过的 OCR 调用比战斗密集，区域也更大，是更可能的瓶颈。
func TestStorySkipRealOCRCost(t *testing.T) {
	dir, err := filepath.Abs("../../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCR(dir)
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()

	base := &fakeRuntime{screens: testScreens()}
	rt := &realOCRRuntime{fakeRuntime: base, ocr: engine}
	session, err := NewFeature(testSpec()).NewSession(rt, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	defer session.Close()

	// 一张真实剧情截图当固定输入，跑多帧看每帧代价。
	img := loadStoryImage(t)
	base2 := time.Now()
	var slowest time.Duration
	var total time.Duration
	const frames = 30
	for i := 0; i < frames; i++ {
		frame := assist.Frame{Image: img, At: base2.Add(time.Duration(i) * 200 * time.Millisecond)}
		start := time.Now()
		_, _, err := session.Step(nil, frame)
		cost := time.Since(start)
		total += cost
		if cost > slowest {
			slowest = cost
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d 帧共 %v，每帧 %v（最慢 %v），OCR 调用 %d 次",
		frames, total, total/frames, slowest, rt.calls)
}

func loadStoryImage(t *testing.T) *image.RGBA {
	t.Helper()
	paths := []string{
		"../../../.tmp/ok-ww/tests/images/21-9_quest_skip.png",
		"../../../.tmp/ok-ww/tests/images/find_confirm.png",
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		// 截图器会把任意分辨率拉伸成 1920x1080，这里照做。
		dst := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		src := img.Bounds()
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Over, nil)
		return dst
	}
	t.Skip("没有可用的 1920x1080 剧情截图")
	return nil
}

var _ = geom.Rect{}
