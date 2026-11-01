package vision

import (
	"errors"
	"image"
	"image/draw"
	"sync"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

// Service 是 Matcher 的默认实现，组合 OCR 引擎与模板仓库。
type Service struct {
	ocr   OCR
	store TemplateStore

	mu    sync.Mutex
	frame *image.RGBA
	cache map[ocrCacheKey][]TextBlock
}

type ocrCacheKey struct {
	rect      geom.Rect
	hasColor  bool
	colorLow  [3]uint8
	colorHigh [3]uint8
	minScore  float64
}

// NewMatcher 创建视觉服务。ocr 为 nil 时仍可使用颜色与模板能力。
func NewMatcher(ocr OCR, store TemplateStore) Matcher {
	return &Service{ocr: ocr, store: store, cache: make(map[ocrCacheKey][]TextBlock)}
}

// OCR 识别区域内的文本；同一帧同一区域的重复请求会命中缓存。
func (s *Service) OCR(frame *image.RGBA, area screen.Area, opts OCROptions) ([]TextBlock, error) {
	if frame == nil {
		return nil, errors.New("空帧")
	}
	if s.ocr == nil {
		return nil, errors.New("OCR 引擎未初始化")
	}
	search := resolveRect(area.Rect, frame.Bounds())
	if search.Empty() {
		return nil, nil
	}
	colorRange := opts.ColorRange
	if colorRange == nil {
		colorRange = area.ColorRange
	}
	key := ocrCacheKey{rect: geom.Rect{X1: search.Min.X, Y1: search.Min.Y, X2: search.Max.X, Y2: search.Max.Y}, minScore: opts.MinScore}
	if colorRange != nil {
		key.hasColor = true
		key.colorLow = colorRange[0]
		key.colorHigh = colorRange[1]
	}

	s.mu.Lock()
	if s.frame != frame {
		s.frame = frame
		s.cache = make(map[ocrCacheKey][]TextBlock)
	}
	if blocks, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return append([]TextBlock(nil), blocks...), nil
	}
	s.mu.Unlock()

	crop, origin := cropRGBA(frame, search)
	var input image.Image = crop
	if colorRange != nil {
		input = filterByColorRange(crop, *colorRange)
	}
	engineOpts := opts
	engineOpts.ColorRange = nil
	blocks, err := s.ocr.Recognize(input, engineOpts)
	for i := range blocks {
		blocks[i].Rect = offsetRect(blocks[i].Rect, origin.X, origin.Y)
	}
	if opts.MinScore > 0 {
		filtered := blocks[:0]
		for _, block := range blocks {
			if block.Confidence >= opts.MinScore {
				filtered = append(filtered, block)
			}
		}
		blocks = filtered
	}
	if err != nil {
		return append([]TextBlock(nil), blocks...), err
	}

	s.mu.Lock()
	if s.frame == frame {
		s.cache[key] = append([]TextBlock(nil), blocks...)
	}
	s.mu.Unlock()
	return append([]TextBlock(nil), blocks...), nil
}

// Template 在区域内部做模板匹配，返回的坐标是整帧绝对坐标。
func (s *Service) Template(frame *image.RGBA, area screen.Area, ref TemplateRef, threshold float64) ([]Match, error) {
	if frame == nil {
		return nil, errors.New("空帧")
	}
	if s.store == nil {
		return nil, errors.New("模板仓库未初始化")
	}
	if threshold <= 0 {
		threshold = area.TemplateMatchThreshold
	}
	if threshold <= 0 {
		threshold = 0.7
	}
	raw, mask, err := s.store.Load(ref)
	if err != nil {
		return nil, err
	}
	search := resolveRect(area.Rect, frame.Bounds())
	rect := geom.Rect{X1: search.Min.X, Y1: search.Min.Y, X2: search.Max.X, Y2: search.Max.Y}
	scales := area.TemplateScales
	if len(scales) == 0 {
		scales = []float64{1}
	}
	var best []Match
	for _, scale := range scales {
		cRaw, cMask := raw, mask
		if scale > 0 && scale != 1 {
			cRaw = ScaleTemplate(raw, scale)
			if mask != nil {
				cMask = ScaleTemplate(mask, scale)
			}
		}
		var matches []Match
		var err error
		if area.TemplateWhiteMask {
			matches, err = MatchTemplateWhiteMask(frame, rect, cRaw, threshold)
			for i := range matches {
				matches[i].Rect = offsetRect(matches[i].Rect, search.Min.X, search.Min.Y)
			}
		} else {
			matches, err = MatchTemplate(frame, rect, cRaw, cMask, threshold)
		}
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 && (len(best) == 0 || matches[0].Confidence > best[0].Confidence) {
			best = matches
		}
	}
	return best, nil
}

// Color 统计区域内的颜色信息。
func (s *Service) Color(frame *image.RGBA, area screen.Area) (ColorStats, error) {
	if frame == nil {
		return ColorStats{}, errors.New("空帧")
	}
	rect := area.Rect
	if rect.Empty() {
		b := frame.Bounds()
		rect = geom.Rect{X1: b.Min.X, Y1: b.Min.Y, X2: b.Max.X, Y2: b.Max.Y}
	}
	return AnalyzeColor(frame, rect), nil
}

func resolveRect(rect geom.Rect, bounds image.Rectangle) image.Rectangle {
	if rect.Empty() {
		return bounds
	}
	return image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(bounds)
}

func cropRGBA(img *image.RGBA, rect image.Rectangle) (*image.RGBA, image.Point) {
	out := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(out, out.Bounds(), img, rect.Min, draw.Src)
	return out, rect.Min
}

func offsetRect(rect geom.Rect, dx, dy int) geom.Rect {
	return geom.Rect{X1: rect.X1 + dx, Y1: rect.Y1 + dy, X2: rect.X2 + dx, Y2: rect.Y2 + dy}
}

var _ Matcher = (*Service)(nil)
