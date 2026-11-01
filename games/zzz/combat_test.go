package zzz

import (
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

func TestCombatSpecReusesSharedFlow(t *testing.T) {
	spec := combatSpec()
	if spec.HealthBar == nil {
		t.Fatal("缺少血条证据")
	}
	if spec.Keys.Skill != "e" || spec.Keys.Liberation != "q" || spec.Keys.Dodge != "lshift" {
		t.Fatalf("按键绑定 = %+v", spec.Keys)
	}
	feature := combat.NewFeature(spec)
	if err := feature.Validate(feature.DefaultSettings()); err != nil {
		t.Fatalf("默认设置不合法: %v", err)
	}

}
