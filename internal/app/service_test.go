package app

import (
	"path/filepath"
	"testing"
)

func TestServiceListGames(t *testing.T) {
	svc, err := NewService(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()
	games := svc.ListGames()
	if len(games) != 1 || games[0].ID != "zzz" {
		t.Fatalf("games = %+v", games)
	}
	if len(games[0].Features) != 1 || games[0].Features[0].ID != "story_skip" {
		t.Fatalf("features = %+v", games[0].Features)
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
}
