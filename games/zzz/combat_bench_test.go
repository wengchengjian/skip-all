package zzz

import (
	"image"
	"image/color"
	"testing"

	zzzassets "github.com/wengchengjian/skip-all/assets/games/zzz"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// benchFrame 造一帧 1920x1080 的画面，模拟战斗截图。
func benchFrame() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y += 7 {
		for x := 0; x < 1920; x += 5 {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 90, 255})
		}
	}
	return img
}

// avatarTemplate 是加载好的一张头像模板。
type avatarTemplate struct {
	ID   string
	Raw  image.Image
	Mask image.Image
}

// benchAvatars 加载真实的头像模板（71 套），量全量扫描与白名单两种情况的开销。
func benchAvatars(b *testing.B) []avatarTemplate {
	store := vision.NewTemplateStoreFS(zzzassets.FS(), "templates")
	ids := avatarIDs("avatar1")
	loaded := make([]avatarTemplate, 0, len(ids))
	for _, id := range ids {
		raw, mask, err := store.Load(vision.TemplateRef{SubDir: "avatar1", ID: id})
		if err != nil {
			b.Fatalf("加载头像模板 %s: %v", id, err)
		}
		loaded = append(loaded, avatarTemplate{ID: id, Raw: raw, Mask: mask})
	}
	return loaded
}

// BenchmarkAvatarFullScan 是"第一次认人"的代价：71 张头像全部比对。
func BenchmarkAvatarFullScan(b *testing.B) {
	img := benchFrame()
	area := geom.Rect{X1: 104, Y1: 40, X2: 274, Y2: 110}
	templates := benchAvatars(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tpl := range templates {
			if _, err := vision.MatchTemplate(img, area, tpl.Raw, tpl.Mask, 0.8); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkAvatarWhitelist 是"认出来之后"的代价：只比对 3 张。
func BenchmarkAvatarWhitelist(b *testing.B) {
	img := benchFrame()
	area := geom.Rect{X1: 104, Y1: 40, X2: 274, Y2: 110}
	templates := benchAvatars(b)[:3]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tpl := range templates {
			if _, err := vision.MatchTemplate(img, area, tpl.Raw, tpl.Mask, 0.8); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkChainBarRatio 是连携/失衡判据（一次颜色占比）的代价。
func BenchmarkChainBarRatio(b *testing.B) {
	img := benchFrame()
	rect := geom.Rect{X1: 813, Y1: 884, X2: 1098, Y2: 900}
	colors := [2][3]uint8{{200, 120, 40}, {255, 205, 120}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vision.ColorRatio(img, rect, colors)
	}
}
