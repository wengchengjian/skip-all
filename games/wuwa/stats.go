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
		"rules":      ruleCount(agentRules),
		"char_rules": ruleCharCount(agentRules),
		"rule_chars": ruleChars(agentRules),
	}
}

// ruleCount 统计所有角色的规则条目数。
func ruleCount(rules map[string][]wuwaRule) int {
	n := 0
	for _, rs := range rules {
		n += len(rs)
	}
	return n
}

// ruleCharCount 统计角色专属规则条目数（不含回退表），文档里的"137 条"用这个数。
func ruleCharCount(rules map[string][]wuwaRule) int {
	n := 0
	for key, rs := range rules {
		if key != baseRuleKey {
			n += len(rs)
		}
	}
	return n
}

// ruleChars 统计有专属规则的角色数（不含回退表）。
func ruleChars(rules map[string][]wuwaRule) int {
	n := 0
	for key, rs := range rules {
		if key != baseRuleKey && len(rs) > 0 {
			n++
		}
	}
	return n
}
