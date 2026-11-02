package combat

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestSwitchNoLocks 验证上游 get_switch_priority() 的 NO 锁：
// 候选角色最近重击还在窗口内时跳过，窗口过期后恢复常规打分。
func TestSwitchNoLocks(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 1, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleMainDPS, "b": RoleSubDPS, "c": RoleSupport}
	spec.SwitchNoAfterHeavy = map[string]time.Duration{"a": 5 * time.Second}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.lastHeavy["a"] = base
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(time.Second)})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("a 在重击 NO 锁内应跳过，actions=%+v", actions)
	}
	session2 := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	session2.lastHeavy["a"] = base.Add(-6 * time.Second)
	_, actions = step(t, session2, assist.Frame{Image: concertoFullFrame(), At: base})
	if len(actions) != 1 || actions[0].Key != "1" {
		t.Fatalf("NO 锁过期后应按定位切 a，actions=%+v", actions)
	}
}

// TestSwitchSynergyAfter 验证 Phrolova 的 "last_liberation > 14s + 当前 Cantarella -> MUST"。
func TestSwitchSynergyAfter(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"char_cantarella", "char_x", "char_phrolova"}}
	spec.Roles = map[string]Role{"char_cantarella": RoleSubDPS, "char_x": RoleMainDPS, "char_phrolova": RoleSupport}
	spec.SwitchSynergyAfter = map[string]map[string]time.Duration{
		"char_phrolova": {"char_cantarella": 14 * time.Second},
	}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.lastLiberation["char_phrolova"] = base.Add(-15 * time.Second)
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("Phrolova 满足 14 秒条件应被 MUST 切入，actions=%+v", actions)
	}
}
