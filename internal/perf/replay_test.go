//go:build windows

package perf

import (
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/games"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
	"golang.org/x/image/draw"
)

// countingRuntime 包在真实 Runtime 外面，数每帧各识别接口被调了几次、各花多久。
//
// 用来回答"模板匹配到底多久跑一次"这类问题：只看单帧基准会误判，
// 因为功能可能几十帧才查一次模板。
type countingRuntime struct {
	inner assist.Runtime

	ocrCalls, tplCalls, colorCalls atomic.Int64
	ocrTime, tplTime, colorTime    atomic.Int64

	mu        sync.Mutex
	tplByRef  map[string]*refCount
	ocrByArea map[string]*refCount
}

type refCount struct {
	calls int
	ns    int64
}

func (c *countingRuntime) hit(m map[string]*refCount, key string, d time.Duration) {
	c.mu.Lock()
	rc := m[key]
	if rc == nil {
		rc = &refCount{}
		m[key] = rc
	}
	rc.calls++
	rc.ns += int64(d)
	c.mu.Unlock()
}

func (c *countingRuntime) dump() {
	c.mu.Lock()
	defer c.mu.Unlock()
	type row struct {
		key   string
		calls int
		ns    int64
	}
	var rows []row
	for k, v := range c.tplByRef {
		rows = append(rows, row{k, v.calls, v.ns})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ns > rows[j].ns })
	for _, r := range rows {
		fmt.Printf("TPLREF|ref=%s|calls=%d|total_ms=%d\n", r.key, r.calls, r.ns/1e6)
	}
}

func (c *countingRuntime) Screen(name string) (*screen.Screen, error) { return c.inner.Screen(name) }
func (c *countingRuntime) Log() *slog.Logger                          { return slog.Default() }

func (c *countingRuntime) OCR(frame assist.Frame, a screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	start := time.Now()
	blocks, err := c.inner.OCR(frame, a, opts)
	c.ocrCalls.Add(1)
	c.ocrTime.Add(int64(time.Since(start)))
	return blocks, err
}

func (c *countingRuntime) MatchTemplate(frame assist.Frame, a screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error) {
	start := time.Now()
	m, err := c.inner.MatchTemplate(frame, a, ref, threshold)
	d := time.Since(start)
	c.tplCalls.Add(1)
	c.tplTime.Add(int64(d))
	c.hit(c.tplByRef, ref.SubDir+"/"+ref.ID, d)
	return m, err
}

func (c *countingRuntime) ColorStats(frame assist.Frame, a screen.Area) (vision.ColorStats, error) {
	start := time.Now()
	s, err := c.inner.ColorStats(frame, a)
	c.colorCalls.Add(1)
	c.colorTime.Add(int64(time.Since(start)))
	return s, err
}

// loadAllShots 读出全部真实截图并标准化到 1920x1080。
func loadAllShots(t *testing.T) []*image.RGBA {
	t.Helper()
	dir := filepath.Join("..", "..", ".tmp", "ok-ww", "tests", "images")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("缺少截图目录: %v", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".png" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var shots []*image.RGBA
	for _, name := range names {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		dst := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		shots = append(shots, dst)
	}
	if len(shots) == 0 {
		t.Skip("没有可用截图")
	}
	return shots
}

// TestPerfReplay 用真实截图序列驱动功能会话，量每帧代价与各识别接口的调用频率。
//
// 单帧重复测（TestPerfBaseline）会低估模板匹配这类"偶尔才跑"的开销，
// 也会高估按帧缓存的效果——这里换成连续不同的帧，更接近真实运行。
func TestPerfReplay(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑回放测量")
	}
	shots := loadAllShots(t)
	t.Logf("截图 %d 张", len(shots))

	// 只对"功能执行"这一段做 CPU profile：截图加载（PNG 解码 + 缩放）占了
	// 绝大部分 CPU，不排除掉会把真实热点淹没。
	if path := os.Getenv("PERF_CPUPROFILE"); path != "" {
		pf, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer pf.Close()
		if err := pprof.StartCPUProfile(pf); err != nil {
			t.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	engine := openOCR(t)
	defer engine.Close()

	wuwaGame, ok := games.Get("wuwa")
	if !ok {
		t.Fatal("没有 wuwa 游戏")
	}
	screens, err := screen.LoadFS(wuwaGame.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面配置: %v", err)
	}
	// 必须接真实模板仓库：空仓库会让 MatchTemplate 立刻报错，
	// 读数变成 0ms，看不出模板匹配的真实代价。
	store := vision.NewTemplateStoreFS(wuwaGame.Assets(), "templates")
	matcher := vision.NewMatcher(engine, store)
	base := time.Now()

	for _, f := range wuwaGame.Features() {
		cr := &countingRuntime{
			inner:     &perfRuntime{screens: screens, matcher: matcher},
			tplByRef:  make(map[string]*refCount),
			ocrByArea: make(map[string]*refCount),
		}
		sess, err := f.NewSession(cr, f.DefaultSettings())
		if err != nil {
			t.Logf("跳过 %s: %v", f.ID(), err)
			continue
		}
		ds := make([]time.Duration, 0, len(shots))
		for i, shot := range shots {
			frame := assist.Frame{Image: shot, At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
			start := time.Now()
			_, _, _ = sess.Step(nil, frame)
			ds = append(ds, time.Since(start))
		}
		_ = sess.Close()

		sort.Slice(ds, func(a, b int) bool { return ds[a] < ds[b] })
		var total time.Duration
		for _, d := range ds {
			total += d
		}
		n := len(ds)
		ocrN, tplN, colorN := cr.ocrCalls.Load(), cr.tplCalls.Load(), cr.colorCalls.Load()
		fmt.Printf("REPLAY|feature=%s|frames=%d|mean_us=%d|p50_us=%d|p95_us=%d|max_us=%d|"+
			"ocr_calls=%d|ocr_ms=%d|tpl_calls=%d|tpl_ms=%d|color_calls=%d|color_ms=%d\n",
			f.ID(), n, total.Microseconds()/int64(n), ds[n/2].Microseconds(), ds[n*95/100].Microseconds(),
			ds[n-1].Microseconds(),
			ocrN, cr.ocrTime.Load()/1e6, tplN, cr.tplTime.Load()/1e6,
			colorN, cr.colorTime.Load()/1e6)
		cr.dump()
	}
}
