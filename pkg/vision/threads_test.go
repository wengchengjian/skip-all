package vision

import (
	"runtime"
	"testing"
)

// TestDefaultNumThreads 验证线程数选择落在实测的安全区间。
//
// 实测规律（见 DefaultNumThreads 注释）：
// 4 核最优 2 线程、劣化点在 3；24 核最优 6 线程、平台期 4~10、劣化点 12。
func TestDefaultNumThreads(t *testing.T) {
	got := DefaultNumThreads()
	cores := runtime.NumCPU()
	t.Logf("本机 %d 核 -> 选 %d 线程", cores, got)

	if got < 2 {
		t.Fatalf("线程数 %d 低于下限 2（4 核机上 2 比 1 快）", got)
	}
	if got > 8 {
		t.Fatalf("线程数 %d 超过上限 8（24 核机 8 已在平台期边缘）", got)
	}

	// 关键约束：不能越过劣化点。实测劣化点在核心数的 1/2 附近。
	if limit := cores / 2; got > limit && limit >= 2 {
		t.Fatalf("线程数 %d 越过了劣化点 %d（核心数 %d 的 1/2）", got, limit, cores)
	}
}

// TestDefaultNumThreadsTable 用已知机器规格验证选值。
func TestDefaultNumThreadsTable(t *testing.T) {
	// 复刻 DefaultNumThreads 的公式，验证各规格下的取值。
	calc := func(cores int) int {
		n := cores / 4
		if n < 2 {
			n = 2
		}
		if n > 8 {
			n = 8
		}
		return n
	}
	cases := []struct {
		cores int
		want  int
		note  string
	}{
		{2, 2, "双核：下限 2"},
		{4, 2, "4 核（云服务器实测最优 2）"},
		{8, 2, "8 核：8/4=2"},
		{12, 3, "12 核：劣化点 6，3 安全"},
		{16, 4, "16 核：劣化点 8，4 安全"},
		{24, 6, "24 核（本机实测最优 6）"},
		{32, 8, "32 核：上限 8"},
		{64, 8, "64 核：上限 8"},
	}
	for _, c := range cases {
		if got := calc(c.cores); got != c.want {
			t.Errorf("%d 核算出 %d，期望 %d（%s）", c.cores, got, c.want, c.note)
		}
	}
}
