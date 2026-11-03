package vision

import (
	"runtime"
	"testing"
)

// TestProductionPathUsesAdaptiveThreads 验证生产路径（service.go 直接构造
// PaddleOptions、不填 NumThreads）也能拿到自适应线程数。
//
// 这是回归守卫：之前 NewPaddleOCRWithOptions 对零值兜底成 1，
// 导致线上一直是单线程跑 ONNX，实测比自适应值慢近一倍。
func TestProductionPathUsesAdaptiveThreads(t *testing.T) {
	want := DefaultNumThreads()
	got := resolveNumThreads(0)
	if got != want {
		t.Fatalf("零值 NumThreads 解析成 %d，期望自适应值 %d", got, want)
	}
	if got == 1 && runtime.NumCPU() >= 8 {
		t.Fatalf("%d 核机器上仍解析成单线程，自适应没生效", runtime.NumCPU())
	}
	// 显式指定时必须尊重调用方。
	if got := resolveNumThreads(3); got != 3 {
		t.Fatalf("显式 NumThreads=3 被改成了 %d", got)
	}
}
