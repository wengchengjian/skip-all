package vision

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func testPattern(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*17 + y*31 + 13) % 256)
			img.SetRGBA(x, y, color.RGBA{R: v, G: uint8(255 - v), B: uint8((int(v) + 90) % 256), A: 255})
		}
	}
	return img
}

func paste(dst *image.RGBA, src image.Image, at image.Point) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x+at.X, y+at.Y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
}

func TestMatchTemplate(t *testing.T) {
	tpl := testPattern(20, 10)
	source := image.NewRGBA(image.Rect(0, 0, 100, 80))
	paste(source, tpl, image.Pt(30, 20))

	matches, err := MatchTemplate(source, geom.Rect{X1: 0, Y1: 0, X2: 100, Y2: 80}, tpl, nil, 0.9)
	if err != nil {
		t.Fatalf("MatchTemplate 失败: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("匹配数量 = %d, want 1", len(matches))
	}
	got := matches[0]
	if got.Rect.X1 != 30 || got.Rect.Y1 != 20 || got.Rect.X2 != 50 || got.Rect.Y2 != 30 {
		t.Errorf("匹配位置 = %+v, want (30,20)-(50,30)", got.Rect)
	}
	if got.Confidence < 0.99 {
		t.Errorf("匹配置信度 = %v, want >0.99", got.Confidence)
	}
}

func TestMatchTemplateWithMask(t *testing.T) {
	tpl := testPattern(20, 10)
	mask := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			if x < 10 {
				mask.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				mask.SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
	}

	source := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			source.Set(x+30, y+20, tpl.At(x, y))
		}
		for x := 10; x < 20; x++ {
			source.Set(x+30, y+20, color.RGBA{R: 7, G: 200, B: 33, A: 255})
		}
	}

	matches, err := MatchTemplate(source, geom.Rect{X1: 0, Y1: 0, X2: 100, Y2: 80}, tpl, mask, 0.9)
	if err != nil {
		t.Fatalf("带掩码匹配失败: %v", err)
	}
	if len(matches) != 1 || matches[0].Rect.X1 != 30 || matches[0].Rect.Y1 != 20 {
		t.Fatalf("带掩码匹配位置错误: %+v", matches)
	}
}

func TestMatchTemplateSearchRegion(t *testing.T) {
	tpl := testPattern(10, 10)
	source := image.NewRGBA(image.Rect(0, 0, 100, 60))
	paste(source, tpl, image.Pt(10, 10))
	paste(source, tpl, image.Pt(70, 40))

	matches, err := MatchTemplate(source, geom.Rect{X1: 0, Y1: 0, X2: 50, Y2: 50}, tpl, nil, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Rect.X1 != 10 || matches[0].Rect.Y1 != 10 {
		t.Fatalf("应只在搜索区内匹配到 (10,10): %+v", matches)
	}
}

func TestMatchTemplateInvalid(t *testing.T) {
	tpl := testPattern(20, 10)
	source := image.NewRGBA(image.Rect(0, 0, 100, 80))
	if _, err := MatchTemplate(source, geom.Rect{X1: 0, Y1: 0, X2: 10, Y2: 5}, tpl, nil, 0.9); err != nil {
		t.Fatalf("模板大于搜索区不应报错: %v", err)
	}
	badMask := image.NewRGBA(image.Rect(0, 0, 5, 5))
	if _, err := MatchTemplate(source, geom.Rect{}, tpl, badMask, 0.9); err == nil {
		t.Fatal("掩码尺寸不一致应报错")
	}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTemplateStoreFS(t *testing.T) {
	raw := testPattern(12, 8)
	fsys := fstest.MapFS{
		"tpl/battle/btn/raw.png":  &fstest.MapFile{Data: encodePNG(t, raw)},
		"tpl/battle/btn/mask.png": &fstest.MapFile{Data: encodePNG(t, image.NewRGBA(image.Rect(0, 0, 12, 8)))},
		"tpl/menu/back/raw.png":   &fstest.MapFile{Data: encodePNG(t, raw)},
	}
	store := NewTemplateStoreFS(fsys, "tpl")
	rawImg, maskImg, err := store.Load(TemplateRef{SubDir: "battle", ID: "btn"})
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if rawImg.Bounds().Dx() != 12 || maskImg == nil {
		t.Fatalf("模板加载结果错误: raw=%v mask=%v", rawImg.Bounds(), maskImg)
	}
	_, maskImg, err = store.Load(TemplateRef{SubDir: "menu", ID: "back"})
	if err != nil {
		t.Fatalf("无掩码模板 Load 失败: %v", err)
	}
	if maskImg != nil {
		t.Error("缺少 mask.png 时应返回 nil 掩码")
	}
	if _, _, err := store.Load(TemplateRef{SubDir: "menu", ID: "missing"}); err == nil {
		t.Error("缺失模板应返回错误")
	}
}
