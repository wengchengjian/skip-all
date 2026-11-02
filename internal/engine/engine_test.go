package engine

import (
	"context"
	"image"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

type fakeDevice struct {
	captures atomic.Int32
	active   atomic.Bool
}

func (f *fakeDevice) Capture() (*image.RGBA, float64, error) {
	f.captures.Add(1)
	return image.NewRGBA(image.Rect(0, 0, 4, 4)), 1, nil
}
func (f *fakeDevice) IsActive() bool  { return f.active.Load() }
func (f *fakeDevice) Activate() error { f.active.Store(true); return nil }

type fakeInput struct {
	middleClicks atomic.Int32
	clicks       atomic.Int32
	keys         atomic.Int32
	holds        atomic.Int32
	releases     atomic.Int32
}

func (f *fakeInput) Click(geom.Point) error { f.clicks.Add(1); return nil }

func (f *fakeInput) ClickHold(_ geom.Point, _ time.Duration) error { return nil }

func (f *fakeInput) Press(geom.Point) error { f.holds.Add(1); return nil }
func (f *fakeInput) Release() error         { f.releases.Add(1); return nil }

func (f *fakeInput) MiddleClick(geom.Point) error { f.middleClicks.Add(1); return nil }
func (f *fakeInput) ClickCurrent() error          { f.clicks.Add(1); return nil }
func (f *fakeInput) KeyTap(string) error          { f.keys.Add(1); return nil }
func (f *fakeInput) ReleaseAll() error            { return nil }

type fakeFeature struct {
	session assist.Session
}

func (f *fakeFeature) ID() string                       { return "fake" }
func (f *fakeFeature) Name() string                     { return "fake" }
func (f *fakeFeature) Description() string              { return "fake" }
func (f *fakeFeature) DefaultSettings() assist.Settings { return nil }
func (f *fakeFeature) Validate(assist.Settings) error   { return nil }
func (f *fakeFeature) NewSession(assist.Runtime, assist.Settings) (assist.Session, error) {
	return f.session, nil
}

type fakeSession struct {
	steps atomic.Int32
}

func (s *fakeSession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	n := s.steps.Add(1)
	if n == 1 {
		return assist.Result{State: "test", Wait: time.Millisecond, Message: "点击一次"}, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1, Y: 2}}}, nil
	}
	return assist.Result{State: "test", Wait: 2 * time.Millisecond}, nil, nil
}
func (s *fakeSession) Close() error { return nil }

type panicSession struct{}

func (*panicSession) Step(context.Context, assist.Frame) (assist.Result, []assist.Action, error) {
	panic("feature boom")
}
func (*panicSession) Close() error { return nil }

type fakeMatcher struct{}

func (fakeMatcher) OCR(*image.RGBA, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	return nil, nil
}
func (fakeMatcher) Template(*image.RGBA, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
	return nil, nil
}
func (fakeMatcher) Color(*image.RGBA, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}

func TestRunExecutesActionsAndStops(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	sess := &fakeSession{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), []FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: 100}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	err := eng.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if in.clicks.Load() != 1 {
		t.Fatalf("clicks = %d, want 1", in.clicks.Load())
	}
	if sess.steps.Load() < 2 {
		t.Fatalf("steps = %d, want >=2", sess.steps.Load())
	}
	if got := eng.Status().State; got != StateStopped {
		t.Fatalf("state = %s, want %s", got, StateStopped)
	}
}

func TestRunRecoversFeaturePanic(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), []FeatureSpec{{Feature: &fakeFeature{session: &panicSession{}}}}, Config{TargetFPS: 100}, nil)
	err := eng.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "会话异常退出") {
		t.Fatalf("Run err = %v, want 会话异常退出", err)
	}
	if got := eng.Status().State; got != StateError {
		t.Fatalf("state = %s, want %s", got, StateError)
	}
}

func TestActionSummary(t *testing.T) {
	got := actionSummary([]assist.Action{
		{Kind: assist.ActionClick, Point: geom.Point{X: 12, Y: 34}},
		{Kind: assist.ActionClickCurrent},
		{Kind: assist.ActionKeyTap, Key: "xbox_a"},
	})
	want := "点击(12,34) + 点击当前光标 + 按键 xbox_a"
	if got != want {
		t.Fatalf("actionSummary = %q, want %q", got, want)
	}
}

func TestPauseSkipsCapture(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	sess := &fakeSession{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), []FeatureSpec{{Feature: &fakeFeature{session: sess}}}, Config{TargetFPS: 100}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		eng.Pause()
	}()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if dev.captures.Load() == 0 {
		t.Fatal("expected captures before pause")
	}
}

type namedFeature struct {
	id      string
	session assist.Session
}

func (f *namedFeature) ID() string                       { return f.id }
func (f *namedFeature) Name() string                     { return f.id }
func (f *namedFeature) Description() string              { return f.id }
func (f *namedFeature) DefaultSettings() assist.Settings { return nil }
func (f *namedFeature) Validate(assist.Settings) error   { return nil }
func (f *namedFeature) NewSession(assist.Runtime, assist.Settings) (assist.Session, error) {
	return f.session, nil
}

// TestRunRunsAllFeatures 验证同一帧截图喂给多个功能、动作按功能顺序串行执行。
func TestRunRunsAllFeatures(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	first := &fakeSession{}
	second := &fakeSession{}
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), []FeatureSpec{
		{Feature: &namedFeature{id: "story_skip", session: first}},
		{Feature: &namedFeature{id: "auto_combat", session: second}},
	}, Config{TargetFPS: 100}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if first.steps.Load() == 0 || second.steps.Load() == 0 {
		t.Fatalf("两个功能都应被驱动: first=%d second=%d", first.steps.Load(), second.steps.Load())
	}
	if in.clicks.Load() < 2 {
		t.Fatalf("两个功能的点击都要执行, clicks = %d", in.clicks.Load())
	}
	st := eng.Status()
	if len(st.Features) != 2 {
		t.Fatalf("Status.Features = %+v", st.Features)
	}
	if st.Features[0].ID != "story_skip" || st.Features[1].ID != "auto_combat" {
		t.Fatalf("功能顺序 = %+v", st.Features)
	}
}

// TestProcessGatePausesAndResumes 验证游戏进程退出时自动暂停、进程回来后自动恢复。
func TestProcessGatePausesAndResumes(t *testing.T) {
	dev := &fakeDevice{}
	dev.active.Store(true)
	in := &fakeInput{}
	sess := &fakeSession{}
	var alive atomic.Bool
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), []FeatureSpec{
		{Feature: &namedFeature{id: "story_skip", session: sess}},
	}, Config{TargetFPS: 100, ProcessAlive: alive.Load}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- eng.Run(ctx) }()

	// 进程不在：不该截图，状态应为暂停。
	time.Sleep(120 * time.Millisecond)
	if n := dev.captures.Load(); n != 0 {
		t.Fatalf("进程不在时不该截图, captures = %d", n)
	}
	if st := eng.Status(); st.State != StatePaused {
		t.Fatalf("进程不在时应暂停, state = %s", st.State)
	}

	// 进程回来：自动恢复并开始截图。
	alive.Store(true)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st := eng.Status(); st.State == StateRunning && dev.captures.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := eng.Status(); st.State != StateRunning {
		t.Fatalf("进程回来后应自动恢复, state = %s", st.State)
	}
	if dev.captures.Load() == 0 {
		t.Fatal("进程回来后应继续截图")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run 未退出")
	}
}
