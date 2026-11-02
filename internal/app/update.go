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

// StartAutoUpdateCheck 在启动后异步检查一次更新；失败只写日志，不阻塞主流程。
func (s *Service) StartAutoUpdateCheck() {
	s.mu.Lock()
	enabled := s.appCfg.Update.AutoCheck
	s.mu.Unlock()
	if !enabled {
		return
	}
	go func() {
		time.Sleep(4 * time.Second)
		if _, err := s.CheckForUpdates(); err != nil {
			s.appendLog("info", "检查更新失败: "+err.Error())
		}
	}()
}

// GetUpdateInfo 返回最近一次检查结果。
func (s *Service) GetUpdateInfo() UpdateView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return toUpdateView(s.updateInfo)
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
	s.mu.Lock()
	s.updateInfo = info
	s.mu.Unlock()
	if info.Available {
		s.appendLog("info", fmt.Sprintf("发现新版本 %s（当前 %s）", info.Latest, current))
		s.emit("skipall:update", view)
	}
	return view, nil
}

// InstallUpdate 下载并校验最新版本，随后交给外部脚本替换 exe 并重启。
func (s *Service) InstallUpdate() ActionResult {
	s.mu.Lock()
	quit := s.quit
	s.mu.Unlock()
	if quit == nil {
		return ActionResult{OK: false, Error: "当前环境不支持自动重启"}
	}
	view, err := s.CheckForUpdates()
	if err != nil {
		return ActionResult{OK: false, Error: "检查更新失败: " + err.Error()}
	}
	if !view.Available {
		return ActionResult{OK: false, Error: "当前已是最新版本"}
	}
	info := update.Info{
		Current: view.Current, Latest: view.Latest, Available: view.Available,
		Notes: view.Notes, URL: view.URL, AssetName: view.AssetName,
		AssetURL: view.AssetURL, AssetDigest: view.AssetDigest, AssetSize: view.AssetSize,
		ChecksumURL: view.ChecksumURL, PublishedAt: view.PublishedAt,
	}
	path, err := info.Download(context.Background(), nil, filepath.Join(s.dataDir, "updates"))
	if err != nil {
		return ActionResult{OK: false, Error: "下载更新失败: " + err.Error()}
	}
	exe, err := os.Executable()
	if err != nil {
		return ActionResult{OK: false, Error: "获取当前程序路径失败: " + err.Error()}
	}
	if err := update.Apply(path, exe); err != nil {
		return ActionResult{OK: false, Error: "启动更新脚本失败: " + err.Error()}
	}
	s.appendLog("info", "更新已下载，正在重启以完成安装")
	go func() {
		time.Sleep(600 * time.Millisecond)
		quit()
	}()
	return ActionResult{OK: true}
}
