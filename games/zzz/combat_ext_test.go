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
	if len(spec.Extensions) != 2 {
		t.Fatalf("扩展数量 = %d", len(spec.Extensions))
	}
	feature := combatFeature()
	settings := feature.DefaultSettings()
	if settings["ext_dodge_flash"] != true || settings["ext_quick_assist"] != true {
		t.Fatalf("扩展开关默认应当打开: %+v", settings)
	}
}
