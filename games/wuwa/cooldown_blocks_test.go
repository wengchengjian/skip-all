//go:build windows

package wuwa

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// TestCooldownOCRBlocks 打印每个区域 OCR 出的原始文本块（含置信度与坐标），
// 用来判断"图标被读成数字"这类误判的特征。
func TestCooldownOCRBlocks(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCR(dir)
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()
	matcher := vision.NewMatcher(engine, nil)

	images := []string{
		"in_combat.png", "in_combat3.png", "con_full.png", "all_cd_1080p.png",
	}
	order := []string{"skill", "echo", "liberation"}
	for _, name := range images {
		f, err := os.Open("../../.tmp/ok-ww/tests/images/" + name)
		if err != nil {
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		base := toLogical(img)
		for _, region := range order {
			rect := cooldownRegions[region]
			blocks, err := matcher.OCR(base, screen.Area{Name: "技能冷却", Rect: rect}, vision.OCROptions{})
			if err != nil {
				t.Fatal(err)
			}
			line := fmt.Sprintf("%-20s %-11s", name, region)
			for _, b := range blocks {
				seconds, ok := combat.ParseCooldownSeconds(b.Text)
				cx := (b.Rect.X1 + b.Rect.X2) / 2
				cy := (b.Rect.Y1 + b.Rect.Y2) / 2
				// 相对于区域中心的偏移
				rcx := rect.X1 + rect.Width()/2
				rcy := rect.Y1 + rect.Height()/2
				line += fmt.Sprintf(" [%q conf=%.2f at(%d,%d) d=(%d,%d) sec=%.1f ok=%v]",
					b.Text, b.Confidence, cx, cy, cx-rcx, cy-rcy, seconds, ok)
			}
			t.Log(line)
		}
	}
}
