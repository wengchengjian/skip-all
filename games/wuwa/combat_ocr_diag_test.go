package wuwa

import (
	"fmt"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// countingOCRRuntime 记录每帧 OCR 被哪些调用点触发。
type countingOCRRuntime struct {
	simRuntime
	frame    int
	perFrame map[int][]string
	calls    int
}

func (r *countingOCRRuntime) OCR(frame assist.Frame, area screen.Area, _ vision.OCROptions) ([]vision.TextBlock, error) {
	r.calls++
	r.perFrame[r.frame] = append(r.perFrame[r.frame], fmt.Sprintf("%s@%s", area.Name, frame.At.Format("05.000")))
	return nil, nil
}

// TestDiagOCRCallSites 打印每一帧触发了哪些 OCR 区域，定位缓存为何失效。
func TestDiagOCRCallSites(t *testing.T) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	rt := &countingOCRRuntime{simRuntime: simRuntime{game: g}, perFrame: map[int][]string{}}
	feature := CombatFeature()
	session, err := feature.NewSession(rt, feature.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	spec := combatSpec()
	base := time.Now()
	for i := 0; i < 40; i++ {
		rt.frame = i
		frame := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
		result, actions, err := session.Step(nil, frame)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			g.apply(a, result.State)
		}
	}
	for i := 0; i < 12; i++ {
		t.Logf("f%02d %v", i, rt.perFrame[i])
	}
	t.Logf("共 %d 次 OCR / 40 帧", rt.calls)
}
