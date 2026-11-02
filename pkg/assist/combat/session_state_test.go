package combat

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestSessionFlyingState 验证上游 BaseChar.flying() 的移植：
// 有滑翔装置模板时，图标区域白色占比 < 0.1 算在空中。
func TestSessionFlyingState(t *testing.T) {
	spec := testSpec()
	spec.Flying = &FlyingCheck{
		Area:              geom.Rect{X1: 10, Y1: 10, X2: 20, Y2: 20},
		ColorRange:        [2][3]uint8{{244, 244, 244}, {255, 255, 255}},
		MaxRatio:          0.1,
		TemplateArea:      geom.Rect{X1: 0, Y1: 0, X2: 30, Y2: 30},
		TemplateSubDir:    "flying",
		TemplateID:        "edge_levitator",
		TemplateThreshold: 0.65,
	}
	rt := testRuntime(true)
	rt.scores = map[string]float64{"edge_levitator": 0.8}
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	dark := assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 40, 40)), At: time.Now()}
	if !session.flying(dark) {
		t.Fatal("黑色图标区应当算在空中")
	}
	lit := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 10; y < 20; y++ {
		for x := 10; x < 20; x++ {
			lit.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	if session.flying(assist.Frame{Image: lit, At: time.Now()}) {
		t.Fatal("白色图标区不该算在空中")
	}
	// 没有滑翔装置模板 -> 永远不判空中。
	rt2 := testRuntime(false)
	session2 := newSession(rt2, spec.withDefaults(), sessionSettings{})
	if session2.flying(dark) {
		t.Fatal("没有滑翔装置时不该判空中")
	}
}

// TestSessionLiberationWindow 验证上游 still_in_liberation() 的窗口：
// 按解放后按角色时长判断，默认 12 秒。
func TestSessionLiberationWindow(t *testing.T) {
	spec := testSpec()
	spec.Team = stubTeam{}
	spec.LiberationWindows = map[string]time.Duration{"agent-x": time.Second}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.noteLiberation(frameAt(base), base)
	if !session.inLiberation(frameAt(base.Add(500*time.Millisecond)), base.Add(500*time.Millisecond)) {
		t.Fatal("窗口内应算解放中")
	}
	if session.inLiberation(frameAt(base.Add(1500*time.Millisecond)), base.Add(1500*time.Millisecond)) {
		t.Fatal("超过窗口不应算解放中")
	}
	// 默认窗口 12 秒。
	spec2 := testSpec()
	spec2.Team = stubTeam{}
	session2 := newSession(testRuntime(true), spec2.withDefaults(), sessionSettings{})
	session2.noteLiberation(frameAt(base), base)
	if !session2.inLiberation(frameAt(base.Add(11*time.Second)), base.Add(11*time.Second)) {
		t.Fatal("默认窗口 12 秒内应算解放中")
	}
	if session2.inLiberation(frameAt(base.Add(13*time.Second)), base.Add(13*time.Second)) {
		t.Fatal("默认窗口超过 12 秒不应算解放中")
	}
}
