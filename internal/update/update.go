package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Asset 是 GitHub release 的一个附件。
type Asset struct {
	Name        string
	DownloadURL string
	Digest      string
	Size        int64
}

// Info 是一次版本检查的结果。
type Info struct {
	Current     string
	Latest      string
	Available   bool
	Notes       string
	URL         string
	AssetName   string
	AssetURL    string
	AssetDigest string
	AssetSize   int64
	ChecksumURL string
	PublishedAt string
}

type release struct {
	TagName     string
	Name        string
	Body        string
	HTMLURL     string
	PublishedAt time.Time
	Assets      []Asset
}

// Checker 查询一个 GitHub 仓库的 latest release。
type Checker struct {
	Repo    string
	APIBase string
	Client  *http.Client
	GOOS    string
	GOARCH  string
}

// NewChecker 创建默认的 Windows/amd64 release 检查器。
func NewChecker(repo string) *Checker {
	return &Checker{Repo: repo, APIBase: "https://api.github.com", Client: &http.Client{Timeout: 12 * time.Second}, GOOS: "windows", GOARCH: "amd64"}
}

// Check 查询 latest release，并挑选 Windows 可执行文件。
func (c *Checker) Check(ctx context.Context, current string) (Info, error) {
	if c == nil || c.Repo == "" {
		return Info{Current: current}, errors.New("未配置更新仓库")
	}
	base := strings.TrimRight(c.APIBase, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	url := base + "/repos/" + strings.Trim(c.Repo, "/") + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Info{Current: current}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "skip-all-updater")
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Info{Current: current}, fmt.Errorf("查询 GitHub release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Info{Current: current}, errors.New("仓库没有发布版本")
	}
	if resp.StatusCode != http.StatusOK {
		return Info{Current: current}, fmt.Errorf("GitHub release 返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Info{Current: current}, err
	}
	rel, err := decodeRelease(data)
	if err != nil {
		return Info{Current: current}, err
	}
	info := Info{Current: current, Latest: strings.TrimPrefix(rel.TagName, "v"), Notes: rel.Body, URL: rel.HTMLURL, PublishedAt: rel.PublishedAt.Format(time.RFC3339)}
	asset, checksum := findAssets(rel.Assets, c.GOOS, c.GOARCH)
	if asset == nil {
		return info, fmt.Errorf("release %s 没有 %s/%s 可执行文件", rel.TagName, c.GOOS, c.GOARCH)
	}
	info.AssetName = asset.Name
	info.AssetURL = asset.DownloadURL
	info.AssetDigest = asset.Digest
	info.AssetSize = asset.Size
	if checksum != nil {
		info.ChecksumURL = checksum.DownloadURL
	}
	info.Available = compareVersions(current, info.Latest) < 0
	return info, nil
}

func decodeRelease(data []byte) (release, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return release{}, fmt.Errorf("解析 GitHub release: %w", err)
	}
	var out release
	out.TagName = rawString(raw["tag_name"])
	out.Name = rawString(raw["name"])
	out.Body = rawString(raw["body"])
	out.HTMLURL = rawString(raw["html_url"])
	if t := rawString(raw["published_at"]); t != "" {
		out.PublishedAt, _ = time.Parse(time.RFC3339, t)
	}
	var assets []map[string]json.RawMessage
	if b := raw["assets"]; len(b) > 0 {
		if err := json.Unmarshal(b, &assets); err != nil {
			return release{}, fmt.Errorf("解析 release assets: %w", err)
		}
	}
	for _, a := range assets {
		out.Assets = append(out.Assets, Asset{
			Name:        rawString(a["name"]),
			DownloadURL: rawString(a["browser_download_url"]),
			Digest:      rawString(a["digest"]),
			Size:        rawInt64(a["size"]),
		})
	}
	return out, nil
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func rawInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0
	}
	return n
}

func findAssets(assets []Asset, goos, goarch string) (*Asset, *Asset) {
	var exe, checksum *Asset
	for i := range assets {
		a := &assets[i]
		name := strings.ToLower(a.Name)
		if name == "checksums.txt" || strings.HasSuffix(name, "-checksums.txt") {
			checksum = a
			continue
		}
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, strings.ToLower(goos)) && strings.Contains(name, strings.ToLower(goarch)) {
			exe = a
		}
	}
	return exe, checksum
}

// Download 下载并校验 Info 指向的 Windows 可执行文件。
func (i Info) Download(ctx context.Context, client *http.Client, dir string) (string, error) {
	if i.AssetURL == "" {
		return "", errors.New("没有可下载的更新附件")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建更新目录: %w", err)
	}
	expected := strings.TrimPrefix(i.AssetDigest, "sha256:")
	if expected == "" && i.ChecksumURL != "" {
		sum, err := fetchChecksum(ctx, client, i.ChecksumURL, i.AssetName)
		if err != nil {
			return "", err
		}
		expected = sum
	}
	if expected == "" {
		return "", errors.New("release 没有提供 sha256 校验值")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.AssetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "skip-all-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载更新: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载更新返回 HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp(dir, "skip-all-update-*.exe")
	if err != nil {
		return "", fmt.Errorf("创建更新文件: %w", err)
	}
	path := f.Name()
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 300<<20))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("写入更新文件: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("关闭更新文件: %w", closeErr)
	}
	if i.AssetSize > 0 && n != i.AssetSize {
		_ = os.Remove(path)
		return "", fmt.Errorf("更新文件大小不符: got %d, want %d", n, i.AssetSize)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		_ = os.Remove(path)
		return "", fmt.Errorf("更新文件 sha256 不符: got %s, want %s", got, expected)
	}
	return path, nil
}

func fetchChecksum(ctx context.Context, client *http.Client, url, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "skip-all-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载 checksums.txt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 checksums.txt 返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksums.txt 找不到 %s", assetName)
}

func compareVersions(current, latest string) int {
	a, aok := parseVersion(current)
	b, bok := parseVersion(latest)
	if !aok || !bok {
		return 0
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V"))
	if v == "" || strings.EqualFold(v, "dev") {
		return nil, false
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out, len(out) > 0
}
