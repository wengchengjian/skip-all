package combat

import (
	"strings"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// branchExt 是一个只在 hit=true 时出手的扩展，用来覆盖"扩展先于连招"这条分支。
type branchExt struct{ hit bool }

func (b *branchExt) Name() string { return "branch" }

func (b *branchExt) Step(Runtime, assist.Frame) (assist.Action, string, bool, error) {
	if !b.hit {
		return assist.Action{}, "", false, nil
	}
	return assist.Action{Kind: assist.ActionKeyTap, Key: "x"}, "扩展-测试", true, nil
}

// TestSessionCoversAllBranches 把一个 Session 从"未进战斗"推到"脱战"，断言每条分支都被走到。
// 目的：防止某个 return 被后续改动变成永远走不到的死分支。
func TestSessionCoversAllBranches(t *testing.T) {
	ext := &branchExt{}
	spec := testSpec()
	spec.Team = healerTeam{slot: 1}
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Extensions = []Extension{ext}
	spec.LockInterval = 500 * time.Millisecond
	spec.Target = &Target{
		Rect:      geom.Rect{X1: 1400, Y1: 900, X2: 1540, Y2: 1045},
		SubDir:    "combat",
		Have:      "has_target",
		Missing:   "no_target",
		Threshold: 0.6,
	}
	spec.AbortArea = "中断"
	spec.AbortTexts = []string{"复活"}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	spec.IntroSteps = []Step{{Kind: StepSkill}}

	rt := testRuntime(true)
	session := newSession(rt, spec.withDefaults(), sessionSettings{})

	seen := map[string]bool{}
	base := time.Now()
	noImage := assist.Frame{At: base}

	step := func(i int, frame assist.Frame) string {
		result, _, err := session.Step(nil, frame)
		if err != nil {
			t.Fatalf("第 %d 帧出错: %v", i, err)
		}
		seen[result.State] = true
		return result.State
	}

	// 1. 没有画面 -> 等待战斗
	step(0, noImage)

	// 2. 进战斗 + 有治疗位 -> 战斗前切治疗
	inCombat := assist.Frame{Image: flowFrame(0), At: base.Add(100 * time.Millisecond)}
	if got := step(1, inCombat); got != "战斗前切治疗" {
		t.Fatalf("第二帧应当切治疗, got %q", got)
	}

	// 3. 没锁定目标 -> 锁定目标（fake 里 Target 模板给 0 分即视为未锁定）
	rt.scores = map[string]float64{"has_target": 0, "no_target": 0}
	unlocked := assist.Frame{Image: flowFrame(5), At: base.Add(200 * time.Millisecond)}
	if got := step(2, unlocked); got != "锁定目标" {
		t.Fatalf("没锁定时应当锁定目标, got %q", got)
	}

	// 4. 锁定完成 + 协奏满 -> 协奏满切人
	rt.scores = map[string]float64{"has_target": 0.95}
	full := assist.Frame{Image: flowFrame(0), At: base.Add(300 * time.Millisecond)}
	if got := step(3, full); got != "协奏满切人" {
		t.Fatalf("协奏满应当切人, got %q", got)
	}

	// 5. 切人后先进入入场技
	plain := assist.Frame{Image: flowFrame(5), At: base.Add(400 * time.Millisecond)}
	if got := step(4, plain); got != "入场-技能" {
		t.Fatalf("切人后应当先进入入场技, got %q", got)
	}

	// 6. 入场技打完回到连招
	if got := step(5, plain); !strings.HasPrefix(got, "连招-") {
		t.Fatalf("入场技之后应当回到连招, got %q", got)
	}

	// 7. 扩展优先于连招
	ext.hit = true
	if got := step(6, plain); got != "扩展-测试" {
		t.Fatalf("扩展应当优先, got %q", got)
	}
	ext.hit = false

	// 8. 出现复活提示 -> 战斗中断
	rt.ocrText = "复活确认"
	if got := step(7, assist.Frame{Image: flowFrame(5), At: base.Add(700 * time.Millisecond)}); got != "战斗中断" {
		t.Fatalf("应当战斗中断, got %q", got)
	}
	rt.ocrText = ""

	// 9. 脱战超过保持时间 -> 战斗结束切治疗
	rt.hit = false
	outOfCombat := assist.Frame{Image: flowFrame(20), At: base.Add(5 * time.Second)}
	if got := step(8, outOfCombat); got != "战斗结束切治疗" {
		t.Fatalf("脱战应当切回治疗, got %q", got)
	}

	// 10. 再之后就是等待战斗
	if got := step(9, assist.Frame{Image: flowFrame(20), At: base.Add(6 * time.Second)}); got != "等待战斗" {
		t.Fatalf("脱战后应当等待战斗, got %q", got)
	}

	want := []string{"等待战斗", "战斗前切治疗", "锁定目标", "协奏满切人", "入场-技能", "扩展-测试", "战斗中断", "战斗结束切治疗"}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("分支 %q 没被触发，可能是死代码", w)
		}
	}
	hasCombo := false
	for state := range seen {
		if strings.HasPrefix(state, "连招-") {
			hasCombo = true
		}
	}
	if !hasCombo {
		t.Error("连招分支没被触发")
	}
}
