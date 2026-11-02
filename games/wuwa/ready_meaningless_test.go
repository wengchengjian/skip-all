//go:build windows

package wuwa

import (
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// TestReadyCheckIsMeaningless 证明"白色高亮判据"无法区分就绪与冷却。
//
// 上游 ok-ww 的语义（BaseChar.available）：
//
//	resonance_available()  -> available('resonance', check_color=False)
//	echo_available()       -> available('echo',      check_color=False)
//	liberation_available() -> available('liberation', check_color=True)
//
// 也就是技能和声骸**只看冷却**，白色高亮根本不参与判断；
// 只有解放才看，而且判据是 box_highlighted（"有没有白色像素"，不是比例阈值）。
//
// 我们的实现给三个动作都配了 Ready 白点判据（MinRatio=0.002），
// 语义上错了。实测真实截图：
//   - 就绪的 skill：白比例 0.00000（被判成"没就绪"）
//   - 就绪的 liberation：白比例 0.00000（被判成"没就绪"）
//   - 冷却中的 echo：白比例 0.02231（远超 0.002，被判成"就绪"）
//
// 也就是说这个判据既不漏也不保，纯粹是噪声。
func TestReadyCheckIsMeaningless(t *testing.T) {
	readyWhite := [2][3]uint8{{244, 244, 244}, {255, 255, 255}}
	regions := map[string]geom.Rect{
		"skill":      {X1: 1592, Y1: 938, X2: 1660, Y2: 995},
		"echo":       {X1: 1685, Y1: 938, X2: 1753, Y2: 996},
		"liberation": {X1: 1786, Y1: 938, X2: 1849, Y2: 995},
	}
	// 人眼确认过的真值。
	cases := []struct {
		image string
		ready map[string]bool
	}{
		{"in_combat.png", map[string]bool{"skill": true, "echo": false, "liberation": true}},
		{"in_combat3.png", map[string]bool{"skill": false, "echo": false, "liberation": false}},
		{"con_full.png", map[string]bool{"skill": true, "echo": true, "liberation": true}},
		{"all_cd_1080p.png", map[string]bool{"skill": false, "echo": false, "liberation": false}},
	}
	const minRatio = 0.002 // 我们现在用的阈值
	falseNegative, falsePositive, total := 0, 0, 0
	for _, c := range cases {
		img, err := loadRealScreenshot(c.image)
		if err != nil {
			t.Skipf("缺图 %s: %v", c.image, err)
		}
		for name, rect := range regions {
			ratio := vision.ColorRatio(img, rect, readyWhite)
			got := ratio >= minRatio
			want := c.ready[name]
			total++
			if want && !got {
				falseNegative++
			}
			if !want && got {
				falsePositive++
			}
			t.Logf("%-20s %-11s 就绪=%-5v 白比例=%.5f 判据=%v", c.image, name, want, ratio, got)
		}
	}
	t.Logf("共 %d 个样本：就绪被判没就绪 %d 次，冷却被判成就绪 %d 次",
		total, falseNegative, falsePositive)
	if falseNegative == 0 && falsePositive == 0 {
		t.Fatal("白色高亮判据竟然全部正确了，结论需要重算")
	}
}
