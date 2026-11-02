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

// realOCRRuntime 用真实 PP-OCR 替换模拟运行时的 OCR，
// 其余识别（模板/颜色）仍由模拟对局提供，量的是线上真实的每帧代价。
type realOCRRuntime struct {
	simRuntime
	ocr   vision.OCR
	calls int
}

func (r *realOCRRuntime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	r.calls++
	// 走真实视觉服务：裁剪、颜色过滤、缓存都由它负责，和线上一致。
	return vision.NewMatcher(r.ocr, nil).OCR(frame.Image, area, opts)
}

// TestCombatRealOCRFrameCost 量"接真实 OCR 后"战斗会话一帧的代价。
// 之前用固定 130ms 的桩，量不到自适应检测边长带来的收益。
func TestCombatRealOCRFrameCost(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCR(dir)
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()

	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	rt := &realOCRRuntime{simRuntime: simRuntime{game: g}, ocr: engine}
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
	var slowest time.Duration
	for i := 0; i < frames; i++ {
		frame := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
		frameStart := time.Now()
		result, actions, err := session.Step(nil, frame)
		if cost := time.Since(frameStart); cost > slowest {
			slowest = cost
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			g.apply(a, result.State)
		}
	}
	elapsed := time.Since(start)
	perFrame := elapsed / frames
	t.Logf("%d 帧共 %v，每帧 %v（最慢 %v），OCR 调用 %d 次", frames, elapsed, perFrame, slowest, rt.calls)
	if perFrame > 100*time.Millisecond {
		t.Fatalf("每帧 %v 超过 10Hz 的 100ms 预算", perFrame)
	}
	// 均值达标不够：决策拍是"最慢的那一帧"卡住的，要看尾部。
	if slowest > 200*time.Millisecond {
		t.Fatalf("最慢一帧 %v，会明显卡住决策拍", slowest)
	}
}
