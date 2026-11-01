package zzz

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// agentPlans 是代理人专属连招，由上游 ZenlessZoneZero-OneDragon 的两处数据机械推导：
//  1. config/auto_battle/专属配队-<角色>.yml：整份配置围绕该角色的循环；
//  2. config/auto_battle_state_handler/<角色>-*.yml：该角色的专用决策。
//
// 推导方式：统计各按钮出现次数，用得多的排前面，能力之间夹一次普攻。
// 上游真实逻辑是状态机（可用性、招架支援、连携换人），这里是近似序列。
var agentPlans = map[string][]combat.Step{
	"ellen": {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"lycaon": {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"lighter": {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
}
