//go:build windows

package vision

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/draw"
)

// TestOCRSplitCost 量 det 与 rec 各占多少，判断还有没有算法层面的空间。
//
// 结论（i7-13700K）：det 占 60~80%。技能区 68x57（detSide=272）det 8.1ms / rec 4.6ms；
// 字幕区 1240x130（detSide=960）det 15.6ms / rec 4.2ms。
// 注意必须先按 Recognize 的做法设置自适应 det 边长，否则小区域会被放大到 960 白跑十几倍。
func TestOCRSplitCost(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑 OCR 拆分测量")
	}
	dir, _ := filepath.Abs("../../assets/models/ppocrv6")
	eng, err := NewPaddleOCRWithOptions(dir, PaddleOptions{})
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer eng.Close()

	f, err := os.Open("../../.tmp/ok-ww/tests/images/all_cd_1080p.png")
	if err != nil {
		t.Skip("缺截图")
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	full := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.CatmullRom.Scale(full, full.Bounds(), img, img.Bounds(), draw.Over, nil)

	type region struct {
		name string
		r    image.Rectangle
	}
	regions := []region{
		{"技能区 68x57", image.Rect(1653, 964, 1721, 1021)},
		{"字幕区 1240x130", image.Rect(340, 880, 1580, 1010)},
	}

	inner := eng.(*paddleOCR)
	for _, rg := range regions {
		crop := image.NewRGBA(image.Rect(0, 0, rg.r.Dx(), rg.r.Dy()))
		draw.Draw(crop, crop.Bounds(), full, rg.r.Min, draw.Src)

		// 先按 Recognize 的做法把 det 边长收到自适应值，否则 68x57 会被放大到 960。
		side := adaptiveDetSideLen(rg.r.Dx(), rg.r.Dy(), inner.detMaxSideLen)
		if sized, ok := inner.engine.(detSideLenSetter); ok {
			sized.SetDetMaxSideLen(side)
		}

		// det 单独
		bestDet := time.Hour
		var boxes int
		for i := 0; i < 8; i++ {
			s := time.Now()
			bs, err := inner.engine.RunDetect(crop)
			if d := time.Since(s); d < bestDet {
				bestDet = d
			}
			if err != nil {
				t.Fatal(err)
			}
			boxes = len(bs)
		}
		// 完整 det+rec
		bestAll := time.Hour
		for i := 0; i < 8; i++ {
			s := time.Now()
			_, err := inner.engine.RunOCR(crop)
			if d := time.Since(s); d < bestAll {
				bestAll = d
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("%-18s detSide=%-4d det %-10v 全部 %-10v rec≈%-10v 文本框 %d",
			rg.name, side, bestDet.Round(time.Microsecond), bestAll.Round(time.Microsecond),
			(bestAll - bestDet).Round(time.Microsecond), boxes)
	}
}
