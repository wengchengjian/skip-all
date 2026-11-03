package app

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/wengchengjian/skip-all/internal/platform"
)

const (
	// memWatchInterval 是采样周期。2 秒足够在页面文件被打爆前反应。
	memWatchInterval = 2 * time.Second
	// memWarnRatio 是告警线占强退线的比例。
	memWarnRatio = 0.8
	// memLimitCapMB 是强退线上限；正常运行时进程在几百 MB。
	memLimitCapMB = 3072
	// memLimitFloorMB 是小内存机器的强退线下限。
	memLimitFloorMB = 1024
)

// memoryLimits 返回告警线与强退线（MB）。
//
// 强退线取物理内存的 1/4，封顶 3GB、保底 1GB。现场 16GB 机器上进程涨到
// 10.5GB 才开始报低内存，那时 DWM 已经被挤崩多次了，所以必须提前刹车。
func memoryLimits() (warnMB, fatalMB float64) {
	fatalMB = platform.TotalPhysicalMB() / 4
	if fatalMB <= 0 || fatalMB > memLimitCapMB {
		fatalMB = memLimitCapMB
	}
	if fatalMB < memLimitFloorMB {
		fatalMB = memLimitFloorMB
	}
	return fatalMB * memWarnRatio, fatalMB
}

// startMemoryWatchdog 监控进程私有提交内存。
//
// native 资源（D3D 纹理、ONNX 会话）不受 Go GC 管理，泄漏时只能靠这里兜底：
// 到告警线打 warn，到强退线停会话并退出。不能静默等到系统级低内存，
// 否则会把 DWM 一起拖崩（黑屏）并连累同机其它服务。
func (s *Service) startMemoryWatchdog() {
	warnMB, fatalMB := memoryLimits()
	s.mu.Lock()
	if s.memStop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.memStop = stop
	s.mu.Unlock()
	s.memoryLog("info", fmt.Sprintf("内存看门狗：告警线 %.0fMB，强退线 %.0fMB", warnMB, fatalMB))

	go func() {
		ticker := time.NewTicker(memWatchInterval)
		defer ticker.Stop()
		warned := false
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				priv := platform.ProcessPrivateMB()
				if priv <= 0 {
					continue // 非 Windows 或取不到
				}
				if priv >= fatalMB {
					s.memoryExceeded(priv, fatalMB)
					return
				}
				if priv >= warnMB && !warned {
					warned = true
					s.memoryLog("warn", fmt.Sprintf("进程私有内存 %.0fMB 已超过告警线 %.0fMB；涨到 %.0fMB 会停止会话并退出", priv, warnMB, fatalMB))
				}
			}
		}
	}()
}

// memoryLog 同时写日志文件与 UI：看门狗的消息不能只留在文件里。
func (s *Service) memoryLog(level, message string) {
	if parseLogLevel(level) < s.logLevel {
		return
	}
	s.appendLog(level, message)
	s.emit("skipall:log", LogEntry{Time: time.Now(), Level: level, Message: message})
}

// memoryExceeded 停掉全部会话并退出进程。
//
// 先给 StopAll 最多 2 秒收尾，避免卡在某个会话的关闭路径上；之后无论是否
// 收尾完成都必须退出——此时继续活着只会把系统拖进换页。
func (s *Service) memoryExceeded(priv, fatalMB float64) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.memoryLog("error", fmt.Sprintf("进程私有内存 %.0fMB 超过强退线 %.0fMB：停止所有会话并退出，避免拖垮系统", priv, fatalMB))
	s.memoryLog("error", fmt.Sprintf("内存诊断：Go 堆 %.0fMB / Sys %.0fMB，goroutine %d，运行中会话 %d",
		float64(ms.HeapAlloc)/(1<<20), float64(ms.Sys)/(1<<20), runtime.NumGoroutine(), len(s.runningSessions())))

	done := make(chan struct{})
	go func() {
		s.StopAll()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	os.Exit(1)
}
