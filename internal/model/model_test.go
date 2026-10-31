package model

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/vision"
)

func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestDownloadFile(t *testing.T) {
	payload := []byte("hello model")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "11")
		w.Write(payload)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "sub", "model.bin")
	var percents []float64
	err := DownloadFile(context.Background(), server.URL, dest, shaHex(payload), func(percent float64, _ string) {
		percents = append(percents, percent)
	})
	if err != nil {
		t.Fatalf("DownloadFile 失败: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("下载内容错误: %v %q", err, data)
	}
	if len(percents) == 0 || percents[len(percents)-1] != 100 {
		t.Errorf("进度回调错误: %v", percents)
	}
}

func TestDownloadFileChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bad"))
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "model.bin")
	err := DownloadFile(context.Background(), server.URL, dest, shaHex([]byte("good")), nil)
	if err == nil {
		t.Fatal("校验失败应返回错误")
	}
	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("校验失败后不应留下目标文件")
	}
}

func TestDownloadFileHTTPError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	err := DownloadFile(context.Background(), server.URL, filepath.Join(t.TempDir(), "x"), "", nil)
	if err == nil {
		t.Fatal("HTTP 404 应返回错误")
	}
}

func TestExtractZip(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"det.onnx":     []byte("DET"),
		"sub/rec.onnx": []byte("REC"),
	})
	zipPath := filepath.Join(t.TempDir(), "m.zip")
	if err := os.WriteFile(zipPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := ExtractZip(zipPath, dir); err != nil {
		t.Fatalf("ExtractZip 失败: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "det.onnx")); err != nil || string(b) != "DET" {
		t.Errorf("det.onnx 解压错误: %v %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "sub", "rec.onnx")); err != nil || string(b) != "REC" {
		t.Errorf("rec.onnx 解压错误: %v %q", err, b)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	data := makeZip(t, map[string][]byte{"../evil.txt": []byte("x")})
	zipPath := filepath.Join(t.TempDir(), "m.zip")
	if err := os.WriteFile(zipPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	dest := filepath.Join(parent, "out")
	if err := ExtractZip(zipPath, dest); err == nil {
		t.Fatal("路径穿越条目应报错")
	}
	if _, err := os.Stat(filepath.Join(parent, "evil.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Error("不允许写出目标目录")
	}
}

func TestInstallOCR(t *testing.T) {
	zipData := makeZip(t, map[string][]byte{
		"det.onnx":         []byte("DET"),
		"rec.onnx":         []byte("REC"),
		"ppocrv6_dict.txt": []byte("DICT"),
	})
	dllData := []byte("DLL")

	zipHits, dllHits := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ppocrv6.zip":
			zipHits++
			w.Write(zipData)
		case "/onnxruntime.dll":
			dllHits++
			w.Write(dllData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	var lastPercent float64
	err := installOCR(context.Background(), dir, server.URL+"/ppocrv6.zip", server.URL+"/onnxruntime.dll",
		shaHex(zipData), shaHex(dllData), func(percent float64, _ string) { lastPercent = percent })
	if err != nil {
		t.Fatalf("installOCR 失败: %v", err)
	}
	if err := CheckOCR(dir); err != nil {
		t.Fatalf("安装后模型不完整: %v", err)
	}
	if lastPercent != 100 {
		t.Errorf("最终进度 = %v, want 100", lastPercent)
	}
	if _, err := os.Stat(filepath.Join(dir, OCRZipFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Error("安装成功后应删除压缩包")
	}

	// 第二次安装应直接复用已有文件。
	if err := installOCR(context.Background(), dir, server.URL+"/ppocrv6.zip", server.URL+"/onnxruntime.dll",
		shaHex(zipData), shaHex(dllData), nil); err != nil {
		t.Fatalf("重复安装失败: %v", err)
	}
	if zipHits != 1 || dllHits != 1 {
		t.Errorf("重复安装不应再次下载, zip=%d dll=%d", zipHits, dllHits)
	}
}

func TestCheckOCRMissing(t *testing.T) {
	err := CheckOCR(t.TempDir())
	if err == nil {
		t.Fatal("空目录应返回错误")
	}
	if !errors.Is(err, vision.ErrModelMissing) {
		t.Errorf("错误应可被 ErrModelMissing 判定: %v", err)
	}
}
