package vision

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"io/fs"
	"math"
	"path"
	"sync"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

const (
	// TemplateRawFile 是模板原图文件名。
	TemplateRawFile = "raw.png"
	// TemplateMaskFile 是模板掩码文件名。
	TemplateMaskFile = "mask.png"
)

// TemplateStoreFS 从 fs.FS 的 root/<sub_dir>/<template_id>/ 目录读取模板图片。
type TemplateStoreFS struct {
	fsys  fs.FS
	root  string
	mu    sync.Mutex
	cache map[string]*templateImages
}

type templateImages struct {
	raw  image.Image
	mask image.Image
}

// NewTemplateStoreFS 创建基于文件系统的模板仓库。root 可为空，表示 fs 根目录。
func NewTemplateStoreFS(fsys fs.FS, root string) TemplateStore {
	return &TemplateStoreFS{fsys: fsys, root: root, cache: make(map[string]*templateImages)}
}

// Load 读取模板的 raw.png 与 mask.png；mask.png 不存在时返回 nil 掩码。
func (s *TemplateStoreFS) Load(ref TemplateRef) (image.Image, image.Image, error) {
	if s == nil || s.fsys == nil {
		return nil, nil, errors.New("模板仓库未初始化")
	}
	if ref.ID == "" {
		return nil, nil, errors.New("模板 ID 为空")
	}
	key := path.Join(s.root, ref.SubDir, ref.ID)
	s.mu.Lock()
	if cached, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return cached.raw, cached.mask, nil
	}
	s.mu.Unlock()

	raw, err := decodeImageFS(s.fsys, path.Join(key, TemplateRawFile))
	if err != nil {
		return nil, nil, fmt.Errorf("读取模板 %s: %w", key, err)
	}
	mask, err := decodeImageFS(s.fsys, path.Join(key, TemplateMaskFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("读取模板掩码 %s: %w", key, err)
	}
	imgs := &templateImages{raw: raw, mask: mask}
	s.mu.Lock()
	if s.cache == nil {
		s.cache = make(map[string]*templateImages)
	}
	s.cache[key] = imgs
	s.mu.Unlock()
	return raw, mask, nil
}

func decodeImageFS(fsys fs.FS, name string) (image.Image, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// WhiteMaskRange 是白色掩膜的 RGB 范围，与上游 convert_dialog_icon 一致。
var WhiteMaskRange = [2][3]uint8{{210, 210, 210}, {255, 255, 255}}

// WhiteMask 把图像二值化成白掩膜：RGB 都落在 WhiteMaskRange 内为白，否则为黑。
// 上游用它把"发亮图标 vs 任意背景"压成同一张剪影，抗蓝/灰染色。
func WhiteMask(img image.Image) *image.RGBA {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	pr := newPixelReader(img)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			rr, gg, bb := pr.at(b.Min.X+x, b.Min.Y+y)
			if rr >= WhiteMaskRange[0][0] && rr <= WhiteMaskRange[1][0] &&
				gg >= WhiteMaskRange[0][1] && gg <= WhiteMaskRange[1][1] &&
				bb >= WhiteMaskRange[0][2] && bb <= WhiteMaskRange[1][2] {
				out.SetRGBA(x, y, colorRGBA(255))
			} else {
				out.SetRGBA(x, y, colorRGBA(0))
			}
		}
	}
	return out
}

func colorRGBA(v uint8) color.RGBA { return color.RGBA{v, v, v, 255} }

// MatchTemplateWhiteMask 先把源区域和模板都做白色掩膜再匹配。
func MatchTemplateWhiteMask(source image.Image, search geom.Rect, raw image.Image, threshold float64) ([]Match, error) {
	if source == nil || raw == nil {
		return nil, errors.New("模板匹配缺少图像")
	}
	r := image.Rect(search.X1, search.Y1, search.X2, search.Y2).Intersect(source.Bounds())
	if r.Empty() {
		return nil, nil
	}
	crop := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(crop, crop.Bounds(), source, r.Min, draw.Src)
	return MatchTemplate(WhiteMask(crop), geom.Rect{X1: 0, Y1: 0, X2: r.Dx(), Y2: r.Dy()}, WhiteMask(raw), nil, threshold)
}

// MatchTemplate 在 source 的 search 区域内做带 mask 的 TM_CCOEFF_NORMED 等价匹配。
// 返回置信度最高且不低于 threshold 的一个结果，坐标是 source 的绝对坐标。
func MatchTemplate(source image.Image, search geom.Rect, raw, mask image.Image, threshold float64) ([]Match, error) {
	if source == nil || raw == nil {
		return nil, errors.New("模板匹配缺少图像")
	}
	if mask != nil && mask.Bounds().Size() != raw.Bounds().Size() {
		return nil, errors.New("模板掩码尺寸与模板不一致")
	}
	searchRect := image.Rect(search.X1, search.Y1, search.X2, search.Y2).Intersect(source.Bounds())
	if searchRect.Empty() {
		return nil, nil
	}
	tw, th := raw.Bounds().Dx(), raw.Bounds().Dy()
	if tw <= 0 || th <= 0 || tw > searchRect.Dx() || th > searchRect.Dy() {
		return nil, nil
	}

	// 模板掩码内像素的灰度值与相对坐标。
	points := make([]templatePoint, 0, tw*th)
	var sumT, sumT2 float64
	rb := raw.Bounds()
	prRaw := newPixelReader(raw)
	var prMask pixelReader
	if mask != nil {
		prMask = newPixelReader(mask)
	}
	for dy := 0; dy < th; dy++ {
		for dx := 0; dx < tw; dx++ {
			if mask != nil && grayValueAt(prMask, mask.Bounds().Min.X+dx, mask.Bounds().Min.Y+dy) <= 0 {
				continue
			}
			v := grayValueAt(prRaw, rb.Min.X+dx, rb.Min.Y+dy)
			points = append(points, templatePoint{dx: dx, dy: dy, v: v})
			sumT += v
			sumT2 += v * v
		}
	}
	if len(points) == 0 {
		return nil, errors.New("模板掩码没有有效像素")
	}
	k := float64(len(points))
	meanT := sumT / k
	varT := sumT2/k - meanT*meanT
	if varT <= 1e-9 {
		return nil, nil
	}

	sw, sh := searchRect.Dx(), searchRect.Dy()
	gray := make([]float64, sw*sh)
	prSrc := newPixelReader(source)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			gray[y*sw+x] = grayValueAt(prSrc, searchRect.Min.X+x, searchRect.Min.Y+y)
		}
	}

	bestScore := math.Inf(-1)
	bestX, bestY := 0, 0
	for y := 0; y <= sh-th; y++ {
		for x := 0; x <= sw-tw; x++ {
			var sumI, sumI2, sumIT float64
			for _, p := range points {
				v := gray[(y+p.dy)*sw+x+p.dx]
				sumI += v
				sumI2 += v * v
				sumIT += v * p.v
			}
			meanI := sumI / k
			varI := sumI2/k - meanI*meanI
			if varI <= 1e-9 {
				continue
			}
			score := (sumIT/k - meanT*meanI) / math.Sqrt(varT*varI)
			score = math.Max(-1, math.Min(1, score))
			if score > bestScore {
				bestScore = score
				bestX, bestY = x, y
			}
		}
	}
	if math.IsInf(bestScore, -1) || bestScore < threshold {
		return nil, nil
	}
	return []Match{{
		Rect:       geom.Rect{X1: searchRect.Min.X + bestX, Y1: searchRect.Min.Y + bestY, X2: searchRect.Min.X + bestX + tw, Y2: searchRect.Min.Y + bestY + th},
		Confidence: bestScore,
	}}, nil
}

type templatePoint struct {
	dx int
	dy int
	v  float64
}

func grayValueAt(pr pixelReader, x, y int) float64 {
	r, g, b := pr.at(x, y)
	return 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
}
