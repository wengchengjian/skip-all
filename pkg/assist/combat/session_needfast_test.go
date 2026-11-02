package combat

import (
	"testing"
	"time"
)

// TestNeedFastPerform 验证上游 BaseChar.need_fast_perform()：
// 后台任一候选对当前角色为 MUST 时，当前角色应尽快收尾让位。
func TestNeedFastPerform(t *testing.T) {
	spec := testSpec()
	spec.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	spec.SwitchSynergy = map[string][]string{"b": {"a"}}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	if !session.needFastPerform(frameAt(base), base) {
		t.Fatal("b 对 a 是 MUST，应判定需要速切")
	}
	// 没有协同 / 解放窗口时不成立。
	spec2 := testSpec()
	spec2.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	session2 := newSession(testRuntime(true), spec2.withDefaults(), sessionSettings{})
	if session2.needFastPerform(frameAt(base), base) {
		t.Fatal("没有 MUST 候选时不该判定需要速切")
	}
	// 仍处解放窗口的候选也返回 MUST。
	spec3 := testSpec()
	spec3.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	spec3.LiberationMustSwitch = map[string]bool{"b": true}
	session3 := newSession(testRuntime(true), spec3.withDefaults(), sessionSettings{})
	session3.lastLiberation["b"] = base
	if !session3.needFastPerform(frameAt(base.Add(time.Second)), base.Add(time.Second)) {
		t.Fatal("b 仍在解放窗口内，应判定需要速切")
	}
}
