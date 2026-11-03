package combat

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// freshRuntime 模拟异步运行时：OCR 返回"基于更早一帧"的时间。
type freshRuntime struct {
	*fakeRuntime
	staleBy time.Duration
	calls   int
}

func (r *freshRuntime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	blocks, _, err := r.OCRFresh(frame, area, opts)
	return blocks, err
}

func (r *freshRuntime) OCRFresh(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, time.Time, error) {
	r.calls++
	blocks, err := r.fakeRuntime.OCR(frame, area, opts)
	return blocks, frame.At.Add(-r.staleBy), err
}

// TestAbortIgnoresStaleResult 验证中断提示判定不会拿"明显滞后"的结果下结论。
//
// 中断提示决定"立刻停手"，是瞬时判据。异步返回的结果如果基于很久之前的
// 画面，那时角色可能还没复活；拿它断言"现在没中断"会让机器人继续放技能。
func TestAbortIgnoresStaleResult(t *testing.T) {
	cases := []struct {
		name    string
		staleBy time.Duration
		// wantRecheck 表示这次采样应当被作废（下一拍重新识别）。
		wantRecheck bool
	}{
		{"滞后 10ms（一拍之内，可信）", 10 * time.Millisecond, false},
		{"滞后 300ms（明显滞后，作废）", 300 * time.Millisecond, true},
	}
	for _, c := range cases {
		base := testRuntime(true)
		rt := &freshRuntime{fakeRuntime: base, staleBy: c.staleBy}
		spec := testSpec()
		spec.AbortArea = "中断"
		spec.AbortTexts = []string{"复活", "失败"}
		session := newSession(rt, spec.withDefaults(), sessionSettings{})
		now := time.Now()
		step(t, session, frameAt(now))
		checkedAt := session.abortCheckedAt
		session.Close()

		// 滞后在容忍范围内：保留采样时间（下一拍不会重复识别）。
		// 超出范围：采样时间被清空，下一拍会重新识别。
		cleared := checkedAt.IsZero()
		t.Logf("%-32s staleBy=%v 采样时间被清空=%v", c.name, c.staleBy, cleared)
		if cleared != c.wantRecheck {
			t.Fatalf("%s: 采样时间清空=%v，期望 %v", c.name, cleared, c.wantRecheck)
		}
	}
}

// TestAbortStillDetectsOnFreshResult 验证不滞后的结果仍然能正常识别中断。
func TestAbortStillDetectsOnFreshResult(t *testing.T) {
	base := testRuntime(true)
	rt := &freshRuntime{fakeRuntime: base}
	spec := testSpec()
	spec.AbortArea = "中断"
	spec.AbortTexts = []string{"复活"}
	// 让 OCR 返回含"复活"的文本。
	base.ocrText = "复活"
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	defer session.Close()
	step(t, session, frameAt(time.Now()))
	if session.abortAt.IsZero() {
		t.Fatal("识别到复活提示但没有记录中断时间")
	}
}
