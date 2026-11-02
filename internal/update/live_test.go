//go:build manual

package update

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLiveRelease 对真实 GitHub 仓库做一次检查与下载；需要网络。
// 运行：SKIP_ALL_UPDATE_LIVE=1 go test -tags manual ./internal/update -run TestLiveRelease -v
func TestLiveRelease(t *testing.T) {
	if os.Getenv("SKIP_ALL_UPDATE_LIVE") == "" {
		t.Skip("未设置 SKIP_ALL_UPDATE_LIVE")
	}
	repo := os.Getenv("SKIP_ALL_UPDATE_REPO")
	if repo == "" {
		repo = "wengchengjian/skip-all"
	}
	c := NewChecker(repo)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	info, err := c.Check(ctx, "0.1.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	t.Logf("latest=%s available=%v asset=%s size=%d digest=%s", info.Latest, info.Available, info.AssetName, info.AssetSize, info.AssetDigest)
	if !info.Available || info.AssetName == "" {
		t.Fatalf("info = %+v", info)
	}
	if os.Getenv("SKIP_ALL_UPDATE_DOWNLOAD") == "" {
		return
	}
	dir := t.TempDir()
	// 预先写入 1/4 内容，验证 Range 断点续传真的从断点继续。
	if os.Getenv("SKIP_ALL_UPDATE_RESUME") != "" {
		prefix := int64(info.AssetSize) / 4
		resp, err := http.Get(info.AssetURL)
		if err != nil {
			t.Fatalf("prefetch: %v", err)
		}
		part := filepath.Join(dir, info.AssetName+".part")
		f, err := os.Create(part)
		if err != nil {
			resp.Body.Close()
			t.Fatalf("create part: %v", err)
		}
		n, err := io.Copy(f, io.LimitReader(resp.Body, prefix))
		resp.Body.Close()
		f.Close()
		if err != nil || n != prefix {
			t.Fatalf("prefetch bytes: n=%d err=%v", n, err)
		}
		t.Logf("预置断点 %d 字节", n)
	}
	path, err := info.Download(ctx, nil, dir)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != info.AssetSize {
		t.Fatalf("size = %d, want %d", fi.Size(), info.AssetSize)
	}
	t.Logf("downloaded %s (%d bytes)", path, fi.Size())
}
