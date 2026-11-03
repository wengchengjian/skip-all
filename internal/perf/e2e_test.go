//go:build windows

package perf

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/internal/engine"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// costDevice 模拟截图：固定耗时 + 返回同一张图。
type costDevice struct {
	cost time.Duration
	img  *image.RGBA
	mu   sync.Mutex
	n    int
}

func (d *costDevice) Capture() (*image.RGBA, float64, error) {
	time.Sleep(d.cost)
	d.mu.Lock()
	d.n++
	d.mu.Unlock()
	return d.img, 1, nil
}
func (d *costDevice) IsActive() bool  { return true }
func (d *costDevice) Activate() error { return nil }

type nopInput struct{}

func (nopInput) Click(geom.Point) error                    { return nil }
func (nopInput) MiddleClick(geom.Point) error              { return nil }
func (nopInput) ClickHold(geom.Point, time.Duration) error { return nil }
func (nopInput) Press(geom.Point) error                    { return nil }
func (nopInput) Release() error                            { return nil }
func (nopInput) ClickCurrent() error                       { return nil }
func (nopInput) KeyTap(string) error                       { return nil }
func (nopInput) ReleaseAll() error                         { return nil }

// nullSession 什么都不做，Wait=0（每拍都要跑）。
type nullSession struct{ calls int }

func (s *nullSession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	s.calls++
	return assist.Result{State: "空", Wait: 0}, nil, nil
}
func (s *nullSession) Close() error { return nil }

type nullFeature struct{ session *nullSession }

func (f *nullFeature) ID() string                       { return "null" }
func (f *nullFeature) Name() string                     { return "空功能" }
func (f *nullFeature) Description() string              { return "只用于量引擎开销" }
func (f *nullFeature) DefaultSettings() assist.Settings { return nil }
func (f *nullFeature) Validate(assist.Settings) error   { return nil }
func (f *nullFeature) NewSession(assist.Runtime, assist.Settings) (assist.Session, error) {
	return f.session, nil
}

// TestEngineTickOverhead 量引擎自身每拍的开销（不含功能与截图）。
//
// 用一个 Wait=0 的空功能 + 固定 13ms 的截图耗时跑真实引擎循环：
// 实际周期 - 13ms 就是引擎的固定开销。功能层的成本由 feature.*.step 单独量。
//
// 注意：不用真实功能做端到端帧率测量——假输入不会让游戏画面变化，
// 功能会卡在"要反复点击"的状态，帧率反而更低，那个数不可信。
func TestEngineTickOverhead(t *testing.T) {
	if os.Getenv("PERF_BASELINE") == "" {
		t.Skip("设置 PERF_BASELINE=1 才跑")
	}
	shot := loadShot(t, "all_cd_1080p.png")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	for _, c := range []struct {
		name    string
		fps     int
		capture time.Duration
	}{
		{"截图0ms  FPS=30", 30, 0},
		{"截图13ms FPS=30", 30, 13 * time.Millisecond},
		{"截图13ms FPS=10", 10, 13 * time.Millisecond},
	} {
		sess := &nullSession{}
		dev := &costDevice{cost: c.capture, img: shot}
		eng := engine.New(dev, nopInput{}, screen.NewSet(), vision.NewMatcher(nil, nil), logger,
			[]engine.FeatureSpec{{Feature: &nullFeature{session: sess}}},
			engine.Config{TargetFPS: c.fps}, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		done := make(chan struct{})
		start := time.Now()
		go func() { defer close(done); _ = eng.Run(ctx) }()
		time.Sleep(3 * time.Second)
		cancel()
		<-done
		elapsed := time.Since(start).Seconds()

		dev.mu.Lock()
		n := dev.n
		dev.mu.Unlock()
		period := time.Duration(float64(elapsed) / float64(n) * float64(time.Second))
		fmt.Printf("ENGINE|case=%s|ticks=%d|fps=%.2f|period_ms=%.2f|overhead_us=%d\n",
			c.name, n, float64(n)/elapsed, float64(period.Microseconds())/1000.0,
			(period - c.capture).Microseconds())
	}
}
