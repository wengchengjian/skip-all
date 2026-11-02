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

// TestTickPeriodAbsorbsWorkTime 验证固定步长节拍：即使每拍干活动作耗时不固定，
// 周期也按"本拍开始时间 + wait"推进，不会把干活时间累加到周期上。
func TestTickPeriodAbsorbsWorkTime(t *testing.T) {
	// 每帧 Step 里故意忙 40ms，wait 100ms。
	// 老的"干完活再睡 wait"会是 140ms 周期；固定步长节拍应当仍是 ~100ms。
	busy := &busySession{busy: 40 * time.Millisecond}
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &fakeFeature{session: busy}}}, Config{TargetFPS: 10}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(1900 * time.Millisecond)
	cancel()
	<-done

	steps := int(busy.steps.Load())
	// 1.9 秒 / 100ms ≈ 19 帧；允许 ±30%。
	if steps < 13 || steps > 25 {
		t.Fatalf("1.9 秒内 %d 帧，固定步长节拍应约 19 帧（干活动作不该累加进周期）", steps)
	}
	t.Logf("1.9 秒内 %d 帧，每帧 Step 忙 40ms、wait 100ms", steps)
}

// TestTickLagIsRecorded 验证落后被记录下来，供观测节拍是否被拖慢。
func TestTickLagIsRecorded(t *testing.T) {
	busy := &busySession{busy: 150 * time.Millisecond}
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &fakeFeature{session: busy}}}, Config{TargetFPS: 10}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(1200 * time.Millisecond)
	st := eng.Status()
	cancel()
	<-done
	if st.TickID == 0 {
		t.Fatal("没有记录 tick ID")
	}
	if st.TickLag <= 0 {
		t.Fatalf("每拍忙 150ms 超过 100ms 周期，应当记录到落后，实际 TickLag=%v", st.TickLag)
	}
	t.Logf("tick=%d lag=%v", st.TickID, st.TickLag)
}

// busySession 在 Step 里阻塞一段时间，模拟识别/动作耗时。
type busySession struct {
	busy  time.Duration
	steps atomic.Int32
}

func (s *busySession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.steps.Add(1)
	time.Sleep(s.busy)
	return assist.Result{State: "忙碌", Wait: 100 * time.Millisecond}, nil, nil
}
func (s *busySession) Close() error { return nil }

// TestTickSkipsWhenTooFarBehind 验证落后超过 5 个周期时跳拍而不是无限追赶。
func TestTickSkipsWhenTooFarBehind(t *testing.T) {
	clock := newTickClock(100 * time.Millisecond)
	base := time.Now()
	t1 := clock.Begin(base)
	clock.Schedule(t1.Time, 100*time.Millisecond)
	// 计划时间过了 800ms 才来（落后 8 个周期）。
	t2 := clock.Begin(base.Add(900 * time.Millisecond))
	if t2.Lag != 0 {
		t.Fatalf("落后 8 个周期应当跳拍（Lag 归零），实际 %v", t2.Lag)
	}
	clock.Schedule(t2.Time, 100*time.Millisecond)
	// 下一拍按时到达，不应该因为上一拍落后而补发。
	t3 := clock.Begin(base.Add(1000 * time.Millisecond))
	if t3.ID != 3 || t3.Lag != 0 {
		t.Fatalf("tick=%d lag=%v", t3.ID, t3.Lag)
	}
	if got := clock.Dropped(); got != 1 {
		t.Fatalf("跳拍计数 = %d，应当 = 1", got)
	}
}

// TestTickIDMonotonic 验证 tick ID 单调递增、并暴露到 Status。
func TestTickIDMonotonic(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	sess := &busySession{busy: time.Millisecond}
	eng := New(dev, &fakeInput{}, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: 50}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()

	var last uint64
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		st := eng.Status()
		if st.TickID < last {
			t.Fatalf("tick ID 回退: %d -> %d", last, st.TickID)
		}
		last = st.TickID
	}
	cancel()
	<-done
	if last == 0 {
		t.Fatal("Status 里没有 tick ID")
	}
	t.Logf("最终 tick=%d", last)
}
