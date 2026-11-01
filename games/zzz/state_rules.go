package zzz

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// StateRule 是一条"条件 -> 动作"规则。
type StateRule struct {
	Cond string        // 上游 states 表达式
	Ops  []combat.Step // 命中的按键序列
}

// StateCheck 是上游 agent.py 里的状态检测定义。
type StateCheck struct {
	TemplateID string
	Lower      [3]uint8
	Upper      [3]uint8
	Connect    int
	Way        string // 上游 AgentStateCheckWay，决定怎么判
}

// agentRules 是从上游 ZenlessZoneZero-OneDragon 的
// config/auto_battle_state_handler/站场模板-<角色>.yml 与 速切模板-<角色>.yml
// 提取出来的"条件 -> 动作"规则表。
// 只保留带按键动作的规则，顺序与上游文件一致（上游从上往下匹配，先命中先生效）。
var agentRules = map[string][]StateRule{
	"anby": {
		{Cond: "[自定义-黄光切人]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-红光闪避]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[安比-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}}},
		{Cond: "[安比-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}}},
	},
	"anton": {
		{Cond: "[安东-特殊技可用] & (![自定义-安东-爆发状态, 0, 12] | ([按键-切换角色-下一个, 0, 1]|[按键-切换角色-上一个, 0, 1]))", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[自定义-安东-爆发状态, 0, 14]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[安东-能量]{110, 120}", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"asaba_harumasa": {
		{Cond: "[按键-切换角色-下一个, 0, 0.4]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-失衡时间, 0, 2]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}}},
		{Cond: "[按键可用-终结技] & [自定义-失衡时间, 0, 10]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "![按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"ben": {
		{Cond: "[本-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"billy": {
		{Cond: "[比利-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[比利-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"corin": {
		{Cond: "[自定义-失衡时间, 0, 1]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-终结技] & [自定义-失衡时间, 0, 10]", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}}},
		{Cond: "[按键可用-特殊攻击] & [自定义-失衡时间, 2, 10] & ![自定义-可琳-这次不打特殊技, 0, 12]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "![按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-终结技被强制释放, 0, 1]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[可琳-终结技可用] & [自定义-失衡时间, 0, 10]", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}}},
	},
	"ellen": {
		{Cond: "[自定义-失衡时间, 0, 10] & [艾莲-能量]{40, 120} & [按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepDodge}}},
		{Cond: "[自定义-失衡时间, 0, 10] & [艾莲-能量]{40, 120} & [按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepAttack}, {Kind: combat.StepDodge}}},
		{Cond: "[自定义-艾莲-正在打普攻,0,10]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"grace": {
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "![按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"hoshimi_miyabi": {
		{Cond: "[按键-切换角色-下一个, 0, 0.8]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[雅-落霜]{6, 6}", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-终结技] & ![自定义-星见雅使用过拔刀斩, 0, 5] & [雅-落霜]{0, 3}", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"jane_doe": {
		{Cond: "![简-狂热心流]{60, 101} & [简-萨霍夫跳]", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}, {Kind: combat.StepDodge}}},
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"lighter": {
		{Cond: "[自定义-终结技被强制释放, 0, 1]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[莱特-终结技可用]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[莱特-能量]{80, 120} & [莱特-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"lycaon": {
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
	},
	"nekomata": {
		{Cond: "[猫又-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[猫又-终结技可用] & !([按键-切换角色-下一个, 0, 1]|[按键-切换角色-上一个, 0, 1]) & ![自定义-动作不打断, 0, 30]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[猫又-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"nicole": {
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"piper": {
		{Cond: "[自定义-黄光切人]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-红光闪避]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"pulchra": {
		{Cond: "[自定义-终结技被强制释放, 0, 1]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[波可娜-终结技可用]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[波可娜-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"qingyi": {
		{Cond: "[青衣-电压]{80, 101}", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-终结技] & ![自定义-动作不打断, 0, 30] & ![自定义-青衣-醉花云月转, 0, 3]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "![按键可用-特殊攻击] & [自定义-青衣-特殊技续普攻, 0, 2]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-特殊攻击] & [自定义-青衣-特殊技续普攻, 0, 2]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[青衣-电压]{0, 90}", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"rina": {
		{Cond: "[自定义-快速支援换人, 0, 0.5]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"seed": {
		{Cond: "[席德-钢能]{100,999}", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"soldier_11": {
		{Cond: "[按键可用-终结技]", Ops: []combat.Step{{Kind: combat.StepLiberation}, {Kind: combat.StepAttack}}},
		{Cond: "[11号-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"soukaku": {
		{Cond: "[苍角-特殊技可用] & ![自定义-苍角-展旗, 0, 1]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
	},
	"yanagi": {
		{Cond: "[按键-切换角色-下一个, 0, 0.4]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[按键可用-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[按键可用-终结技] & ![自定义-动作不打断, 0, 15]", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[自定义-柳-森罗万象, 0, 15]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"yidhari": {
		{Cond: "[自定义-伊德海莉-蓄力中,0,2] | [自定义-伊德海莉-旋转状态,0,2]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[自定义-伊德海莉-追碾状态, 0, 2]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-伊德海莉-蓄力中, 0, 2] & [伊德海莉-特殊技可用]", Ops: []combat.Step{{Kind: combat.StepSkill}}},
		{Cond: "[自定义-伊德海莉-旋转状态, 0, 2]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
	},
	"zhu_yuan": {
		{Cond: "[按键可用-终结技] & ![自定义-临时站场] & ![朱鸢-子弹数, 0, 999]{7, 9}", Ops: []combat.Step{{Kind: combat.StepLiberation}}},
		{Cond: "[自定义-朱鸢-下发普攻前滑]", Ops: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}}},
		{Cond: "[自定义-朱鸢-特殊攻击]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[朱鸢-子弹数, 0, 999]{2, 9}", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-红光闪避]", Ops: []combat.Step{{Kind: combat.StepAttack}}},
		{Cond: "[自定义-血量扣减]", Ops: []combat.Step{{Kind: combat.StepDodge}}},
	},
}

// agentStateChecks 是从上游 src/zzz_od/game_data/agent.py 的 AgentStateDef 提取的
// 状态检测（颜色范围 + 连通块数量），用于判定 [<角色>-<状态>] 这类条件。
var agentStateChecks = map[string]StateCheck{
	"伊芙琳-燎火":     {TemplateID: "", Lower: [3]uint8{70, 70, 70}, Upper: [3]uint8{255, 255, 255}, Connect: 5, Way: "BACKGROUND_GRAY_RANGE_LENGTH"},
	"伊芙琳-燎索点":    {TemplateID: "", Lower: [3]uint8{70, 70, 70}, Upper: [3]uint8{255, 255, 255}, Connect: 5, Way: "COLOR_RANGE_CONNECT"},
	"前台-终结技可用":   {TemplateID: "ultimate_3_1", Lower: [3]uint8{250, 150, 20}, Upper: [3]uint8{255, 255, 70}, Connect: 1000, Way: "COLOR_RANGE_CONNECT"},
	"前台-血量扣减":    {TemplateID: "life_deduction_2_1", Lower: [3]uint8{140, 30, 30}, Upper: [3]uint8{160, 50, 50}, Connect: 10000, Way: "FOREGROUND_COLOR_RANGE_LENGTH"},
	"后台-1-终结技可用": {TemplateID: "ultimate_2_2", Lower: [3]uint8{250, 150, 20}, Upper: [3]uint8{255, 255, 70}, Connect: 5, Way: "COLOR_RANGE_EXIST"},
	"后台-2-终结技可用": {TemplateID: "ultimate_3_3", Lower: [3]uint8{250, 150, 20}, Upper: [3]uint8{255, 255, 70}, Connect: 5, Way: "COLOR_RANGE_EXIST"},
	"扳机-绝意":      {TemplateID: "", Lower: [3]uint8{0, 50, 0}, Upper: [3]uint8{255, 255, 255}, Connect: 1, Way: "FOREGROUND_COLOR_RANGE_LENGTH"},
	"朱鸢-子弹数":     {TemplateID: "zhu_yuan", Lower: [3]uint8{240, 60, 0}, Upper: [3]uint8{255, 180, 20}, Connect: 5, Way: "COLOR_RANGE_CONNECT"},
	"柚叶-甜度点":     {TemplateID: "alice", Lower: [3]uint8{221, 107, 113}, Upper: [3]uint8{255, 255, 255}, Connect: 6, Way: "COLOR_RANGE_CONNECT"},
	"波可娜-猎步":     {TemplateID: "", Lower: [3]uint8{200, 120, 30}, Upper: [3]uint8{255, 255, 255}, Connect: 1, Way: "COLOR_RANGE_CONNECT"},
	"简-狂热心流":     {TemplateID: "jane_red", Lower: [3]uint8{200, 20, 20}, Upper: [3]uint8{255, 255, 255}, Connect: 10, Way: "FOREGROUND_COLOR_RANGE_LENGTH"},
	"简-萨霍夫跳":     {TemplateID: "jane_attack", Lower: [3]uint8{100, 20, 20}, Upper: [3]uint8{255, 255, 255}, Connect: 20, Way: "COLOR_RANGE_EXIST"},
	"艾莲-急冻充能":    {TemplateID: "ellen", Lower: [3]uint8{200, 245, 250}, Upper: [3]uint8{255, 255, 255}, Connect: 2, Way: "COLOR_RANGE_CONNECT"},
	"苍角-涡流":      {TemplateID: "soukaku", Lower: [3]uint8{0, 220, 220}, Upper: [3]uint8{175, 255, 255}, Connect: 15, Way: "COLOR_RANGE_CONNECT"},
	"薇薇安-护羽":     {TemplateID: "", Lower: [3]uint8{170, 170, 200}, Upper: [3]uint8{255, 255, 255}, Connect: 5, Way: "COLOR_RANGE_CONNECT"},
	"薇薇安-飞羽":     {TemplateID: "", Lower: [3]uint8{150, 110, 170}, Upper: [3]uint8{255, 255, 255}, Connect: 5, Way: "COLOR_RANGE_CONNECT"},
	"青衣-电压":      {TemplateID: "qingyi", Lower: [3]uint8{100, 20, 20}, Upper: [3]uint8{255, 255, 255}, Connect: 20, Way: "BACKGROUND_GRAY_RANGE_LENGTH"},
}
