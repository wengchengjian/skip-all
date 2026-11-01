package wuwa

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// wuwaForteExt 是鸣潮的共鸣回路提示。
// 上游 BaseChar 的 is_mouse_forte_full()/is_e_forte_full() 在血条下方的回路提示位
// 匹配 mouse_forte / e_forte 两个图标：出现就说明"重击/强化共鸣技能"能打了。
// 对应动作：E 图标 -> 按共鸣技能；鼠标图标 -> 点普攻（重击）。
type wuwaForteExt struct {
	Area        screen.Area
	SubDir      string
	EID         string
	MouseID     string
	SkillKey    string
	AttackPoint geom.Point
	Threshold   float64
	Interval    time.Duration
	last        time.Time
}

func (e *wuwaForteExt) Name() string { return "forte" }

func (e *wuwaForteExt) Step(rt combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if !e.last.IsZero() && now.Sub(e.last) < e.Interval {
		return assist.Action{}, "", false, nil
	}
	threshold := e.Threshold
	if threshold <= 0 {
		threshold = 0.6
	}
	// 先看 E 图标：强化共鸣技能优先。
	if matches, err := rt.MatchTemplate(frame, e.Area, vision.TemplateRef{SubDir: e.SubDir, ID: e.EID}, threshold); err != nil {
		return assist.Action{}, "", false, err
	} else if len(matches) > 0 {
		e.last = now
		return assist.Action{Kind: assist.ActionKeyTap, Key: e.SkillKey}, "回路-E", true, nil
	}
	// 再看鼠标图标：重击。
	if matches, err := rt.MatchTemplate(frame, e.Area, vision.TemplateRef{SubDir: e.SubDir, ID: e.MouseID}, threshold); err != nil {
		return assist.Action{}, "", false, err
	} else if len(matches) > 0 {
		e.last = now
		return assist.Action{Kind: assist.ActionClick, Point: e.AttackPoint}, "回路-重击", true, nil
	}
	return assist.Action{}, "", false, nil
}

// wuwaExtensions 返回鸣潮专属的战斗能力。
func wuwaExtensions() []combat.Extension {
	return []combat.Extension{
		&wuwaForteExt{
			Area:        screen.Area{Name: "回路提示", Rect: geom.Rect{X1: 1080, Y1: 950, X2: 1220, Y2: 1045}},
			SubDir:      "forte",
			EID:         "e_forte",
			MouseID:     "mouse_forte",
			SkillKey:    "e",
			AttackPoint: geom.Point{X: 960, Y: 540},
			Threshold:   0.6,
			Interval:    500 * time.Millisecond,
		},
	}
}
