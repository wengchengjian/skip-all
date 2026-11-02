package assist

import (
	"image"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// recordingRuntime 记录每次 OCR 收到的时间，用来量"决策拿到的信息有多旧"。
type recordingRuntime struct {
	cost time.Duration
	mu   sync.Mutex
	seen []time.Time
}

func (r *recordingRuntime) Screen(string) (*screen.Screen, error) { return nil, nil }

func (r *recordingRuntime) OCR(frame Frame, area screen.Area, _ vision.OCROptions) ([]vision.TextBlock, error) {
	r.mu.Lock()
	r.seen = append(r.seen, frame.At)
	r.mu.Unlock()
	time.Sleep(r.cost)
	return []vision.TextBlock{{Text: "ok", Confidence: 1}}, nil
}

func (r *recordingRuntime) MatchTemplate(Frame, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
	return nil, nil
}
func (r *recordingRuntime) ColorStats(Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}
func (r *recordingRuntime) Log() *slog.Logger { return slog.Default() }

func (r *recordingRuntime) times() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Time(nil), r.seen...)
}

// TestAsyncOCRStaleness 量异步识别带来的信息陈旧度。
// 决策拿到的是上一次 worker 完成时的结果，它基于的是更早的那一帧。
func TestAsyncOCRStaleness(t *testing.T) {
	const tickInterval = 100 * time.Millisecond
	const ocrCost = 130 * time.Millisecond

	inner := &recordingRuntime{cost: ocrCost}
	rt := NewAsyncRuntime(inner, 1)
	defer rt.Close()

	area := screen.Area{Name: "区域", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	base := time.Now()
	var worst time.Duration
	var sum time.Duration
	var n int

	for i := 0; i < 40; i++ {
		now := base.Add(time.Duration(i) * tickInterval)
		frame := Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: now}
		_, _ = rt.OCR(frame, area, vision.OCROptions{})

		// 量"决策这一刻"与"结果所基于的帧"之间的差。
		if i > 5 {
			times := inner.times()
			if len(times) > 0 {
				// worker 最近一次处理的那一帧的时间
				last := times[len(times)-1]
				age := now.Sub(last)
				if age < 0 {
					age = 0
				}
				if age > worst {
					worst = age
				}
				sum += age
				n++
			}
		}
		time.Sleep(tickInterval)
	}
	if n > 0 {
		t.Logf("识别耗时 %v，tick %v：决策拿到的信息平均旧 %v，最旧 %v",
			ocrCost, tickInterval, sum/time.Duration(n), worst)
	}
	st := rt.Stats()
	t.Logf("提交 %d，完成 %d，合并 %d", st.Submitted, st.Completed, st.Dropped)
}
