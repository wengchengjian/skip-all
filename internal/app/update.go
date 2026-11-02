package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wengchengjian/skip-all/internal/update"
)

// GetVersion 返回当前构建版本。
func (s *Service) GetVersion() string { return Version }

// StartAutoUpdateCheck 在启动后异步检查一次更新。
// 发现新版本且开启 auto_download 时后台下载；是否安装由用户决定。
func (s *Service) StartAutoUpdateCheck() {
	s.mu.Lock()
	enabled := s.appCfg.Update.AutoCheck
	autoDownload := s.appCfg.Update.AutoDownload
	s.mu.Unlock()
	if !enabled {
		return
	}
	go func() {
		// 每次启动先清掉已经装过的旧版本缓存，与网络无关。
		s.cleanUpdates("")
		time.Sleep(4 * time.Second)
		view, err := s.CheckForUpdates()
		if err != nil {
			s.appendLog("info", "检查更新失败: "+err.Error())
			return
		}
		if !view.Available {
			// 已是最新版本：本地残留的旧包也清掉。
			s.cleanUpdates("")
			return
		}
		if !autoDownload {
			s.cleanUpdates(view.Latest)
			return
		}
		if err := s.DownloadUpdate(); err != nil {
			s.appendLog("info", "后台下载更新失败: "+err.Error())
		}
	}()
}

// updatesDir 返回更新包缓存目录；每个版本一个子目录。
func (s *Service) updatesDir() string { return filepath.Join(s.dataDir, "updates") }

// GetUpdateInfo 返回最近一次检查结果。
func (s *Service) GetUpdateInfo() UpdateView {
	s.mu.Lock()
	info := s.updateInfo
	s.mu.Unlock()
	view := toUpdateView(info)
	s.decorateUpdateView(&view)
	return view
}

// CheckForUpdates 查询 GitHub latest release 并缓存结果。
func (s *Service) CheckForUpdates() (UpdateView, error) {
	s.mu.Lock()
	checker := s.updateChecker
	if checker == nil {
		checker = update.NewChecker(s.appCfg.Update.Repo)
		s.updateChecker = checker
	}
	current := Version
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	info, err := checker.Check(ctx, current)
	if err != nil {
		return UpdateView{Current: current}, err
	}
	view := toUpdateView(info)
	s.decorateUpdateView(&view)
	s.mu.Lock()
	s.updateInfo = info
	s.mu.Unlock()
	if info.Available {
		s.appendLog("info", fmt.Sprintf("发现新版本 %s（当前 %s）", info.Latest, current))
		s.emit("skipall:update", view)
	}
	return view, nil
}

// DownloadUpdate 下载当前检查到的新版本；已下载过则直接复用本地文件。
// 同一时间只允许一个下载任务，并发调用会等待前一个完成。
func (s *Service) DownloadUpdate() error {
	s.updateDownloadMu.Lock()
	defer s.updateDownloadMu.Unlock()

	s.mu.Lock()
	info := s.updateInfo
	s.mu.Unlock()
	if !info.Available {
		return fmt.Errorf("当前没有可用更新")
	}
	if _, ok := update.FindLocal(s.updatesDir(), info.Latest); ok {
		// 该版本已下载过：顺手清掉更旧的缓存。
		s.cleanUpdates(info.Latest)
		return nil
	}
	s.appendLog("info", "开始下载更新 "+info.Latest)
	path, err := info.DownloadWithProgress(context.Background(), nil, s.updatesDir(), func(done, total int64) {
		if total <= 0 {
			return
		}
		percent := int(done * 100 / total)
		s.emit("skipall:update-progress", map[string]any{"version": info.Latest, "percent": percent, "done": done, "total": total})
	})
	if err != nil {
		return err
	}
	s.appendLog("info", "更新已下载: "+path+"，可在设置页选择是否安装")
	s.cleanUpdates(info.Latest)
	s.mu.Lock()
	view := toUpdateView(s.updateInfo)
	s.mu.Unlock()
	s.decorateUpdateView(&view)
	s.emit("skipall:update", view)
	return nil
}

// cleanUpdates 删除已装过和比 keep 旧的更新包，只留最新一个版本。
func (s *Service) cleanUpdates(keep string) {
	freed, err := update.CleanLocal(s.updatesDir(), Version, keep)
	if err != nil {
		s.appendLog("warn", "清理更新缓存失败: "+err.Error())
		return
	}
	if freed > 0 {
		s.appendLog("info", fmt.Sprintf("清理更新缓存，释放 %s", humanSize(freed)))
	}
}

// humanSize 把字节数格式化成 MB/KB，用于日志。
func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// InstallUpdate 安装最新版本：优先用本地已下载的最新包，缺失时再下载。
func (s *Service) InstallUpdate() ActionResult {
	s.mu.Lock()
	quit := s.quit
	s.mu.Unlock()
	if quit == nil {
		return ActionResult{OK: false, Error: "当前环境不支持自动重启"}
	}
	// 本地可能缓存了多个版本：先记下最新的那个，离线时直接用它。
	local, hasLocal := update.PickLocal(s.updatesDir(), Version)
	view, err := s.CheckForUpdates()
	if err != nil {
		if hasLocal {
			return s.applyUpdate(local.Path, quit)
		}
		return ActionResult{OK: false, Error: "检查更新失败: " + err.Error()}
	}
	if !view.Available {
		if hasLocal {
			return s.applyUpdate(local.Path, quit)
		}
		return ActionResult{OK: false, Error: "当前已是最新版本"}
	}
	// 远端有更新：本地缓存已覆盖该版本就直接装，否则下载后再取最新本地包。
	if hasLocal && update.CompareVersions(local.Version, view.Latest) >= 0 {
		return s.applyUpdate(local.Path, quit)
	}
	if err := s.DownloadUpdate(); err != nil {
		return ActionResult{OK: false, Error: "下载更新失败: " + err.Error()}
	}
	local, ok := update.PickLocal(s.updatesDir(), Version)
	if !ok {
		return ActionResult{OK: false, Error: "更新包不存在"}
	}
	return s.applyUpdate(local.Path, quit)
}

// applyUpdate 把更新包交给外部脚本替换 exe 并重启。
func (s *Service) applyUpdate(path string, quit func()) ActionResult {
	exe, err := os.Executable()
	if err != nil {
		return ActionResult{OK: false, Error: "获取当前程序路径失败: " + err.Error()}
	}
	if err := update.Apply(path, exe); err != nil {
		return ActionResult{OK: false, Error: "启动更新脚本失败: " + err.Error()}
	}
	s.appendLog("info", "更新已就绪，正在重启以完成安装")
	go func() {
		time.Sleep(600 * time.Millisecond)
		quit()
	}()
	return ActionResult{OK: true}
}

// decorateUpdateView 补上本地已下载状态。
func (s *Service) decorateUpdateView(view *UpdateView) {
	if view.Latest == "" {
		return
	}
	if _, ok := update.FindLocal(s.updatesDir(), view.Latest); ok {
		view.Downloaded = true
	}
}
