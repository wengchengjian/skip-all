// Package vision 提供 OCR、颜色和模板匹配能力。
package vision

import (
	"image"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// TextBlock 是一条 OCR 结果。
type TextBlock struct {
	Text       string
	Rect       geom.Rect
	Confidence float64
}

// Match 是一次模板匹配结果。
type Match struct {
	Rect       geom.Rect
	Confidence float64
}

// OCROptions 控制 OCR 预处理。
type OCROptions struct {
	ColorRange *[2][3]uint8
	MinScore   float64
}

// TemplateRef 指向一个模板资源。
type TemplateRef struct {
	SubDir string
	ID     string
}

// ColorStats 是区域颜色统计。
type ColorStats struct {
	GrayMaskPercent float64
	ColorfulPercent float64
	MeanSaturation  float64
}

// OCR 定义 OCR 引擎接口。
type OCR interface {
	Recognize(img image.Image, opts OCROptions) ([]TextBlock, error)
	Close() error
}

// TemplateStore 定义模板图片读取接口。
type TemplateStore interface {
	Load(ref TemplateRef) (image.Image, image.Image, error) // raw, mask
}

// Matcher 是供 Feature Runtime 使用的视觉服务接口。
type Matcher interface {
	OCR(frame *image.RGBA, area screen.Area, opts OCROptions) ([]TextBlock, error)
	Template(frame *image.RGBA, area screen.Area, ref TemplateRef, threshold float64) ([]Match, error)
	Color(frame *image.RGBA, area screen.Area) (ColorStats, error)
}
