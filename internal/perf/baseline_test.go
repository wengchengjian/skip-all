//go:build windows

package perf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/games"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
	"golang.org/x/image/draw"
)

// stat 是一次度量的统计结果。
type stat struct {
	name      string
	n         int
	min       time.Duration
	mean      time.Duration
	p50       time.Duration
	p95       time.Duration
	bytesPer  int64
	allocsPer int64
}

func (s stat) emit() {
	fmt.Printf("PERF|name=%s|n=%d|min_us=%d|mean_us=%d|p50_us=%d|p95_us=%d|alloc_bytes=%d|alloc_ops=%d\n",
		s.name, s.n, s.min.Microseconds(), s.mean.Microseconds(), s.p50.Microseconds(), s.p95.Microseconds(),
		s.bytesPer, s.allocsPer)
}

// measure 跑 warmup 次热身，再跑 n 次取值；同时用 MemStats 差分算每次的分配量。
func measure(name string, warmup, n int, fn func()) stat {
	for i := 0; i < warmup; i++ {
		fn()
	}
	ds := make([]time.Duration, 0, n)
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		ds = append(ds, time.Since(start))
	}
	runtime.ReadMemStats(&m2)
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	return stat{
		name:      name,
		n:         n,
		min:       ds[0],
		mean:      total / time.Duration(n),
		p50:       ds[n/2],
		p95:       ds[n*95/100],
		bytesPer:  int64(m2.TotalAlloc-m1.TotalAlloc) / int64(n),
		allocsPer: int64(m2.Mallocs-m1.Mallocs) / int64(n),
	}
}

// loadShot 读一张真实截图并标准化到 1920x1080，和截图器的行为一致。
func loadShot(t *testing.T, name string) *image.RGBA {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", ".tmp", "ok-ww", "tests", "images", name))
	if err != nil {
		t.Skipf("缺少真实截图 %s: %v", name, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("解码 %s: %v", name, err)
	}
	dst := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

func openOCR(t *testing.T) vision.OCR {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "assets", "models", "ppocrv6"))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCRWithOptions(dir, vision.PaddleOptions{})
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	return engine
}

func mkArea(name string, x1, y1, x2, y2 int) screen.Area {
	return screen.Area{Name: name, Rect: geom.Rect{X1: x1, Y1: y1, X2: x2, Y2: y2}}
}

// crop 抠出区域，用于绕过 Matcher 缓存量纯 OCR 代价。
func crop(img *image.RGBA, r geom.Rect) *image.RGBA {
	rect := image.Rect(r.X1, r.Y1, r.X2, r.Y2).Intersect(img.Bounds())
	out := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(out, out.Bounds(), img, rect.Min, draw.Src)
	return out
}

var (
	colorWhite = color.RGBA{255, 255, 255, 255}
	colorDark  = color.RGBA{20, 20, 20, 255}
)

type memStore struct {
	raw  image.Image
	mask image.Image
}

func (m *memStore) Load(vision.TemplateRef) (image.Image, image.Image, error) {
	return m.raw, m.mask, nil
}

func makeTemplate(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%7 == 0 {
				img.SetRGBA(x, y, colorWhite)
			} else {
				img.SetRGBA(x, y, colorDark)
			}
		}
	}
	return img
}

// perfRuntime 把 assist.Runtime 接到真实 vision.Matcher 上。
type perfRuntime struct {
	screens *screen.Set
	matcher vision.Matcher
	logger  *slog.Logger
}

func (r *perfRuntime) Screen(name string) (*screen.Screen, error) {
	sc, ok := r.screens.Get(name)
	if !ok {
		return nil, fmt.Errorf("没有画面 %q", name)
	}
	return sc, nil
}

func (r *perfRuntime) OCR(frame assist.Frame, a screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	return r.matcher.OCR(frame.Image, a, opts)
}

func (r *perfRuntime) MatchTemplate(frame assist.Frame, a screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error) {
	return r.matcher.Template(frame.Image, a, ref, threshold)
}

func (r *perfRuntime) ColorStats(frame assist.Frame, a screen.Area) (vision.ColorStats, error) {
	return r.matcher.Color(frame.Image, a)
}

func (r *perfRuntime) Log() *slog.Logger { return r.logger }

// TestPerfBaseline 量各阶段的单次代价，输出 PERF| 行供脚本汇总。
func TestPerfBaseline(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑整体性能基线")
	}
	img := loadShot(t, "all_cd_1080p.png")
	engine := openOCR(t)
	defer engine.Close()
	matcher := vision.NewMatcher(engine, nil)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	var stats []stat

	// 三个技能冷区（战斗热路径）。
	skillRects := []geom.Rect{
		{X1: 1653, Y1: 964, X2: 1721, Y2: 1021},
		{X1: 1743, Y1: 964, X2: 1811, Y2: 1021},
		{X1: 1833, Y1: 964, X2: 1901, Y2: 1021},
	}
	skillCrops := make([]*image.RGBA, len(skillRects))
	for i, r := range skillRects {
		skillCrops[i] = crop(img, r)
	}

	// OCR 引擎本身（不含抠图/缓存）。
	stats = append(stats, measure("ocr.engine.skill_x3", 3, 20, func() {
		for _, c := range skillCrops {
			_, _ = engine.Recognize(c, vision.OCROptions{})
		}
	}))

	// OCR 大区域（剧情字幕）。
	storyRect := geom.Rect{X1: 340, Y1: 880, X2: 1580, Y2: 1010}
	storyCrop := crop(img, storyRect)
	stats = append(stats, measure("ocr.engine.story", 3, 15, func() {
		_, _ = engine.Recognize(storyCrop, vision.OCROptions{})
	}))

	// Matcher 完整路径（含像素校验和 + 抠图 + OCR），用交替帧强制未命中。
	img2 := image.NewRGBA(img.Bounds())
	copy(img2.Pix, img.Pix)
	img2.Pix[0] ^= 0xFF
	flip := false
	stats = append(stats, measure("ocr.matcher.miss_x3", 2, 12, func() {
		f := img
		if flip {
			f = img2
		}
		flip = !flip
		for i, r := range skillRects {
			_ = i
			_, _ = matcher.OCR(f, screen.Area{Name: "s", Rect: r}, vision.OCROptions{})
		}
	}))

	// 缓存命中：同帧同区域第二次调用。
	_, _ = matcher.OCR(img, screen.Area{Name: "s", Rect: storyRect}, vision.OCROptions{})
	stats = append(stats, measure("ocr.matcher.hit", 5, 200, func() {
		_, _ = matcher.OCR(img, screen.Area{Name: "s", Rect: storyRect}, vision.OCROptions{})
	}))

	// 颜色统计。
	stats = append(stats, measure("vision.colorstats", 5, 100, func() {
		_, _ = matcher.Color(img, mkArea("对话框", 400, 700, 1500, 1000))
	}))

	// 全屏颜色矩形（战斗血条检测）。
	full := geom.Rect{X1: 0, Y1: 0, X2: 1920, Y2: 1080}
	red := [2][3]uint8{{174, 55, 55}, {225, 85, 76}}
	stats = append(stats, measure("vision.colorrect_full", 3, 100, func() {
		_ = vision.FindColorRectangles(img, full, red, 50, 5, 14, 0.95)
	}))

	// 白色文字行投影。
	stats = append(stats, measure("vision.textbands", 5, 300, func() {
		_ = vision.FindTextBands(img, storyRect, vision.TextBandOptions{})
	}))

	// 模板匹配。
	tpl := makeTemplate(48, 48)
	matcherT := vision.NewMatcher(nil, &memStore{raw: tpl})
	stats = append(stats, measure("vision.template_match", 3, 100, func() {
		_, _ = matcherT.Template(img, mkArea("确认", 860, 600, 1060, 700),
			vision.TemplateRef{SubDir: "x", ID: "y"}, 0.8)
	}))

	// --- 功能会话：整帧 Step ---
	wuwaGame, ok := games.Get("wuwa")
	if !ok {
		t.Fatal("没有 wuwa 游戏")
	}
	screens, err := screen.LoadFS(wuwaGame.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面配置: %v", err)
	}
	rt := &perfRuntime{screens: screens, matcher: matcher, logger: logger}

	for _, f := range wuwaGame.Features() {
		t.Logf("wuwa feature: id=%s name=%s", f.ID(), f.Name())
	}

	base := time.Now()
	for _, f := range wuwaGame.Features() {
		sess, err := f.NewSession(rt, f.DefaultSettings())
		if err != nil {
			t.Logf("跳过 feature %s: %v", f.ID(), err)
			continue
		}
		name := "feature." + f.ID() + ".step"
		var i int
		stats = append(stats, measure(name, 5, 60, func() {
			frame := assist.Frame{Image: img, At: base.Add(time.Duration(i) * 150 * time.Millisecond)}
			i++
			_, _, _ = sess.Step(nil, frame)
		}))
		_ = sess.Close()
	}

	for _, s := range stats {
		s.emit()
	}
}
