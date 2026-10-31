package engine

import (
	"context"
	"image"
	"log/slog"
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
	clicks atomic.Int32
	keys   atomic.Int32
}

func (f *fakeInput) Click(geom.Point) error { f.clicks.Add(1); return nil }
func (f *fakeInput) ClickCurrent() error    { f.clicks.Add(1); return nil }
func (f *fakeInput) KeyTap(string) error    { f.keys.Add(1); return nil }
func (f *fakeInput) ReleaseAll() error      { return nil }

type fakeFeature struct {
	session *fakeSession
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
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), &fakeFeature{session: sess}, nil, Config{TargetFPS: 100}, nil)
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
	eng := New(dev, in, screen.NewSet(), fakeMatcher{}, slog.Default(), &fakeFeature{session: sess}, nil, Config{TargetFPS: 100}, nil)
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
