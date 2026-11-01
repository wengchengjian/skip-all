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
	Area   screen.Area
	SubDir string
	IDs    []string
	// Key 是兜底按键（认不出哪个槽位挂着支援时用）。
	Key string
	// Slots/Keys 是三位角色的头像区域与对应按键：支援灯亮在谁身上就把谁换上来
	// （上游用 按键-切换角色 + agent_name 做同一件事）。
	Slots    []geom.Rect
	Keys     []string
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
			return assist.Action{Kind: assist.ActionKeyTap, Key: a.assistTarget(frame)}, "快速支援", true, nil
		}
	}
	return assist.Action{}, "", false, nil
}

// zzzChainExt 是绝区零的连携技。
// 上游：连携条(813,889)-(1098,890) 的 HSV 金色线命中说明进入连携选择，
// 左右两侧 连携技-1/2 区域里高亮的那一侧对应 按键-连携技-左(q) / 右(e)。
type zzzChainExt struct {
	Bar      geom.Rect
	BarRGB   [2][3]uint8
	BarRate  float64
	Left     geom.Rect
	Right    geom.Rect
	LeftKey  string
	RightKey string
	Interval time.Duration
	last     time.Time
	// avatarIDs 是连携槽位里要试的头像模板（与 templates/avatar1 一致）。
	avatarIDs []string
	threshold float64
}

func (c *zzzChainExt) Name() string { return "chain" }

func (c *zzzChainExt) Step(rt combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if !c.last.IsZero() && now.Sub(c.last) < c.Interval {
		return assist.Action{}, "", false, nil
	}
	if vision.ColorRatio(frame.Image, c.Bar, c.BarRGB) < c.BarRate {
		return assist.Action{}, "", false, nil
	}
	c.last = now

	// 先认人：两边分别匹配头像模板，取分数最高的 id。
	leftID := c.matchSide(rt, frame, c.Left)
	rightID := c.matchSide(rt, frame, c.Right)
	leftScore := chainRoleScore(leftID)
	rightScore := chainRoleScore(rightID)
	if leftScore != rightScore {
		if rightScore > leftScore {
			return assist.Action{Kind: assist.ActionKeyTap, Key: c.RightKey}, "连携技-右", true, nil
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: c.LeftKey}, "连携技-左", true, nil
	}

	// 认不出来或定位同分：退回比亮度，两侧都暗默认左边。
	left := vision.ColorRatio(frame.Image, c.Left, brightRange)
	right := vision.ColorRatio(frame.Image, c.Right, brightRange)
	key, state := c.LeftKey, "连携技-左"
	if right > left {
		key, state = c.RightKey, "连携技-右"
	}
	return assist.Action{Kind: assist.ActionKeyTap, Key: key}, state, true, nil
}

// matchSide 在一个连携槽位里认人，返回匹配度最高的代理人 id。
func (c *zzzChainExt) matchSide(rt combat.Runtime, frame assist.Frame, area geom.Rect) string {
	best, bestID := 0.0, ""
	threshold := c.threshold
	if threshold <= 0 {
		threshold = 0.8
	}
	for _, id := range c.avatarIDs {
		matches, err := rt.MatchTemplate(frame, screen.Area{Name: "连携槽位", Rect: area}, vision.TemplateRef{SubDir: "avatar1", ID: id}, threshold)
		if err != nil || len(matches) == 0 {
			continue
		}
		if matches[0].Confidence > best {
			best, bestID = matches[0].Confidence, id
		}
	}
	return bestID
}

// chainRoleScore 决定连携时优先让谁出场：主C最高，其次副C，辅助/治疗最低。
func chainRoleScore(id string) int {
	switch agentRoles[id] {
	case combat.RoleMainDPS:
		return 3
	case combat.RoleSubDPS:
		return 2
	case combat.RoleSupport:
		return 1
	case combat.RoleHealer:
		return 1
	}
	return 0
}

// zzzUltimateExt 是绝区零的终结技：只有按钮点亮（可用）时才按。
// 上游按角色属性分别判定（agent.py 的 ULTIMATE_*），这里统一按按钮亮度近似。
type zzzUltimateExt struct {
	Rect      geom.Rect
	ReadyRGB  [2][3]uint8
	ReadyRate float64
	Key       string
	Interval  time.Duration
	last      time.Time
}

func (u *zzzUltimateExt) Name() string { return "ultimate" }

func (u *zzzUltimateExt) Step(_ combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if !u.last.IsZero() && now.Sub(u.last) < u.Interval {
		return assist.Action{}, "", false, nil
	}
	if vision.ColorRatio(frame.Image, u.Rect, u.ReadyRGB) < u.ReadyRate {
		return assist.Action{}, "", false, nil
	}
	u.last = now
	return assist.Action{Kind: assist.ActionKeyTap, Key: u.Key}, "终结技", true, nil
}

// brightRange 是"亮起来"的通用色域（按钮高亮、头像点亮）。
var brightRange = [2][3]uint8{{210, 200, 180}, {255, 255, 255}}

// assistTarget 找支援灯亮得最明显的槽位，返回它的按键；认不出就退回兜底键。
func (a *zzzQuickAssistExt) assistTarget(frame assist.Frame) string {
	best, bestKey := 0.0, ""
	for i, slot := range a.Slots {
		if i >= len(a.Keys) {
			break
		}
		ratio := vision.ColorRatio(frame.Image, slot, brightRange)
		if ratio > best {
			best, bestKey = ratio, a.Keys[i]
		}
	}
	if best < 0.15 || bestKey == "" {
		return a.Key
	}
	return bestKey
}

// zzzExtensions 返回绝区零专属的战斗能力。
func zzzExtensions(team *zzzTeam) []combat.Extension {
	return []combat.Extension{
		// 上游 state_handler 的"条件 -> 动作"规则，优先于通用连招。
		&zzzStateRuleExt{
			Team:      team,
			SkillArea: geom.Rect{X1: 1620, Y1: 917, X2: 1720, Y2: 1017},
			UltArea:   geom.Rect{X1: 1728, Y1: 778, X2: 1826, Y2: 876},
			Threshold: 0.05,
		},
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
		&zzzChainExt{
			Bar:       geom.Rect{X1: 813, Y1: 884, X2: 1098, Y2: 900},
			BarRGB:    [2][3]uint8{{200, 120, 40}, {255, 205, 120}},
			BarRate:   0.5,
			Left:      geom.Rect{X1: 479, Y1: 828, X2: 597, Y2: 946},
			Right:     geom.Rect{X1: 1322, Y1: 828, X2: 1440, Y2: 946},
			LeftKey:   "q",
			RightKey:  "e",
			Interval:  900 * time.Millisecond,
			avatarIDs: avatarIDs("avatar1"),
			threshold: 0.78,
		},
		&zzzUltimateExt{
			Rect:      geom.Rect{X1: 1728, Y1: 778, X2: 1826, Y2: 876},
			ReadyRGB:  [2][3]uint8{{200, 200, 200}, {255, 255, 255}},
			ReadyRate: 0.35,
			Key:       "q",
			Interval:  1200 * time.Millisecond,
		},
		&zzzQuickAssistExt{
			Area: screen.Area{
				Name:                   "支援按钮",
				Type:                   screen.AreaTemplate,
				Rect:                   geom.Rect{X1: 1580, Y1: 860, X2: 1780, Y2: 1050},
				TemplateMatchThreshold: 0.85,
			},
			SubDir: "combat",
			IDs:    []string{"btn_assist_1", "btn_assist_2"},
			Key:    "space",
			Slots: []geom.Rect{
				{X1: 104, Y1: 40, X2: 274, Y2: 110},
				{X1: 559, Y1: 40, X2: 662, Y2: 91},
				{X1: 741, Y1: 40, X2: 844, Y2: 91},
			},
			Keys:     []string{"1", "2", "3"},
			Interval: 800 * time.Millisecond,
		},
	}
}
