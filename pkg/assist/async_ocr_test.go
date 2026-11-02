package assist

import (
	"image"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// slowRuntime 的 OCR 会阻塞指定时长，并记录被调用的帧指针。
type slowRuntime struct {
	cost  time.Duration
	calls atomic.Int32
	// ptrs 记录每次 OCR 拿到的图像指针，用来验证零拷贝。
	mu   sync.Mutex
	ptrs []*image.RGBA
}

func newSlowRuntime(cost time.Duration) *slowRuntime {
	return &slowRuntime{cost: cost}
}

func (r *slowRuntime) Screen(string) (*screen.Screen, error) { return nil, nil }

func (r *slowRuntime) OCR(frame Frame, area screen.Area, _ vision.OCROptions) ([]vision.TextBlock, error) {
	r.calls.Add(1)
	r.mu.Lock()
	r.ptrs = append(r.ptrs, frame.Image)
	r.mu.Unlock()
	time.Sleep(r.cost)
	// 返回区域名当"识别结果"，方便断言。
	return []vision.TextBlock{{Text: area.Name, Confidence: 1}}, nil
}

func (r *slowRuntime) MatchTemplate(Frame, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
	return nil, nil
}
func (r *slowRuntime) ColorStats(Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}
func (r *slowRuntime) Log() *slog.Logger { return slog.Default() }

func (r *slowRuntime) pointers() []*image.RGBA {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*image.RGBA(nil), r.ptrs...)
}

// TestAsyncOCRDoesNotBlock 验证第一次之后的 OCR 调用立刻返回，不等识别完成。
func TestAsyncOCRDoesNotBlock(t *testing.T) {
	inner := newSlowRuntime(120 * time.Millisecond)
	rt := NewAsyncRuntime(inner, 2)
	defer rt.Close()

	area := screen.Area{Name: "冷却", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	frame := Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080))}

	// 首帧：没有历史结果，同步算一次（这是设计上的例外）。
	start := time.Now()
	_, _ = rt.OCR(frame, area, vision.OCROptions{})
	first := time.Since(start)
	if first < 100*time.Millisecond {
		t.Fatalf("首帧应当同步等一次识别，实际只花了 %v", first)
	}

	// 后续帧：应当立刻返回上一次结果。
	var worst time.Duration
	for i := 0; i < 20; i++ {
		start := time.Now()
		blocks, _ := rt.OCR(frame, area, vision.OCROptions{})
		cost := time.Since(start)
		if cost > worst {
			worst = cost
		}
		if len(blocks) != 1 || blocks[0].Text != "冷却" {
			t.Fatalf("第 %d 次调用拿到 %v，应当立刻返回上一次结果", i, blocks)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("首帧 %v，后续最慢一次 %v（识别本身是 %v）", first, worst, inner.cost)
	if worst > 20*time.Millisecond {
		t.Fatalf("后续 OCR 调用最慢 %v，异步没有生效", worst)
	}
}

// TestAsyncOCRZeroCopy 验证 worker 直接读原帧缓冲，不做深拷贝。
func TestAsyncOCRZeroCopy(t *testing.T) {
	inner := newSlowRuntime(50 * time.Millisecond)
	rt := NewAsyncRuntime(inner, 1)
	defer rt.Close()

	area := screen.Area{Name: "区域", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	frame := Frame{Image: img}

	_, _ = rt.OCR(frame, area, vision.OCROptions{}) // 首帧同步
	time.Sleep(100 * time.Millisecond)
	_, _ = rt.OCR(frame, area, vision.OCROptions{}) // 走异步
	time.Sleep(150 * time.Millisecond)

	ptrs := inner.pointers()
	for i, p := range ptrs {
		if p != img {
			t.Fatalf("第 %d 次 OCR 拿到的图像指针不是原帧（发生了拷贝）", i)
		}
	}
	if len(ptrs) < 2 {
		t.Fatalf("只记录了 %d 次 OCR", len(ptrs))
	}
}

// TestAsyncOCRDropsWhenBusy 验证慢识别时请求被合并，不会堆成山。
func TestAsyncOCRDropsWhenBusy(t *testing.T) {
	inner := newSlowRuntime(200 * time.Millisecond)
	rt := NewAsyncRuntime(inner, 1)
	defer rt.Close()

	area := screen.Area{Name: "慢", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	frame := Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080))}
	_, _ = rt.OCR(frame, area, vision.OCROptions{})
	// 识别要 200ms，这里连打 40 次（每 5ms 一次）。
	for i := 0; i < 40; i++ {
		_, _ = rt.OCR(frame, area, vision.OCROptions{})
		time.Sleep(5 * time.Millisecond)
	}
	st := rt.Stats()
	t.Logf("提交 %d，完成 %d，合并 %d，在途 %d", st.Submitted, st.Completed, st.Dropped, st.Pending)
	if st.Dropped == 0 {
		t.Fatal("慢识别期间请求应当被合并，实际一次都没合并")
	}
	if st.Pending > 1 {
		t.Fatalf("同一区域在途请求 %d 个，应当最多 1 个", st.Pending)
	}
}

// TestAsyncOCRHoldsFrameAcrossTicks 验证 worker 持有帧期间，缓冲不会被下一拍覆写。
// 这是零拷贝的前提：帧必须能跨拍存活。
func TestAsyncOCRHoldsFrameAcrossTicks(t *testing.T) {
	inner := newSlowRuntime(150 * time.Millisecond)
	rt := NewAsyncRuntime(inner, 1)
	defer rt.Close()

	area := screen.Area{Name: "跨拍", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	var releases atomic.Int32
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	frame := Frame{
		Image:   img,
		Retain:  func() {},
		Release: func() { releases.Add(1) },
	}
	_, _ = rt.OCR(frame, area, vision.OCROptions{}) // 首帧同步
	time.Sleep(200 * time.Millisecond)
	_, _ = rt.OCR(frame, area, vision.OCROptions{}) // 异步，worker 会 Hold

	// 立刻模拟"下一拍来了"：主流程 Drop 自己那一份。
	frame.Drop()

	// worker 还没跑完时，缓冲不应被释放。
	time.Sleep(50 * time.Millisecond)
	if got := releases.Load(); got != 1 {
		t.Fatalf("worker 还在识别时缓冲被释放了 %d 次，零拷贝前提被破坏", got)
	}
	// worker 完成后才释放。
	time.Sleep(200 * time.Millisecond)
	if got := releases.Load(); got < 2 {
		t.Fatalf("worker 完成后缓冲只释放了 %d 次", got)
	}
	t.Logf("释放次数 %d", releases.Load())
}
