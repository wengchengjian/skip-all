package wuwa

import (
	"image"
	"image/color"
	"io/fs"
	"testing"

	wuwaassets "github.com/wengchengjian/skip-all/assets/games/wuwa"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

func benchFrame() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y += 7 {
		for x := 0; x < 1920; x += 5 {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 90, 255})
		}
	}
	return img
}

// BenchmarkWuwaAvatarBatch 量的是"分批认人"一帧的代价：wuwaScanBatch 张 × 3 个槽位。
func BenchmarkWuwaAvatarBatch(b *testing.B) {
	store := vision.NewTemplateStoreFS(wuwaassets.FS(), "templates")
	entries, err := fs.ReadDir(wuwaassets.FS(), "templates/team")
	if err != nil {
		b.Fatalf("列模板失败: %v", err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	if len(ids) < wuwaScanBatch {
		b.Skip("模板不足")
	}
	batch := ids[:wuwaScanBatch]
	raws := make([]image.Image, 0, len(batch))
	masks := make([]image.Image, 0, len(batch))
	for _, id := range batch {
		raw, mask, err := store.Load(vision.TemplateRef{SubDir: "team", ID: id})
		if err != nil {
			b.Fatalf("加载 %s: %v", id, err)
		}
		raws = append(raws, raw)
		masks = append(masks, mask)
	}
	img := benchFrame()
	area := geom.Rect{X1: 1750, Y1: 205, X2: 1900, Y2: 305}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for slot := 0; slot < 3; slot++ {
			for j := range raws {
				if _, err := vision.MatchTemplate(img, area, raws[j], masks[j], 0.75); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}
