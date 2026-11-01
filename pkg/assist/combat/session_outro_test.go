package combat

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

type outroStateExt struct{ got []CombatState }

func (e *outroStateExt) Name() string { return "outro" }

func (e *outroStateExt) Step(Runtime, assist.Frame) (assist.Action, string, bool, error) {
	return assist.Action{}, "", false, nil
}

func (e *outroStateExt) SetCombatState(s CombatState) { e.got = append(e.got, s) }

// outroTeam 的槽位可以手动改，用来模拟切人真的换了人。
type outroTeam struct {
	slot int
	ids  []string
}

func (t *outroTeam) Current(Runtime, assist.Frame) (int, bool, error) { return t.slot, true, nil }

func (t *outroTeam) Agent(Runtime, assist.Frame) (string, bool, error) {
	if t.slot < len(t.ids) {
		return t.ids[t.slot], true, nil
	}
	return "", false, nil
}

// TestSessionTracksLastOutro 验证 check_outro() 的会话记账：
// 切人动作发出时记下离场角色，下一帧通过 CombatState.LastOutro 推给扩展；
// 入场窗口结束后 InIntro 变 false（原子自身会叠这个条件）。
func TestSessionTracksLastOutro(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	team := &outroTeam{slot: 0, ids: []string{"a", "b", "c"}}
	spec.Team = team
	spec.Roles = map[string]Role{"a": RoleMainDPS, "b": RoleSubDPS, "c": RoleHealer}
	spec.Intro = &IntroSpec{MaxDuration: 500 * time.Millisecond}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}
	ext := &outroStateExt{}
	spec.Extensions = []Extension{ext}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()

	// 第一帧：协奏满，从 a 切走。
	if result, _ := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base}); result.State != "协奏满切人" {
		t.Fatalf("第一帧应当切人: %q", result.State)
	}
	if session.outroAgent != "a" {
		t.Fatalf("切走时应记下 a，实际 %q", session.outroAgent)
	}
	// 第二帧：b 已入场，处于入场窗口。
	team.slot = 1
	step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(100 * time.Millisecond)})
	last := ext.got[len(ext.got)-1]
	if last.Agent != "b" || !last.InIntro || last.LastOutro != "a" {
		t.Fatalf("b 入场时应推送 {Agent:b InIntro:true LastOutro:a}，实际 %+v", last)
	}
	// 第三帧：入场窗口结束，LastOutro 仍是 a，但 InIntro=false。
	step(t, session, assist.Frame{Image: frameAt(base.Add(600 * time.Millisecond)).Image, At: base.Add(600 * time.Millisecond)})
	last = ext.got[len(ext.got)-1]
	if last.Agent != "b" || last.InIntro || last.LastOutro != "a" {
		t.Fatalf("入场结束后应推送 {Agent:b InIntro:false LastOutro:a}，实际 %+v", last)
	}
	// 第二次切人：a 之后又切走 b，来源要更新成 b。
	if result, _ := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(3 * time.Second)}); result.State != "协奏满切人" {
		t.Fatalf("第二帧应当再次切人: %q", result.State)
	}
	team.slot = 2
	step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(3100 * time.Millisecond)})
	last = ext.got[len(ext.got)-1]
	if last.Agent != "c" || !last.InIntro || last.LastOutro != "b" {
		t.Fatalf("c 入场时应推送 {Agent:c InIntro:true LastOutro:b}，实际 %+v", last)
	}
}
