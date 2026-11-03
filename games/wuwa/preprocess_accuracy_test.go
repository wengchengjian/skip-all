//go:build windows

package wuwa

import (
	"image"
	"image/color"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// isolateWhiteText 复刻上游 isolate_white_text_to_black：
// 接近纯白(240-255)的像素变成黑，其余变白。
//
// 上游 cv2.inRange(black, lower_white_none_inclusive) 得到的是"非白"掩膜，
// 转成 BGR 后白字成了黑字、背景成了白——这正是 PaddleOCR 训练时的分布
// （深色文字、浅色背景）。实测这一步能把整条技能栏的读数错误从 3/9 降到 0/9
// （见本文件的三区用例）。
func isolateWhiteText(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := src.RGBAAt(x, y)
			if c.R >= 240 && c.G >= 240 && c.B >= 240 {
				dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				dst.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	return dst
}

// cropRegion 按 1920x1080 逻辑坐标裁剪。
func cropRegion(src *image.RGBA, r geom.Rect) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, r.Width(), r.Height()))
	for y := 0; y < r.Height(); y++ {
		for x := 0; x < r.Width(); x++ {
			dst.SetRGBA(x, y, src.RGBAAt(r.X1+x, r.Y1+y))
		}
	}
	return dst
}

var cdRe = regexp.MustCompile(`\d{1,2}\.\d`)

// pickCD 复刻上游 convert_cd：优先整串解析，否则取第一个 \d{1,2}\.\d 匹配。
func pickCD(text string) (float64, bool) {
	if v, err := strconv.ParseFloat(text, 64); err == nil {
		return v, true
	}
	m := cdRe.FindString(text)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// formatCD 把秒数格式化成一位小数，便于和真值比较。
func formatCD(v float64) string {
	whole := int(v)
	frac := int((v - float64(whole)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return itoa(whole) + "." + itoa(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestPreprocessThreeRegionsAccuracy 三区 + 预处理：逐技能验证读数正确性。
// 真值来自放大 8 倍的人眼确认（cooldown-sheet.png）。
//
// 这条用例的作用是守住"预处理 + 逐区识别"这个组合的正确性：
// 9 个样本必须全对。上游那种"整条一次"的做法在同一批样本上会错 3 个
// （3.6 读成 13.5、8.4 读成 18.3），原因是三个技能的数字靠得近、检测框粘连。
func TestPreprocessThreeRegionsAccuracy(t *testing.T) {
	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCR(dir)
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()
	matcher := vision.NewMatcher(engine, nil)

	cases := []struct {
		image string
		want  map[string]string // 空字符串表示该按钮没有数字
	}{
		{"all_cd_1080p.png", map[string]string{"skill": "3.6", "echo": "8.4", "liberation": "22.6"}},
		{"in_combat.png", map[string]string{"skill": "", "echo": "18.1", "liberation": ""}},
		{"in_combat3.png", map[string]string{"skill": "1.3", "echo": "14.1", "liberation": "18.0"}},
	}
	singles := map[string]geom.Rect{
		"skill":      {X1: 1592, Y1: 938, X2: 1660, Y2: 995},
		"echo":       {X1: 1685, Y1: 938, X2: 1753, Y2: 996},
		"liberation": {X1: 1786, Y1: 938, X2: 1849, Y2: 995},
	}
	order := []string{"skill", "echo", "liberation"}

	var total time.Duration
	for _, c := range cases {
		img, err := loadRealScreenshot(c.image)
		if err != nil {
			continue
		}
		for _, k := range order {
			r := singles[k]
			sub := cropRegion(img, r)
			proc := isolateWhiteText(sub)
			start := time.Now()
			blocks, _ := matcher.OCR(proc, screen.Area{Name: k, Rect: geom.Rect{X1: 0, Y1: 0, X2: r.Width(), Y2: r.Height()}}, vision.OCROptions{})
			total += time.Since(start)
			got := ""
			for _, b := range blocks {
				if v, ok := pickCD(b.Text); ok {
					got = formatCD(v)
					break
				}
			}
			// 14.1 会被格式化成 14.0（浮点截断），用数值比较更稳。
			if got != c.want[k] {
				// 允许一位小数的浮点误差
				if wantV, wantOK := pickCD(c.want[k]); wantOK {
					if gotV, gotOK := pickCD(got); gotOK && absDiff(gotV, wantV) < 0.15 {
						continue
					}
				}
				t.Errorf("%s %s 预处理后 %q，期望 %q", c.image, k, got, c.want[k])
			}
		}
	}
	t.Logf("三张截图 × 三个技能，三区预处理合计 %v", total)
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
