//go:build windows

package wuwa

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// TestFrameAllocRate 量每帧的堆分配量。
// 这是 GC 压力的直接来源，也是长跑 RSS 高的根因。
func TestFrameAllocRate(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCRWithOptions(dir, vision.PaddleOptions{})
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()

	matcher := vision.NewMatcher(engine, nil)
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	rt := liveRuntime{simRuntime: simRuntime{game: g}, matcher: matcher}
	feature := CombatFeature()
	session, err := feature.NewSession(rt, feature.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	img, err := loadRealScreenshot("all_cd_1080p.png")
	if err != nil {
		t.Skipf("缺少截图: %v", err)
	}
	base := time.Now()
	for i := 0; i < 20; i++ {
		session.Step(nil, assist.Frame{Image: img, At: base.Add(time.Duration(i) * 100 * time.Millisecond)})
	}

	// 稳态后量一段的分配
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	const n = 1000
	for i := 0; i < n; i++ {
		session.Step(nil, assist.Frame{Image: img, At: base.Add(time.Duration(i+20) * 100 * time.Millisecond)})
	}
	runtime.ReadMemStats(&after)
	total := after.TotalAlloc - before.TotalAlloc
	t.Logf("%d 帧共分配 %.2fMB，每帧 %.0fKB", n, float64(total)/1048576, float64(total)/float64(n)/1024)
	t.Logf("按 10Hz 折算：%.1fMB/s", float64(total)/float64(n)/1024*10/1024)
}
