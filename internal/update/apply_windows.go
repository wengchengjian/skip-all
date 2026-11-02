//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// Apply 启动一个隐藏的 PowerShell 辅助进程：等待当前进程退出后替换 exe 并重启。
func Apply(source, destination string) error {
	script, err := os.CreateTemp(filepath.Dir(source), "skip-all-update-*.ps1")
	if err != nil {
		return fmt.Errorf("创建更新脚本: %w", err)
	}
	scriptPath := script.Name()
	body := `param([int]$ProcessId, [string]$Source, [string]$Destination)
$log = Join-Path $env:TEMP 'skip-all-update.log'
try {
    Wait-Process -Id $ProcessId -Timeout 30 -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 300
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
    Start-Process -FilePath $Destination
    Add-Content -LiteralPath $log -Value ("updated " + (Get-Date).ToString("o") + " " + $Destination)
} catch {
    Add-Content -LiteralPath $log -Value ("update failed " + (Get-Date).ToString("o") + " " + $_)
    exit 1
} finally {
    Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
}
`
	if _, err := script.WriteString(body); err != nil {
		_ = script.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("写入更新脚本: %w", err)
	}
	if err := script.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("关闭更新脚本: %w", err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-ProcessId", strconv.Itoa(os.Getpid()), "-Source", source, "-Destination", destination)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("启动更新脚本: %w", err)
	}
	return nil
}
