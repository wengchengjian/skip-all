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
	// 队伍识别既给通用流程用，也给今汐特化扩展用，所以只建一次。
	team := newWuwaTeam()
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
			// 上游 switch_next_char 要求协奏值正好满（current_con == 1）才切；
			// 0.8~0.99 之间它是 sleep 50ms 再查。这里取 0.95 当满的门限。
			FullRatio: 0.95,
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
		Team:        team,
		AttackPoint: geom.Point{X: 960, Y: 540},
		Keys: combat.Keys{
			Skill:      "e",
			Echo:       "q",
			Liberation: "r",
			Dodge:      "lshift",
			Switch:     []string{"1", "2", "3"},
		},
		// 技能就绪判据：**不配**。照上游 BaseChar.available 的语义，
		// 可用性只看冷却，白色高亮不参与判断：
		//   resonance_available()  -> available('resonance', check_color=False)
		//   echo_available()       -> available('echo',      check_color=False)
		//   liberation_available() -> available('liberation', check_color=True)
		// 后者的 check_color 走 box_highlighted，判据是"有没有白色像素"(>0)，
		// 不是比例阈值。
		//
		// 实测（4 张真实截图 × 3 个按钮，共 12 个样本）：按钮的白色像素比例
		// 与是否就绪**完全无关**——就绪的 skill 白比例 0.00000，冷却中的 echo
		// 白比例 0.02231。按 MinRatio=0.002 判，12 个样本里就绪被判没就绪 2 次、
		// 冷却被判成就绪 7 次，纯噪声。配这个判据只会让技能被误锁住，
		// 退化成只会普攻。见 games/wuwa/ready_meaningless_test.go。,
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
		// 空中判据：上游 BaseChar.flying() —— 队伍带滑翔装置（edge_levitator 模板，
		// 开战 find_one threshold=0.65）时，edge_levitator 图标区域白色像素占比 < 0.1 算在空中。
		// 模板记录 4K (3015,1887) 82x82，折半后图标区 (1507,943)-(1549,985)。
		Flying: &combat.FlyingCheck{
			Area:              geom.Rect{X1: 1507, Y1: 943, X2: 1549, Y2: 985},
			ColorRange:        [2][3]uint8{{244, 244, 244}, {255, 255, 255}},
			MaxRatio:          0.1,
			TemplateArea:      geom.Rect{X1: 1499, Y1: 935, X2: 1557, Y2: 993},
			TemplateSubDir:    "flying",
			TemplateID:        "edge_levitator",
			TemplateThreshold: 0.65,
		},
		// 解放窗口：上游 BaseChar.still_in_liberation() 基类 12 秒；Encore 9.5、Xiangliyao 25 各自重写。
		LiberationWindows: map[string]time.Duration{
			"char_encore":     9500 * time.Millisecond,
			"char_xiangliyao": 25 * time.Second,
		},
		// 共鸣回路是否已满：上游 BaseChar.is_forte_full()，4K (2251,1993)-(2311,2016) 折半，
		// 近白像素（244-255/246-255/250-255）占比 > 0.08 视为满。
		ForteFull: &combat.ForteCheck{
			Area:       geom.Rect{X1: 1125, Y1: 996, X2: 1156, Y2: 1008},
			ColorRange: [2][3]uint8{{244, 246, 250}, {255, 255, 255}},
			MinRatio:   0.08,
		},
		// 上游只有少数角色的技能明确等回路状态，其余角色不门控。
		ForteGate: agentForteGate,
		// 角色定位（上游 char_type）与大招就绪一起决定协奏满切谁。
		Roles:         agentRoles,
		SwitchSynergy: agentSynergy,
		SwitchSynergyAfter: map[string]map[string]time.Duration{
			"char_phrolova": {"char_cantarella": 14 * time.Second},
		},
		SwitchSynergyTeam:     map[string]string{"char_moning": "char_linnai"},
		SwitchSynergyUltReady: map[string]string{"char_zhezhi": "char_carlotta"},
		LiberationMustSwitch: map[string]bool{
			"char_brant":  true,
			"char_encore": true,
			"char_lupa":   true,
			"char_zani":   true,
		},
		SwitchIntroMUST: map[string]bool{
			"char_camellya": true,
			"char_jinhsi":   true,
		},
		SwitchIntroAfter: map[string]time.Duration{"char_luhesi": 24 * time.Second, "char_suisui": 40 * time.Second},
		// 上游 get_switch_priority 的 NO 锁：Brant 的 perform_anchor<4s、
		// Encore 的 last_heavy<4.6s。用最近一次技能/重击时间近似。
		SwitchNoAfterSkill:      map[string]time.Duration{"char_brant": 4 * time.Second, "char_zani": 1600 * time.Millisecond},
		SwitchNoAfterHeavy:      map[string]time.Duration{"char_encore": 4600 * time.Millisecond},
		SwitchNoAfterLiberation: map[string]time.Duration{"char_phrolova": 24 * time.Second, "char_ciaccona": 20 * time.Second},
		SwitchNoAfterPerform:    map[string]time.Duration{"char_douling": 8 * time.Second, "char_suisui": 16 * time.Second},
		ResonanceMustSwitch:     map[string]bool{"char_encore": true},
		// 上游 wait_switch()：Zani 的夜落 2.2s、Carlotta 的声骸后 2.5s 先不切走；
		// Cantarella 的 perform_under_outro 在 last_heavy<8s 内继续循环。
		HoldSwitchAfterHeavy: map[string]time.Duration{"char_zani": 2200 * time.Millisecond, "char_cantarella": 8 * time.Second},
		HoldSwitchAfterEcho:  map[string]time.Duration{"char_carlotta": 2500 * time.Millisecond},
		// check_outro 只改循环时长的分支：Hiyuki<-Linnai 18s、Augusta<-Iuno 14s。
		HoldSwitchAfterOutro: map[string]map[string]time.Duration{
			"char_hiyuki":  {"char_linnai": 18 * time.Second},
			"char_augusta": {"char_iuno": 14 * time.Second},
		},
		BuffTimes: agentBuffTime,
		Switch:    &combat.SwitchPolicy{MinInterval: 3 * time.Second},
		// 角色专属连招（上游 do_perform() 的出手顺序）。
		PlansByAgent: agentPlans,
		// 入场：上游 wait_down()/wait_intro() —— 有入场技时持续点普攻，
		// 直到角色落地（用"技能可用"判断），超时 1.2 秒。
		// 只有今汐（Jinhsi）重写了 handle_intro，属于个例，这里按基类行为实现。
		Intro: &combat.IntroSpec{MaxDuration: 1200 * time.Millisecond},
		// 今汐重写了 handle_intro：入场后技能可用就一直按，进冷却后用声骸/普攻收尾，
		// 超时 1.5 秒（上游 Jinhsi.handle_intro）。
		IntroByAgent: map[string]combat.IntroSpec{
			"char_jinhsi": {Mode: combat.IntroBurn, MaxDuration: 1500 * time.Millisecond},
		},
		// 复活/失败界面出现时停手。
		AbortArea:  "中断提示",
		AbortTexts: []string{"复活", "失败", "重新挑战", "挑战失败"},
		// 鸣潮专属能力：共鸣回路提示（强化 E / 重击）。
		Extensions: wuwaExtensions(team),
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
