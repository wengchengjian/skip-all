package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		current, latest string
		want            int
	}{
		{"0.1.0", "0.2.0", -1},
		{"v0.2.0", "0.2.0", 0},
		{"1.0.0", "0.9.9", 1},
		{"dev", "0.2.0", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.current, c.latest); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.current, c.latest, got, c.want)
		}
	}
}

func TestNewCheckerUsesRuntimePlatform(t *testing.T) {
	c := NewChecker("o/r")
	if c.GOOS != runtime.GOOS || c.GOARCH != runtime.GOARCH {
		t.Fatalf("platform = %s/%s, want %s/%s", c.GOOS, c.GOARCH, runtime.GOOS, runtime.GOARCH)
	}
}

func TestFindAssets(t *testing.T) {
	assets := []Asset{
		{Name: "skip-all-windows-amd64.exe"},
		{Name: "checksums.txt"},
		{Name: "skip-all-linux-amd64"},
	}
	exe, sum := findAssets(assets, "windows", "amd64")
	if exe == nil || exe.Name != "skip-all-windows-amd64.exe" {
		t.Fatalf("exe = %+v", exe)
	}
	if sum == nil || sum.Name != "checksums.txt" {
		t.Fatalf("checksum = %+v", sum)
	}
}

func TestCheckRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/releases/latest" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","body":"notes","html_url":"https://example/release","published_at":"2026-01-02T03:04:05Z","assets":[{"name":"skip-all-windows-amd64.exe","browser_download_url":"https://example/exe","size":12},{"name":"checksums.txt","browser_download_url":"https://example/sum"}]}`))
	}))
	defer srv.Close()
	c := NewChecker("o/r")
	c.APIBase = srv.URL
	info, err := c.Check(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "0.2.0" || info.AssetName != "skip-all-windows-amd64.exe" || info.ChecksumURL == "" {
		t.Fatalf("info = %+v", info)
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	data := []byte("new binary")
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	mux.HandleFunc("/exe", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) })
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sumHex + "  skip-all-windows-amd64.exe\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	info := Info{Latest: "0.2.0", AssetName: "skip-all-windows-amd64.exe", AssetURL: srv.URL + "/exe", AssetSize: int64(len(data)), ChecksumURL: srv.URL + "/checksums.txt"}
	path, err := info.Download(context.Background(), srv.Client(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("download = %q", got)
	}
	if filepath.Base(path) == "" {
		t.Fatal("empty path")
	}
	if filepath.Base(filepath.Dir(path)) != "0.2.0" {
		t.Fatalf("download dir = %s, want version subdir", path)
	}
}

func TestPickLocalChoosesNewestVersion(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"0.1.2", "0.1.3", "0.2.0"} {
		sub := filepath.Join(dir, v)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "skip-all-windows-amd64.exe"), []byte("bin-"+v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// 未完成的 .part 不应被选中。
	if err := os.WriteFile(filepath.Join(dir, "0.2.0", "skip-all-windows-amd64.exe.part"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok := PickLocal(dir, "0.1.1")
	if !ok || got.Version != "0.2.0" {
		t.Fatalf("PickLocal = %+v, ok=%v", got, ok)
	}
	if _, ok := PickLocal(dir, "0.2.0"); ok {
		t.Fatal("已是最新版本时不应返回本地更新")
	}
}

func TestCleanLocalKeepsOnlyNewest(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"0.1.1", "0.1.2", "0.1.3"} {
		sub := filepath.Join(dir, v)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "skip-all-windows-amd64.exe"), make([]byte, 1024), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// installed=0.1.1：0.1.1 已装过、0.1.2 比 keep=0.1.3 旧，两者都删。
	freed, err := CleanLocal(dir, "0.1.1", "0.1.3")
	if err != nil {
		t.Fatalf("CleanLocal: %v", err)
	}
	if freed != 2*1024 {
		t.Fatalf("freed = %d, want %d", freed, 2*1024)
	}
	if _, err := os.Stat(filepath.Join(dir, "0.1.3", "skip-all-windows-amd64.exe")); err != nil {
		t.Fatalf("最新版本应保留: %v", err)
	}
	for _, v := range []string{"0.1.1", "0.1.2"} {
		if _, err := os.Stat(filepath.Join(dir, v)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s 应被删除", v)
		}
	}
}

func TestCleanLocalMissingDir(t *testing.T) {
	freed, err := CleanLocal(filepath.Join(t.TempDir(), "none"), "0.1.0", "")
	if err != nil || freed != 0 {
		t.Fatalf("freed=%d err=%v", freed, err)
	}
}

func TestDownloadResumesPartialFile(t *testing.T) {
	data := []byte("0123456789abcdef")
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	mux.HandleFunc("/exe", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=4-" {
			t.Errorf("Range = %q, want bytes=4-", r.Header.Get("Range"))
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[4:])
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sumHex + "  skip-all-windows-amd64.exe\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	dir := t.TempDir()
	part := filepath.Join(dir, "0.2.0", "skip-all-windows-amd64.exe.part")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, data[:4], 0o600); err != nil {
		t.Fatal(err)
	}
	info := Info{Latest: "0.2.0", AssetName: "skip-all-windows-amd64.exe", AssetURL: srv.URL + "/exe", AssetSize: int64(len(data)), ChecksumURL: srv.URL + "/checksums.txt"}
	path, err := info.Download(context.Background(), srv.Client(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("resumed download = %q", got)
	}
}
