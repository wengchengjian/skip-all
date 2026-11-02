//go:build windows

package wuwa

import (
	"image"
	"image/png"
	"os"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"golang.org/x/image/draw"
)

// cooldownRegions 是鸣潮三个技能按钮的冷却数字区域，与 combat.go 的 Cooldowns 一致。
var cooldownRegions = map[string]geom.Rect{
	"skill":      {X1: 1592, Y1: 938, X2: 1660, Y2: 995},
	"echo":       {X1: 1685, Y1: 938, X2: 1753, Y2: 996},
	"liberation": {X1: 1786, Y1: 938, X2: 1849, Y2: 995},
}

// loadRealScreenshot 读一张上游 ok-ww 的真实截图，并标准化成 1920x1080。
// 截图器在线上也是这么做的（任意分辨率拉伸到逻辑分辨率）。
func loadRealScreenshot(name string) (*image.RGBA, error) {
	f, err := os.Open("../../.tmp/ok-ww/tests/images/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return toLogical(img), nil
}

// toLogical 把任意尺寸的图拉伸到 1920x1080 逻辑分辨率。
func toLogical(img image.Image) *image.RGBA {
	b := img.Bounds()
	if rgba, ok := img.(*image.RGBA); ok && b.Dx() == 1920 && b.Dy() == 1080 {
		return rgba
	}
	dst := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
