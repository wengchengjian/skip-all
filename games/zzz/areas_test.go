package zzz

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// 绝区零的按键区域取自上游 assets/game_data/screen_info/battle.yml。
// 这张表之前纠正过两个估错的区域（快速支援、战斗结果），所以固化成测试。
func TestButtonAreasMatchUpstream(t *testing.T) {
	spec := combatSpec()

	want := map[combat.StepKind]geom.Rect{
		// 按键-特殊攻击 / 按键-终结技
		combat.StepSkill:      {X1: 1620, Y1: 917, X2: 1720, Y2: 1017},
		combat.StepLiberation: {X1: 1728, Y1: 778, X2: 1826, Y2: 876},
	}
	for kind, rect := range want {
		if got := spec.Ready[kind].Area; got != rect {
			t.Errorf("%s 就绪区域 = %+v，应当 = %+v", kind, got, rect)
		}
		if got := spec.Cooldowns[kind]; got != rect {
			t.Errorf("%s 冷却区域 = %+v，应当 = %+v", kind, got, rect)
		}
	}

	// 快速支援按钮 = 上游 按键-切换后援 (1615,776)-(1726,877)。
	wantAssist := geom.Rect{X1: 1615, Y1: 776, X2: 1726, Y2: 877}
	var assist geom.Rect
	for _, ext := range spec.Extensions {
		if e, ok := ext.(*zzzQuickAssistExt); ok {
			assist = e.Area.Rect
		}
	}
	if assist != wantAssist {
		t.Errorf("快速支援区域 = %+v，应当 = %+v", assist, wantAssist)
	}

	// 战斗判据与中断区域在画面配置里，检查它们写的是上游坐标。
	data, err := os.ReadFile(filepath.Join("..", "..", "assets", "games", "zzz", "screens", "combat.yml"))
	if err != nil {
		t.Fatalf("读 combat.yml: %v", err)
	}
	cfg := string(data)
	for _, want := range []string{"1403", "916", "1505", "1017", "520", "840", "1850", "1060"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("combat.yml 里缺少坐标 %s", want)
		}
	}
}
