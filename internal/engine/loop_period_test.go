package engine

import (
	"context"
	"image"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// waitSession 固定返回 Wait，用来量"会话给的等待"如何决定循环周期。
type waitSession struct {
	wait  time.Duration
	steps atomic.Int32
}

func (s *waitSession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.steps.Add(1)
	return assist.Result{State: "x", Wait: s.wait}, nil, nil
}
func (s *waitSession) Close() error { return nil }

type slowDevice struct {
	captures atomic.Int32
	active   atomic.Bool
	cost     time.Duration
}

func (d *slowDevice) Capture() (*image.RGBA, float64, error) {
	d.captures.Add(1)
	time.Sleep(d.cost)
	return image.NewRGBA(image.Rect(0, 0, 4, 4)), 1, nil
}
func (d *slowDevice) IsActive() bool  { return d.active.Load() }
func (d *slowDevice) Activate() error { d.active.Store(true); return nil }

// TestLoopPeriodFollowsSessionWait 量真实循环周期：会话给的 Wait 与截图耗时如何叠加。
func TestLoopPeriodFollowsSessionWait(t *testing.T) {
	cases := []struct {
		name       string
		fps        int
		wait       time.Duration
		capture    time.Duration
		wantApprox time.Duration
	}{
		{"Wait 优先于 FPS 间隔", 10, 150 * time.Millisecond, 0, 150 * time.Millisecond},
		{"Wait 为 0 时用 FPS 间隔", 10, 0, 0, 100 * time.Millisecond},
		{"截图耗时叠加在 Wait 上", 10, 150 * time.Millisecond, 30 * time.Millisecond, 180 * time.Millisecond},
	}
	for _, c := range cases {
		dev := &slowDevice{cost: c.capture}
		dev.active.Store(true)
		in := &fakeInput{}
		sess := &waitSession{wait: c.wait}
		eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(),
			[]FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: c.fps}, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		done := make(chan struct{})
		go func() { defer close(done); _ = eng.Run(ctx) }()
		time.Sleep(900 * time.Millisecond)
		cancel()
		<-done
		steps := sess.steps.Load()
		if steps < 2 {
			t.Fatalf("%s: 只跑了 %d 帧", c.name, steps)
		}
		// 900ms 内应当跑 900/周期 帧左右，允许 ±25% 抖动。
		lo := int(float64(900*time.Millisecond) / float64(c.wantApprox) * 0.75)
		hi := int(float64(900*time.Millisecond) / float64(c.wantApprox) * 1.25)
		if int(steps) < lo || int(steps) > hi {
			t.Fatalf("%s: 900ms 内 %d 帧，按 %v 周期应在 %d~%d 帧", c.name, steps, c.wantApprox, lo, hi)
		}
		t.Logf("%s: 周期≈%v, 900ms 内 %d 帧", c.name, c.wantApprox, steps)
	}
}
