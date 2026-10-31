package vision

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewPaddleOCRModelMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := NewPaddleOCR(dir)
	if err == nil {
		t.Fatal("模型缺失时应返回错误")
	}
	if !errors.Is(err, ErrModelMissing) {
		t.Fatalf("错误应可被 ErrModelMissing 判定: %v", err)
	}
}

func TestFindPaddleFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"det.onnx", "rec.onnx", "ppocrv6_dict.txt", "onnxruntime.dll"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := findPaddleFiles(dir)
	if err != nil {
		t.Fatalf("findPaddleFiles 失败: %v", err)
	}
	if files.det != filepath.Join(dir, "det.onnx") {
		t.Errorf("det 路径错误: %s", files.det)
	}
	if files.dict != filepath.Join(dir, "ppocrv6_dict.txt") {
		t.Errorf("dict 路径错误: %s", files.dict)
	}
	if files.onnxRuntime != filepath.Join(dir, "onnxruntime.dll") {
		t.Errorf("onnxruntime 路径错误: %s", files.onnxRuntime)
	}
}
