//go:build windows

package wuwa

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// asyncRealRuntime 把真实 OCR 包进异步运行时，量异步对战斗帧代价的影响。
type asyncRealRuntime struct {
	simRuntime
	async *assist.AsyncRuntime
	inner *realOCRInner
}

type realOCRInner struct {
	simRuntime
	ocr vision.OCR
}

func (r *realOCRInner) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	return vision.NewMatcher(r.ocr, nil).OCR(frame.Image, area, opts)
}

// TestCombatAsyncOCRFrameCost 对比同步与异步 OCR 下的战斗每帧代价。
func TestCombatAsyncOCRFrameCost(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCR(dir)
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()

	run := func(workers int) (perFrame, slowest time.Duration, calls int) {
		g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
		inner := &realOCRInner{simRuntime: simRuntime{game: g}, ocr: engine}
		var rt assist.Runtime = inner
		var async *assist.AsyncRuntime
		if workers > 1 {
			async = assist.NewAsyncRuntime(inner, workers)
			rt = async
		}
		defer func() {
			if async != nil {
				async.Close()
			}
		}()
		feature := CombatFeature()
		session, err := feature.NewSession(rt, feature.DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		spec := combatSpec()
		base := time.Now()
		const frames = 40
		start := time.Now()
		var worst time.Duration
		for i := 0; i < frames; i++ {
			frame := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
			f0 := time.Now()
			result, actions, err := session.Step(nil, frame)
			if c := time.Since(f0); c > worst {
				worst = c
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range actions {
				g.apply(a, result.State)
			}
		}
		return time.Since(start) / frames, worst, 0
	}

	syncAvg, syncWorst, _ := run(1)
	t.Logf("同步 OCR：每帧 %v，最慢 %v", syncAvg, syncWorst)

	asyncAvg, asyncWorst, _ := run(2)
	t.Logf("异步 OCR（2 worker）：每帧 %v，最慢 %v", asyncAvg, asyncWorst)
}
