package wuwa

import "github.com/wengchengjian/skip-all/pkg/assist/combat"

// agentRules 由脚本从上游 ok-ww 各角色 do_perform() 的条件分支机械提取而来：
// 每个条目是"在什么条件下做哪个动作"。无法用现有原子表达的条件（内部计时器等）
// 不进入规则，统计见 UNKNOWN。
var agentRules = map[string][]wuwaRule{
	"Augusta":          {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_aalto":       {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_aemeath":     {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_baizhi":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_brant":       {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "回路满&技能可用&声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "回路未满", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_camellya":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_cantarella":  {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "回路未满", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "回路未满", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_cartethyia":  {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_chouyuan":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_ciaccona":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}},
	"char_danjin":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "回路满", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "回路满", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_denia":       {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_encore":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_galbrena":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "回路满", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "回路满", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_hiyuki":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_jianxin":     {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_jingran":     {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "回路满", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_jiyan":       {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "回路未满&技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_lingyang":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_lumi":        {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "解放可用", Ops: []combat.Step{{Kind: combat.StepLiberation}}}},
	"char_lupa":        {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}},
	"char_mortefi":     {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}},
	"char_phoebe":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_phrolova":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_qingxiao":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}},
	"char_rebecca":     {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_roccia":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_shorekeeper": {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}},
	"char_suoming":     {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_taoqi":       {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_xiangliyao":  {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
	"char_xigelika":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"char_yangyang":    {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_yinlin":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepHeavy}}}, {Cond: "声骸可用", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_yuanwu":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepEcho}}}},
	"char_zhezhi":      {{Cond: "1", Ops: []combat.Step{{Kind: combat.StepAttack}}}, {Cond: "1", Ops: []combat.Step{{Kind: combat.StepLiberation}}}, {Cond: "回路满&技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}, {Cond: "回路满&技能可用", Ops: []combat.Step{{Kind: combat.StepAttack}}}},
	"yangyang_sp":      {{Cond: "技能可用", Ops: []combat.Step{{Kind: combat.StepSkill}}}},
}
