//go:build windows

package windows

import (
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessAlive 报告 names 中的游戏进程是否仍在运行，供引擎做进程门控：
// 进程不在时暂停截图，进程回来后再恢复。
// names 为空表示不启用门控；快照创建或枚举失败时同样返回 true——宁可少暂停一次，
// 也不要因为读不到进程列表而误判游戏已退出，把会话打成 error。
func ProcessAlive(names []string) bool {
	if len(names) == 0 {
		return true
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return true
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return true
	}
	for {
		if matchProcessName(names, windows.UTF16ToString(entry.ExeFile[:])) {
			return true
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			// 枚举结束（ERROR_NO_MORE_FILES）说明名单内进程都不在。
			return false
		}
	}
}

// matchProcessName 判断 exe（可以是完整路径）的文件名是否命中 names 中的任意一项，忽略大小写。
func matchProcessName(names []string, exe string) bool {
	base := filepath.Base(exe)
	for _, name := range names {
		if strings.EqualFold(base, name) {
			return true
		}
	}
	return false
}
