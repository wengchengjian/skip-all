package assist

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// ocrRequest 是一次异步 OCR 请求。
type ocrRequest struct {
	key   ocrKey
	frame Frame
	area  screen.Area
	opts  vision.OCROptions
	// seq 是该请求出发时的帧序号，用于丢弃过期结果。
	seq uint64
}

// ocrKey 标识"同一个待识别区域"。
type ocrKey struct {
	name string
	rect [4]int
}

func keyFor(area screen.Area) ocrKey {
	return ocrKey{name: area.Name, rect: [4]int{area.Rect.X1, area.Rect.Y1, area.Rect.X2, area.Rect.Y2}}
}

// ocrResult 是异步 OCR 的结果快照。
type ocrResult struct {
	blocks []vision.TextBlock
	seq    uint64
	at     time.Time
	// err 是该次识别的错误；非 nil 时调用方应当降级。
	err error
}

// AsyncRuntime 把 Runtime 的 OCR 变成异步：提交识别请求后立即返回上一次的结果，
// 由 worker 池在后台算新结果。这样一次 130ms 的识别不会把整拍拖住。
//
// 关键约束：
//   - 帧像素用 Retain/Release 跨拍持有，worker 直接读原缓冲，不做深拷贝。
//   - 每个 key 最多只有一个在途请求，避免慢识别时任务堆成山。
//   - 结果按帧序号单调前进，旧结果不会覆盖新结果。
//   - 只有"从未识别过"的区域才同步等待一次，保证首帧不会拿到空结果。
type AsyncRuntime struct {
	inner Runtime
	pool  int

	mu       sync.Mutex
	latest   map[ocrKey]ocrResult
	inflight map[ocrKey]bool
	seq      uint64
	jobs     chan ocrRequest
	stop     chan struct{}
	wg       sync.WaitGroup

	// stats 是观测数据。
	submitted atomic.Int64
	completed atomic.Int64
	dropped   atomic.Int64
	cacheHits atomic.Int64
	// worstAge 是观测到的最大信息年龄（异步结果基于的帧与当前帧的差）。
	worstAge atomic.Int64
}

// AsyncStats 是异步 OCR 的观测数据。
type AsyncStats struct {
	// Submitted 是提交的请求数，Completed 是完成的，Dropped 是因为
	// 已有在途请求而合并掉的（说明识别跟不上调用频率）。
	Submitted int64
	Completed int64
	Dropped   int64
	CacheHits int64
	// Pending 是当前在途请求数。
	Pending int
	// WorstAge 是观测到的最大信息年龄：异步返回的结果可能基于
	// 一两拍之前的帧，调用方用 OCRFresh 可以拿到具体时间。
	WorstAge time.Duration
}

// NewAsyncRuntime 在 rt 外面包一层异步 OCR。workers 是并发识别数。
func NewAsyncRuntime(rt Runtime, workers int) *AsyncRuntime {
	if workers <= 0 {
		workers = 1
	}
	a := &AsyncRuntime{
		inner:    rt,
		pool:     workers,
		latest:   make(map[ocrKey]ocrResult),
		inflight: make(map[ocrKey]bool),
		// 队列容量取 4 倍并发：满了说明识别严重落后，丢弃比堆积好。
		jobs: make(chan ocrRequest, workers*4),
		stop: make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
	return a
}

func (a *AsyncRuntime) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.stop:
			return
		case req := <-a.jobs:
			blocks, err := a.inner.OCR(req.frame, req.area, req.opts)
			// 无论成功失败都要释放这一帧的引用。
			req.frame.Drop()

			a.mu.Lock()
			prev, ok := a.latest[req.key]
			// 只接受不比现有结果旧的结果，防止慢 worker 覆盖新结果。
			if !ok || req.seq >= prev.seq {
				a.latest[req.key] = ocrResult{blocks: blocks, seq: req.seq, at: req.frame.At, err: err}
			}
			delete(a.inflight, req.key)
			a.mu.Unlock()
			a.completed.Add(1)
		}
	}
}

// OCR 提交一次识别并立刻返回上一次的结果。
// 只有该区域从未识别成功过时才同步调用一次，避免首帧拿到空结果。
func (a *AsyncRuntime) OCR(frame Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	blocks, _, err := a.OCRFresh(frame, area, opts)
	return blocks, err
}

// OCRFresh 与 OCR 相同，但额外返回结果所基于的那一帧的时间。
// 同步路径返回零值（表示就是当前帧）；异步路径返回实际的那一帧时间。
func (a *AsyncRuntime) OCRFresh(frame Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, time.Time, error) {
	key := keyFor(area)

	a.mu.Lock()
	prev, had := a.latest[key]
	if had && prev.err == nil {
		a.cacheHits.Add(1)
	}
	busy := a.inflight[key]
	a.seq++
	seq := a.seq
	a.mu.Unlock()

	// 首帧没有历史结果：同步做一次，保证决策不会因为"还没算完"而瞎掉。
	if !had {
		blocks, err := a.inner.OCR(frame, area, opts)
		a.mu.Lock()
		a.latest[key] = ocrResult{blocks: blocks, seq: seq, at: frame.At, err: err}
		a.mu.Unlock()
		// 同步路径：结果就是这一帧的，年龄为零。
		return blocks, time.Time{}, err
	}

	// 记录信息年龄：这次决策拿到的结果基于的是 prev.at 那一帧。
	if !prev.at.IsZero() && !frame.At.IsZero() {
		if age := frame.At.Sub(prev.at); age > 0 {
			if prev := time.Duration(a.worstAge.Load()); int64(age) > int64(prev) {
				a.worstAge.Store(int64(age))
			}
		}
	}

	// 有历史结果：返回旧的，同时把新请求丢给 worker。
	if busy {
		// 上一请求还在跑，合并掉这次请求（等它完成自然会更新）。
		a.dropped.Add(1)
		return append([]vision.TextBlock(nil), prev.blocks...), prev.at, prev.err
	}

	frame.Hold()
	select {
	case a.jobs <- ocrRequest{key: key, frame: frame, area: area, opts: opts, seq: seq}:
		a.submitted.Add(1)
		a.mu.Lock()
		a.inflight[key] = true
		a.mu.Unlock()
	default:
		// 队列满：放弃这次刷新，返回旧结果。
		frame.Drop()
		a.dropped.Add(1)
	}
	return append([]vision.TextBlock(nil), prev.blocks...), prev.at, prev.err
}

// Screen 转发给底层 Runtime。
func (a *AsyncRuntime) Screen(name string) (*screen.Screen, error) { return a.inner.Screen(name) }

// MatchTemplate 转发给底层 Runtime（模板匹配很快，不需要异步）。
func (a *AsyncRuntime) MatchTemplate(frame Frame, area screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error) {
	return a.inner.MatchTemplate(frame, area, ref, threshold)
}

// ColorStats 转发给底层 Runtime（颜色统计很快，不需要异步）。
func (a *AsyncRuntime) ColorStats(frame Frame, area screen.Area) (vision.ColorStats, error) {
	return a.inner.ColorStats(frame, area)
}

// Log 转发给底层 Runtime。
func (a *AsyncRuntime) Log() *slog.Logger { return a.inner.Log() }

// Stats 返回观测数据。
func (a *AsyncRuntime) Stats() AsyncStats {
	a.mu.Lock()
	pending := len(a.inflight)
	a.mu.Unlock()
	return AsyncStats{
		Submitted: a.submitted.Load(),
		Completed: a.completed.Load(),
		Dropped:   a.dropped.Load(),
		CacheHits: a.cacheHits.Load(),
		Pending:   pending,
		WorstAge:  time.Duration(a.worstAge.Load()),
	}
}

// Close 停止 worker 池并等待在途任务结束。
func (a *AsyncRuntime) Close() {
	close(a.stop)
	a.wg.Wait()
	// 清空队列里还没跑的请求，释放它们持有的帧引用。
	for {
		select {
		case req := <-a.jobs:
			req.frame.Drop()
		default:
			return
		}
	}
}
