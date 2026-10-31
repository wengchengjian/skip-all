package vision

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"sync"

	ocr "github.com/getcharzp/go-ocr"
	"github.com/getcharzp/go-ocr/paddle"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// ErrModelMissing 表示 OCR 模型文件缺失，可用 errors.Is 判断。
var ErrModelMissing = errors.New("OCR 模型缺失")

// PaddleOptions 是 PaddleOCR 引擎的可调参数，零值使用默认值。
type PaddleOptions struct {
	// DetMaxSideLen 是检测模型输入的最长边。
	DetMaxSideLen int
	// DetOutsideExpandPix 是检测框外扩像素。
	DetOutsideExpandPix int
	// RecHeight 是识别模型输入高度。
	RecHeight int
	// RecModelNumClasses 是识别模型类别数；PP-OCRv6 为 18709。
	RecModelNumClasses int64
	// ThreadCount 是识别 session 并发数。
	ThreadCount int
}

// DefaultPaddleOptions 返回 PaddleOCR 的默认参数。
func DefaultPaddleOptions() PaddleOptions {
	return PaddleOptions{
		DetMaxSideLen:       960,
		DetOutsideExpandPix: 10,
		RecHeight:           48,
		RecModelNumClasses:  18709,
		ThreadCount:         1,
	}
}

type paddleOCR struct {
	mu     sync.Mutex
	engine ocr.Engine
}

// NewPaddleOCR 使用 modelDir 下的 det.onnx、rec.onnx、字典和 onnxruntime 动态库创建 OCR 引擎。
// 模型文件缺失时返回包装了 ErrModelMissing 的错误。
func NewPaddleOCR(modelDir string) (OCR, error) {
	return NewPaddleOCRWithOptions(modelDir, DefaultPaddleOptions())
}

// NewPaddleOCRWithOptions 与 NewPaddleOCR 相同，可自定义引擎参数。
func NewPaddleOCRWithOptions(modelDir string, opts PaddleOptions) (OCR, error) {
	files, err := findPaddleFiles(modelDir)
	if err != nil {
		return nil, err
	}
	if opts.DetMaxSideLen <= 0 {
		opts.DetMaxSideLen = 960
	}
	if opts.DetOutsideExpandPix <= 0 {
		opts.DetOutsideExpandPix = 10
	}
	if opts.RecHeight <= 0 {
		opts.RecHeight = 48
	}
	if opts.RecModelNumClasses <= 0 {
		opts.RecModelNumClasses = 18709
	}
	if opts.ThreadCount <= 0 {
		opts.ThreadCount = 1
	}
	engine, err := paddle.NewEngine(paddle.Config{
		OnnxRuntimeLibPath:  files.onnxRuntime,
		DetModelPath:        files.det,
		RecModelPath:        files.rec,
		DictPath:            files.dict,
		DetMaxSideLen:       opts.DetMaxSideLen,
		DetOutsideExpandPix: opts.DetOutsideExpandPix,
		RecHeight:           opts.RecHeight,
		RecModelNumClasses:  opts.RecModelNumClasses,
		ThreadCount:         opts.ThreadCount,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 PaddleOCR 引擎: %w", err)
	}
	return &paddleOCR{engine: engine}, nil
}

// Recognize 识别整张图片中的文本。
func (p *paddleOCR) Recognize(img image.Image, opts OCROptions) (blocks []TextBlock, err error) {
	if img == nil {
		return nil, errors.New("OCR 输入为空")
	}
	defer func() {
		if r := recover(); r != nil {
			blocks = nil
			err = fmt.Errorf("PaddleOCR 运行失败: %v", r)
		}
	}()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.engine == nil {
		return nil, errors.New("PaddleOCR 引擎已关闭")
	}
	input := img
	if opts.ColorRange != nil {
		input = filterByColorRange(img, *opts.ColorRange)
	}
	results, runErr := p.engine.RunOCR(input)
	blocks = make([]TextBlock, 0, len(results))
	for _, r := range results {
		if opts.MinScore > 0 && float64(r.Score) < opts.MinScore {
			continue
		}
		blocks = append(blocks, TextBlock{
			Text:       r.Text,
			Rect:       geom.Rect{X1: r.Box[0], Y1: r.Box[1], X2: r.Box[2], Y2: r.Box[3]},
			Confidence: float64(r.Score),
		})
	}
	if runErr != nil {
		return blocks, fmt.Errorf("PaddleOCR 识别: %w", runErr)
	}
	return blocks, nil
}

// Close 释放 OCR 引擎占用的 ONNX 资源。
func (p *paddleOCR) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.engine != nil {
		p.engine.Destroy()
		p.engine = nil
	}
	return nil
}

type paddleFiles struct {
	det         string
	rec         string
	dict        string
	onnxRuntime string
}

// findPaddleFiles 在 modelDir 及其 unzipped 子目录中查找模型文件。
func findPaddleFiles(modelDir string) (paddleFiles, error) {
	var out paddleFiles
	if modelDir == "" {
		return out, fmt.Errorf("%w: 模型目录为空", ErrModelMissing)
	}
	roots := []string{modelDir, filepath.Join(modelDir, "unzipped")}
	find := func(names ...string) string {
		for _, root := range roots {
			for _, name := range names {
				p := filepath.Join(root, name)
				if info, err := os.Stat(p); err == nil && !info.IsDir() {
					return p
				}
			}
		}
		return ""
	}
	out.det = find("det.onnx")
	if out.det == "" {
		return out, fmt.Errorf("%w: %s", ErrModelMissing, filepath.Join(modelDir, "det.onnx"))
	}
	out.rec = find("rec.onnx")
	if out.rec == "" {
		return out, fmt.Errorf("%w: %s", ErrModelMissing, filepath.Join(modelDir, "rec.onnx"))
	}
	out.dict = find("ppocrv6_dict.txt")
	if out.dict == "" {
		out.dict = findDictFile(roots)
	}
	if out.dict == "" {
		return out, fmt.Errorf("%w: %s", ErrModelMissing, filepath.Join(modelDir, "ppocrv6_dict.txt"))
	}
	out.onnxRuntime = find("onnxruntime.dll", "onnxruntime.so", "libonnxruntime.so", "onnxruntime.dylib")
	if out.onnxRuntime == "" {
		return out, fmt.Errorf("%w: %s", ErrModelMissing, filepath.Join(modelDir, "onnxruntime.dll"))
	}
	return out, nil
}

func findDictFile(roots []string) string {
	for _, root := range roots {
		matches, err := filepath.Glob(filepath.Join(root, "*_dict.txt"))
		if err != nil || len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		return matches[0]
	}
	return ""
}

var _ OCR = (*paddleOCR)(nil)
