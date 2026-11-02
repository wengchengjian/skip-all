package assist

import (
	"image"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// TestAsyncOCRFreshReportsFrameTime 验证 OCRFresh 会告诉调用方
// 结果基于的是哪一帧，从而能算出信息年龄。
func TestAsyncOCRFreshReportsFrameTime(t *testing.T) {
	inner := newSlowRuntime(80 * time.Millisecond)
	rt := NewAsyncRuntime(inner, 1)
	defer rt.Close()

	area := screen.Area{Name: "区域", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	base := time.Now()
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))

	// 首帧同步：返回零值时间，表示"就是这一帧"。
	f0 := Frame{Image: img, At: base}
	_, at, err := rt.OCRFresh(f0, area, vision.OCROptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !at.IsZero() {
		t.Fatalf("同步路径应当返回零值时间，实际 %v", at)
	}

	time.Sleep(120 * time.Millisecond)
	// 第二帧：走异步，返回的是第一帧的时间。
	f1 := Frame{Image: img, At: base.Add(500 * time.Millisecond)}
	_, at, err = rt.OCRFresh(f1, area, vision.OCROptions{})
	if err != nil {
		t.Fatal(err)
	}
	if at.IsZero() {
		t.Fatal("异步路径应当返回结果所基于的帧时间")
	}
	if !at.Equal(f0.At) {
		t.Fatalf("返回的帧时间 = %v，应当是首帧的 %v", at, f0.At)
	}
	age := f1.At.Sub(at)
	t.Logf("第二帧拿到的信息年龄 %v", age)
	if age != 500*time.Millisecond {
		t.Fatalf("信息年龄 = %v，应当 500ms", age)
	}
}

// TestAsyncOCRStatsWorstAge 验证 WorstAge 指标会被记录。
func TestAsyncOCRStatsWorstAge(t *testing.T) {
	inner := newSlowRuntime(60 * time.Millisecond)
	rt := NewAsyncRuntime(inner, 1)
	defer rt.Close()

	area := screen.Area{Name: "区域", Rect: geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 10}}
	base := time.Now()
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	_, _ = rt.OCR(Frame{Image: img, At: base}, area, vision.OCROptions{})
	time.Sleep(100 * time.Millisecond)
	for i := 1; i < 10; i++ {
		_, _ = rt.OCR(Frame{Image: img, At: base.Add(time.Duration(i) * 100 * time.Millisecond)}, area, vision.OCROptions{})
		time.Sleep(30 * time.Millisecond)
	}
	st := rt.Stats()
	t.Logf("WorstAge=%v 提交=%d 完成=%d 合并=%d", st.WorstAge, st.Submitted, st.Completed, st.Dropped)
	if st.WorstAge <= 0 {
		t.Fatal("WorstAge 没有被记录")
	}
}
