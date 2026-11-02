//go:build windows

package engine

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// blockingSession 在 Step 里阻塞指定时长，模拟同步 OCR。
type blockingSession struct {
	name  string
	cost  time.Duration
	steps []time.Time
}

func (s *blockingSession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.steps = append(s.steps, time.Now())
	time.Sleep(s.cost)
	return assist.Result{State: s.name, Wait: 100 * time.Millisecond}, nil, nil
}
func (s *blockingSession) Close() error { return nil }

// TestSyncOCRExceedsBudget 验证工作耗时超过一拍预算时的行为。
// tick 时钟只能在"工作耗时 < wait"时吸收耗时；
// 一旦超过，周期就等于工作耗时，Lag 持续增长，决策拍被拖慢。
func TestSyncOCRExceedsBudget(t *testing.T) {
	cases := []struct {
		name       string
		cost       time.Duration
		wantFaster bool // 是否仍能维持 100ms 周期
	}{
		{"OCR 60ms（预算内）", 60 * time.Millisecond, true},
		{"OCR 130ms（超预算）", 130 * time.Millisecond, false},
		{"OCR 250ms（严重超预算）", 250 * time.Millisecond, false},
	}
	for _, c := range cases {
		dev := &fakeDevice{}
		dev.active.Store(true)
		sess := &blockingSession{name: "x", cost: c.cost}
		eng := New(dev, &fakeInput{}, screen.NewSet(), fakeMatcher{}, slog.Default(),
			[]FeatureSpec{{Feature: &namedFeature{id: "x", session: sess}}}, Config{TargetFPS: 10}, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		done := make(chan struct{})
		go func() { defer close(done); _ = eng.Run(ctx) }()
		time.Sleep(1800 * time.Millisecond)
		st := eng.Status()
		cancel()
		<-done

		var period time.Duration
		if n := len(sess.steps); n > 1 {
			period = sess.steps[n-1].Sub(sess.steps[0]) / time.Duration(n-1)
		}
		t.Logf("%-28s 实际周期 %v，拍数 %d，TickLag %v，跳拍 %d",
			c.name, period, st.TickID, st.TickLag, st.TickDropped)
		if c.wantFaster {
			if period > 110*time.Millisecond {
				t.Fatalf("%s: 周期 %v 应当约等于 100ms（预算内应被吸收）", c.name, period)
			}
		} else {
			// 超预算时周期必然被拖长到接近工作耗时。
			if period < c.cost {
				t.Fatalf("%s: 周期 %v 小于工作耗时 %v，数据异常", c.name, period, c.cost)
			}
		}
	}
}

// TestTickDroppedGrowsWhenOverBudget 验证超预算时跳拍计数增长——
// 这是"识别拖垮循环"的可观测信号。
func TestTickDroppedGrowsWhenOverBudget(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	sess := &blockingSession{name: "x", cost: 700 * time.Millisecond}
	eng := New(dev, &fakeInput{}, screen.NewSet(), fakeMatcher{}, slog.Default(),
		[]FeatureSpec{{Feature: &namedFeature{id: "x", session: sess}}}, Config{TargetFPS: 10}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan struct{})
	go func() { defer close(done); _ = eng.Run(ctx) }()
	time.Sleep(2800 * time.Millisecond)
	st := eng.Status()
	cancel()
	<-done
	t.Logf("拍数 %d，跳拍 %d，TickLag %v", st.TickID, st.TickDropped, st.TickLag)
	if st.TickDropped == 0 {
		t.Fatal("每拍 700ms 远超 100ms 预算，跳拍计数应当增长")
	}
}
