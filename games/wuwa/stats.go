package wuwa

// CombatStats 返回自动战斗移植规模的统计，供测试与文档核对使用。
func CombatStats() map[string]int {
	return map[string]int{
		"plans":    len(agentPlans),
		"roles":    len(agentRoles),
		"forte":    2,
		"libready": 6,
	}
}
