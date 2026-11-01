package wuwa

import (
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// CombatFeature 创建鸣潮自动战斗功能。
func CombatFeature() assist.Feature {
	return combat.NewFeature(combatSpec())
}

// combatSpec 的按键与连招参考 ok-ww：
// 共鸣技能 E、声骸 Q、共鸣解放 R、闪避 左 Shift，角色头像 1/2/3。
// 战斗判据照搬上游 has_health_bar()：整帧找 enemy_health_color_red
// （R 174-225 / G 55-85 / B 55-76）、宽 >=50、高 5-14、填充率 >=0.95 的实心矩形。
// 上游在未进入战斗时用 min_width=50（100/3840），进入后放宽到 6；这里保持 50，
// 靠 Hold 把证据间断连起来，避免开放世界里把红色 UI 当血条。
func combatSpec() combat.Spec {
	return combat.Spec{
		HealthBar: &combat.HealthBar{
			Color:     [2][3]uint8{{174, 55, 55}, {225, 85, 76}},
			MinWidth:  50,
			MinHeight: 5,
			MaxHeight: 14,
		},
		// Boss 血条：上游 is_boss() 的区域 (1269/3840, 58/2160)-(2533/3840, 200/2160)，
		// 颜色 boss_health_color R245-255 / G30-185 / B4-75，要求区域内恰好一条。
		BossBar: &combat.BossBar{
			Color:     [2][3]uint8{{245, 30, 4}, {255, 185, 75}},
			Rect:      geom.Rect{X1: 634, Y1: 29, X2: 1266, Y2: 100},
			MinWidth:  50,
			MinHeight: 6,
		},
		AttackPoint: geom.Point{X: 960, Y: 540},
		Keys: combat.Keys{
			Skill:      "e",
			Echo:       "q",
			Liberation: "r",
			Dodge:      "lshift",
			Switch:     []string{"1", "2", "3"},
		},
		Plan: []combat.Step{
			{Kind: combat.StepAttack},
			{Kind: combat.StepSkill},
			{Kind: combat.StepAttack},
			{Kind: combat.StepAttack},
			{Kind: combat.StepEcho},
			{Kind: combat.StepAttack},
			{Kind: combat.StepLiberation},
			{Kind: combat.StepAttack},
			{Kind: combat.StepDodge},
		},
	}
}
