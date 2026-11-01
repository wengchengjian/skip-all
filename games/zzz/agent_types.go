package zzz

// 代理人的定位分组，来自上游 agent.py 的 AgentTypeEnum。
const (
	AgentTypeAttack  = "attack"
	AgentTypeStun    = "stun"
	AgentTypeAnomaly = "anomaly"
	AgentTypeSupport = "support"
	AgentTypeDefense = "defense"
	AgentTypeRupture = "rupture"
)

// agentsByType 把代理人按上游定位分组，规则里的 [后台-N-击破] 这类条件用它判定。
var agentsByType = map[string]map[string]bool{
	AgentTypeAnomaly: {"alice": true, "aria": true, "burnice_white": true, "grace": true, "hoshimi_miyabi": true, "jane_doe": true, "piper": true, "promeia": true, "velina": true, "vivian": true, "yanagi": true},
	AgentTypeAttack:  {"anton": true, "asaba_harumasa": true, "billy": true, "cissia": true, "corin": true, "ellen": true, "evelyn_chevalier": true, "hugo_vlad": true, "nekomata": true, "orphie": true, "pyrois_wise": true, "seed": true, "soldier_0_anby": true, "soldier_11": true, "starlight_billy_kid": true, "zhu_yuan": true},
	AgentTypeDefense: {"ben": true, "caesar_king": true, "panyinhu": true, "seth_lowell": true, "zhao": true},
	AgentTypeRupture: {"banyue": true, "manato": true, "yidhari": true, "yixuan": true},
	AgentTypeStun:    {"anby": true, "dialyn": true, "ju_fufu": true, "koleda": true, "lighter": true, "lycaon": true, "nangongyu": true, "norma": true, "pulchra": true, "qingyi": true, "trigger": true},
	AgentTypeSupport: {"astra_yao": true, "lucia": true, "lucy": true, "nicole": true, "rina": true, "soukaku": true, "yuzuha": true},
}
