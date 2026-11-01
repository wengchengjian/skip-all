package wuwa

import (
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
