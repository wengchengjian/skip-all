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

// TestSwitchSynergyTeam 验证 Mornye 的队伍条件：队里有 Linnai 且当前不是 Linnai。
func TestSwitchSynergyTeam(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"a", "char_linnai", "char_moning"}}
	spec.Roles = map[string]Role{"a": RoleMainDPS, "char_linnai": RoleSubDPS, "char_moning": RoleSupport}
	spec.SwitchSynergyTeam = map[string]string{"char_moning": "char_linnai"}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: time.Now()})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("队伍有 Linnai 时 Mornye 应被 MUST 切入，actions=%+v", actions)
	}
}

// TestSwitchHold 验证 Zani/Carlotta 的 wait_switch()：重击动画未结束时先不切走。
func TestSwitchHold(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleMainDPS, "b": RoleSubDPS, "c": RoleSupport}
	spec.HoldSwitchAfterHeavy = map[string]time.Duration{"a": 2200 * time.Millisecond}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.introUntil = base.Add(time.Second)
	session.lastHeavy["a"] = base
	if _, ok := session.concertoSwitch(assist.Frame{Image: concertoFullFrame(), At: base}, base); ok {
		t.Fatal("重击 hold 窗口内不应切走")
	}
	if _, ok := session.concertoSwitch(assist.Frame{Image: concertoFullFrame(), At: base.Add(3 * time.Second)}, base.Add(3*time.Second)); !ok {
		t.Fatal("hold 窗口过期后应允许切走")
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
