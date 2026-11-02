//go:build windows

package windows

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestStatusDotIsPNG(t *testing.T) {
	data := StatusDot(color.RGBA{R: 64, G: 186, B: 106, A: 255}, 16)
	if len(data) == 0 {
		t.Fatal("StatusDot 返回空")
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

func TestStatusDotColorsDiffer(t *testing.T) {
	running := StatusDot(color.RGBA{R: 64, G: 186, B: 106, A: 255}, 16)
	idle := StatusDot(color.RGBA{R: 150, G: 154, B: 162, A: 255}, 16)
	if bytes.Equal(running, idle) {
		t.Fatal("不同状态应生成不同颜色")
	}
}
