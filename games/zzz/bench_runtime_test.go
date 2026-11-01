package zzz

import (
	"image"
	"log/slog"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// benchRuntime 是最小的 combat.Runtime 实现：所有模板都认不出，OCR 返回空。
type benchRuntime struct {
	img *image.RGBA
}

func newBenchRuntime() *benchRuntime { return &benchRuntime{} }

func (b *benchRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	return nil, nil
}

func (b *benchRuntime) MatchTemplate(assist.Frame, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
	return nil, nil
}

func (b *benchRuntime) ColorStats(assist.Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}

func (b *benchRuntime) Log() *slog.Logger { return slog.Default() }
