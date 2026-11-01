package zzz

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// agentPlans 是代理人专属连招，由上游 ZenlessZoneZero-OneDragon 的
// config/auto_battle/*.yml 里按 agent_name 分块统计按钮出现次数机械推导：
// 用得最多的能力排在前面，能力之间夹一次普攻。
// 上游真实逻辑是状态机（判断特殊技/终结技是否可用、招架支援、连携），这里是近似序列。
var agentPlans = map[string][]combat.Step{
	"ellen":   {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"lycaon":  {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"lighter": {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
}
