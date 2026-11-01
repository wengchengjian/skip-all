package zzz

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// CombatFeature 创建绝区零自动战斗功能。
func CombatFeature() assist.Feature { return combatFeature() }

func combatFeature() *combat.Feature { return combat.NewFeature(combatSpec()) }

// combatSpec 的按键与判据来自上游 ZenlessZoneZero-OneDragon：
// 普通攻击 = 鼠标左键，闪避 = Shift，特殊攻击 = E，终结技 = Q，
// 角色切换-下一个 = 空格（zzz_od/config/game_config.py 的 _KEY_DEFAULTS）。
// 连招由通用 Plan 驱动；终结技/连携/支援/闪光闪避放在 zzzExtensions() 里，
// 它们抢在通用连招之前执行。
// 战斗判据照它 is_normal_attack_btn_available 的做法：普通攻击按钮
// （battle/btn_normal_attack，带 mask）在战斗画面 (1403,916)-(1505,1017)
// 里匹配到就说明在战斗，阈值 0.9。
func combatSpec() combat.Spec {
	// Team 与规则引擎共用同一个实例：规则需要知道当前站场的是谁。
	team := newZZZTeam()
	return combat.Spec{
		Screen:      "绝区零战斗",
		DetectAreas: []string{"普通攻击按钮"},
		AttackPoint: geom.Point{X: 960, Y: 540},
		Keys: combat.Keys{
			Skill:      "e",
			Liberation: "q",
			Dodge:      "shift",
			Switch:     []string{"space"},
		},
		// 技能就绪判据：特殊攻击 / 终结技按钮亮起才放（上游按角色属性判定，这里按亮度近似）。
		Ready: map[combat.StepKind]combat.ReadyCheck{
			combat.StepSkill:      {Area: geom.Rect{X1: 1620, Y1: 917, X2: 1720, Y2: 1017}, ColorRange: readyBright, MinRatio: 0.05},
			combat.StepLiberation: {Area: geom.Rect{X1: 1728, Y1: 778, X2: 1826, Y2: 876}, ColorRange: readyBright, MinRatio: 0.05},
		},
		// 冷却数字：特殊技/终结技按钮上会显示剩余秒数（上游 has_cd 的同款判断）。
		Cooldowns: map[combat.StepKind]geom.Rect{
			combat.StepSkill:      {X1: 1620, Y1: 917, X2: 1720, Y2: 1017},
			combat.StepLiberation: {X1: 1728, Y1: 778, X2: 1826, Y2: 876},
		},
		// 复活/失败界面出现时暂停出招（绝区零没有鸣潮那种入场技，IntroSteps 留空）。
		AbortArea:  "中断提示",
		AbortTexts: []string{"重新挑战", "挑战失败", "撤退", "复活"},
		// 代理人定位来自上游 AgentTypeEnum，切人时按定位打分。
		Roles:  agentRoles,
		Switch: &combat.SwitchPolicy{MinInterval: 3 * time.Second},
		// 认人 + 角色专属连招（模板来自上游 avatar_1_*/avatar_2_*）。
		Team:         team,
		PlansByAgent: agentPlans,
		// 绝区零专属能力：闪光闪避、快速支援（通用流程先问它们，再走连招）。
		Extensions: zzzExtensions(team),
		Plan: []combat.Step{
			{Kind: combat.StepAttack},
			{Kind: combat.StepAttack},
			{Kind: combat.StepSkill},
			{Kind: combat.StepAttack},
			{Kind: combat.StepAttack},
			{Kind: combat.StepDodge},
		},
	}
}

// readyBright 是"按钮点亮"的色域。
var readyBright = [2][3]uint8{{210, 200, 180}, {255, 255, 255}}
