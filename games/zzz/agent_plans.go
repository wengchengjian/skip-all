package zzz

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// agentPlans 是代理人专属连招，从上游 ZenlessZoneZero-OneDragon 的
// config/auto_battle_state_handler 里按角色拆分的模板整理：
// 站场模板-<角色>.yml（该角色在场时的循环）与 速切模板-<角色>.yml（速切循环）。
//
// 上游这些模板是带条件的状态机（角色能量、层数、按键可用性），本项目取其中
// 实际出现的按钮顺序，能力之间夹一次普攻，作为近似连招；条件判断靠
// combat.Spec.Ready / Cooldowns 这层通用机制承担。
var agentPlans = map[string][]combat.Step{
	"anby":           {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"anton":          {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"asaba_harumasa": {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"ben":            {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"billy":          {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"corin":          {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"ellen":          {{Kind: combat.StepDodge}, {Kind: combat.StepAttack}},
	"grace":          {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"hoshimi_miyabi": {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"jane_doe":       {{Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"lighter":        {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"lycaon":         {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"nekomata":       {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"nicole":         {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"orphie":         {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"pulchra":        {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"qingyi":         {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"rina":           {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"seth_lowell":    {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"soldier_11":     {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"soukaku":        {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"yanagi":         {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"yidhari":        {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}},
	"zhu_yuan":       {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}},
}
