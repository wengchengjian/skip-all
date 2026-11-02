package combat

import (
	"testing"
	"time"
)

// TestSettingsIntervalIsApplied 检查设置里的 interval 是否真的生效。
// 上游 ok-ww 的触发间隔可配；如果设置只校验不应用，用户在界面上改间隔就没效果。
func TestSettingsIntervalIsApplied(t *testing.T) {
	spec := testSpec()
	spec.Interval = 150 * time.Millisecond // 默认值，和设置里的 0.05 不同
	feature := NewFeature(spec)
	settings := feature.DefaultSettings()
	settings[SettingInterval] = 0.05
	session, err := feature.NewSession(testRuntime(true), settings)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	result, _, err := session.Step(nil, frameAt(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if result.Wait != 50*time.Millisecond {
		t.Fatalf("设置 interval=0.05 后 Wait=%v，设置没有生效", result.Wait)
	}
}
