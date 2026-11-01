package wuwa

// CombatStats 返回自动战斗移植规模的统计，供测试与文档核对使用。
func CombatStats() map[string]int {
	pairs := 0
	for _, triggers := range agentSynergy {
		pairs += len(triggers)
	}
	heavy := 0
	for _, plan := range agentPlans {
		for _, step := range plan {
			if step.Kind == "heavy" {
				heavy++
				break
			}
		}
	}
	return map[string]int{
		"plans":      len(agentPlans),
		"roles":      len(agentRoles),
		"forte":      2,
		"libready":   6,
		"buff_times": len(agentBuffTime),
		"forte_gate": len(agentForteGate),
		"synergy":    pairs,
		"heavy":      heavy,
	}
}
