package zzz

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// teamRules 从上游 config/auto_battle/专属配队-<角色>.yml 提取的"整队节奏"规则：
// 条件里会出现后台位定位、连携换人、失衡等，动作除了技能还有切换角色/连携。
// 与按角色的 state_handler 规则互补——这一层管"什么时候换谁"，那一层管"这个人怎么打"。
var teamRules = map[string][]StateRule{
	"ellen": {
		{Cond: "[前台-艾莲] & [自定义-艾莲-冰渊潜袭,1.5,2]", Ops: []combat.Step{combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[连携技-1-邦布]", Ops: []combat.Step{combat.Step{Kind: combat.StepKey, Key: "q"}}},
		{Cond: "[自定义-闪避]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-快速支援换人]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-招架支援]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{combat.Step{Kind: combat.StepLiberation}}},
		{Cond: "[妮可-能量]{85, 120}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[露西-能量]{65, 120} & ![自定义-露西-加油, -5, 15]", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "![切换角色-露西]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-闪避]", Ops: []combat.Step{combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[苍角-涡流]{0, 0}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[苍角-涡流]{1, 1}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[前台-能量]{40, 120} & ![自定义-凯撒-招架反击, 0, 3]", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[自定义-连携技换人]", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[柏妮思-能量]{60, 120} & ![柏妮思-燃点]{20, 100}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[柏妮思-能量]{40, 120} & ![柏妮思-燃点]{0, 50}", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}, combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[自定义-快速支援换人]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}, combat.Step{Kind: combat.StepDodge}}},
		{Cond: "[按键可用-终结技] & [薇薇安-飞羽]{0, 1} & [薇薇安-护羽]{0, 1}", Ops: []combat.Step{combat.Step{Kind: combat.StepLiberation}}},
		{Cond: "[薇薇安-飞羽]{0, 2} & [薇薇安-护羽]{0, 3} & [薇薇安-能量]{65, 120}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[自定义-快速支援换人]", Ops: []combat.Step{combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepLiberation}}},
		{Cond: "![自定义-艾莲-冰渊潜袭,1.5,3] & [按键可用-终结技]", Ops: []combat.Step{combat.Step{Kind: combat.StepLiberation}}},
		{Cond: "[前台-能量, 0, 999]{80, 120}", Ops: []combat.Step{combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepSkill}, combat.Step{Kind: combat.StepAttack}, combat.Step{Kind: combat.StepAttack}}},
		{Cond: "![艾莲-急冻充能, 0, 999]{3, 6}", Ops: []combat.Step{combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepDodge}}},
	},
	"lycaon": {
		{Cond: "[连携技-1-邦布]", Ops: []combat.Step{combat.Step{Kind: combat.StepKey, Key: "q"}}},
		{Cond: "[自定义-闪避]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-快速支援换人]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-招架支援]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[前台-能量]{40, 120} & ![自定义-凯撒-招架反击, 0, 3]", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "![自定义-苍角-展旗, 0, 25] & [苍角-涡流]{3, 3}", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[前台-能量]{60, 120} & [苍角-涡流]{2, 2}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[前台-能量]{60, 120} & [苍角-涡流]{1, 1}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[前台-能量]{90, 120} & [苍角-涡流]{0, 0}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[苍角-涡流]{0, 0}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[苍角-涡流]{1, 1}", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "![切换角色-莱卡恩] & [按键可用-终结技]", Ops: []combat.Step{combat.Step{Kind: combat.StepLiberation}}},
	},
	"lighter": {
		{Cond: "[自定义-闪避]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-快速支援换人]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[自定义-招架支援]", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}}},
		{Cond: "[前台-能量]{40, 120} & ![自定义-凯撒-招架反击, 0, 3]", Ops: []combat.Step{combat.Step{Kind: combat.StepSkill}}},
		{Cond: "[按键可用-终结技] & ![莱特-士气]{80, 100}", Ops: []combat.Step{combat.Step{Kind: combat.StepLiberation}}},
		{Cond: "[莱特-能量]{40, 120}", Ops: []combat.Step{combat.Step{Kind: combat.StepAttack}, combat.Step{Kind: combat.StepDodge}, combat.Step{Kind: combat.StepSkill}, combat.Step{Kind: combat.StepAttack}}},
	},
}
