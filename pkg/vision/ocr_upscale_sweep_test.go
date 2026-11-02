//go:build windows

package vision

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// TestOCRCooldownUpscaleSweep 找出冷却区域的最小安全放大倍数。
// 冷却区是 68x57，det 会把它缩放到 DetMaxSideLen；缩太小数字就认错
// （实测 side=160 时 echo 被认成 18.4，真实值 8.4）。
func TestOCRCooldownUpscaleSweep(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("../../.tmp/ok-ww/tests/images/all_cd_1080p.png")
	if err != nil {
		t.Skipf("缺少真实截图: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	// 真实值：skill 3.6、echo 8.4、liberation 22.6（side=960 时读到）。
	rects := []struct {
		name string
		r    geom.Rect
		want string
	}{
		{"skill", geom.Rect{X1: 1592, Y1: 938, X2: 1660, Y2: 995}, "3.6"},
		{"echo", geom.Rect{X1: 1685, Y1: 938, X2: 1753, Y2: 996}, "8.4"},
		{"liberation", geom.Rect{X1: 1786, Y1: 938, X2: 1849, Y2: 995}, "22.6"},
	}
	// 68 宽、57 高，测试实际 side 从 96 到 480。
	for _, side := range []int{96, 128, 192, 272, 320, 480} {
		opts := DefaultPaddleOptions()
		opts.DetMaxSideLen = side
		opts.RuntimeLib = filepath.Join(dir, "onnxruntime.dll")
		engine, err := NewPaddleOCRWithOptions(dir, opts)
		if err != nil {
			t.Skipf("加载 OCR 失败: %v", err)
		}
		var total time.Duration
		okAll := true
		for _, c := range rects {
			crop := cropLogical(img, c.r)
			start := time.Now()
			blocks, _ := engine.Recognize(crop, OCROptions{})
			total += time.Since(start)
			joined := ""
			for _, b := range blocks {
				joined += b.Text
			}
			match := joined == c.want
			if !match {
				okAll = false
			}
			t.Logf("side=%3d %-11s %-8q want %-6q match=%v", side, c.name, joined, c.want, match)
		}
		_ = engine.Close()
		t.Logf("side=%3d 合计 %v  全部正确=%v", side, total, okAll)
	}
}

var _ = fmt.Sprintf
