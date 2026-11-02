package combat

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// abortRuntime 记录 OCR 调用次数，用来验证中断提示的采样间隔。
type abortRuntime struct {
	*fakeRuntime
	calls int
}

func (r *abortRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	r.calls++
	return nil, nil
}

// TestAbortCheckIsSampled 验证"中断提示没出现"这条最常见路径不会每帧都跑 OCR。
// OCR 单次约 130ms，如果每帧都做，10Hz 的决策拍会被完全吃掉。
func TestAbortCheckIsSampled(t *testing.T) {
	base := testRuntime(true)
	rt := &abortRuntime{fakeRuntime: base}
	spec := testSpec()
	spec.AbortArea = "中断"
	spec.AbortTexts = []string{"复活", "失败"}
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	defer session.Close()

	at := time.Now()
	const frames = 40 // 4 秒 @10Hz
	for i := 0; i < frames; i++ {
		step(t, session, frameAt(at.Add(time.Duration(i)*100*time.Millisecond)))
	}
	// 500ms 采样间隔下，4 秒最多 8 次；宽松上限 12。
	if rt.calls > 12 {
		t.Fatalf("4 秒内 OCR %d 次，中断提示没有按间隔采样", rt.calls)
	}
	t.Logf("4 秒内 OCR %d 次", rt.calls)
}
