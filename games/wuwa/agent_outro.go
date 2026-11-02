package wuwa

import "github.com/wengchengjian/skip-all/pkg/assist/combat"

// agentOutroRules 是 check_outro() 六类"会改变出手"的分支的近似序列：
//  1. Brant 从露帕入场：perfom_in_outro() 的 回路满打共鸣 -> 解放 -> 普攻 -> 声骸
//  2. Changli 从布兰特入场：do_perform_outro() 的 重击 -> 解放 -> 普攻 -> 声骸
//  3. Carlotta 从折枝入场：do_perform_outro() 的 共鸣 -> 重击 -> 声骸
//     4-5. Cantarella 从洛可塔/散华入场：解放 -> 共鸣 -> 重击/声骸
//  6. Phrolova 从坎特蕾拉入场：解放 -> 共鸣 -> 普攻 -> 声骸
//  7. Lupa 从长离入场：解放 -> 共鸣 -> 重击 -> 声骸
//
// 上游这些分支是带 while/timeout 的循环，依赖 forte 层数等视觉量；这里按
// 上游动作顺序抽成近似队列，只在 InIntro && LastOutro 匹配时替换普通规则。
var cantarellaOutroRules = []wuwaRule{
	{Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepSkill}, {Kind: combat.StepHeavy}, {Kind: combat.StepEcho}}},
	{Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepHeavy}, {Kind: combat.StepEcho}}},
	{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
}

var agentOutroRules = map[string]map[string][]wuwaRule{
	"char_brant": {
		"char_lupa": {
			{Cond: "回路满&技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
			{Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
			{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
		},
	},
	"chang_changli": {
		"char_brant": {
			{Cond: "回路满", Ops: []combat.Step{{Kind: combat.StepHeavy}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
			{Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
			{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
		},
	},
	"char_carlotta": {
		"char_zhezhi": {
			{Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepHeavy}, {Kind: combat.StepEcho}}},
			{Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepHeavy}, {Kind: combat.StepEcho}}},
			{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
		},
	},
	"char_cantarella": {
		"char_roccia": cantarellaOutroRules,
		"char_sanhua": cantarellaOutroRules,
	},
	"char_phrolova": {
		"char_cantarella": {
			{Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
			{Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
			{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
		},
	},
	"char_lupa": {
		"chang_changli": {
			{Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepHeavy}, {Kind: combat.StepEcho}}},
			{Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepHeavy}, {Kind: combat.StepEcho}}},
			{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepEcho}}},
		},
	},
}
