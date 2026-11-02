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
	keys     atomic.Int32
	clicks   atomic.Int32
	presses  atomic.Int32
	releases atomic.Int32
	// held 记录当前是否处于按住状态，用来验证 Press/Release 成对。
	held atomic.Bool
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
	time.Sleep(d)
	return nil
}
func (r *realisticInput) Press(geom.Point) error {
	r.presses.Add(1)
	r.held.Store(true)
	time.Sleep(10 * time.Millisecond)
	return nil
}
func (r *realisticInput) Release() error {
	r.releases.Add(1)
	r.held.Store(false)
	return nil
}
func (r *realisticInput) ClickCurrent() error { time.Sleep(30 * time.Millisecond); return nil }
func (r *realisticInput) KeyTap(string) error {
	r.keys.Add(1)
	time.Sleep(15 * time.Millisecond)
	return nil
}
func (r *realisticInput) ReleaseAll() error { r.held.Store(false); return nil }

// heavySession 每帧都要求一次 600ms 重击，模拟连招里的重击步骤。
type heavySession struct {
	steps atomic.Int32
}

func (s *heavySession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.steps.Add(1)
	return assist.Result{State: "连招-重击", Wait: 100 * time.Millisecond},
		[]assist.Action{{Kind: assist.ActionClickHold, Point: geom.Point{X: 960, Y: 540}, Duration: 600 * time.Millisecond}}, nil
}
func (s *heavySession) Close() error { return nil }

// TestLongHoldDoesNotStallLoop 验证按住 600ms 不会把引擎拖住。
// 之前 ClickHold 是同步阻塞的：动作执行期间不截图、不决策，
// 实测 2.8 秒只推进 5 帧（10 FPS 应有 28 帧），等于失明半秒。
// 现在超过 maxBlockingHold 的按住拆成 Press + 定时 Release，循环应当维持满帧。
func TestLongHoldDoesNotStallLoop(t *testing.T) {
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
	t.Logf("2.8 秒内推进 %d 帧（目标 10 FPS 约 28 帧），Press %d 次 / Release %d 次",
		steps, in.presses.Load(), in.releases.Load())
	if steps < 20 {
		t.Fatalf("按住 600ms 仍然阻塞了循环：2.8 秒只推进 %d 帧", steps)
	}
	if in.presses.Load() == 0 {
		t.Fatal("没有走 Press/Release 路径，长按住还在同步阻塞")
	}
	if in.releases.Load() == 0 {
		t.Fatal("按住之后没有抬起")
	}
}

// TestShortHoldStaysBlocking 验证短按住仍然走同步 ClickHold：
// 20ms 级的点击拆成 Press/Release 反而会多一次调度开销。
func TestShortHoldStaysBlocking(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &realisticInput{}
	sess := &shortHoldSession{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: 10}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(700 * time.Millisecond)
	cancel()
	<-done
	if in.presses.Load() != 0 {
		t.Fatalf("20ms 的短按住不该走 Press/Release，实际 Press %d 次", in.presses.Load())
	}
}

type shortHoldSession struct{ steps atomic.Int32 }

func (s *shortHoldSession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.steps.Add(1)
	return assist.Result{State: "短按", Wait: 100 * time.Millisecond},
		[]assist.Action{{Kind: assist.ActionClickHold, Point: geom.Point{X: 10, Y: 10}, Duration: 20 * time.Millisecond}}, nil
}
func (s *shortHoldSession) Close() error { return nil }

// TestReleaseOnPause 验证暂停时会把按住的键抬起来，不会一直按住。
func TestReleaseOnPause(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &realisticInput{}
	sess := &heavySession{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: 10}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(300 * time.Millisecond)
	eng.Pause()
	if in.held.Load() {
		t.Fatal("暂停后仍然按住左键")
	}
	cancel()
	<-done
}
