package zzz

import (
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

func TestCombatSpecUsesUpstreamKeys(t *testing.T) {
	spec := combatSpec()
	if spec.Keys.Skill != "e" || spec.Keys.Liberation != "q" || spec.Keys.Dodge != "shift" {
		t.Fatalf("按键绑定 = %+v", spec.Keys)
	}
	if len(spec.Keys.Switch) != 1 || spec.Keys.Switch[0] != "space" {
		t.Fatalf("切换按键 = %v", spec.Keys.Switch)
	}
	if len(spec.DetectAreas) == 0 {
		t.Fatal("缺少战斗证据区域")
	}
	feature := combat.NewFeature(spec)
	if err := feature.Validate(feature.DefaultSettings()); err != nil {
		t.Fatalf("默认设置不合法: %v", err)
	}
}

func TestAssetsLoadCombatScreenAndTemplate(t *testing.T) {
	game := New()
	set, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面失败: %v", err)
	}
	sc, ok := set.Get("绝区零战斗")
	if !ok {
		t.Fatal("缺少 绝区零战斗 画面")
	}
	area, ok := sc.Area("普通攻击按钮")
	if !ok {
		t.Fatal("缺少 普通攻击按钮 区域")
	}
	if area.TemplateID != "btn_normal_attack" || area.TemplateMatchThreshold < 0.9 {
		t.Fatalf("区域配置 = %+v", area)
	}
}
