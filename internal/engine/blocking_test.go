package engine

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// realisticInput 模拟真实输入库的阻塞耗时（input_sendinput.go 里的常量）。
type realisticInput struct {
	keys   atomic.Int32
	clicks atomic.Int32
	holds  atomic.Int32
}

func (r *realisticInput) Click(geom.Point) error {
	r.clicks.Add(1)
	time.Sleep(30 * time.Millisecond)
	return nil
}
func (r *realisticInput) MiddleClick(geom.Point) error {
	time.Sleep(30 * time.Millisecond)
	return nil
}
func (r *realisticInput) ClickHold(_ geom.Point, d time.Duration) error {
	r.holds.Add(1)
	time.Sleep(d)
	return nil
}
func (r *realisticInput) ClickCurrent() error { time.Sleep(30 * time.Millisecond); return nil }
func (r *realisticInput) KeyTap(string) error {
	r.keys.Add(1)
	time.Sleep(15 * time.Millisecond)
	return nil
}
func (r *realisticInput) ReleaseAll() error { return nil }

// heavySession 每帧都要求一次 600ms 重击，模拟连招里的重击步骤。
type heavySession struct {
	steps atomic.Int32
	at    []time.Time
}

func (s *heavySession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.steps.Add(1)
	return assist.Result{State: "连招-重击", Wait: 100 * time.Millisecond},
		[]assist.Action{{Kind: assist.ActionClickHold, Point: geom.Point{X: 960, Y: 540}, Duration: 600 * time.Millisecond}}, nil
}
func (s *heavySession) Close() error { return nil }

// TestBlockingHoldStallsLoop 量"重击按住 600ms"对循环节拍的影响：
// 引擎是单线程串行的，动作执行期间不截图、不决策。
func TestBlockingHoldStallsLoop(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &realisticInput{}
	sess := &heavySession{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: 10}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(2800 * time.Millisecond)
	cancel()
	<-done

	steps := int(sess.steps.Load())
	// 每帧 = 截图(≈0) + Step(≈0) + 重击 600ms + Wait 100ms ≈ 700ms。
	// 2.8 秒最多 4 帧，与"10 FPS 应有 28 帧"相差一个数量级。
	t.Logf("2.8 秒内只推进了 %d 帧（目标 10 FPS 应有约 28 帧）", steps)
	if steps > 6 {
		t.Fatalf("重击没有阻塞循环？2.8 秒跑了 %d 帧", steps)
	}
}
