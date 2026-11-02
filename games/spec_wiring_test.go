package games

import (
	"testing"

	"github.com/wengchengjian/skip-all/games/wuwa"
	"github.com/wengchengjian/skip-all/games/zzz"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
)

// TestCombatSpecsWireAllFeatures 防止"代码写了但没接到 Spec 里"这种静默失败。
// 之前鸣潮的 9 项（连招表/就绪/冷却/大招就绪/定位/入场技/中断/扩展）都写好却漏接线，
// 编译与测试全绿，功能实际不生效。
func TestCombatSpecsWireAllFeatures(t *testing.T) {
	checks := map[string]combat.Spec{
		"鸣潮":  wuwa.CombatSpec(),
		"绝区零": zzz.CombatSpec(),
	}
	for name, spec := range checks {
		if spec.Extensions == nil || len(spec.Extensions) == 0 {
			t.Errorf("%s：没有接任何战斗扩展", name)
		}
		if len(spec.PlansByAgent) == 0 {
			t.Errorf("%s：角色专属连招没接", name)
		}
		if len(spec.Roles) == 0 {
			t.Errorf("%s：角色定位表没接（切人打分不生效）", name)
		}
		if spec.Team == nil {
			t.Errorf("%s：角色识别没接", name)
		}
		if spec.Switch == nil {
			t.Errorf("%s：切人策略没接", name)
		}
		// Ready 允许为空：上游 ok-ww 的 resonance_available/echo_available
		// 都是 available(check_color=False)，即只看冷却不看高亮。
		// 配一个测不准的白点判据反而会把技能锁死。
		if len(spec.Cooldowns) == 0 {
			t.Errorf("%s：冷却判据没接", name)
		}
		if spec.AbortArea == "" || len(spec.AbortTexts) == 0 {
			t.Errorf("%s：中断保护没接", name)
		}
	}
}
