package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

type fakeOCR struct {
	calls  int
	blocks []TextBlock
	err    error
	check  func(img image.Image) error
}

func (f *fakeOCR) Recognize(img image.Image, opts OCROptions) ([]TextBlock, error) {
	f.calls++
	if f.check != nil {
		if err := f.check(img); err != nil {
			return nil, err
		}
	}
	return append([]TextBlock(nil), f.blocks...), f.err
}

func (f *fakeOCR) Close() error { return nil }

func TestServiceOCRCacheAndOffset(t *testing.T) {
	engine := &fakeOCR{blocks: []TextBlock{{Text: "确认", Rect: geom.Rect{X1: 5, Y1: 6, X2: 25, Y2: 16}, Confidence: 0.9}}}
	svc := NewMatcher(engine, nil)
	frame := image.NewRGBA(image.Rect(0, 0, 200, 100))
	area := screen.Area{Name: "确认", Type: screen.AreaText, Text: "确认", Rect: geom.Rect{X1: 50, Y1: 20, X2: 150, Y2: 80}}

	blocks, err := svc.OCR(frame, area, OCROptions{})
	if err != nil {
		t.Fatalf("OCR 失败: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("结果数 = %d, want 1", len(blocks))
	}
	if blocks[0].Rect != (geom.Rect{X1: 55, Y1: 26, X2: 75, Y2: 36}) {
		t.Errorf("区域偏移错误: %+v", blocks[0].Rect)
	}

	if _, err := svc.OCR(frame, area, OCROptions{}); err != nil {
		t.Fatal(err)
	}
	if engine.calls != 1 {
		t.Errorf("同帧同区域应命中缓存, 调用次数 = %d", engine.calls)
	}

	newFrame := image.NewRGBA(image.Rect(0, 0, 200, 100))
	if _, err := svc.OCR(newFrame, area, OCROptions{}); err != nil {
		t.Fatal(err)
	}
	if engine.calls != 2 {
		t.Errorf("换帧后应重新识别, 调用次数 = %d", engine.calls)
	}
}

func TestServiceOCRColorFilter(t *testing.T) {
	checked := false
	engine := &fakeOCR{blocks: []TextBlock{{Text: "红字", Confidence: 0.9}}, check: func(img image.Image) error {
		checked = true
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				if r != g || g != bl {
					return errNotGray
				}
				if (r>>8) != 0 && (r>>8) != 255 {
					return errNotBinary
				}
			}
		}
		return nil
	}}
	svc := NewMatcher(engine, nil)
	frame := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := 20; y < 30; y++ {
		for x := 30; x < 40; x++ {
			frame.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	area := screen.Area{Rect: geom.Rect{X1: 20, Y1: 10, X2: 80, Y2: 40}}
	rangeValue := &[2][3]uint8{{255, 0, 0}, {255, 0, 0}}
	if _, err := svc.OCR(frame, area, OCROptions{ColorRange: rangeValue}); err != nil {
		t.Fatalf("颜色过滤 OCR 失败: %v", err)
	}
	if !checked {
		t.Error("未检查过滤后的图像")
	}
}

func TestServiceOCRUsesAreaColorRange(t *testing.T) {
	filtered := false
	engine := &fakeOCR{blocks: []TextBlock{{Text: "白字", Confidence: 0.9}}, check: func(img image.Image) error {
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				if r == g && g == bl && (r>>8) == 255 {
					filtered = true
				}
			}
		}
		return nil
	}}
	svc := NewMatcher(engine, nil)
	frame := image.NewRGBA(image.Rect(0, 0, 60, 30))
	frame.SetRGBA(10, 10, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	area := screen.Area{
		Rect:       geom.Rect{X1: 0, Y1: 0, X2: 60, Y2: 30},
		ColorRange: &[2][3]uint8{{240, 240, 240}, {255, 255, 255}},
	}
	if _, err := svc.OCR(frame, area, OCROptions{}); err != nil {
		t.Fatalf("区域颜色过滤 OCR 失败: %v", err)
	}
	if !filtered {
		t.Error("应使用区域自带的 color_range 过滤")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

const errNotGray = errString("过滤后不是灰度图")
const errNotBinary = errString("过滤后不是黑白图")

type fakeStore struct {
	raw  image.Image
	mask image.Image
}

func (f *fakeStore) Load(ref TemplateRef) (image.Image, image.Image, error) {
	return f.raw, f.mask, nil
}

func TestServiceTemplateAbsoluteRect(t *testing.T) {
	tpl := testPattern(10, 8)
	frame := image.NewRGBA(image.Rect(0, 0, 100, 80))
	paste(frame, tpl, image.Pt(40, 30))
	svc := NewMatcher(nil, &fakeStore{raw: tpl})
	area := screen.Area{Rect: geom.Rect{X1: 30, Y1: 20, X2: 90, Y2: 70}, TemplateMatchThreshold: 0.9}

	matches, err := svc.Template(frame, area, TemplateRef{SubDir: "x", ID: "y"}, 0.9)
	if err != nil {
		t.Fatalf("模板匹配失败: %v", err)
	}
	if len(matches) != 1 || matches[0].Rect.X1 != 40 || matches[0].Rect.Y1 != 30 {
		t.Fatalf("模板匹配应返回整帧绝对坐标: %+v", matches)
	}
}

func TestServiceColor(t *testing.T) {
	svc := NewMatcher(nil, nil)
	frame := image.NewRGBA(image.Rect(0, 0, 20, 20))
	stats, err := svc.Color(frame, screen.Area{Rect: geom.Rect{X1: 0, Y1: 0, X2: 20, Y2: 20}})
	if err != nil {
		t.Fatal(err)
	}
	if stats.GrayMaskPercent != 1 {
		t.Errorf("全黑区域灰度占比 = %v, want 1", stats.GrayMaskPercent)
	}
}
