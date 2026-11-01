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
// 战斗证据用它自己的锁定目标框模板 box_target_enemy。
func combatSpec() combat.Spec {
	return combat.Spec{
		Screen:      "鸣潮战斗",
		DetectAreas: []string{"敌人锁定", "敌人锁定-内圈"},
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
