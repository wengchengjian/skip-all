package combat

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestLiberationMustSwitch 验证 Brant/Encore/Lupa 的 still_in_liberation() -> MUST：
// 候选角色还在解放窗口内时，切人打分必须压过常规定位分。
func TestLiberationMustSwitch(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 1, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleSupport, "b": RoleMainDPS, "c": RoleSubDPS}
	spec.LiberationMustSwitch = map[string]bool{"a": true}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.lastLiberation["a"] = base
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(time.Second)})
	if len(actions) != 1 || actions[0].Key != "1" {
		t.Fatalf("解放窗口内的 a 应当被 MUST 切入，actions=%+v", actions)
	}
	// 窗口过期后回到常规定位打分（sub_dps c 应胜出）。
	session2 := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	session2.lastLiberation["a"] = base.Add(-13 * time.Second)
	_, actions = step(t, session2, assist.Frame{Image: concertoFullFrame(), At: base})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("解放窗口过期后应按定位切 c，actions=%+v", actions)
	}
}

// TestSwitchIntroMUST 验证 Camellya / Jinhsi 的 has_intro -> MUST：
// 协奏入场时无条件优先切给它们。
func TestSwitchIntroMUST(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 1, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleSupport, "b": RoleMainDPS, "c": RoleSubDPS}
	spec.SwitchIntroMUST = map[string]bool{"a": true}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: time.Now()})
	if len(actions) != 1 || actions[0].Key != "1" {
		t.Fatalf("协奏入场时 a 应被 MUST 切入，actions=%+v", actions)
	}
}
