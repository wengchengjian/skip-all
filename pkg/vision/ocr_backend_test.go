//go:build windows

package vision

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func whiteBlock(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := h / 6; y < h*5/6; y++ {
		for x := w / 8; x < w*7/8; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	return img
}

// TestOCRDetSideLenCost 量 DetMaxSideLen 对小裁剪区的耗时影响。
// go-ocr 的 preprocessDetImage 会把整图缩放到 DetMaxSideLen（最长边），
// 68x57 的冷却区域也会被放大到 960x832——det 算量只跟这个参数有关，
// 跟原图多小无关。这是当前 OCR 慢的主要原因。
func TestOCRDetSideLenCost(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		w, h, side int
	}{
		{"冷却 68x57 side=960", 68, 57, 960},
		{"冷却 68x57 side=480", 68, 57, 480},
		{"冷却 68x57 side=320", 68, 57, 320},
		{"中断 800x360 side=960", 800, 360, 960},
		{"中断 800x360 side=480", 800, 360, 480},
	}
	for _, c := range cases {
		opts := DefaultPaddleOptions()
		opts.DetMaxSideLen = c.side
		opts.RuntimeLib = filepath.Join(dir, "onnxruntime.dll")
		engine, err := NewPaddleOCRWithOptions(dir, opts)
		if err != nil {
			t.Skipf("加载 OCR 失败: %v", err)
		}
		img := whiteBlock(c.w, c.h)
		_, _ = engine.Recognize(img, OCROptions{}) // 预热
		var total time.Duration
		const n = 5
		for i := 0; i < n; i++ {
			start := time.Now()
			_, _ = engine.Recognize(img, OCROptions{})
			total += time.Since(start)
		}
		_ = engine.Close()
		t.Logf("%-24s 平均 %v", c.name, total/n)
	}
}

// TestOCRCooldownDecision 用真实截图验证"是否在冷却"这个决策本身，
// 而不只是看 OCR 原始文本。决策只看有没有正数，能容忍 det 把数字拆成两块。
func TestOCRCooldownDecision(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("../../.tmp/ok-ww/tests/images/all_cd_1080p.png")
	if err != nil {
		t.Skipf("缺少真实截图: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	rects := map[string]geom.Rect{
		"skill":      {X1: 1592, Y1: 938, X2: 1660, Y2: 995},
		"echo":       {X1: 1685, Y1: 938, X2: 1753, Y2: 996},
		"liberation": {X1: 1786, Y1: 938, X2: 1849, Y2: 995},
	}
	for _, side := range []int{960, 480, 320, 160} {
		opts := DefaultPaddleOptions()
		opts.DetMaxSideLen = side
		opts.RuntimeLib = filepath.Join(dir, "onnxruntime.dll")
		engine, err := NewPaddleOCRWithOptions(dir, opts)
		if err != nil {
			t.Skipf("加载 OCR 失败: %v", err)
		}
		var total time.Duration
		for name, r := range rects {
			crop := cropLogical(img, r)
			start := time.Now()
			blocks, _ := engine.Recognize(crop, OCROptions{})
			total += time.Since(start)
			cooling := false
			var texts []string
			for _, b := range blocks {
				texts = append(texts, b.Text)
				if _, ok := parseCooldownText(b.Text); ok {
					cooling = true
				}
			}
			t.Logf("side=%3d %-11s cooling=%v texts=%v", side, name, cooling, texts)
			if !cooling {
				t.Errorf("side=%d %s 判成没在冷却，真实截图里是冷却中", side, name)
			}
		}
		_ = engine.Close()
		t.Logf("side=%3d 三个区域合计 %v", side, total)
	}
}

// parseCooldownText 是 combat 包 parseCooldownSeconds 的等价实现（首字符是数字即视为冷却）。
func parseCooldownText(text string) (float64, bool) {
	var v float64
	_, err := fmt.Sscanf(text, "%f", &v)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
