package combat

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// lifecycleRuntime 只实现 assist.Runtime，用来覆盖适配器与生命周期方法。
type lifecycleRuntime struct{ called bool }

func (l *lifecycleRuntime) Screen(string) (*screen.Screen, error) {
	return nil, errors.New("测试里没有画面配置")
}

func (l *lifecycleRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	return nil, nil
}

func (l *lifecycleRuntime) MatchTemplate(assist.Frame, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
	l.called = true
	return nil, nil
}

func (l *lifecycleRuntime) ColorStats(assist.Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}

func (l *lifecycleRuntime) Log() *slog.Logger { return slog.Default() }

// TestRuntimeAdapterForwards 保证扩展拿到的 Runtime 真的会转到 assist.Runtime。
func TestRuntimeAdapterForwards(t *testing.T) {
	rt := &lifecycleRuntime{}
	adapter := runtimeAdapter{rt: rt}
	_, err := adapter.MatchTemplate(assist.Frame{}, screen.Area{}, vision.TemplateRef{SubDir: "x", ID: "y"}, 0.8)
	if err != nil {
		t.Fatalf("适配器出错: %v", err)
	}
	if !rt.called {
		t.Fatal("适配器没有把调用转给 assist.Runtime")
	}
}

// TestFeatureLifecycle 覆盖 Feature 的 NewSession / Description / Session.Close。
func TestFeatureLifecycle(t *testing.T) {
	feature := NewFeature(testSpec())
	if feature.Description() == "" {
		t.Error("功能描述为空")
	}

	if _, err := feature.NewSession(nil, nil); err == nil {
		t.Error("运行时为 nil 时应当报错")
	}

	session, err := feature.NewSession(&lifecycleRuntime{}, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if session == nil {
		t.Fatal("会话为 nil")
	}
	if err := session.Close(); err != nil {
		t.Errorf("Close 返回错误: %v", err)
	}
}

// TestNewSessionRejectsInvalidSpec 覆盖 NewSession 里的配置校验分支。
func TestNewSessionRejectsInvalidSpec(t *testing.T) {
	bad := testSpec()
	bad.Keys = Keys{} // 没有按键也没有攻击点
	feature := NewFeature(bad)
	if _, err := feature.NewSession(&lifecycleRuntime{}, nil); err == nil {
		t.Error("配置无效时应当报错")
	}
}
