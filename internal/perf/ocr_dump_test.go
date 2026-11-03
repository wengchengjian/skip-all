//go:build windows

package perf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// TestOCRResultDump 把真实截图上的 OCR 结果逐条打印出来，用于改动前后对照。
//
// 换预处理的插值算法会改变喂给模型的像素，必须验证识别结果不变。
func TestOCRResultDump(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑")
	}
	out := os.Getenv("OCR_DUMP")
	if out == "" {
		t.Skip("设置 OCR_DUMP=<路径> 才写结果")
	}
	shots := loadAllShots(t)
	engine := openOCR(t)
	defer engine.Close()
	matcher := vision.NewMatcher(engine, nil)

	regions := []struct {
		name string
		rect geom.Rect
	}{
		{"skill1", geom.Rect{X1: 1653, Y1: 964, X2: 1721, Y2: 1021}},
		{"skill2", geom.Rect{X1: 1743, Y1: 964, X2: 1811, Y2: 1021}},
		{"skill3", geom.Rect{X1: 1833, Y1: 964, X2: 1901, Y2: 1021}},
		{"confirm", geom.Rect{X1: 950, Y1: 590, X2: 1500, Y2: 730}},
		{"dialog", geom.Rect{X1: 480, Y1: 780, X2: 1440, Y2: 935}},
	}

	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for i, shot := range shots {
		for _, rg := range regions {
			blocks, err := matcher.OCR(shot, screen.Area{Name: rg.name, Rect: rg.rect}, vision.OCROptions{})
			if err != nil {
				t.Fatalf("截图 %d %s: %v", i, rg.name, err)
			}
			for _, b := range blocks {
				fmt.Fprintf(f, "%02d|%s|%s|%.3f\n", i, rg.name, b.Text, b.Confidence)
			}
			if len(blocks) == 0 {
				fmt.Fprintf(f, "%02d|%s|<empty>|0\n", i, rg.name)
			}
		}
	}
	t.Logf("已写入 %s", filepath.Clean(out))
}
