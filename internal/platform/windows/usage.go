//go:build windows

package windows

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")

	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// memoryStatusEx 对应 Win32 MEMORYSTATUSEX。
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// processMemoryCounters 对应 Win32 PROCESS_MEMORY_COUNTERS。
type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

var (
	usageMu      sync.Mutex
	usageLastCPU uint64 // 上次采样的内核+用户态时间（100ns）
	usageLastAt  time.Time
)

// ProcessUsage 返回本进程的 CPU 占用率与工作集大小。
// CPU 按两次调用之间的增量算，第一次调用返回 0；占用率按全部核心合计为 100% 折算。
func ProcessUsage() (cpuPercent, memMB float64) {
	now := time.Now()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err == nil {
		total := filetimeTicks(kernel) + filetimeTicks(user)
		usageMu.Lock()
		if !usageLastAt.IsZero() {
			if wall := now.Sub(usageLastAt).Seconds(); wall > 0 && total >= usageLastCPU {
				cpu := float64(total-usageLastCPU) / 1e7 // 100ns → 秒
				cpuPercent = cpu / wall * 100 / float64(runtime.NumCPU())
			}
		}
		usageLastCPU = total
		usageLastAt = now
		usageMu.Unlock()
	}

	var counters processMemoryCounters
	counters.CB = uint32(unsafe.Sizeof(counters))
	if ret, _, _ := procGetProcessMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB)); ret != 0 {
		memMB = float64(counters.WorkingSetSize) / (1024 * 1024)
	}
	return cpuPercent, memMB
}

// PrivateMemoryMB 返回本进程的私有提交内存（MB）。
//
// 这是 native 泄漏（D3D 纹理、ONNX 会话等不受 Go GC 管理）最先体现的指标；
// 工作集会被系统裁剪，不能用来看泄漏趋势。
func PrivateMemoryMB() float64 {
	var counters processMemoryCounters
	counters.CB = uint32(unsafe.Sizeof(counters))
	if ret, _, _ := procGetProcessMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB)); ret != 0 {
		return float64(counters.PagefileUsage) / (1024 * 1024)
	}
	return 0
}

// TotalPhysicalMB 返回本机物理内存总量（MB）。
func TotalPhysicalMB() float64 {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	if ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status))); ret == 0 {
		return 0
	}
	return float64(status.TotalPhys) / (1024 * 1024)
}

// filetimeTicks 把 FILETIME 转成 100ns 计数。
func filetimeTicks(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}
