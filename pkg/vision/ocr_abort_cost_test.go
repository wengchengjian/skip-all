//go:build windows

package vision

import (
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestOCRAbortRegionCost 量中断提示区域（800x360）在不同 side 下的耗时与识别结果。
// 这个区域比技能冷却大得多，自适应规则（最长边 4 倍上限）在这里
// 仍然是 960，等于没省。需要看它到底需不需要 960。
func TestOCRAbortRegionCost(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	// 带中文界面的截图，中断区域里会有"挑战失败"这类字。
	images := []string{
		"../../.tmp/ok-ww/tests/images/tacet_elimination_final.jpg",
		"../../.tmp/ok-ww/tests/images/in_combat.png",
	}
	// 鸣潮中断提示区域，与 assets/games/wuwa/screens/combat.yml 一致。
	abortRect := geom.Rect{X1: 560, Y1: 360, X2: 1360, Y2: 720}
	for _, side := range []int{960, 640, 480, 320} {
		opts := DefaultPaddleOptions()
		opts.DetMaxSideLen = side
		opts.RuntimeLib = filepath.Join(dir, "onnxruntime.dll")
		engine, err := NewPaddleOCRWithOptions(dir, opts)
		if err != nil {
			t.Skipf("加载 OCR 失败: %v", err)
		}
		for _, path := range images {
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			img, err := decodeImage(f)
			f.Close()
			if err != nil {
				t.Logf("跳过 %s: %v", filepath.Base(path), err)
				continue
			}
			crop := cropLogical(img, abortRect)
			start := time.Now()
			blocks, _ := engine.Recognize(crop, OCROptions{})
			cost := time.Since(start)
			var texts []string
			for _, b := range blocks {
				texts = append(texts, strings.TrimSpace(b.Text))
			}
			t.Logf("side=%3d %-28s %v -> %v", side, filepath.Base(path), cost, texts)
		}
		_ = engine.Close()
	}
}

func decodeImage(f *os.File) (image.Image, error) {
	img, err := png.Decode(f)
	if err == nil {
		return img, nil
	}
	f.Seek(0, 0)
	return jpeg.Decode(f)
}
