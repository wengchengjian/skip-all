package zzz

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// zzzDodgeExt 是绝区零的闪避/格挡，照上游 config/dodge/智能格挡.sample.yml：
//   - 黄光：切到"支援攻击"优先级最高的后台角色做格挡，再补普攻
//     （上游优先级：特殊情况 > 击破 > 命破 > 防护 > 异常 > 下一个）；
//   - 红光：闪避，再补普攻（闪A）。
//
// 上游把这两段写成 triggers -> operations，这里压成一个小队列，每次 Step 出一手。
type zzzDodgeExt struct {
	Rect     geom.Rect
	Yellow   [2][3]uint8
	Red      [2][3]uint8
	Ratio    float64
	DodgeKey string
	// Slots 是三位角色的头像区域，用来认清后台有谁；Keys 是对应槽位键。
	Slots []geom.Rect
	Keys  []string
	// KnownIDs 返回已认出的代理人模板：有白名单就只比这几张，不扫 71 张。
	KnownIDs func() []string
	// SlotIDs 直接返回三个槽位上的代理人（复用队伍识别结果，避免重复扫头像）。
	SlotIDs     func(rt combat.Runtime, frame assist.Frame) ([]string, error)
	AttackPoint geom.Point
	// Interval 是两次闪避之间的最小间隔，避免同一段闪光连按。
	Interval time.Duration
	last     time.Time
	queue    []assist.Action
}

func (d *zzzDodgeExt) Name() string { return "dodge_flash" }

func (d *zzzDodgeExt) Step(rt combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	// 队列里还有后续动作（格挡换人后的普攻）就先打完。
	if len(d.queue) > 0 {
		action := d.queue[0]
		d.queue = d.queue[1:]
		return action, "格挡-连招", true, nil
	}
	if !d.last.IsZero() && now.Sub(d.last) < d.Interval {
		return assist.Action{}, "", false, nil
	}
	ratio := d.Ratio
	if ratio <= 0 {
		ratio = 0.03
	}
	yellow := vision.ColorRatio(frame.Image, d.Rect, d.Yellow) >= ratio
	red := vision.ColorRatio(frame.Image, d.Rect, d.Red) >= ratio
	if !yellow && !red {
		return assist.Action{}, "", false, nil
	}
	d.last = now
	attack := assist.Action{Kind: assist.ActionClick, Point: d.AttackPoint}
	if yellow {
		// 黄光是格挡时机：按"支援攻击"优先级挑一个后台角色换上来。
		if key := d.parryTarget(rt, frame); key != "" {
			d.queue = append(d.queue, attack)
			return assist.Action{Kind: assist.ActionKeyTap, Key: key}, "格挡-切人", true, nil
		}
	}
	d.queue = append(d.queue, attack)
	return assist.Action{Kind: assist.ActionKeyTap, Key: d.DodgeKey}, "闪避", true, nil
}

// parryTarget 按上游"支援攻击"的定位优先级挑后台角色，返回它的槽位键。
func (d *zzzDodgeExt) parryTarget(rt combat.Runtime, frame assist.Frame) string {
	if len(d.Slots) == 0 || len(d.Keys) == 0 {
		return ""
	}
	best := ""
	bestScore := -1
	// 优先复用队伍识别结果（已被缓存），没有再自己扫。
	if d.SlotIDs != nil {
		if ids, err := d.SlotIDs(rt, frame); err == nil && len(ids) >= len(d.Keys) {
			best, bestScore := "", -1
			for i := 1; i < len(ids) && i < len(d.Keys); i++ {
				if score := parryScore(ids[i]); score > bestScore {
					bestScore, best = score, d.Keys[i]
				}
			}
			if bestScore >= 0 {
				return best
			}
			return ""
		}
	}
	candidates := avatarIDs("avatar1")
	if d.KnownIDs != nil {
		if known := d.KnownIDs(); len(known) > 0 {
			candidates = known
		}
	}
	ids := make([]string, len(d.Slots))
	for i, slot := range d.Slots {
		bestConf, bestID := 0.0, ""
		for _, id := range candidates {
			matches, err := rt.MatchTemplate(frame, screen.Area{Name: "角色头像", Rect: slot}, vision.TemplateRef{SubDir: "avatar1", ID: id}, 0.78)
			if err != nil {
				continue
			}
			if len(matches) > 0 && matches[0].Confidence > bestConf {
				bestConf, bestID = matches[0].Confidence, id
			}
		}
		ids[i] = bestID
	}
	for i := 1; i < len(ids) && i < len(d.Keys); i++ { // 后台两位
		score := parryScore(ids[i])
		if score > bestScore {
			bestScore, best = score, d.Keys[i]
		}
	}
	if bestScore < 0 {
		return ""
	}
	return best
}

// parryScore 是上游 支援攻击模板 的定位优先级：击破 > 命破 > 防护 > 异常 > 其它。
func parryScore(id string) int {
	switch {
	case agentsByType[AgentTypeStun][id]:
		return 5
	case agentsByType[AgentTypeRupture][id]:
		return 4
	case agentsByType[AgentTypeDefense][id]:
		return 3
	case agentsByType[AgentTypeAnomaly][id]:
		return 2
	case id != "":
		return 1
	}
	return 0
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
	// knownIDs 返回已经认出来的代理人模板：有白名单就只比这几张，
	// 不再对 71 张头像逐个匹配。
	knownIDs  func() []string
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
	ids := c.avatarIDs
	if c.knownIDs != nil {
		if known := c.knownIDs(); len(known) > 0 {
			ids = known
		}
	}
	for _, id := range ids {
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
			// 角色能量/层数条在血条下方一带（经验值，实机可调）。
			StateArea: geom.Rect{X1: 40, Y1: 940, X2: 420, Y2: 1070},
			FlashArea: geom.Rect{X1: 560, Y1: 240, X2: 1360, Y2: 840},
			// 连携条可见 = 敌人失衡。
			ChainBar:  geom.Rect{X1: 813, Y1: 884, X2: 1098, Y2: 900},
			ChainRGB:  [2][3]uint8{{200, 120, 40}, {255, 205, 120}},
			ChainRate: 0.5,
			// 快速支援按钮：上游 按键-切换后援 (1615,776)-(1726,877)。
			AssistArea: geom.Rect{X1: 1615, Y1: 776, X2: 1726, Y2: 877},
			AssistIDs:  []string{"btn_assist_1", "btn_assist_2"},
		},
		&zzzDodgeExt{
			// 闪光出现在角色周围，取屏幕中央一块；比值是经验值，实机可调。
			Rect:     geom.Rect{X1: 560, Y1: 240, X2: 1360, Y2: 840},
			Yellow:   [2][3]uint8{{225, 195, 60}, {255, 255, 165}},
			Red:      [2][3]uint8{{215, 35, 35}, {255, 110, 110}},
			Ratio:    0.03,
			DodgeKey: "shift",
			Slots: []geom.Rect{
				{X1: 104, Y1: 40, X2: 274, Y2: 110},
				{X1: 559, Y1: 40, X2: 662, Y2: 91},
				{X1: 741, Y1: 40, X2: 844, Y2: 91},
			},
			Keys:        []string{"1", "2", "3"},
			KnownIDs:    team.LearnedIDs,
			SlotIDs:     team.Slots,
			AttackPoint: geom.Point{X: 960, Y: 540},
			Interval:    600 * time.Millisecond,
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
			knownIDs:  team.LearnedIDs,
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
