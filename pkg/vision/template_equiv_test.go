package vision

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// refMatchTemplate 是滑动窗口优化前的朴素实现，用作等价性对照。
func refMatchTemplate(source image.Image, search geom.Rect, raw, mask image.Image, threshold float64) ([]Match, error) {
	searchRect := image.Rect(search.X1, search.Y1, search.X2, search.Y2).Intersect(source.Bounds())
	if searchRect.Empty() {
		return nil, nil
	}
	tw, th := raw.Bounds().Dx(), raw.Bounds().Dy()
	if tw <= 0 || th <= 0 || tw > searchRect.Dx() || th > searchRect.Dy() {
		return nil, nil
	}
	type pt struct {
		dx, dy int
		v      float64
	}
	var points []pt
	var sumT, sumT2 float64
	rb := raw.Bounds()
	for dy := 0; dy < th; dy++ {
		for dx := 0; dx < tw; dx++ {
			if mask != nil && grayValueAt(newPixelReader(mask), mask.Bounds().Min.X+dx, mask.Bounds().Min.Y+dy) <= 0 {
				continue
			}
			v := grayValueAt(newPixelReader(raw), rb.Min.X+dx, rb.Min.Y+dy)
			points = append(points, pt{dx, dy, v})
			sumT += v
			sumT2 += v * v
		}
	}
	if len(points) == 0 {
		return nil, nil
	}
	k := float64(len(points))
	meanT := sumT / k
	varT := sumT2/k - meanT*meanT
	if varT <= 1e-9 {
		return nil, nil
	}
	sw, sh := searchRect.Dx(), searchRect.Dy()
	gray := make([]float64, sw*sh)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			gray[y*sw+x] = grayValueAt(newPixelReader(source), searchRect.Min.X+x, searchRect.Min.Y+y)
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

// TestMatchTemplateEquivalence 用随机画面逐例对照优化前后的模板匹配结果。
//
// 优化把 sumI/sumI2 改成列和滑动窗口，浮点累加顺序变了，所以要比对
// 位置与置信度，不能只看"测试还绿"。
func TestMatchTemplateEquivalence(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	noise := func(w, h int) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := uint8(rng.Intn(256))
				img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
			}
		}
		return img
	}

	for iter := 0; iter < 24; iter++ {
		sw, sh := 60+rng.Intn(80), 40+rng.Intn(60)
		tw, th := 8+rng.Intn(24), 8+rng.Intn(24)
		source := noise(sw, sh)
		// 把一块模板内容贴进源图，保证有真实匹配点。
		tpl := noise(tw, th)
		px, py := rng.Intn(sw-tw+1), rng.Intn(sh-th+1)
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				source.SetRGBA(px+x, py+y, tpl.RGBAAt(x, y))
			}
		}
		search := geom.Rect{X1: 0, Y1: 0, X2: sw, Y2: sh}

		want, err1 := refMatchTemplate(source, search, tpl, nil, 0.5)
		got, err2 := MatchTemplate(source, search, tpl, nil, 0.5)
		if err1 != nil || err2 != nil {
			t.Fatalf("iter=%d 出错: %v / %v", iter, err1, err2)
		}
		if len(want) != len(got) {
			t.Fatalf("iter=%d 结果数 %d vs %d", iter, len(want), len(got))
		}
		if len(want) == 0 {
			continue
		}
		if want[0].Rect != got[0].Rect {
			t.Fatalf("iter=%d 位置不一致 %v vs %v", iter, want[0].Rect, got[0].Rect)
		}
		if d := math.Abs(want[0].Confidence - got[0].Confidence); d > 1e-9 {
			t.Fatalf("iter=%d 置信度差 %g（%v vs %v）", iter, d, want[0].Confidence, got[0].Confidence)
		}
	}
}
