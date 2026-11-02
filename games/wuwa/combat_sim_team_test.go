package wuwa

import (
	"strings"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// TestSimHiyukiLinnaiSuisui 复现真实队伍（绯雪/琳奈/穗穗）的一整场模拟战斗。
// 要求：进战斗后不会卡死在某个角色上只普攻——技能要放、协奏满了要切人。
func TestSimHiyukiLinnaiSuisui(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	runSim(t, g, 600) // 60 秒，10 FPS
	t.Log(summarise(g))

	if g.skillUses == 0 {
		t.Fatalf("整场没有放过一次共鸣技能，说明连招没生效:\n%s", summarise(g))
	}
	if g.switches == 0 {
		t.Fatalf("协奏满了也没有切人:\n%s", summarise(g))
	}
	// 至少要轮转过两个以上角色，不能一直站一个人。
	if len(g.team) > 1 && g.switches < 2 {
		t.Fatalf("切人次数过少（%d），像是卡在单个角色上:\n%s", g.switches, summarise(g))
	}
}

// TestSimNoStuckOnOneAgent 检查 60 秒内每个角色都上过场：
// 用户报告的现象是"进战斗后自动切到某个人，然后一直普攻"。
func TestSimNoStuckOnOneAgent(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	runSim(t, g, 600)
	seen := map[int]int{}
	for _, a := range g.actions {
		seen[a.slot]++
	}
	for slot, n := range seen {
		if n == 0 {
			t.Fatalf("%d 号位整场没上过场:\n%s", slot+1, summarise(g))
		}
	}
	if len(seen) < 3 {
		t.Fatalf("只有 %d 个槽位出过场，应当三个都轮到:\n%s", len(seen), summarise(g))
	}
}

// TestSimSuisuiAloneCanAct 单独验证"切到穗穗后不会只普攻"。
// 穗穗是治疗位，规则表里它的动作是 重击 -> 技能，两者都应当出现。
func TestSimSuisuiAloneCanAct(t *testing.T) {
	g := newSimGame("char_suisui", "char_linnai", "char_hiyuki")
	g.current = 0
	runSim(t, g, 200)
	t.Log(summarise(g))
	if g.skillUses == 0 {
		t.Fatalf("穗穗在场 20 秒一次技能都没放:\n%s", summarise(g))
	}
}

// TestSimActionsAreNotAllAttacks 直接盯住用户报告的症状：
// 动作序列里普攻占绝大多数，说明决策退化成了"只会点普攻"。
func TestSimActionsAreNotAllAttacks(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	runSim(t, g, 600)
	attacks := 0
	others := 0
	for _, a := range g.actions {
		switch a.kind {
		case assist.ActionClick, assist.ActionClickHold:
			attacks++
		default:
			others++
		}
	}
	if others == 0 || attacks > others*20 {
		t.Fatalf("普攻 %d 次、其它动作 %d 次，决策退化成只会普攻:\n%s", attacks, others, summarise(g))
	}
}

// TestSimSwitchKeysMatchSlots 检查切人时按的键和模拟对局里真正换到的人对得上。
func TestSimSwitchKeysMatchSlots(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	runSim(t, g, 600)
	for _, a := range g.actions {
		if a.kind != assist.ActionKeyTap || len(a.key) != 1 || a.key[0] < '1' || a.key[0] > '3' {
			continue
		}
		want := int(a.key[0] - '1')
		if a.slot == want {
			t.Fatalf("按了 %s 但当前已经是该槽位（空切）:\n%s", a.key, summarise(g))
		}
	}
	if !strings.Contains(summarise(g), "切人=") {
		t.Fatal("summary 格式异常")
	}
}
