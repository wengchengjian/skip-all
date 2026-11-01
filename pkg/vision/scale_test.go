package vision

import (
	"image"
	"image/color"
	"testing"
)

func TestScaleTemplateScalesBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	out := ScaleTemplate(img, 0.5)
	if out == nil || out.Bounds().Dx() != 2 || out.Bounds().Dy() != 2 {
		t.Fatalf("缩放结果 = %v", out.Bounds())
	}
	if ScaleTemplate(nil, 1) != nil {
		t.Fatal("空模板应当返回 nil")
	}
}
