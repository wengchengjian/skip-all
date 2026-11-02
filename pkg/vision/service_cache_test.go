package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// countingOCR 记录被真正调用了几次。
type countingOCR struct {
	calls int
	text  string
}

func (c *countingOCR) Recognize(image.Image, OCROptions) ([]TextBlock, error) {
	c.calls++
	return []TextBlock{{Text: c.text, Confidence: 1}}, nil
}
func (c *countingOCR) Close() error { return nil }

// TestServiceCacheInvalidatesOnBufferReuse 验证帧缓冲被复用（同一指针、
// 内容变了）时，OCR 缓存必须失效。
//
// 这是帧缓冲池引入后的真实风险：截图器会把同一块内存交给下一拍，
// 只比较 image 指针的缓存会一直命中旧结果。
func TestServiceCacheInvalidatesOnBufferReuse(t *testing.T) {
	ocr := &countingOCR{text: "第一帧"}
	svc := NewMatcher(ocr, nil)
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	area := screen.Area{Name: "区域", Rect: geom.Rect{X1: 100, Y1: 100, X2: 200, Y2: 160}}

	// 第一帧：填白，识别一次。
	fill(img, area.Rect, color.RGBA{255, 255, 255, 255})
	if _, err := svc.OCR(img, area, OCROptions{}); err != nil {
		t.Fatal(err)
	}
	if ocr.calls != 1 {
		t.Fatalf("首次识别调用 %d 次，应当 1 次", ocr.calls)
	}

	// 同一帧重复请求：应当命中缓存，不再调用。
	if _, err := svc.OCR(img, area, OCROptions{}); err != nil {
		t.Fatal(err)
	}
	if ocr.calls != 1 {
		t.Fatalf("同一帧重复请求触发了 %d 次识别，缓存没生效", ocr.calls)
	}

	// 模拟下一拍复用同一块缓冲：指针不变，内容变了。
	fill(img, area.Rect, color.RGBA{10, 20, 30, 255})
	if _, err := svc.OCR(img, area, OCROptions{}); err != nil {
		t.Fatal(err)
	}
	if ocr.calls != 2 {
		t.Fatalf("缓冲内容变了但 OCR 只被调用 %d 次，缓存返回了上一拍的旧结果", ocr.calls)
	}

	// 再拍一次，内容又变：应当再识别。
	fill(img, area.Rect, color.RGBA{200, 100, 50, 255})
	if _, err := svc.OCR(img, area, OCROptions{}); err != nil {
		t.Fatal(err)
	}
	if ocr.calls != 3 {
		t.Fatalf("第三次内容变化后 OCR 调用 %d 次，应当 3 次", ocr.calls)
	}
}

func fill(img *image.RGBA, r geom.Rect, c color.RGBA) {
	for y := r.Y1; y < r.Y2; y++ {
		for x := r.X1; x < r.X2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
