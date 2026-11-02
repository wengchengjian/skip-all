package wuwa

import (
	"fmt"
	"strings"
	"testing"
)

// TestDiagHiyukiRules 打印绯雪在场时规则引擎逐帧给出的动作，
// 用来定位"只普攻不出技能"的根因。
func TestDiagHiyukiRules(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	g.current = 0
	runSim(t, g, 60)
	var b strings.Builder
	for _, a := range g.actions {
		b.WriteString(fmt.Sprintf("f%03d %s kind=%v key=%s\n", a.frame, a.state, a.kind, a.key))
	}
	t.Log(b.String())
	counts := map[string]int{}
	for _, a := range g.actions {
		counts[a.state]++
	}
	t.Logf("state 计数: %v", counts)
	if counts["规则-1"] > 40 {
		t.Fatalf("绯雪在场上 60 帧里有 %d 帧都命中同一条无条件规则，后续规则被饿死", counts["规则-1"])
	}
}
