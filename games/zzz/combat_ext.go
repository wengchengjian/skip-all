package zzz

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// zzzDodgeExt 是绝区零的闪光闪避。
// 上游用 YOLO 模型分类红光/黄光（auto_battle_dodge_context.check_dodge_flash），
// 这里用颜色占比近似：屏幕中央出现大片黄/红闪光就按闪避键。
type zzzDodgeExt struct {
	Rect   geom.Rect
	Colors [][2][3]uint8
	Ratio  float64
	Key    string
	// Interval 是两次闪避之间的最小间隔，避免同一段闪光连按。
	Interval time.Duration
	last     time.Time
}

func (d *zzzDodgeExt) Name() string { return "dodge_flash" }

func (d *zzzDodgeExt) Step(_ combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if !d.last.IsZero() && now.Sub(d.last) < d.Interval {
		return assist.Action{}, "", false, nil
	}
	for _, c := range d.Colors {
		if vision.ColorRatio(frame.Image, d.Rect, c) >= d.Ratio {
			d.last = now
			return assist.Action{Kind: assist.ActionKeyTap, Key: d.Key}, "闪避-闪光", true, nil
		}
	}
	return assist.Action{}, "", false, nil
}

// zzzQuickAssistExt 是绝区零的快速支援/招架支援。
// 上游在支援按钮可用时按"按键-快速支援"（快速支援模板-全角色.sample.yml）。
// 这里直接匹配支援按钮模板（btn_assist_1/2），命中就按切换键。
type zzzQuickAssistExt struct {
	Area     screen.Area
	SubDir   string
	IDs      []string
	Key      string
	Interval time.Duration
	last     time.Time
}

func (a *zzzQuickAssistExt) Name() string { return "quick_assist" }

func (a *zzzQuickAssistExt) Step(rt combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if !a.last.IsZero() && now.Sub(a.last) < a.Interval {
		return assist.Action{}, "", false, nil
	}
	for _, id := range a.IDs {
		matches, err := rt.MatchTemplate(frame, a.Area, vision.TemplateRef{SubDir: a.SubDir, ID: id}, a.Area.TemplateMatchThreshold)
		if err != nil {
			return assist.Action{}, "", false, err
		}
		if len(matches) > 0 {
			a.last = now
			return assist.Action{Kind: assist.ActionKeyTap, Key: a.Key}, "快速支援", true, nil
		}
	}
	return assist.Action{}, "", false, nil
}

// zzzExtensions 返回绝区零专属的战斗能力。
func zzzExtensions() []combat.Extension {
	return []combat.Extension{
		&zzzDodgeExt{
			// 闪光出现在角色周围，取屏幕中央一块；比值是经验值，实机可调。
			Rect:     geom.Rect{X1: 560, Y1: 240, X2: 1360, Y2: 840},
			Ratio:    0.03,
			Key:      "shift",
			Interval: 600 * time.Millisecond,
			Colors: [][2][3]uint8{
				{{225, 195, 60}, {255, 255, 165}}, // 黄光
				{{215, 35, 35}, {255, 110, 110}},  // 红光
			},
		},
		&zzzQuickAssistExt{
			Area: screen.Area{
				Name:                   "支援按钮",
				Type:                   screen.AreaTemplate,
				Rect:                   geom.Rect{X1: 1580, Y1: 860, X2: 1780, Y2: 1050},
				TemplateMatchThreshold: 0.85,
			},
			SubDir:   "combat",
			IDs:      []string{"btn_assist_1", "btn_assist_2"},
			Key:      "space",
			Interval: 800 * time.Millisecond,
		},
	}
}
