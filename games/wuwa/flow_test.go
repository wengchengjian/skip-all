package wuwa

import (
	"image"
	"image/color"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// flowRuntime 只用真实像素做判据：模板一律认不出，OCR 返回空，画面配置给出中断区域。
type flowRuntime struct {
	ocr string
	// hit 里的模板 id 一律返回命中（用来模拟队伍数字与角色头像）。
	hit map[string]bool
	// skill 为 true 时，技能就绪区域是亮的（用来模拟"落地"）。
	skill bool
}

func (flowRuntime) Screen(name string) (*screen.Screen, error) {
	area := &screen.Area{Name: "中断提示", Rect: geom.Rect{X1: 520, Y1: 840, X2: 1850, Y2: 1060}}
	return screen.NewScreen(name, area), nil
}

func (r flowRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	if r.ocr == "" {
		return nil, nil
	}
	return []vision.TextBlock{{Text: r.ocr, Rect: geom.Rect{X1: 600, Y1: 900, X2: 900, Y2: 950}}}, nil
}

func (r flowRuntime) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	if r.hit[ref.ID] {
		return []vision.Match{{Rect: area.Rect, Confidence: 0.95}}, nil
	}
	return nil, nil
}

func (flowRuntime) ColorStats(assist.Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}

func (flowRuntime) Log() *slog.Logger { return slog.Default() }

// flowFrame 造一帧：可选画敌人血条、可选把协奏环涂满。
func flowFrame(withHealthBar, fullConcerto bool) *image.RGBA {
	return flowFrameOpts(withHealthBar, fullConcerto, false, false)
}

// flowFrameOpts 额外控制"技能就绪亮起"与"回路满亮起"。
func flowFrameOpts(withHealthBar, fullConcerto, skillLit, forteLit bool) *image.RGBA {
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
	if withHealthBar {
		for y := 100; y < 108; y++ {
			for x := 800; x < 1000; x++ {
				img.SetRGBA(x, y, color.RGBA{200, 70, 65, 255}) // enemy_health_color_red 内
			}
		}
	}
	if fullConcerto {
		spec := combatSpec()
		ring := spec.Concerto.Rect
		h := float64(ring.Y2 - ring.Y1)
		cx, cy := float64(ring.X2-ring.X1)/2, h/2
		for y := 0; y < ring.Y2-ring.Y1; y++ {
			for x := 0; x < ring.X2-ring.X1; x++ {
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
				d := math.Hypot(dx, dy)
				if d < spec.Concerto.Inner*h || d > spec.Concerto.Outer*h {
					continue
				}
				img.SetRGBA(ring.X1+x, ring.Y1+y, color.RGBA{220, 205, 110, 255}) // 衍射色
			}
		}
	}
	return img
}

// TestCombatFlowEndToEnd 用真实像素把鸣潮的战斗流程串一遍：
// 没有血条 -> 等待战斗；有血条 -> 进战斗；协奏环涂满 -> 切人。
func TestCombatFlowEndToEnd(t *testing.T) {
	feature := CombatFeature()
	session, err := feature.NewSession(flowRuntime{}, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	defer session.Close()
	base := time.Now()

	// 1. 没有画面 -> 等待战斗
	result, _, err := session.Step(nil, assist.Frame{At: base})
	if err != nil {
		t.Fatalf("空帧出错: %v", err)
	}
	if result.State != "等待战斗" {
		t.Fatalf("空帧应当等待战斗, got %q", result.State)
	}

	// 2. 有敌人血条、协奏没满 -> 进入战斗流程（不该切人）
	result, _, err = session.Step(nil, assist.Frame{Image: flowFrame(true, false), At: base.Add(100 * time.Millisecond)})
	if err != nil {
		t.Fatalf("战斗帧出错: %v", err)
	}
	if result.State == "等待战斗" || result.State == "协奏满切人" {
		t.Fatalf("有血条时应当进入战斗但还不切人, got %q", result.State)
	}

	// 3. 协奏环涂满 -> 切人
	result, actions, err := session.Step(nil, assist.Frame{Image: flowFrame(true, true), At: base.Add(4 * time.Second)})
	if err != nil {
		t.Fatalf("协奏帧出错: %v", err)
	}
	if result.State != "协奏满切人" {
		t.Fatalf("协奏满应当切人, got %q", result.State)
	}
	if len(actions) != 1 || actions[0].Kind != assist.ActionKeyTap {
		t.Fatalf("切人应当是按键动作: %+v", actions)
	}
}

// TestCombatFlowAbort 验证出现复活提示时停手。
func TestCombatFlowAbort(t *testing.T) {
	session, err := CombatFeature().NewSession(flowRuntime{ocr: "点击复活"}, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	defer session.Close()
	base := time.Now()
	// 先进入战斗。
	if _, _, err := session.Step(nil, assist.Frame{Image: flowFrame(true, false), At: base}); err != nil {
		t.Fatalf("战斗帧出错: %v", err)
	}
	// 出现复活文字 -> 战斗中断。
	result, actions, err := session.Step(nil, assist.Frame{Image: flowFrame(true, false), At: base.Add(2 * time.Second)})
	if err != nil {
		t.Fatalf("中断帧出错: %v", err)
	}
	if result.State != "战斗中断" {
		t.Fatalf("复活提示时应当战斗中断, got %q", result.State)
	}
	if len(actions) != 0 {
		t.Fatalf("中断时不该出手: %+v", actions)
	}
}

// TestCombatFlowAfterSwitch 覆盖切人之后的链路：切人确认 -> 入场 -> 连招。
func TestCombatFlowAfterSwitch(t *testing.T) {
	// 队伍栏：1、2 号数字可见，3 号缺失 => 当前角色是 3 号位；头像认成忌炎。
	rt := flowRuntime{hit: map[string]bool{
		"char_1_text": true,
		"char_2_text": true,
		"char_jiyan":  true,
		"has_target":  true,
	}}
	session, err := CombatFeature().NewSession(rt, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	defer session.Close()
	base := time.Now()

	// 第一帧：进战斗后先切治疗位（配置里 HealerKey = "3"）。
	result, _, err := session.Step(nil, assist.Frame{Image: flowFrame(true, false), At: base})
	if err != nil || result.State != "战斗前切治疗" {
		t.Fatalf("开打前应当先切治疗: state=%q err=%v", result.State, err)
	}
	// 第二帧：有血条 + 协奏满 -> 切人
	result, actions, err := session.Step(nil, assist.Frame{Image: flowFrame(true, true), At: base.Add(3 * time.Second)})
	if err != nil || result.State != "协奏满切人" {
		t.Fatalf("协奏满应当切人: state=%q err=%v", result.State, err)
	}
	if len(actions) == 0 {
		t.Fatal("切人应当有动作")
	}

	// 切人后的下一帧：队伍仍显示 1、2 可见（3 缺失），视为还没切过去 -> 补普攻。
	result, _, err = session.Step(nil, assist.Frame{Image: flowFrame(true, false), At: base.Add(3400 * time.Millisecond)})
	if err != nil {
		t.Fatalf("切人确认帧出错: %v", err)
	}
	// 规则引擎接管时会是 规则-*（内容就是继续出手/普攻），没人接管时是 切人未生效。
	// 两者都是"没切过去就别站着"的兜底行为。
	if result.State != "切人未生效" && !strings.HasPrefix(result.State, "规则-") {
		t.Fatalf("队伍数字没变应当继续出手, got %q", result.State)
	}
}

// TestCombatFlowIntroThenCombo 覆盖切人成功后的入场与连招：
// 队伍数字变化 = 切人生效；技能亮起 = 落地；之后进入角色连招。
func TestCombatFlowIntroThenCombo(t *testing.T) {
	hit := map[string]bool{"char_1_text": true, "char_2_text": true, "char_jiyan": true, "has_target": true}
	rt := flowRuntime{hit: hit}
	session, err := CombatFeature().NewSession(rt, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	defer session.Close()
	base := time.Now()

	// 先切治疗，再协奏满切人。
	if result, _, _ := session.Step(nil, assist.Frame{Image: flowFrame(true, false), At: base}); result.State != "战斗前切治疗" {
		t.Fatalf("应当先切治疗, got %q", result.State)
	}
	if result, _, _ := session.Step(nil, assist.Frame{Image: flowFrame(true, true), At: base.Add(3 * time.Second)}); result.State != "协奏满切人" {
		t.Fatalf("协奏满应当切人, got %q", result.State)
	}

	// 切人生效：3 号数字变得可见、2 号消失（当前角色换成 2 号位）。
	hit["char_3_text"] = true
	delete(hit, "char_2_text")

	// 技能亮起 = 落地，入场结束；之后应进入连招。
	result, _, err := session.Step(nil, assist.Frame{Image: flowFrameOpts(true, false, true, false), At: base.Add(3400 * time.Millisecond)})
	if err != nil {
		t.Fatalf("入场帧出错: %v", err)
	}
	result, actions, err := session.Step(nil, assist.Frame{Image: flowFrameOpts(true, false, true, false), At: base.Add(3600 * time.Millisecond)})
	if err != nil {
		t.Fatalf("连招帧出错: %v", err)
	}
	if !strings.HasPrefix(result.State, "连招-") && !strings.HasPrefix(result.State, "入场") {
		t.Fatalf("落地后应当进入连招, got %q actions=%+v", result.State, actions)
	}
}
