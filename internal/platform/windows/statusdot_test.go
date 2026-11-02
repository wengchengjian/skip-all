//go:build windows

package windows

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestGradientDotIsPNG(t *testing.T) {
	data := GradientDot(color.RGBA{R: 118, G: 240, B: 156, A: 255}, color.RGBA{R: 16, G: 124, B: 72, A: 255}, 16)
	if len(data) == 0 {
		t.Fatal("GradientDot 返回空")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("不是合法 PNG: %v", err)
	}
	if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
		t.Fatalf("尺寸 = %v", img.Bounds())
	}
	// 中心不透明、四角透明，说明画的是圆点而不是方块。
	_, _, _, centerA := img.At(8, 8).RGBA()
	_, _, _, cornerA := img.At(0, 0).RGBA()
	if centerA == 0 {
		t.Fatal("中心应不透明")
	}
	if cornerA != 0 {
		t.Fatalf("左上角应透明，alpha = %d", cornerA)
	}
}

func TestGradientDotHasVisibleGradient(t *testing.T) {
	// 左上应比右下亮，否则渐变没生效。
	data := GradientDot(color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBA{R: 0, G: 0, B: 0, A: 255}, 16)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	lightR, _, _, _ := img.At(5, 5).RGBA()
	darkR, _, _, _ := img.At(12, 12).RGBA()
	if lightR <= darkR {
		t.Fatalf("左上应比右下亮: %d vs %d", lightR, darkR)
	}
}

func TestGradientDotColorsDiffer(t *testing.T) {
	running := GradientDot(color.RGBA{R: 118, G: 240, B: 156, A: 255}, color.RGBA{R: 16, G: 124, B: 72, A: 255}, 16)
	idle := GradientDot(color.RGBA{R: 206, G: 212, B: 222, A: 255}, color.RGBA{R: 92, G: 98, B: 110, A: 255}, 16)
	if bytes.Equal(running, idle) {
		t.Fatal("不同状态应生成不同颜色")
	}
}
