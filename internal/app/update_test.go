package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wengchengjian/skip-all/internal/update"
)

func TestInstallUpdatePrefersNewestLocal(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(Options{DataDir: dir, Quit: func() {}})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()

	// 本地缓存了多个版本，InstallUpdate 应挑版本号最大的那个。
	updates := filepath.Join(dir, "updates")
	for _, v := range []string{"0.1.1", "0.1.3", "0.1.2"} {
		sub := filepath.Join(updates, v)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "skip-all-windows-amd64.exe"), []byte("bin-"+v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	view := svc.GetUpdateInfo()
	if view.Downloaded {
		t.Fatal("未检查前不应报告已下载")
	}
	if local, ok := update.PickLocal(updates, Version); !ok || local.Version != "0.1.3" {
		t.Fatalf("PickLocal = %+v, ok=%v", local, ok)
	}
}

func TestGetUpdateInfoMarksDownloaded(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(Options{DataDir: dir})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Shutdown()

	svc.mu.Lock()
	svc.updateInfo = update.Info{Current: Version, Latest: "9.9.9", Available: true}
	svc.mu.Unlock()
	sub := filepath.Join(dir, "updates", "9.9.9")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "skip-all-windows-amd64.exe"), []byte("bin"), 0o600); err != nil {
		t.Fatal(err)
	}
	if view := svc.GetUpdateInfo(); !view.Downloaded || view.Latest != "9.9.9" {
		t.Fatalf("view = %+v", view)
	}
}
