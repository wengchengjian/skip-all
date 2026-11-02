package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

// NewChecker 创建 release 检查器；平台默认取当前运行环境。
func NewChecker(repo string) *Checker {
	return &Checker{Repo: repo, APIBase: "https://api.github.com", Client: &http.Client{Timeout: 12 * time.Second}, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
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
	goos, goarch := c.GOOS, c.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	asset, checksum := findAssets(rel.Assets, goos, goarch)
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

// Download 下载并校验 Info 指向的当前平台可执行文件。
func (i Info) Download(ctx context.Context, client *http.Client, dir string) (string, error) {
	return i.DownloadWithProgress(ctx, client, dir, nil)
}

// LocalAsset 是本地已下载完成的更新文件。
type LocalAsset struct {
	Path    string
	Version string
}

// localDir 返回某个版本的缓存目录：多个版本各占一个子目录，避免互相覆盖。
func localDir(dir, version string) string {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if v == "" {
		v = "unknown"
	}
	return filepath.Join(dir, v)
}

// assetFile 返回版本目录下的可执行文件名。
func assetFile(assetName string) string {
	name := filepath.Base(assetName)
	if name == "" || name == "." {
		return "skip-all-update.exe"
	}
	return name
}

// FindLocal 返回指定版本已下载完成的文件；缺失时返回 false。
func FindLocal(dir, version string) (LocalAsset, bool) {
	entries, err := os.ReadDir(localDir(dir, version))
	if err != nil {
		return LocalAsset{}, false
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".part") {
			continue
		}
		path := filepath.Join(localDir(dir, version), e.Name())
		fi, err := e.Info()
		if err != nil || fi.Size() == 0 {
			continue
		}
		return LocalAsset{Path: path, Version: strings.TrimPrefix(strings.TrimSpace(version), "v")}, true
	}
	return LocalAsset{}, false
}

// PickLocal 返回本地已下载、且比 current 新的最新版本。
// 本地存在多个版本时取版本号最大的那个。
func PickLocal(dir, current string) (LocalAsset, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return LocalAsset{}, false
	}
	var best LocalAsset
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		version := e.Name()
		if compareVersions(current, version) >= 0 {
			continue
		}
		asset, ok := FindLocal(dir, version)
		if !ok {
			continue
		}
		if best.Path == "" || compareVersions(best.Version, version) < 0 {
			best = asset
		}
	}
	return best, best.Path != ""
}

// CleanLocal 清理本地更新缓存，返回释放的字节数。
// installed 是当前运行版本，不高于它的缓存都已装过，直接删除；
// keep 是刚下载的版本（可为空），比它旧的版本目录同样删除，只留最新的。
func CleanLocal(dir, installed, keep string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	var freed int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		version := e.Name()
		stale := compareVersions(installed, version) >= 0
		if !stale && keep != "" && compareVersions(version, keep) < 0 {
			stale = true
		}
		if !stale {
			continue
		}
		path := filepath.Join(dir, version)
		size := dirSize(path)
		if err := os.RemoveAll(path); err != nil {
			return freed, err
		}
		freed += size
	}
	return freed, nil
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// DownloadWithProgress 与 Download 相同，但会按已下载字节回调进度。
// 下载使用固定 .part 文件 + Range 断点续传；校验失败会删除，网络中断则保留。
func (i Info) DownloadWithProgress(ctx context.Context, client *http.Client, dir string, progress func(done, total int64)) (string, error) {
	if i.AssetURL == "" {
		return "", errors.New("没有可下载的更新附件")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	dir = localDir(dir, i.Latest)
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
	name := assetFile(i.AssetName)
	part := filepath.Join(dir, name+".part")
	offset := int64(0)
	if fi, err := os.Stat(part); err == nil {
		offset = fi.Size()
		if i.AssetSize > 0 && offset > i.AssetSize {
			_ = os.Remove(part)
			offset = 0
		} else if i.AssetSize > 0 && offset == i.AssetSize {
			if ok, _ := verifyFile(part, expected, i.AssetSize); ok {
				return finishPart(part, dir, name)
			}
			_ = os.Remove(part)
			offset = 0
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.AssetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "skip-all-updater")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载更新: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
		// 从 offset 继续写。
	case http.StatusOK:
		offset = 0
	case http.StatusRequestedRangeNotSatisfiable:
		if ok, _ := verifyFile(part, expected, i.AssetSize); ok {
			return finishPart(part, dir, name)
		}
		_ = os.Remove(part)
		return "", errors.New("断点续传范围无效，已清理临时文件")
	default:
		return "", fmt.Errorf("下载更新返回 HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("创建更新文件: %w", err)
	}
	if offset == 0 {
		if err := f.Truncate(0); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("清理更新文件: %w", err)
		}
	} else if _, err := f.Seek(offset, io.SeekStart); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("定位更新文件: %w", err)
	}
	h := sha256.New()
	if offset > 0 {
		existing, err := os.Open(part)
		if err != nil {
			_ = f.Close()
			return "", fmt.Errorf("读取断点文件: %w", err)
		}
		_, hashErr := io.Copy(h, io.LimitReader(existing, offset))
		_ = existing.Close()
		if hashErr != nil {
			_ = f.Close()
			return "", fmt.Errorf("计算断点 sha256: %w", hashErr)
		}
	}
	writer := io.MultiWriter(f, h)
	if progress != nil && i.AssetSize > 0 {
		progress(offset, i.AssetSize)
		writer = io.MultiWriter(f, h, &progressWriter{fn: progress, done: offset, total: i.AssetSize})
	}
	n, copyErr := io.Copy(writer, io.LimitReader(resp.Body, 300<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("写入更新文件: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("关闭更新文件: %w", closeErr)
	}
	total := offset + n
	if i.AssetSize > 0 && total > i.AssetSize {
		_ = os.Remove(part)
		return "", fmt.Errorf("更新文件大小超出: got %d, want %d", total, i.AssetSize)
	}
	if i.AssetSize > 0 && total != i.AssetSize {
		// 保留 part，下一次从 total 继续。
		return "", fmt.Errorf("更新文件未下载完整: got %d, want %d", total, i.AssetSize)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		_ = os.Remove(part)
		return "", fmt.Errorf("更新文件 sha256 不符: got %s, want %s", got, expected)
	}
	return finishPart(part, dir, name)
}

// finishPart 把校验通过的 .part 文件改名为最终文件，避免下次被当作未完成下载。
func finishPart(part, dir, name string) (string, error) {
	final := filepath.Join(dir, name)
	_ = os.Remove(final) // Windows 下 Rename 不能覆盖已存在的目标。
	if err := os.Rename(part, final); err != nil {
		return "", fmt.Errorf("保存更新文件: %w", err)
	}
	return final, nil
}

// progressWriter 把写入字节数换算成下载进度回调。
type progressWriter struct {
	fn    func(done, total int64)
	done  int64
	total int64
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.done > p.total {
		p.done = p.total
	}
	p.fn(p.done, p.total)
	return len(b), nil
}

func verifyFile(path, expected string, size int64) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if size > 0 && fi.Size() != size {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), expected), nil
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

// CompareVersions 比较两个版本号：a<b 返回 -1，相等 0，a>b 返回 1。
func CompareVersions(a, b string) int { return compareVersions(a, b) }

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
