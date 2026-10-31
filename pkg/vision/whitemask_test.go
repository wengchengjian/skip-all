package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// whiteGlyph 在 frame 的 rect 内画一个"跳过图标"似的白色图形。
func whiteGlyph(img *image.RGBA, rect image.Rectangle, level uint8) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// 画一个三角形 + 竖条，形状固定，亮度可调
			inTriangle := x-rect.Min.X < (y-rect.Min.Y)*2 && (rect.Max.Y-y) > (x-rect.Min.X)/2
			inBar := x > rect.Max.X-4
			if inTriangle || inBar {
				img.SetRGBA(x, y, color.RGBA{level, level, level, 255})
			}
		}
	}
}

func darkFrame(w, h int, level uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{level, level, level, 255})
		}
	}
	return img
}

func TestWhiteMaskBinarizesLitPixels(t *testing.T) {
	img := darkFrame(10, 10, 40)
	img.SetRGBA(3, 3, color.RGBA{230, 235, 240, 255}) // 白色：保留
	img.SetRGBA(4, 4, color.RGBA{150, 200, 230, 255}) // 偏蓝、R 不够：丢掉
	mask := WhiteMask(img)
	if got := mask.RGBAAt(3, 3).R; got != 255 {
		t.Fatalf("亮像素应变成白色，实际 %d", got)
	}
	if got := mask.RGBAAt(4, 4).R; got != 0 {
		t.Fatalf("带色像素应变成黑色，实际 %d", got)
	}
	if got := mask.RGBAAt(0, 0).R; got != 0 {
		t.Fatalf("暗背景应变成黑色，实际 %d", got)
	}
}

func TestMatchTemplateWhiteMaskFindsLitIcon(t *testing.T) {
	frame := darkFrame(200, 120, 50)
	whiteGlyph(frame, image.Rect(30, 20, 58, 48), 245)
	template := darkFrame(28, 28, 0)
	whiteGlyph(template, template.Bounds(), 255)

	matches, err := MatchTemplateWhiteMask(frame, geom.Rect{X1: 0, Y1: 0, X2: 200, Y2: 120}, template, 0.85)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("应命中白色图标")
	}
	if matches[0].Rect.X1 != 30 || matches[0].Rect.Y1 != 20 {
		t.Fatalf("命中位置 = %v, want (30,20)", matches[0].Rect)
	}
}

func TestMatchTemplateWhiteMaskIgnoresDimIcon(t *testing.T) {
	// 图标不够亮（R 低于 210）时按上游 convert_dialog_icon 的规则不参与匹配。
	frame := darkFrame(200, 120, 50)
	whiteGlyph(frame, image.Rect(30, 20, 58, 48), 150)
	template := darkFrame(28, 28, 0)
	whiteGlyph(template, template.Bounds(), 255)

	matches, err := MatchTemplateWhiteMask(frame, geom.Rect{X1: 0, Y1: 0, X2: 200, Y2: 120}, template, 0.85)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("暗图标不应命中，实际 %v", matches[0].Rect)
	}
}
