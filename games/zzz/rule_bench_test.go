package zzz

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// BenchmarkRuleEngineStep 量规则引擎单帧的开销（369 条规则 + 条件求值）。
func BenchmarkRuleEngineStep(b *testing.B) {
	front := geom.Rect{X1: 104, Y1: 40, X2: 274, Y2: 110}
	team := &zzzTeam{
		front:     screen.Area{Name: "前台头像", Rect: front},
		frontIDs:  avatarIDs("avatar1"),
		threshold: 0.8,
	}
	ext := &zzzStateRuleExt{
		Team:       team,
		SkillArea:  geom.Rect{X1: 1620, Y1: 917, X2: 1720, Y2: 1017},
		UltArea:    geom.Rect{X1: 1728, Y1: 778, X2: 1826, Y2: 876},
		Threshold:  0.05,
		StateArea:  geom.Rect{X1: 40, Y1: 940, X2: 420, Y2: 1070},
		FlashArea:  geom.Rect{X1: 560, Y1: 240, X2: 1360, Y2: 840},
		ChainBar:   geom.Rect{X1: 813, Y1: 884, X2: 1098, Y2: 900},
		ChainRGB:   [2][3]uint8{{200, 120, 40}, {255, 205, 120}},
		ChainRate:  0.5,
		AssistArea: geom.Rect{X1: 1615, Y1: 776, X2: 1726, Y2: 877},
		AssistIDs:  []string{"btn_assist_1", "btn_assist_2"},
	}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y += 9 {
		for x := 0; x < 1920; x += 7 {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 120, 255})
		}
	}
	frame := assist.Frame{Image: img, At: time.Now()}
	rt := newBenchRuntime()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame.At = frame.At.Add(100 * time.Millisecond)
		_, _, _, _ = ext.Step(rt, frame)
	}
}
