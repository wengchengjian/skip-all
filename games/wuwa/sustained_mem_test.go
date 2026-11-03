//go:build windows

package wuwa

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

type procMemCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func procMem() (rssMB, peakMB, commitMB float64) {
	var c procMemCounters
	c.Cb = uint32(unsafe.Sizeof(c))
	h, _ := windows.GetCurrentProcess()
	getProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.Cb))
	return float64(c.WorkingSetSize) / 1048576,
		float64(c.PeakWorkingSetSize) / 1048576,
		float64(c.PagefileUsage) / 1048576
}

// liveRuntime 在 simRuntime 基础上把 OCR 换成真实引擎。
type liveRuntime struct {
	simRuntime
	matcher vision.Matcher
}

func (r liveRuntime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	return r.matcher.OCR(frame.Image, area, opts)
}

// frameBuf 模拟生产里的帧缓冲池：Retain/Release 配对，泄漏就会在这里暴露。
type frameBuf struct {
	img  *image.RGBA
	refs int
}

func (b *frameBuf) Retain() { b.refs++ }
func (b *frameBuf) Release() {
	b.refs--
	if b.refs < 0 {
		panic("帧引用计数为负：Release 多于 Retain")
	}
}

// allRealScreenshots 列出上游测试图目录下所有截图。
func allRealScreenshots(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("../../.tmp/ok-ww/tests/images")
	if err != nil {
		t.Skipf("没有测试图目录: %v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if strings.HasSuffix(n, ".png") || strings.HasSuffix(n, ".jpg") || strings.HasSuffix(n, ".jpeg") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// TestSustainedRunMemory 长时间随机负载下的内存与引用计数检查。
//
// 与短跑版本的区别：
//   - 全部 43 张真实截图参与，每帧随机取
//   - 按时间跑（默认 10 分钟，可用 SUSTAIN_SECONDS 覆盖）
//   - 帧缓冲用 Retain/Release 记账，跑完必须归零
//
// 环境变量：
//
//	SUSTAIN_SECONDS=600   运行时长
//	SUSTAIN_SEED=1        随机种子
func TestSustainedRunMemory(t *testing.T) {
	// 默认 10 秒适合常规回归；长跑用 SUSTAIN_SECONDS 覆盖。
	secs := 10
	if v := os.Getenv("SUSTAIN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	seed := int64(1)
	if v := os.Getenv("SUSTAIN_SEED"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			seed = n
		}
	}

	dir, err := filepath.Abs("../../assets/models/ppocrv6")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := vision.NewPaddleOCRWithOptions(dir, vision.PaddleOptions{})
	if err != nil {
		t.Skipf("加载 OCR 失败: %v", err)
	}
	defer engine.Close()

	matcher := vision.NewMatcher(engine, nil)
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	inner := liveRuntime{simRuntime: simRuntime{game: g}, matcher: matcher}
	async := assist.NewAsyncRuntime(inner, 2)
	closed := false
	defer func() {
		if !closed {
			async.Close()
		}
	}()

	feature := CombatFeature()
	session, err := feature.NewSession(async, feature.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// 全部截图入池
	shots := allRealScreenshots(t)
	pool := make([]*frameBuf, 0, len(shots))
	for _, s := range shots {
		img, err := loadRealScreenshot(s)
		if err != nil {
			continue
		}
		pool = append(pool, &frameBuf{img: img})
	}
	if len(pool) == 0 {
		t.Skip("没有可用截图")
	}
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	rss0, _, commit0 := procMem()
	t.Logf("准备就绪：%d 张截图，线程 %d，时长 %ds，种子 %d",
		len(pool), vision.DefaultNumThreads(), secs, seed)
	t.Logf("基线 RSS %.0fMB 提交 %.0fMB（含 %d 张预载截图）", rss0, commit0, len(pool))

	rng := rand.New(rand.NewSource(seed))
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	sampleAt := time.Now().Add(30 * time.Second)
	baseAt := time.Now()

	frames, peakRSS := 0, rss0
	var lastRSS float64 = rss0
	for time.Now().Before(deadline) {
		buf := pool[rng.Intn(len(pool))]
		// 模拟生产：本拍持有一份，异步 worker 各持一份
		buf.Retain()
		advance := time.Duration(80+rng.Intn(41)) * time.Millisecond
		baseAt = baseAt.Add(advance)
		frame := assist.Frame{
			Image:   buf.img,
			At:      baseAt,
			Retain:  buf.Retain,
			Release: buf.Release,
		}
		if _, _, err := session.Step(nil, frame); err != nil {
			t.Fatalf("第 %d 帧出错: %v", frames, err)
		}
		buf.Release()
		frames++

		if time.Now().After(sampleAt) {
			runtime.GC()
			time.Sleep(30 * time.Millisecond)
			rss, peak, commit := procMem()
			if rss > peakRSS {
				peakRSS = rss
			}
			t.Logf("第 %-7d 帧  RSS %.0fMB  提交 %.0fMB  峰值 %.0fMB  较上采样 %+.0fMB",
				frames, rss, commit, peak, rss-lastRSS)
			lastRSS = rss
			sampleAt = time.Now().Add(30 * time.Second)
		}
	}

	// 关闭异步池再做引用计数检查：在途请求持有引用是正常的，
	// 但 Close 必须把所有在途与队列中的引用都释放掉，否则就是泄漏。
	inFlight := 0
	for _, b := range pool {
		if b.refs != 0 {
			inFlight++
		}
	}
	async.Close()
	closed = true
	time.Sleep(100 * time.Millisecond)

	leaked := 0
	for i, b := range pool {
		if b.refs != 0 {
			leaked++
			if leaked <= 5 {
				t.Errorf("第 %d 张截图 Close 后引用计数仍为 %d", i, b.refs)
			}
		}
	}
	if leaked > 0 {
		t.Errorf("Close 后仍有 %d/%d 张截图引用未释放", leaked, len(pool))
	}
	t.Logf("关闭前在途引用 %d 张；Close 后未释放 %d 张", inFlight, leaked)

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	rss1, peak1, commit1 := procMem()
	st := async.Stats()
	t.Logf("结束：%d 帧，RSS %.0fMB（较基线 %+.0fMB）提交 %.0fMB 峰值 %.0fMB",
		frames, rss1, rss1-rss0, commit1, peak1)
	t.Logf("异步 OCR：提交 %d 完成 %d 合并 %d 最旧 %v",
		st.Submitted, st.Completed, st.Dropped, st.WorstAge)
	t.Logf("帧引用计数：全部归零=%v", leaked == 0)
	if rss1-rss0 > 80 {
		t.Errorf("内存较基线增长 %.0fMB，疑似泄漏", rss1-rss0)
	}
}
