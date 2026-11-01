package zzz

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// combatSpec 是绝区零的自动战斗配置。
// 流程与鸣潮完全共用（pkg/assist/combat），这里只给出按键与画面证据：
// 特殊技 E、终结技 Q、闪避 左 Shift、切换 1/2/3。
// 接入时只需补一张战斗截图做 templates/combat/in_combat，再把 CombatFeature()
// 加进 Features()；在此之前不注册，避免用户看到一个无法生效的功能。
func combatSpec() combat.Spec {
	return combat.Spec{
		Screen:      "绝区零战斗",
		DetectAreas: []string{"战斗中"},
		AttackPoint: geom.Point{X: 960, Y: 540},
		Keys: combat.Keys{
			Skill:      "e",
			Liberation: "q",
			Dodge:      "lshift",
			Switch:     []string{"1", "2", "3"},
		},
		Plan: []combat.Step{
			{Kind: combat.StepAttack},
			{Kind: combat.StepSkill},
			{Kind: combat.StepAttack},
			{Kind: combat.StepAttack},
			{Kind: combat.StepDodge},
			{Kind: combat.StepAttack},
			{Kind: combat.StepLiberation},
			{Kind: combat.StepAttack},
		},
	}
}
