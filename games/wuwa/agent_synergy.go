package wuwa

// agentSynergy 是"某角色在场时必须切给谁"的协同表，从上游 ok-ww 各角色
// get_switch_priority() 里的 `if has_intro and current_char.char_name in {...}: return MUST`
// 机械提取（候选角色 -> 触发它的在场角色）：
//
//	维里奈 <- 冰璃 / 布兰特 <- 露帕 / 坎特蕾拉 <- 洛可塔、散华 /
//	露帕 <- 长离 / 冰璃 <- 琳奈、露西拉 / 露西拉 <- 维里奈、守岸人 / 露西 <- 蕾贝卡 /
//	卡提希娅 <- 折枝 / 琳奈 <- 莫宁 / 莫宁 <- 爱弥斯、青霄
var agentSynergy = map[string][]string{
	"char_verina":     {"char_hiyuki"},
	"char_brant":      {"char_lupa"},
	"char_cantarella": {"char_roccia", "char_sanhua"},
	"char_lupa":       {"chang_changli"},
	"char_hiyuki":     {"char_linnai", "char_lucilla"},
	"char_lucilla":    {"char_verina", "char_shorekeeper"},
	"char_lucy":       {"char_rebecca"},
	"char_carlotta":   {"char_zhezhi"},
	"char_linnai":     {"char_moning"},
	"char_moning":     {"char_aemeath", "char_qingxiao"},
}
