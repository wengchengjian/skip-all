package engine

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// TestTickPeriodFollowsTargetFPS 验证节拍语义：
//   - TargetFPS 决定拍照上限（周期 = 1/TargetFPS）
//   - 功能返回的 Wait 只决定"这个功能多久后再叫我"，不能把周期拉快
//   - 全部功能都在等时可以直接睡到最近到期时刻（空闲回退）
//
// 改前是"功能 Wait 覆盖 TargetFPS"，战斗 Interval=150ms 会把
// 配置的 200ms 节拍改快；现在不会了。
func TestTickPeriodFollowsTargetFPS(t *testing.T) {
	cases := []struct {
		name      string
		fps       int
		waits     []time.Duration
		wantTicks int // 1.1 秒内期望的总拍数（大致）
		tol       float64
	}{
		{"FPS=5(200ms) 功能Wait=150ms → 周期按 FPS", 5, []time.Duration{150 * time.Millisecond}, 5, 0.4},
		{"FPS=20(50ms) 功能Wait=150ms → 周期按功能", 20, []time.Duration{150 * time.Millisecond}, 7, 0.4},
		{"FPS=20(50ms) 功能Wait=0 → 周期按 FPS", 20, []time.Duration{0}, 22, 0.4},
		{"FPS=20(50ms) 功能Wait=1s → 空闲回退", 20, []time.Duration{time.Second}, 2, 0.6},
	}
	for _, c := range cases {
		sessions := make([]*waitSession, len(c.waits))
		specs := make([]FeatureSpec, len(c.waits))
		for i, w := range c.waits {
			sessions[i] = &waitSession{wait: w}
			specs[i] = FeatureSpec{Feature: &fakeFeature{session: sessions[i]}}
		}
		dev := &fakeDevice{}
		dev.active.Store(true)
		eng := New(dev, &fakeInput{}, screen.NewSet(), fakeMatcher{}, slog.Default(), specs, Config{TargetFPS: c.fps}, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
		done := make(chan struct{})
		go func() { defer close(done); _ = eng.Run(ctx) }()
		time.Sleep(1100 * time.Millisecond)
		st := eng.Status()
		cancel()
		<-done

		n := int(st.TickID)
		lo := int(float64(c.wantTicks) * (1 - c.tol))
		hi := int(float64(c.wantTicks) * (1 + c.tol))
		t.Logf("%-38s 1.1 秒 %d 拍（期望 ~%d）", c.name, n, c.wantTicks)
		if n < lo || n > hi {
			t.Errorf("%s: %d 拍不在 [%d,%d]", c.name, n, lo, hi)
		}
	}
}

// slowOne 记录自己被调用了几次，用来验证"慢功能被跳过、快功能照跑"。
type slowOne struct {
	calls atomic.Int32
	wait  time.Duration
}

func (s *slowOne) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.calls.Add(1)
	return assist.Result{State: "慢", Wait: s.wait}, nil, nil
}
func (s *slowOne) Close() error { return nil }

type fastOne struct{ calls atomic.Int32 }

func (f *fastOne) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	f.calls.Add(1)
	return assist.Result{State: "快", Wait: 0}, nil, nil
}
func (f *fastOne) Close() error { return nil }

// TestSlowFeatureDoesNotStarveFastOne 验证多功能的独立节流：
// 慢功能（Wait=500ms）不该拖慢快功能（Wait=0）的调用频率。
func TestSlowFeatureDoesNotStarveFastOne(t *testing.T) {
	slow := &slowOne{wait: 500 * time.Millisecond}
	fast := &fastOne{}
	dev := &fakeDevice{}
	dev.active.Store(true)
	eng := New(dev, &fakeInput{}, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{
			{Feature: &fakeFeature{session: slow}},
			{Feature: &fakeFeature{session: fast}},
		}, Config{TargetFPS: 20}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(1100 * time.Millisecond)
	cancel()
	<-done

	sc, fc := int(slow.calls.Load()), int(fast.calls.Load())
	t.Logf("1.1 秒：慢功能(Wait=500ms) %d 次，快功能(Wait=0) %d 次", sc, fc)
	if sc < 1 || sc > 4 {
		t.Errorf("慢功能调用 %d 次，按 500ms 应在 2~3 次", sc)
	}
	if fc < 14 {
		t.Errorf("快功能只调用 %d 次，被慢功能拖住了（按 50ms 应约 22 次）", fc)
	}
}
