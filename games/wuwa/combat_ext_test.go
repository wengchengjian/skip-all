package wuwa

import (
	"image"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// forteFake 按模板 id 返回命中，用来验证回路提示的优先级。
type forteFake struct {
	hit map[string]bool
}

func (f forteFake) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	if f.hit[ref.ID] {
		return []vision.Match{{Rect: area.Rect, Confidence: 0.9}}, nil
	}
	return nil, nil
}

func TestForteExtensionPriority(t *testing.T) {
	ext := &wuwaForteExt{
		Area:        screen.Area{Name: "回路提示"},
		SubDir:      "forte",
		EID:         "e_forte",
		MouseID:     "mouse_forte",
		SkillKey:    "e",
		Threshold:   0.6,
		Interval:    500 * time.Millisecond,
		AttackPoint: geom.Point{X: 960, Y: 540},
	}
	frame := assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()}

	// 两个图标都在 -> 先按 E（强化共鸣技能）。
	action, state, ok, err := ext.Step(forteFake{hit: map[string]bool{"e_forte": true, "mouse_forte": true}}, frame)
	if err != nil || !ok || state != "回路-E" || action.Key != "e" {
		t.Fatalf("应当先打强化E: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 只有鼠标图标 -> 重击。
	ext2 := *ext
	ext2.last = time.Time{}
	action, state, ok, err = ext2.Step(forteFake{hit: map[string]bool{"mouse_forte": true}}, frame)
	if err != nil || !ok || state != "回路-重击" || action.Kind != assist.ActionClick {
		t.Fatalf("应当重击: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 都没有 -> 不出手。
	ext3 := *ext
	ext3.last = time.Time{}
	if _, _, ok, _ := ext3.Step(forteFake{hit: map[string]bool{}}, frame); ok {
		t.Fatal("没有回路提示时不该出手")
	}
}
