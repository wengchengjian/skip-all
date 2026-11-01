package zzz

// CombatStats 返回自动战斗移植规模的统计，供测试与文档核对使用。
func CombatStats() map[string]int {
	rules, teamRuleCount, plans := 0, 0, len(agentPlans)
	for _, items := range agentRules {
		rules += len(items)
	}
	for _, items := range teamRules {
		teamRuleCount += len(items)
	}
	return map[string]int{
		"agents":     len(agentRules),
		"rules":      rules,
		"team_rules": teamRuleCount,
		"checks":     len(agentStateChecks),
		"plans":      plans,
		"avatars":    len(avatarIDs("avatar1")),
	}
}
