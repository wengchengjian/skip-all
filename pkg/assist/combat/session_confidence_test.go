package combat

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// confidenceRuntime 让 OCR 返回指定置信度的文本块。
type confidenceRuntime struct {
	*fakeRuntime
	text       string
	confidence float64
}

func (r *confidenceRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	return []vision.TextBlock{{Text: r.text, Confidence: r.confidence, Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}}, nil
}

// TestCooldownIgnoresLowConfidenceText 验证图标花纹被误读出的低置信度数字
// 不会把就绪的技能判成冷却。
//
// 真实截图里出现过的情形：空的解放按钮被 OCR 读成 "2"，置信度 0.10；
// 真正的冷却数字置信度都在 0.95 以上。误判成冷却会让技能永远不放，
// 退化成只会普攻——正是用户实测到的问题形态。
func TestCooldownIgnoresLowConfidenceText(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		confidence float64
		want       bool
	}{
		{"图标误读 低置信度", "2", 0.10, false},
		{"图标误读 中等置信度", "2", 0.40, false},
		{"真实冷却 高置信度", "18.1", 1.00, true},
		{"真实冷却 边界置信度", "1.3", 0.95, true},
	}
	for _, c := range cases {
		rt := &confidenceRuntime{fakeRuntime: testRuntime(true), text: c.text, confidence: c.confidence}
		spec := testSpec()
		spec.Cooldowns = map[StepKind]geom.Rect{StepSkill: {X1: 0, Y1: 0, X2: 10, Y2: 10}}
		session := newSession(rt, spec.withDefaults(), sessionSettings{})
		got, err := session.coolingDown(frameAt(time.Now()), StepSkill, geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10})
		session.Close()
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: 文本 %q 置信度 %.2f 判成 cooling=%v，期望 %v", c.name, c.text, c.confidence, got, c.want)
		}
	}
}
