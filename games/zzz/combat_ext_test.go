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
	if len(spec.Extensions) != 4 {
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
