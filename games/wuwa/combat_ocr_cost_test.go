package wuwa

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// slowOCRRuntime 在 OCR 上模拟真实 PP-OCR 的耗时（实测约 130ms）。
type slowOCRRuntime struct {
	simRuntime
	cost  time.Duration
	calls int
}

func (r *slowOCRRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	r.calls++
	time.Sleep(r.cost)
	return nil, nil
}

// TestCombatStepCostWithRealOCR 量战斗会话一帧在真实 OCR 耗时下的代价。
// 鸣潮配置里技能冷却走 OCR，用来判断"决策拍能不能维持 10Hz"。
func TestCombatStepCostWithRealOCR(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	rt := &slowOCRRuntime{simRuntime: simRuntime{game: g}, cost: 130 * time.Millisecond}
	feature := CombatFeature()
	session, err := feature.NewSession(rt, feature.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	spec := combatSpec()

	base := time.Now()
	const frames = 30
	start := time.Now()
	for i := 0; i < frames; i++ {
		frame := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
		result, actions, err := session.Step(nil, frame)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			g.apply(a, result.State)
		}
	}
	elapsed := time.Since(start)
	perFrame := elapsed / frames
	t.Logf("%d 帧共 %v，每帧 %v，OCR 调用 %d 次", frames, elapsed, perFrame, rt.calls)
	if perFrame > 100*time.Millisecond {
		t.Fatalf("每帧 %v 超过 10Hz 的 100ms 预算，决策拍会被 OCR 拖住", perFrame)
	}
}

// TestCooldownOCRCacheIsEffective 检查冷却 OCR 的缓存是否真的减少了调用次数。
func TestCooldownOCRCacheIsEffective(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	rt := &slowOCRRuntime{simRuntime: simRuntime{game: g}}
	feature := CombatFeature()
	session, err := feature.NewSession(rt, feature.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	spec := combatSpec()
	base := time.Now()
	const frames = 60 // 6 秒
	for i := 0; i < frames; i++ {
		frame := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
		result, actions, err := session.Step(nil, frame)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			g.apply(a, result.State)
		}
	}
	t.Logf("%d 帧（6 秒）共调用 OCR %d 次", frames, rt.calls)
	// 三种技能 × 每 500ms 一次 × 6 秒 ≈ 36 次上限。
	if rt.calls > 60 {
		t.Fatalf("6 秒内 OCR %d 次，缓存没起作用", rt.calls)
	}
}
