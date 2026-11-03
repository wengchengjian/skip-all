package combat

import (
	"strings"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestCombatRuleOrder 固定决策链的优先级顺序。
//
// 顺序本身是行为：同一帧有多个证据时，排在前面的规则先出手。
// 这个测试让顺序变更必须显式出现在 diff 里，而不是被悄悄改掉。
func TestCombatRuleOrder(t *testing.T) {
	want := []string{"战斗前切治疗", "扩展能力", "协奏满切人", "战斗中断", "锁定目标", "连招"}
	if len(combatRules) != len(want) {
		t.Fatalf("规则数 = %d, want %d", len(combatRules), len(want))
	}
	for i, w := range want {
		if combatRules[i].name != w {
			t.Errorf("第 %d 条规则 = %q, want %q", i, combatRules[i].name, w)
		}
		if combatRules[i].run == nil {
			t.Errorf("第 %d 条规则 %q 没有实现", i, w)
		}
	}
}

// TestRulePriorityExtensionBeforeCombo 验证扩展证据与连招同时可用时扩展先出手。
func TestRulePriorityExtensionBeforeCombo(t *testing.T) {
	ext := &branchExt{hit: true}
	spec := testSpec()
	spec.Keys.Attack = "r"
	spec.Plan = []Step{{Kind: StepAttack}}
	spec.Extensions = []Extension{ext}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	res, acts, err := session.Step(nil, assist.Frame{Image: flowFrame(1), At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.State, "扩展-") {
		t.Fatalf("扩展应当优先于连招, state=%q", res.State)
	}
	if len(acts) != 1 {
		t.Fatalf("应当给出 1 个动作, got %d", len(acts))
	}
}

// TestRulePriorityConcertoBeforeAbort 验证协奏满与战斗中断同时成立时，协奏切人先出手。
//
// 这条顺序容易写反：中断看起来更"紧急"，但上游是先进协奏切人再检查中断。
func TestRulePriorityConcertoBeforeAbort(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleMainDPS, "b": RoleSubDPS, "c": RoleSupport}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	spec.AbortArea = "中断"
	spec.AbortTexts = []string{"复活"}

	rt := testRuntime(true)
	rt.ocrText = "复活确认"
	session := newSession(rt, spec.withDefaults(), sessionSettings{})

	res, _, err := session.Step(nil, assist.Frame{Image: concertoFullFrame(), At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "协奏满切人" {
		t.Fatalf("协奏满应当先于战斗中断, state=%q", res.State)
	}
}
