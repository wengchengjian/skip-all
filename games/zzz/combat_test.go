package zzz

import (
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

func TestCombatSpecReusesSharedFlow(t *testing.T) {
	spec := combatSpec()
	if spec.Screen != "绝区零战斗" {
		t.Fatalf("Screen = %q", spec.Screen)
	}
	if spec.Keys.Skill != "e" || spec.Keys.Liberation != "q" || spec.Keys.Dodge != "lshift" {
		t.Fatalf("按键绑定 = %+v", spec.Keys)
	}
	feature := combat.NewFeature(spec)
	if err := feature.Validate(feature.DefaultSettings()); err != nil {
		t.Fatalf("默认设置不合法: %v", err)
	}
	game := New()
	set, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面失败: %v", err)
	}
	if _, ok := set.Get("绝区零战斗"); !ok {
		t.Fatal("缺少 绝区零战斗 画面")
	}
}
