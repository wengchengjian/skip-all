package zzz

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

type fakeRuntime struct {
	matches map[string][]vision.Match
}

func (f fakeRuntime) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	return f.matches[ref.ID], nil
}

func TestDodgeFlashExtension(t *testing.T) {
	ext := &zzzDodgeExt{
		Rect:     geom.Rect{X1: 560, Y1: 240, X2: 1360, Y2: 840},
		Ratio:    0.03,
		Key:      "shift",
		Interval: time.Second,
		Colors:   [][2][3]uint8{{{225, 195, 60}, {255, 255, 165}}},
	}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 300; y < 800; y++ {
		for x := 600; x < 1320; x++ {
			img.SetRGBA(x, y, color.RGBA{250, 220, 90, 255})
		}
	}
	action, state, ok, err := ext.Step(fakeRuntime{}, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "闪避-闪光" || action.Key != "shift" {
		t.Fatalf("黄光应当触发闪避: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}
	// 间隔内不重复触发。
	if _, _, ok, _ := ext.Step(fakeRuntime{}, assist.Frame{Image: img, At: time.Now()}); ok {
		t.Fatal("闪避间隔内不该重复触发")
	}
	// 没有闪光不触发。
	empty := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	ext2 := &zzzDodgeExt{Rect: geom.Rect{X1: 560, Y1: 240, X2: 1360, Y2: 840}, Ratio: 0.03, Key: "shift", Interval: time.Second,
		Colors: [][2][3]uint8{{{225, 195, 60}, {255, 255, 165}}}}
	if _, _, ok, _ := ext2.Step(fakeRuntime{}, assist.Frame{Image: empty, At: time.Now()}); ok {
		t.Fatal("没有闪光不该触发闪避")
	}
}

func TestQuickAssistExtension(t *testing.T) {
	ext := &zzzQuickAssistExt{
		Area:     screen.Area{Name: "支援按钮", Rect: geom.Rect{X1: 1580, Y1: 860, X2: 1780, Y2: 1050}, TemplateMatchThreshold: 0.85},
		SubDir:   "combat",
		IDs:      []string{"btn_assist_1", "btn_assist_2"},
		Key:      "space",
		Interval: time.Second,
	}
	rt := fakeRuntime{matches: map[string][]vision.Match{"btn_assist_1": {{Rect: geom.Rect{X1: 1600, Y1: 900, X2: 1678, Y2: 978}, Confidence: 0.95}}}}
	action, state, ok, err := ext.Step(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()})
	if err != nil || !ok || state != "快速支援" || action.Key != "space" {
		t.Fatalf("支援按钮命中应当按空格: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}
	if _, _, ok, _ := ext.Step(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()}); ok {
		t.Fatal("支援间隔内不该重复触发")
	}
}

func TestCombatFeatureHasExtensions(t *testing.T) {
	spec := combatSpec()
	if len(spec.Extensions) != 5 {
		t.Fatalf("扩展数量 = %d", len(spec.Extensions))
	}
	feature := combatFeature()
	settings := feature.DefaultSettings()
	for _, key := range []string{"ext_dodge_flash", "ext_quick_assist", "ext_chain", "ext_ultimate"} {
		if settings[key] != true {
			t.Fatalf("扩展开关 %s 默认应当打开: %+v", key, settings)
		}
	}
}

func TestChainExtension(t *testing.T) {
	ext := &zzzChainExt{
		Bar:      geom.Rect{X1: 813, Y1: 884, X2: 1098, Y2: 900},
		BarRGB:   [2][3]uint8{{200, 120, 40}, {255, 205, 120}},
		BarRate:  0.5,
		Left:     geom.Rect{X1: 479, Y1: 828, X2: 597, Y2: 946},
		Right:    geom.Rect{X1: 1322, Y1: 828, X2: 1440, Y2: 946},
		LeftKey:  "q",
		RightKey: "e",
		Interval: time.Second,
	}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// 连携条：金色横线
	for y := 886; y < 898; y++ {
		for x := 820; x < 1092; x++ {
			img.SetRGBA(x, y, color.RGBA{240, 170, 60, 255})
		}
	}
	// 左侧点亮
	for y := 850; y < 920; y++ {
		for x := 500; x < 580; x++ {
			img.SetRGBA(x, y, color.RGBA{250, 250, 240, 255})
		}
	}
	action, state, ok, err := ext.Step(fakeRuntime{}, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "连携技-左" || action.Key != "q" {
		t.Fatalf("连携条+左侧高亮应当按 q: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}
	// 没有连携条不触发
	empty := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	ext2 := *ext
	ext2.last = time.Time{}
	if _, _, ok, _ := ext2.Step(fakeRuntime{}, assist.Frame{Image: empty, At: time.Now()}); ok {
		t.Fatal("没有连携条不该触发")
	}
}

func TestUltimateExtension(t *testing.T) {
	ext := &zzzUltimateExt{
		Rect:      geom.Rect{X1: 1728, Y1: 778, X2: 1826, Y2: 876},
		ReadyRGB:  [2][3]uint8{{200, 200, 200}, {255, 255, 255}},
		ReadyRate: 0.35,
		Key:       "q",
		Interval:  time.Second,
	}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 790; y < 860; y++ {
		for x := 1740; x < 1814; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	action, state, ok, err := ext.Step(fakeRuntime{}, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "终结技" || action.Key != "q" {
		t.Fatalf("按钮点亮应当按 q: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}
	empty := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	ext2 := &zzzUltimateExt{Rect: ext.Rect, ReadyRGB: ext.ReadyRGB, ReadyRate: ext.ReadyRate, Key: "q", Interval: time.Second}
	if _, _, ok, _ := ext2.Step(fakeRuntime{}, assist.Frame{Image: empty, At: time.Now()}); ok {
		t.Fatal("按钮没亮不该按终结技")
	}
}

func TestZZZTeamTemplatesLoaded(t *testing.T) {
	team := newZZZTeam()
	if len(team.frontIDs) == 0 || len(team.backIDs) == 0 {
		t.Fatalf("头像模板没加载: front=%d back=%d", len(team.frontIDs), len(team.backIDs))
	}
}

func TestZZZTeamAgentMatchesFrontAvatar(t *testing.T) {
	team := newZZZTeam()
	target := team.frontIDs[0]
	rt := fakeRuntime{matches: map[string][]vision.Match{target: {{Rect: geom.Rect{X1: 104, Y1: 40, X2: 274, Y2: 110}, Confidence: 0.95}}}}
	id, ok, err := team.Agent(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()})
	if err != nil || !ok || id != target {
		t.Fatalf("应当识别到前台角色 %s, got %q ok=%v err=%v", target, id, ok, err)
	}
}

func TestZZZAgentPlansPresent(t *testing.T) {
	if len(agentPlans) == 0 {
		t.Fatal("缺少代理人专属连招")
	}
	for id, plan := range agentPlans {
		if len(plan) == 0 {
			t.Fatalf("代理人 %s 的连招为空", id)
		}
	}
}

// areaFake 按 (区域, 模板) 给命中结果，用来验证连携选边。
type areaFake struct {
	byRect map[geom.Rect]map[string]float64
}

func (f areaFake) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	if score, ok := f.byRect[area.Rect][ref.ID]; ok && score > 0 {
		return []vision.Match{{Rect: area.Rect, Confidence: score}}, nil
	}
	return nil, nil
}

func TestChainPicksSideByRole(t *testing.T) {
	left := geom.Rect{X1: 479, Y1: 828, X2: 597, Y2: 946}
	right := geom.Rect{X1: 1322, Y1: 828, X2: 1440, Y2: 946}
	ext := &zzzChainExt{
		Bar:       geom.Rect{X1: 813, Y1: 884, X2: 1098, Y2: 900},
		BarRGB:    [2][3]uint8{{200, 120, 40}, {255, 205, 120}},
		BarRate:   0.5,
		Left:      left,
		Right:     right,
		LeftKey:   "q",
		RightKey:  "e",
		Interval:  time.Second,
		avatarIDs: []string{"ellen", "lycaon"},
		threshold: 0.78,
	}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 886; y < 898; y++ {
		for x := 820; x < 1092; x++ {
			img.SetRGBA(x, y, color.RGBA{240, 170, 60, 255})
		}
	}
	// 左边是副C（莱卡恩），右边是主C（艾莲）-> 应当选右边。
	rt := areaFake{byRect: map[geom.Rect]map[string]float64{
		left:  {"lycaon": 0.9},
		right: {"ellen": 0.9},
	}}
	action, state, ok, err := ext.Step(rt, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "连携技-右" || action.Key != "e" {
		t.Fatalf("应当选主C那一侧: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 两边都认不出 -> 退回亮度（都暗默认左边）。
	rt2 := areaFake{byRect: map[geom.Rect]map[string]float64{}}
	ext2 := *ext
	ext2.last = time.Time{}
	action, state, ok, err = ext2.Step(rt2, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "连携技-左" || action.Key != "q" {
		t.Fatalf("认不出时退回左侧: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}
}

func TestQuickAssistTargetsLitSlot(t *testing.T) {
	slots := []geom.Rect{
		{X1: 104, Y1: 40, X2: 274, Y2: 110},
		{X1: 559, Y1: 40, X2: 662, Y2: 91},
		{X1: 741, Y1: 40, X2: 844, Y2: 91},
	}
	assistArea := screen.Area{Name: "支援按钮", Rect: geom.Rect{X1: 1580, Y1: 860, X2: 1780, Y2: 1050}}
	ext := &zzzQuickAssistExt{
		Area:     assistArea,
		SubDir:   "combat",
		IDs:      []string{"btn_assist_1"},
		Key:      "space",
		Slots:    slots,
		Keys:     []string{"1", "2", "3"},
		Interval: time.Second,
	}
	// 3 号位（第三人）头像亮起 -> 应当按 3。
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 45; y < 86; y++ {
		for x := 745; x < 840; x++ {
			img.SetRGBA(x, y, color.RGBA{250, 240, 200, 255})
		}
	}
	rt := areaFake{byRect: map[geom.Rect]map[string]float64{assistArea.Rect: {"btn_assist_1": 0.95}}}
	action, state, ok, err := ext.Step(rt, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "快速支援" || action.Key != "3" {
		t.Fatalf("应当按点亮的三号位: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 全暗 -> 退回兜底键。
	ext2 := *ext
	ext2.last = time.Time{}
	empty := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	action, _, ok, _ = ext2.Step(rt, assist.Frame{Image: empty, At: time.Now()})
	if !ok || action.Key != "space" {
		t.Fatalf("认不出槽位时应退回兜底键: action=%+v ok=%v", action, ok)
	}
}

func TestStateRuleFiresWhenUltReady(t *testing.T) {
	front := geom.Rect{X1: 104, Y1: 40, X2: 274, Y2: 110}
	ultArea := geom.Rect{X1: 1728, Y1: 778, X2: 1826, Y2: 876}
	skillArea := geom.Rect{X1: 1620, Y1: 917, X2: 1720, Y2: 1017}
	team := &zzzTeam{
		front:     screen.Area{Name: "前台头像", Rect: front},
		frontIDs:  []string{"grace"},
		threshold: 0.8,
	}
	ext := &zzzStateRuleExt{Team: team, SkillArea: skillArea, UltArea: ultArea, Threshold: 0.05}

	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// 终结技按钮亮起
	for y := 790; y < 860; y++ {
		for x := 1740; x < 1814; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	rt := areaFake{byRect: map[geom.Rect]map[string]float64{front: {"grace": 0.95}}}
	action, state, ok, err := ext.Step(rt, assist.Frame{Image: img, At: time.Now()})
	if err != nil || !ok || state != "规则-终结技" || action.Key != "q" {
		t.Fatalf("终结技可用时应当按规则放终结技: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 按钮不亮 -> 终结技规则不成立（该角色还配了"特殊技不可用"这样的反向规则，属正常）。
	ext2 := &zzzStateRuleExt{Team: team, SkillArea: skillArea, UltArea: ultArea, Threshold: 0.05}
	dark := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	if _, state, ok, _ := ext2.Step(rt, assist.Frame{Image: dark, At: time.Now()}); ok && state == "规则-终结技" {
		t.Fatal("按钮没亮时不该命中终结技规则")
	}
}

func TestExtractedRulesShape(t *testing.T) {
	if len(agentRules) < 20 {
		t.Fatalf("规则数量太少: %d 个代理人", len(agentRules))
	}
	total := 0
	for id, rules := range agentRules {
		if len(rules) == 0 {
			t.Fatalf("%s 没有规则", id)
		}
		for _, rule := range rules {
			if rule.Cond == "" || len(rule.Ops) == 0 {
				t.Fatalf("%s 有空规则: %+v", id, rule)
			}
			total++
		}
	}
	if total < 60 {
		t.Fatalf("规则总数太少: %d", total)
	}
}

func TestStateCheckAtomUsesColorRange(t *testing.T) {
	// 雅-落霜：颜色范围取自上游 agent.py
	check, ok := agentStateChecks["雅-落霜"]
	if !ok {
		t.Skip("没有该状态定义")
	}
	e := &zzzStateRuleExt{StateArea: geom.Rect{X1: 40, Y1: 940, X2: 420, Y2: 1070}}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// 画两块符合颜色范围的色块，满足连通块数量要求。
	for i := 0; i < 3; i++ {
		x0 := 60 + i*60
		for y := 960; y < 1000; y++ {
			for x := x0; x < x0+40; x++ {
				img.SetRGBA(x, y, color.RGBA{check.Lower[0], check.Lower[1], check.Lower[2], 255})
			}
		}
	}
	frame := assist.Frame{Image: img, At: time.Now()}
	if !e.atom(nil, frame, "雅-落霜") {
		t.Fatalf("颜色块满足条件时该状态应成立: %+v", check)
	}

	// 同色块放到区域外就不成立。
	empty := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	if e.atom(nil, assist.Frame{Image: empty, At: time.Now()}, "雅-落霜") {
		t.Fatal("没有色块时不该成立")
	}
}
