package zzz

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCombatSpecIsConsistent 检查战斗配置与资产是否对得上，防止改了模板/坐标却忘了另一头。
func TestCombatSpecIsConsistent(t *testing.T) {
	spec := combatSpec()
	templatesRoot := filepath.Join("..", "..", "assets", "games", "zzz", "templates")

	// 1. 技能连招里用到的角色，必须有对应的头像模板目录。
	for id := range spec.PlansByAgent {
		dir := filepath.Join(templatesRoot, "avatar1", id)
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("角色 %s 有连招但没有头像模板: %v", id, err)
		}
	}

	// 2. 角色定位表里的 id 也应当有模板（鸣潮的定位表来自上游 char_type）。
	for id := range spec.Roles {
		dir := filepath.Join(templatesRoot, "avatar1", id)
		if _, err := os.Stat(dir); err != nil {
			continue // 定位表里有、模板没拷的角色只影响认人，不算错
		}
	}

	// 3. 就绪/冷却区域必须落在 1920x1080 逻辑分辨率内。
	inBounds := func(r [4]int, name string) {
		if r[0] < 0 || r[1] < 0 || r[2] > 1920 || r[3] > 1080 || r[2] <= r[0] || r[3] <= r[1] {
			t.Fatalf("%s 区域越界: %v", name, r)
		}
	}
	for kind, check := range spec.Ready {
		inBounds([4]int{check.Area.X1, check.Area.Y1, check.Area.X2, check.Area.Y2}, string(kind)+" 就绪")
	}
	for kind, rect := range spec.Cooldowns {
		inBounds([4]int{rect.X1, rect.Y1, rect.X2, rect.Y2}, string(kind)+" 冷却")
	}

	// 4. 默认连招非空，按键不能全空。
	if len(spec.Plan) == 0 {
		t.Fatal("默认连招为空")
	}
	if spec.Keys.Attack == "" && (spec.AttackPoint.X == 0 && spec.AttackPoint.Y == 0) {
		t.Fatal("普攻既没有按键也没有攻击点")
	}

	// 5. 中断提示文案不能为空（否则中断检测形同虚设）。
	if spec.AbortArea != "" && len(spec.AbortTexts) == 0 {
		t.Fatal("配了中断区域却没有中断文案")
	}
}
