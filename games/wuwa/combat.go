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
// 战斗判据用敌人血条颜色（上游 enemy_health_color_red: R 174-225 / G 55-85 / B 55-76），
// 比锁定目标框稳：锁定可能在切人瞬间丢，血条整场战斗都在。
func combatSpec() combat.Spec {
	return combat.Spec{
		Screen:         "鸣潮战斗",
		DetectAreas:    nil,
		HealthBarArea:  "敌人血条",
		HealthBarRange: [2][3]uint8{{174, 55, 55}, {225, 85, 76}},
		HealthBarRatio: 0.02,
		AttackPoint:    geom.Point{X: 960, Y: 540},
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
