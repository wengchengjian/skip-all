package combat

import (
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestSessionFlowIsStable 跑一段完整的战斗循环，确认状态机不会卡死或反复出同一手。
func TestSessionFlowIsStable(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "healer"}}
	spec.Roles = map[string]Role{"dps": RoleMainDPS, "sub": RoleSubDPS, "healer": RoleHealer}
	spec.Plan = []Step{
		{Kind: StepAttack}, {Kind: StepAttack}, {Kind: StepSkill},
		{Kind: StepAttack}, {Kind: StepLiberation}, {Kind: StepDodge},
	}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})

	base := time.Now()
	states := map[string]int{}
	lastState := ""
	repeat := 0
	for i := 0; i < 40; i++ {
		frame := assist.Frame{Image: flowFrame(i), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
		result, _, err := session.Step(nil, frame)
		if err != nil {
			t.Fatalf("第 %d 帧出错: %v", i, err)
		}
		if result.State == "" {
			t.Fatalf("第 %d 帧没有状态", i)
		}
		states[result.State]++
		if result.State == lastState {
			repeat++
			if repeat > 12 {
				t.Fatalf("状态 %q 连续出现 %d 次，疑似卡死", result.State, repeat)
			}
		} else {
			repeat = 0
		}
		lastState = result.State
	}
	// 40 帧里应该既出过招，也切过人（协奏满）。
	if states["协奏满切人"] == 0 {
		t.Fatalf("整段战斗没有触发协奏切人: %+v", states)
	}
	if len(states) < 3 {
		t.Fatalf("状态太单一: %+v", states)
	}
}

// flowFrame 造一段会变化的画面：前 10 帧协奏满，之后清空。
func flowFrame(i int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	if i%10 < 3 {
		for y := 0; y < 63; y++ {
			for x := 0; x < 63; x++ {
				d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
				if d >= 22.1 && d <= 26.6 {
					img.SetRGBA(700+x, 960+y, color.RGBA{220, 205, 110, 255})
				}
			}
		}
	}
	return img
}
