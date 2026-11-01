package wuwa

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// agentPlans 是角色专属连招，由上游 ok-ww 的 src/char/<角色>.py 里 do_perform()
// 的出手顺序机械推导而来：先出现的能力排在前面，能力之间夹一次普攻。
// 这是一份"近似序列"，上游真实的 perform() 还带冷却/能量/切换判断，这里没有。
var agentPlans = map[string][]combat.Step{
	"char_aalto":       {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_aemeath":     {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"Augusta":          {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_baizhi":      {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_brant":       {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_camellya":    {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_cantarella":  {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_cartethyia":  {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_chixia":      {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_ciaccona":    {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_danjin":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_denia":       {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_douling":     {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_encore":      {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_galbrena":    {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_hiyuki":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_hsin":        {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_iuno":        {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_jianxin":     {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_jingran":     {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_jiyan":       {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_lingyang":    {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_lucilla":     {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_lucy":        {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_luhesi":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_lumi":        {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_lupa":        {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_mortefi":     {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_phoebe":      {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_phrolova":    {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_qingxiao":    {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_chouyuan":    {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_rebecca":     {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_roccia":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_shorekeeper": {{Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_suisui":      {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_suoming":     {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"char_taoqi":       {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_verina":      {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_xiangliyao":  {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_xigelika":    {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
	"yangyang_sp":      {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_yangyang":    {{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}, {Kind: combat.StepLiberation}, {Kind: combat.StepAttack}},
	"char_yinlin":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_yuanwu":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
	"char_zhezhi":      {{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}, {Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepEcho}, {Kind: combat.StepAttack}},
}
