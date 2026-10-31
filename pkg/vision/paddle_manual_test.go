//go:build manual

package vision

import (
	"image"
	_ "image/jpeg"
	"os"
	"testing"
)

// TestPaddleOCRSmoke 是需要真实模型的冒烟测试，默认构建不包含。
// 运行方式：SKIP_ALL_OCR_MODEL_DIR=<模型目录> SKIP_ALL_OCR_TEST_IMAGE=<图片> go test -tags manual ./pkg/vision -run TestPaddleOCRSmoke -v
func TestPaddleOCRSmoke(t *testing.T) {
	modelDir := os.Getenv("SKIP_ALL_OCR_MODEL_DIR")
	imagePath := os.Getenv("SKIP_ALL_OCR_TEST_IMAGE")
	if modelDir == "" || imagePath == "" {
		t.Skip("未设置 SKIP_ALL_OCR_MODEL_DIR 或 SKIP_ALL_OCR_TEST_IMAGE")
	}
	engine, err := NewPaddleOCR(modelDir)
	if err != nil {
		t.Fatalf("NewPaddleOCR: %v", err)
	}
	defer engine.Close()

	f, err := os.Open(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := engine.Recognize(img, OCROptions{})
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("OCR 未识别出任何文本")
	}
	for _, block := range blocks {
		t.Logf("%+v %q %.3f", block.Rect, block.Text, block.Confidence)
	}
}
