package wuwa

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

type ruleTeam struct{ agent string }

func (t ruleTeam) Agent(combat.Runtime, assist.Frame) (string, bool, error) {
	return t.agent, t.agent != "", nil
}
func (t ruleTeam) Current(combat.Runtime, assist.Frame) (int, bool, error) { return 0, true, nil }
func (t ruleTeam) Slots(combat.Runtime, assist.Frame) ([]string, error)    { return nil, nil }

func ruleFrame(skillLit, forteLit bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	if skillLit {
		for y := 940; y < 992; y++ {
			for x := 1594; x < 1658; x++ {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	if forteLit {
		for y := 996; y < 1008; y++ {
			for x := 1125; x < 1156; x++ {
				img.SetRGBA(x, y, color.RGBA{252, 252, 252, 255})
			}
		}
	}
	return img
}

// TestRuleExtConditions 验证条件求值与动作派发。
func TestRuleExtConditions(t *testing.T) {
	ext := &wuwaRuleExt{
		Team:     ruleTeam{agent: "char_jiyan"},
		Skill:    geom.Rect{X1: 1592, Y1: 938, X2: 1660, Y2: 995},
		Forte:    geom.Rect{X1: 1125, Y1: 996, X2: 1156, Y2: 1008},
		Ready:    [2][3]uint8{{253, 253, 253}, {255, 255, 255}},
		ForteRGB: [2][3]uint8{{244, 246, 250}, {255, 255, 255}},
		Rules: map[string][]wuwaRule{
			"char_jiyan": {
				// 忌炎：回路没满且技能可用时放技能；否则普攻。
				{Cond: "回路未满&技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}},
				{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}},
			},
		},
	}
	base := time.Now()

	// 回路未满 + 技能亮 -> 技能
	action, state, ok, err := ext.Step(nil, assist.Frame{Image: ruleFrame(true, false), At: base})
	if err != nil || !ok || state != "规则-回路未满&技能可用" || action.Key != "e" {
		t.Fatalf("应当命中技能规则: state=%q action=%+v err=%v", state, action, err)
	}

	// 回路满 -> 落到普攻规则
	ext2 := &wuwaRuleExt{Team: ruleTeam{agent: "char_jiyan"}, Skill: ext.Skill, Forte: ext.Forte, Ready: ext.Ready, ForteRGB: ext.ForteRGB, Rules: ext.Rules}
	action, _, ok, _ = ext2.Step(nil, assist.Frame{Image: ruleFrame(true, true), At: base})
	if !ok || action.Kind != assist.ActionClick {
		t.Fatalf("回路满时应当普攻: %+v", action)
	}

	// 认不出角色 -> 不接管
	ext3 := &wuwaRuleExt{Team: ruleTeam{}, Rules: ext.Rules}
	if _, _, ok, _ := ext3.Step(nil, assist.Frame{Image: ruleFrame(true, false), At: base}); ok {
		t.Fatal("认不出角色时不该接管")
	}
}

// TestRuleExtOutroAtom 验证 延奏=角色id 原子：只有入场窗口内且来源匹配才算真。
// 上游 check_outro() 在 has_intro 为 false 时返回 'null'，所以窗口外一律不成立。
func TestRuleExtOutroAtom(t *testing.T) {
	rules := map[string][]wuwaRule{
		"char_brant": {
			{Cond: "延奏=char_lupa", Ops: []combat.Step{{Kind: combat.StepSkill}}},
			{Cond: "(延奏=char_roccia|延奏=char_sanhua)", Ops: []combat.Step{{Kind: combat.StepEcho}}},
			{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		},
	}
	base := time.Now()
	cases := []struct {
		name  string
		state combat.CombatState
		want  string
		key   string
	}{
		{"来源匹配", combat.CombatState{InIntro: true, LastOutro: "char_lupa"}, "规则-延奏=char_lupa", "e"},
		{"多来源之一", combat.CombatState{InIntro: true, LastOutro: "char_sanhua"}, "规则-(延奏=char_roccia|延奏=char_sanhua)", "q"},
		{"来源不匹配", combat.CombatState{InIntro: true, LastOutro: "char_zhezhi"}, "规则-1", ""},
		{"不在入场窗口", combat.CombatState{InIntro: false, LastOutro: "char_lupa"}, "规则-1", ""},
	}
	for _, c := range cases {
		ext := &wuwaRuleExt{Team: ruleTeam{agent: "char_brant"}, Rules: rules}
		ext.SetCombatState(c.state)
		action, state, ok, err := ext.Step(nil, assist.Frame{Image: ruleFrame(false, false), At: base})
		if err != nil || !ok || state != c.want {
			t.Fatalf("%s: state=%q action=%+v err=%v", c.name, state, action, err)
		}
		if action.Key != c.key {
			t.Fatalf("%s: 按键=%q, want %q", c.name, action.Key, c.key)
		}
	}
}
