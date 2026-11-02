package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wengchengjian/skip-all/games"
	"github.com/wengchengjian/skip-all/pkg/config"
)

func TestServiceListGames(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	list := svc.ListGames()
	var zzz *GameView
	for i := range list {
		if list[i].ID == "zzz" {
			zzz = &list[i]
		}
	}
	if zzz == nil {
		t.Fatalf("games = %+v", list)
	}
	features := map[string]bool{}
	for _, feature := range zzz.Features {
		features[feature.ID] = true
	}
	if !features["story_skip"] || !features["auto_combat"] {
		t.Fatalf("features = %+v", zzz.Features)
	}
	if cfg := svc.GetConfig(); len(cfg.Games) != len(list) {
		t.Fatalf("每个游戏都应有一份配置: %+v", cfg.Games)
	}
	for _, game := range list {
		if _, err := os.Stat(filepath.Join(dir, "games", game.ID+".yaml")); err != nil {
			t.Fatalf("游戏 %s 缺少配置文件: %v", game.ID, err)
		}
	}
}

func TestServiceExcludedGames(t *testing.T) {
	svc, err := NewService(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	svc.mu.Lock()
	svc.appCfg.ExcludedGames = GameIDs()
	svc.mu.Unlock()
	if !svc.isExcluded("zzz") {
		t.Fatal("zzz 应被排除")
	}
	if svc.isExcluded("other") {
		t.Fatal("other 不应被排除")
	}
	if res := svc.StartAll(); !res.OK {
		t.Fatalf("全部排除时 StartAll 不应失败: %+v", res)
	}
	if st := svc.GetStatus(); st.Running || st.Paused {
		t.Fatalf("全部排除时不应有会话: %+v", st)
	}
}

func TestServiceConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	view := svc.GetConfig()
	view.App.Capture.TargetFPS = 15
	view.Games["zzz"] = config.GameConfig{
		SchemaVersion: config.SchemaVersion,
		Features: map[string]config.StorySkipConfig{
			"story_skip": {Enabled: false, Settings: map[string]any{"mode": "click"}},
		},
	}
	if err := svc.SaveConfig(view); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if _, err := filepath.Abs(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewService(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	defer loaded.Shutdown()
	if got := loaded.GetConfig().App.Capture.TargetFPS; got != 15 {
		t.Fatalf("TargetFPS = %d, want 15", got)
	}
	feature := loaded.GetConfig().Games["zzz"].Features["story_skip"]
	if feature.Enabled || feature.Settings["mode"] != "click" {
		t.Fatalf("zzz 游戏配置未落盘: %+v", feature)
	}
}

// TestEnabledFeaturesFollowConfig 验证"一个游戏可以同时启用多个功能"：
// 两个都启用时都返回，停用其中一个后只返回另一个。
func TestEnabledFeaturesFollowConfig(t *testing.T) {
	svc, err := NewService(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	game, ok := games.Get("zzz")
	if !ok {
		t.Fatal("缺少 zzz")
	}
	if got := svc.enabledFeatures(game); len(got) != 2 {
		t.Fatalf("默认应启用两个功能, got %d", len(got))
	}
	svc.mu.Lock()
	cfg := svc.gameConfigLocked("zzz")
	cfg.Features["auto_combat"] = config.StorySkipConfig{Enabled: false, Settings: map[string]any{}}
	svc.gameCfg["zzz"] = cfg
	svc.mu.Unlock()
	got := svc.enabledFeatures(game)
	if len(got) != 1 || got[0].ID() != "story_skip" {
		t.Fatalf("停用 auto_combat 后只剩 story_skip, got %+v", got)
	}
	if svc.featureEnabledLocked("zzz", "auto_combat") {
		t.Fatal("auto_combat 应被停用")
	}
	if !svc.featureEnabledLocked("zzz", "story_skip") {
		t.Fatal("story_skip 应仍启用")
	}
}

// TestStartRefusesDisabledFeature 验证单独启动一个已停用的功能会被拒绝。
func TestStartRefusesDisabledFeature(t *testing.T) {
	svc, err := NewService(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	svc.mu.Lock()
	cfg := svc.gameConfigLocked("zzz")
	cfg.Features["story_skip"] = config.StorySkipConfig{Enabled: false, Settings: map[string]any{}}
	svc.gameCfg["zzz"] = cfg
	svc.mu.Unlock()
	res := svc.Start("zzz", "story_skip")
	if res.OK || !strings.Contains(res.Error, "禁用") {
		t.Fatalf("已停用的功能不应启动: %+v", res)
	}
}

// TestGetStatusExposesFeatureStates 验证状态里能拿到每个功能的独立状态（UI 靠它显示徽标）。
func TestGetStatusExposesFeatureStates(t *testing.T) {
	svc, err := NewService(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	if st := svc.GetStatus(); st.Features == nil {
		t.Fatal("没有会话时 Features 也应为空数组而不是 nil")
	}
}
