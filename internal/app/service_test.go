package app

import (
	"os"
	"path/filepath"
	"testing"

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
