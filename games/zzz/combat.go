package zzz

import (
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
		// 绝区零专属能力：闪光闪避、快速支援（通用流程先问它们，再走连招）。
		Extensions: zzzExtensions(),
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
