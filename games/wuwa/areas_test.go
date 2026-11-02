package wuwa

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// 这些数字来自上游 ok-ww 的模板文件名（文件名里带 3840x2160 原始坐标）
// 与 get_con_box() 的盒子定义。改这些区域时必须能对上上游，否则会"看着有功能实际不工作"。
func TestAreasMatchUpstreamCoordinates(t *testing.T) {
	spec := combatSpec()

	// 协奏环：上游 get_con_box() = box_of_screen_scaled(3840,2160, 1431,1942,1557,2068)
	// 折半到 1080p 是 (715.5,971)-(778.5,1034)。
	if spec.Concerto == nil {
		t.Fatal("没有协奏环配置")
	}
	want := geom.Rect{X1: 716, Y1: 971, X2: 779, Y2: 1034}
	if spec.Concerto.Rect != want {
		t.Fatalf("协奏环区域 = %+v，应当与上游折半一致 = %+v", spec.Concerto.Rect, want)
	}

	// 目标锁定：上游 has_target 模板记录在 4K (2868,1917)，折半 (1434,958)；
	// 模板尺寸 61x58，所以检测区必须覆盖 (1434,958)-(1495,1016)。
	if spec.Target == nil || spec.Target.Rect == (geom.Rect{}) {
		t.Fatal("没有目标锁定区域")
	}
	tpl := geom.Rect{X1: 1434, Y1: 958, X2: 1495, Y2: 1016}
	r := spec.Target.Rect
	if r.X1 > tpl.X1 || r.Y1 > tpl.Y1 || r.X2 < tpl.X2 || r.Y2 < tpl.Y2 {
		t.Fatalf("目标锁定区域 %+v 没覆盖上游模板位置 %+v", r, tpl)
	}

	// 首领血条：上游 box_of_screen(1269/3840, 58/2160, 2533/3840, 200/2160) -> (634,29)-(1266,100)
	if spec.BossBar == nil {
		t.Fatal("没有首领血条配置")
	}
	wantBoss := geom.Rect{X1: 634, Y1: 29, X2: 1266, Y2: 100}
	if spec.BossBar.Rect != wantBoss {
		t.Fatalf("首领血条区域 = %+v，应当 = %+v", spec.BossBar.Rect, wantBoss)
	}
}

// TestUltimateSlotsCoverUpstreamIcons 用上游模板文件名里的原始坐标锁定后台大招就绪的槽位。
// 上游 update_lib_portrait_icon() 会去 lib_mark_char_N 里找 lib_ready_<元素> 模板，
// 模板记录在 4K (3653,475) / (3654,742) / (3653,1003) 附近，折半后必须落在对应槽位里。
func TestUltimateSlotsCoverUpstreamIcons(t *testing.T) {
	spec := combatSpec()
	if spec.UltReady == nil {
		t.Fatal("没有后台大招就绪判据")
	}
	if len(spec.UltReady.Slots) != 3 {
		t.Fatalf("槽位数量 = %d，应当是 3", len(spec.UltReady.Slots))
	}
	// 三个槽位的图标左上角（4K 折半），模板本身约 12x13 像素。
	icons := []geom.Rect{
		{X1: 1826, Y1: 237, X2: 1840, Y2: 251}, // wind    (3653,475)
		{X1: 1827, Y1: 371, X2: 1841, Y2: 385}, // electric (3654,742)
		{X1: 1826, Y1: 501, X2: 1840, Y2: 515}, // havoc   (3653,1003)
	}
	for i, icon := range icons {
		slot := spec.UltReady.Slots[i]
		if slot.X1 > icon.X1 || slot.Y1 > icon.Y1 || slot.X2 < icon.X2 || slot.Y2 < icon.Y2 {
			t.Errorf("第 %d 个槽位 %+v 没覆盖上游图标位置 %+v", i+1, slot, icon)
		}
	}
	// 就绪模板必须覆盖 6 种元素。
	if len(spec.UltReady.IDs) != 6 {
		t.Errorf("就绪模板 = %d 个，应当覆盖 6 种元素", len(spec.UltReady.IDs))
	}
}

// TestConcertoColorsMatchUpstream 逐项比对上游 con_colors（6 种元素色）。
func TestConcertoColorsMatchUpstream(t *testing.T) {
	spec := combatSpec()
	want := [][2][3]uint8{
		{{205, 190, 90}, {235, 222, 130}}, // 衍射
		{{150, 95, 210}, {190, 140, 249}}, // 导电
		{{200, 100, 75}, {230, 130, 105}}, // 热熔
		{{60, 150, 210}, {95, 180, 245}},  // 冷凝
		{{70, 215, 155}, {110, 250, 190}}, // 气动
		{{190, 65, 145}, {220, 105, 175}}, // 湮灭
	}
	if len(spec.Concerto.Colors) != len(want) {
		t.Fatalf("协奏色数量 = %d，应当 = %d", len(spec.Concerto.Colors), len(want))
	}
	for i := range want {
		if spec.Concerto.Colors[i] != want[i] {
			t.Errorf("第 %d 组协奏色 = %v，应当 = %v", i, spec.Concerto.Colors[i], want[i])
		}
	}
	// 技能就绪用上游 text_white_color（244-255，src/__init__.py）。
	ready := spec.Ready[combat.StepSkill]
	if ready.ColorRange != [2][3]uint8{{244, 244, 244}, {255, 255, 255}} {
		t.Errorf("技能就绪色 = %v，应当与上游 text_white_color 一致", ready.ColorRange)
	}
}
