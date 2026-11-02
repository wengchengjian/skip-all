package wuwa

import "github.com/wengchengjian/skip-all/pkg/assist/combat"

// agentOutroRules 是 check_outro() 三类"多步特殊收尾"的近似序列：
//  1. Brant 从露帕入场：perfom_in_outro() 的 回路满打共鸣 -> 解放 -> 普攻 -> 声骸
//  2. Changli 从布兰特入场：do_perform_outro() 的 重击 -> 解放 -> 普攻 -> 声骸
//  3. Carlotta 从折枝入场：do_perform_outro() 的 共鸣 -> 重击 -> 声骸
//
// 上游这几个分支是带 while/timeout 的循环，依赖 forte 层数等视觉量；这里按
// 上游动作顺序抽成近似队列，只在 InIntro && LastOutro 匹配时替换普通规则。
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
}
