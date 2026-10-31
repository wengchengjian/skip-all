// Package model 负责 OCR 模型文件的下载、解压与校验。
package model

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wengchengjian/skip-all/pkg/vision"
)

const (
	// OCRModelName 是默认 OCR 模型名称。
	OCRModelName = "ppocrv6"
	// OCRZipFileName 是模型压缩包文件名。
	OCRZipFileName = "ppocrv6.zip"
	// OCRZipURL 是模型压缩包下载地址。
	OCRZipURL = "https://github.com/OneDragon-Anything/OneDragon-Env/releases/download/ppocrv6/ppocrv6.zip"
	// OCRZipSHA256 是模型压缩包的 SHA256。
	OCRZipSHA256 = "ABE2AD83E8B684F4C905619E2271475D921683526DC3B6AB5448D8012F8E24C9"
	// OnnxRuntimeFileName 是 ONNX Runtime 动态库文件名。
	OnnxRuntimeFileName = "onnxruntime.dll"
	// OnnxRuntimeURL 是 onnxruntime.dll 的下载地址。
	OnnxRuntimeURL = "https://modelscope.cn/models/getcharzp/go-ocr/resolve/master/lib/onnxruntime.dll"
	// OnnxRuntimeSHA256 是 onnxruntime.dll 的 SHA256。
	OnnxRuntimeSHA256 = "DEC964AB1EE36CC9B0AE247D13B376627992FC57DEC0454354017AB8FD84F1EA"
)

// ProgressFunc 报告安装进度，percent 为 0-100。
type ProgressFunc func(percent float64, message string)

// DefaultDir 返回默认 OCR 模型目录。
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取用户配置目录: %w", err)
	}
	return filepath.Join(base, "skip-all", "models", OCRModelName), nil
}

// RequiredFiles 返回 OCR 模型运行需要的文件列表（字典文件按通配符匹配）。
func RequiredFiles() []string {
	return []string{"det.onnx", "rec.onnx", "*_dict.txt", OnnxRuntimeFileName}
}

// CheckOCR 检查 dir 下 OCR 模型是否完整，缺失文件时返回包装了 vision.ErrModelMissing 的错误。
func CheckOCR(dir string) error {
	for _, name := range []string{"det.onnx", "rec.onnx", OnnxRuntimeFileName} {
		if !isRegularFile(filepath.Join(dir, name)) {
			return fmt.Errorf("%w: %s", vision.ErrModelMissing, filepath.Join(dir, name))
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*_dict.txt"))
	if len(matches) == 0 {
		return fmt.Errorf("%w: %s", vision.ErrModelMissing, filepath.Join(dir, "*_dict.txt"))
	}
	return nil
}

// InstallOCR 下载并安装 OCR 模型到 dir。已完整的文件会跳过。
func InstallOCR(ctx context.Context, dir string, progress ProgressFunc) error {
	return installOCR(ctx, dir, OCRZipURL, OnnxRuntimeURL, OCRZipSHA256, OnnxRuntimeSHA256, progress)
}

func installOCR(ctx context.Context, dir, zipURL, dllURL, zipSHA, dllSHA string, progress ProgressFunc) error {
	if dir == "" {
		return errors.New("模型目录为空")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建模型目录 %s: %w", dir, err)
	}
	report(progress, 0, "开始安装 OCR 模型")

	if err := CheckOCR(dir); err != nil {
		zipPath := filepath.Join(dir, OCRZipFileName)
		if !fileMatchesSHA256(zipPath, zipSHA) {
			if err := DownloadFile(ctx, zipURL, zipPath, zipSHA, scaleProgress(progress, 0, 70, "下载 OCR 模型")); err != nil {
				return err
			}
		} else {
			report(progress, 70, "OCR 模型压缩包已存在")
		}
		if err := ExtractZip(zipPath, dir); err != nil {
			return fmt.Errorf("解压 OCR 模型: %w", err)
		}
		_ = os.Remove(zipPath)
	} else {
		report(progress, 70, "OCR 模型已存在")
	}

	if !fileMatchesSHA256(filepath.Join(dir, OnnxRuntimeFileName), dllSHA) {
		if err := DownloadFile(ctx, dllURL, filepath.Join(dir, OnnxRuntimeFileName), dllSHA, scaleProgress(progress, 70, 100, "下载 onnxruntime.dll")); err != nil {
			return err
		}
	} else {
		report(progress, 100, "onnxruntime.dll 已存在")
	}
	if err := CheckOCR(dir); err != nil {
		return fmt.Errorf("OCR 模型安装后校验失败: %w", err)
	}
	report(progress, 100, "OCR 模型安装完成")
	return nil
}

// DownloadFile 下载 url 到 dest，支持进度回调和 SHA256 校验；先写临时文件再替换。
func DownloadFile(ctx context.Context, url, dest, wantSHA256 string, progress ProgressFunc) error {
	if url == "" {
		return errors.New("下载地址为空")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("创建下载目录: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载 %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 %s: HTTP %d", url, resp.StatusCode)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时下载文件: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	hasher := sha256.New()
	total := resp.ContentLength
	var written int64
	lastPercent := -1.0
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := tmp.Write(buf[:n]); err != nil {
				tmp.Close()
				return fmt.Errorf("写入下载文件: %w", err)
			}
			hasher.Write(buf[:n])
			written += int64(n)
			if total > 0 {
				percent := float64(written) / float64(total) * 100
				if percent-lastPercent >= 1 {
					lastPercent = percent
					report(progress, percent, "下载中")
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmp.Close()
			return fmt.Errorf("读取下载内容: %w", readErr)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时下载文件: %w", err)
	}
	actual := strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
	if wantSHA256 != "" && actual != strings.ToUpper(wantSHA256) {
		return fmt.Errorf("SHA256 校验失败: 期望 %s 实际 %s", wantSHA256, actual)
	}
	if err := replaceFile(tmpName, dest); err != nil {
		return fmt.Errorf("保存下载文件 %s: %w", dest, err)
	}
	report(progress, 100, "下载完成")
	return nil
}

// ExtractZip 把 zip 解压到 destDir，拒绝包含 .. 或绝对路径的条目。
func ExtractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开压缩包 %s: %w", zipPath, err)
	}
	defer reader.Close()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建解压目录: %w", err)
	}
	for _, file := range reader.File {
		name := filepath.FromSlash(file.Name)
		if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "..") || filepath.VolumeName(name) != "" {
			return fmt.Errorf("压缩包包含非法路径: %s", file.Name)
		}
		target := filepath.Join(destDir, name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建目录 %s: %w", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建目录 %s: %w", filepath.Dir(target), err)
		}
		if err := extractZipFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(file *zip.File, target string) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("读取压缩包条目 %s: %w", file.Name, err)
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(target), ".extract-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时解压文件: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("解压 %s: %w", file.Name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时解压文件: %w", err)
	}
	if err := replaceFile(tmpName, target); err != nil {
		return fmt.Errorf("写入解压文件 %s: %w", target, err)
	}
	return nil
}

func replaceFile(tmpName, dest string) error {
	if err := os.Rename(tmpName, dest); err == nil {
		return nil
	}
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpName, dest)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fileMatchesSHA256(path, want string) bool {
	if want == "" {
		return isRegularFile(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), want)
}

func report(progress ProgressFunc, percent float64, message string) {
	if progress != nil {
		progress(percent, message)
	}
}

func scaleProgress(progress ProgressFunc, start, end float64, message string) ProgressFunc {
	if progress == nil {
		return nil
	}
	return func(percent float64, _ string) {
		if percent < 0 {
			percent = 0
		}
		progress(start+(end-start)*percent/100, message)
	}
}
