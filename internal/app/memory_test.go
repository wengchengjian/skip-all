package app

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/wengchengjian/skip-all/internal/platform"
)

// TestMemoryLimits 钉住看门狗阈值：必须存在且告警线低于强退线。
func TestMemoryLimits(t *testing.T) {
	warnMB, fatalMB := memoryLimits()
	t.Logf("告警线 %.0fMB，强退线 %.0fMB", warnMB, fatalMB)
	if fatalMB <= 0 {
		t.Fatalf("强退线 = %.0f，必须为正", fatalMB)
	}
	if warnMB <= 0 || warnMB >= fatalMB {
		t.Fatalf("告警线 %.0f 不在 (0, 强退线 %.0f) 内", warnMB, fatalMB)
	}
}

// TestProcessPrivateMB 验证能读到本进程的私有提交内存（看门狗的数据源）。
func TestProcessPrivateMB(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 统计私有内存")
	}
	mb := platform.ProcessPrivateMB()
	t.Logf("进程私有内存 %.1fMB", mb)
	if mb <= 0 {
		t.Fatal("读到 0，看门狗会失效")
	}
}

// TestMemoryExceededExits 在子进程里走一遍强退路径，必须打印日志并以 1 退出。
func TestMemoryExceededExits(t *testing.T) {
	if os.Getenv("SKIPALL_MEMWATCH_HELPER") == "1" {
		svc := &Service{sessions: map[string]*gameSession{}}
		svc.memoryExceeded(4000, 3000) // 内部 os.Exit(1)，不会返回
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMemoryExceededExits")
	cmd.Env = append(os.Environ(), "SKIPALL_MEMWATCH_HELPER=1")
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("期望子进程非零退出，实际 err = %v", err)
	}
	if code := ee.ExitCode(); code != 1 {
		t.Fatalf("子进程退出码 = %d，期望 1", code)
	}
}
