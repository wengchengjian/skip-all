package games

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wengchengjian/skip-all/games/wuwa"
	"github.com/wengchengjian/skip-all/games/zzz"
)

// TestDocsMatchCode 用代码里的真实统计核对文档数字。
// 之前出过"文档写 24 个代理人、实际漏了整个 state_handler 目录"的事，这个测试防再犯。
func TestDocsMatchCode(t *testing.T) {
	parity, err := os.ReadFile(filepath.Join("..", "docs", "combat-parity.md"))
	if err != nil {
		t.Fatalf("读 combat-parity.md: %v", err)
	}
	upstream, err := os.ReadFile(filepath.Join("..", "docs", "upstream.md"))
	if err != nil {
		t.Fatalf("读 upstream.md: %v", err)
	}
	wuwaDoc, err := os.ReadFile(filepath.Join("..", "docs", "wuwa-combat.md"))
	if err != nil {
		t.Fatalf("读 wuwa-combat.md: %v", err)
	}
	z := zzz.CombatStats()
	w := wuwa.CombatStats()

	expect := []struct {
		name string
		text string
		want string
	}{
		{"绝区零规则条数", string(parity), fmt.Sprintf("%d 条规则", z["rules"])},
		{"绝区零代理人覆盖", string(parity), fmt.Sprintf("%d 个代理人", z["agents"])},
		{"绝区零状态检测", string(parity), fmt.Sprintf("%d 条颜色判定", z["checks"])},
		{"绝区零连招", string(parity), fmt.Sprintf("%d 个代理人", z["plans"])},
		{"整队规则", string(parity), fmt.Sprintf("%d 条，", z["team_rules"])},
		{"鸣潮连招", string(upstream), fmt.Sprintf("%d 个角色", w["plans"])},
		{"鸣潮定位", string(upstream), fmt.Sprintf("%d 个", w["roles"])},
		{"鸣潮增益时长", string(wuwaDoc), fmt.Sprintf("%d 个角色增益时长", w["buff_times"])},
		{"鸣潮协同对", string(wuwaDoc), fmt.Sprintf("%d 个协同对", w["synergy"])},
		{"鸣潮连招(能力表)", string(wuwaDoc), fmt.Sprintf("%d 个角色", w["plans"])},
		{"鸣潮定位(能力表)", string(wuwaDoc), fmt.Sprintf("%d 个", w["roles"])},
		{"鸣潮重击角色", string(wuwaDoc), fmt.Sprintf("%d 个角色用到", w["heavy"])},
		{"鸣潮回路门控", string(wuwaDoc), fmt.Sprintf("%d 个角色", w["forte_gate"])},
	}
	for _, e := range expect {
		if !strings.Contains(e.text, e.want) {
			t.Errorf("%s：代码统计应出现在文档里为 %q，但没找到", e.name, e.want)
		}
	}
	if z["agents"] < 50 || z["rules"] < 300 {
		t.Errorf("绝区零规则规模异常: %+v", z)
	}
	if w["plans"] < 40 || w["roles"] < 40 {
		t.Errorf("鸣潮规模异常: %+v", w)
	}
}
