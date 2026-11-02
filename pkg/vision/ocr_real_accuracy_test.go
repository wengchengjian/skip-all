//go:build windows

package vision

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"golang.org/x/image/draw"
)

// cropAndScale 按 1920x1080 逻辑坐标裁剪，并缩放到图片实际尺寸。
func cropLogical(src image.Image, r geom.Rect) *image.RGBA {
	b := src.Bounds()
	sx := float64(b.Dx()) / 1920.0
	sy := float64(b.Dy()) / 1080.0
	x1 := int(float64(r.X1) * sx)
	y1 := int(float64(r.Y1) * sy)
	x2 := int(float64(r.X2) * sx)
	y2 := int(float64(r.Y2) * sy)
	if x2 > b.Max.X {
		x2 = b.Max.X
	}
	if y2 > b.Max.Y {
		y2 = b.Max.Y
	}
	sub := src.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(b.Min.X+x1, b.Min.Y+y1, b.Min.X+x2, b.Min.Y+y2))
	dst := image.NewRGBA(image.Rect(0, 0, x2-x1, y2-y1))
	draw.Draw(dst, dst.Bounds(), sub, sub.Bounds().Min, draw.Src)
	return dst
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// TestOCRRealCooldownAccuracy 用真实战斗截图量不同 DetMaxSideLen 下的
// 冷却数字识别结果。耗时低没有意义，前提是数字还得认对。
func TestOCRRealCooldownAccuracy(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	// 上游 ok-ww 的真实战斗截图，含冷却数字。
	images := []string{
		"../../.tmp/ok-ww/tests/images/all_cd_1080p.png",
		"../../.tmp/ok-ww/tests/images/combat_has_cd.png",
	}
	// 鸣潮技能按钮区域（与 games/wuwa/combat.go 的 Cooldowns 一致）。
	rects := map[string]geom.Rect{
		"skill":      {X1: 1592, Y1: 938, X2: 1660, Y2: 995},
		"echo":       {X1: 1685, Y1: 938, X2: 1753, Y2: 996},
		"liberation": {X1: 1786, Y1: 938, X2: 1849, Y2: 995},
	}
	for _, side := range []int{960, 480, 320, 160} {
		opts := DefaultPaddleOptions()
		opts.DetMaxSideLen = side
		engine, err := NewPaddleOCRWithOptions(dir, opts)
		if err != nil {
			t.Skipf("加载 OCR 失败: %v", err)
		}
		for _, path := range images {
			img, err := loadPNG(path)
			if err != nil {
				t.Logf("跳过 %s: %v", path, err)
				continue
			}
			var total time.Duration
			for name, r := range rects {
				crop := cropLogical(img, r)
				start := time.Now()
				blocks, _ := engine.Recognize(crop, OCROptions{})
				cost := time.Since(start)
				total += cost
				texts := make([]string, 0, len(blocks))
				for _, b := range blocks {
					texts = append(texts, fmt.Sprintf("%q(%.2f)", b.Text, b.Confidence))
				}
				t.Logf("side=%3d %-24s %-11s %v -> %v", side, filepath.Base(path), name, cost, texts)
			}
			t.Logf("side=%3d %-24s 三个区域合计 %v", side, filepath.Base(path), total)
		}
		_ = engine.Close()
	}
}
