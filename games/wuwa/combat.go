package wuwa

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
)

// CombatFeature 创建鸣潮自动战斗功能。
func CombatFeature() assist.Feature {
	return combat.NewFeature(combatSpec())
}

// CombatSpec 返回鸣潮自动战斗的完整配置（导出供一致性测试使用）。
func CombatSpec() combat.Spec { return combatSpec() }

// combatSpec 的按键与连招参考 ok-ww：
// 共鸣技能 E、声骸 Q、共鸣解放 R、闪避 左 Shift，角色头像 1/2/3。
// 战斗判据照搬上游 has_health_bar()：整帧找 enemy_health_color_red
// （R 174-225 / G 55-85 / B 55-76）、宽 >=50、高 5-14、填充率 >=0.95 的实心矩形。
// 上游在未进入战斗时用 min_width=50（100/3840），进入后放宽到 6；这里保持 50，
// 靠 Hold 把证据间断连起来，避免开放世界里把红色 UI 当血条。
func combatSpec() combat.Spec {
	return combat.Spec{
		// 画面配置：assets/games/wuwa/screens/combat.yml（含"中断提示"区域）。
		Screen: "鸣潮战斗",
		HealthBar: &combat.HealthBar{
			Color:     [2][3]uint8{{174, 55, 55}, {225, 85, 76}},
			MinWidth:  50,
			MinHeight: 5,
			MaxHeight: 14,
		},
		// Boss 血条：上游 is_boss() 的区域 (1269/3840, 58/2160)-(2533/3840, 200/2160)，
		// 颜色 boss_health_color R245-255 / G30-185 / B4-75，要求区域内恰好一条。
		BossBar: &combat.BossBar{
			Color:     [2][3]uint8{{245, 30, 4}, {255, 185, 75}},
			Rect:      geom.Rect{X1: 634, Y1: 29, X2: 1266, Y2: 100},
			MinWidth:  50,
			MinHeight: 6,
		},
		// 协奏值能量环：上游 get_con_box() 的 (1431,1942)-(1557,2068)（4K、水平居中锚点），
		// 换算到 1080p 是 (716,971)-(779,1034)；环半径按区域高度比例内外 0.35119 / 0.42261。
		Concerto: &combat.Concerto{
			Rect:  geom.Rect{X1: 716, Y1: 971, X2: 779, Y2: 1034},
			Inner: 0.35119,
			Outer: 0.42261,
			// 覆盖率到这个数认为协奏满；上游是拿满环面积做基准换算，这里取经验值。
			FullRatio: 0.8,
			Colors: [][2][3]uint8{
				{{205, 190, 90}, {235, 222, 130}}, // 衍射（黄）
				{{150, 95, 210}, {190, 140, 249}}, // 导电（紫）
				{{200, 100, 75}, {230, 130, 105}}, // 热熔（红）
				{{60, 150, 210}, {95, 180, 245}},  // 冷凝（蓝）
				{{70, 215, 155}, {110, 250, 190}}, // 气动（绿）
				{{190, 65, 145}, {220, 105, 175}}, // 湮灭（暗红）
			},
		},
		// 治疗位：默认按 3 号位（上游是识别 is_healer 后切位；这里用固定槽位，
		// 你的治疗不在 3 号位就改这里，或在设置里关掉“战斗前后切治疗”）。
		HealerKey: "3",
		// 目标识别：上游 has_target() 用 has_target / no_target 两张模板比分数。
		// 帧统一归一化到 1920x1080，模板也按这个基准裁，1:1 比较即可。
		Target: &combat.Target{
			Rect:      geom.Rect{X1: 1400, Y1: 930, X2: 1540, Y2: 1045},
			SubDir:    "combat",
			Have:      "has_target",
			Missing:   "no_target",
			Threshold: 0.6,
		},
		// 没锁住目标时多久按一次中键。
		LockInterval: time.Second,
		// 当前角色判读照上游 in_team()：找缺失的键位数字。
		Team:        newWuwaTeam(),
		AttackPoint: geom.Point{X: 960, Y: 540},
		Keys: combat.Keys{
			Skill:      "e",
			Echo:       "q",
			Liberation: "r",
			Dodge:      "lshift",
			Switch:     []string{"1", "2", "3"},
		},
		// 技能就绪判据照上游 available()：按钮上出现接近纯白的像素即可用。
		Ready: map[combat.StepKind]combat.ReadyCheck{
			combat.StepSkill:      {Area: geom.Rect{X1: 1592, Y1: 938, X2: 1660, Y2: 995}, ColorRange: readyWhite, MinRatio: 0.002},
			combat.StepEcho:       {Area: geom.Rect{X1: 1685, Y1: 938, X2: 1753, Y2: 996}, ColorRange: readyWhite, MinRatio: 0.002},
			combat.StepLiberation: {Area: geom.Rect{X1: 1786, Y1: 938, X2: 1849, Y2: 995}, ColorRange: readyWhite, MinRatio: 0.002},
		},
		// 冷却数字：上游 get_cd()，读到正数就跳过该动作。
		Cooldowns: map[combat.StepKind]geom.Rect{
			combat.StepSkill:      {X1: 1592, Y1: 938, X2: 1660, Y2: 995},
			combat.StepEcho:       {X1: 1685, Y1: 938, X2: 1753, Y2: 996},
			combat.StepLiberation: {X1: 1786, Y1: 938, X2: 1849, Y2: 995},
		},
		// 后台角色大招是否就绪：上游 update_lib_portrait_icon() 的 lib_ready_* 模板。
		UltReady: &combat.UltReadyCheck{
			Slots: []geom.Rect{
				{X1: 1800, Y1: 210, X2: 1900, Y2: 265},
				{X1: 1800, Y1: 340, X2: 1900, Y2: 400},
				{X1: 1800, Y1: 470, X2: 1900, Y2: 530},
			},
			SubDir:    "libready",
			IDs:       []string{"lib_ready_spectro", "lib_ready_electric", "lib_ready_wind", "lib_ready_fire", "lib_ready_ice", "lib_ready_havoc"},
			Threshold: 0.8,
			Bonus:     15,
		},
		// 角色定位（上游 char_type）与大招就绪一起决定协奏满切谁。
		Roles:  agentRoles,
		Switch: &combat.SwitchPolicy{MinInterval: 3 * time.Second},
		// 角色专属连招（上游 do_perform() 的出手顺序）。
		PlansByAgent: agentPlans,
		// 切人后先打入场技：上游 handle_intro() 的通用形态。
		IntroSteps: []combat.Step{{Kind: combat.StepSkill}, {Kind: combat.StepAttack}},
		// 复活/失败界面出现时停手。
		AbortArea:  "中断提示",
		AbortTexts: []string{"复活", "失败", "重新挑战", "挑战失败"},
		// 鸣潮专属能力：共鸣回路提示（强化 E / 重击）。
		Extensions: wuwaExtensions(),
		Plan: []combat.Step{
			{Kind: combat.StepAttack},
			{Kind: combat.StepSkill},
			{Kind: combat.StepAttack},
			{Kind: combat.StepAttack},
			{Kind: combat.StepEcho},
			{Kind: combat.StepAttack},
			{Kind: combat.StepLiberation},
			{Kind: combat.StepAttack},
			{Kind: combat.StepDodge},
		},
	}
}

// readyWhite 是上游 text_white_color：按钮高亮时是接近纯白。
var readyWhite = [2][3]uint8{{253, 253, 253}, {255, 255, 255}}
