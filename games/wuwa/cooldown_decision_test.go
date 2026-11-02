//go:build windows

package wuwa

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// readyScreens 是确认"技能就绪、没有冷却数字"的真实截图及其区域。
// 已用人眼放大确认（cooldown-sheet.png 对照表）：这些格子里没有数字。
var readyScreens = []struct {
	image  string
	region string
}{
	{"in_combat.png", "skill"},
	{"con_full.png", "skill"},
	{"con_full.png", "echo"},
	{"con_full.png", "liberation"},
}

// coolingScreens 是确认"确实有冷却数字"的真实截图。
var coolingScreens = []struct {
	image  string
	region string
	want   string
}{
	{"in_combat.png", "echo", "18.1"},
	{"in_combat3.png", "skill", "1.3"},
	{"all_cd_1080p.png", "echo", "8.4"},
	{"all_cd_1080p.png", "liberation", "22.6"},
}

// TestCooldownDecisionOnRealScreens 用真实截图验证冷却判定不会误判。
// 误判成冷却会让技能永远不放（用户实测到的"只会普攻"就是这样退化的）。
func TestCooldownDecisionOnRealScreens(t *testing.T) {
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

	decide := func(img *image.RGBA, region string) (bool, []string) {
		rect := cooldownRegions[region]
		blocks, err := matcher.OCR(img, screen.Area{Name: "技能冷却", Rect: rect}, vision.OCROptions{})
		if err != nil {
			t.Fatalf("OCR 失败: %v", err)
		}
		var texts []string
		cooling := false
		for _, b := range blocks {
			texts = append(texts, strings.TrimSpace(b.Text))
			if b.Confidence < 0.5 {
				continue
			}
			if seconds, ok := combat.ParseCooldownSeconds(b.Text); ok && seconds > 0 {
				cooling = true
			}
		}
		return cooling, texts
	}

	loaded := map[string]*image.RGBA{}
	load := func(name string) *image.RGBA {
		if img, ok := loaded[name]; ok {
			return img
		}
		f, err := os.Open("../../.tmp/ok-ww/tests/images/" + name)
		if err != nil {
			t.Skipf("缺少截图 %s: %v", name, err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		scaled := toLogical(img)
		loaded[name] = scaled
		return scaled
	}

	for _, c := range readyScreens {
		img := load(c.image)
		cooling, texts := decide(img, c.region)
		if cooling {
			t.Errorf("%s 的 %s 被判成冷却，但该按钮是就绪的（读到 %v）", c.image, c.region, texts)
		} else {
			t.Logf("%-20s %-11s 就绪 -> 判定正确（读到 %v）", c.image, c.region, texts)
		}
	}
	for _, c := range coolingScreens {
		img := load(c.image)
		cooling, texts := decide(img, c.region)
		if !cooling {
			t.Errorf("%s 的 %s 应当判成冷却（期望 %s），实际读到 %v", c.image, c.region, c.want, texts)
		} else {
			t.Logf("%-20s %-11s 冷却 -> 判定正确（读到 %v）", c.image, c.region, texts)
		}
	}
}
