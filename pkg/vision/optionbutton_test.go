package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func filledRect(img *image.RGBA, rect geom.Rect, v uint8) {
	for y := rect.Y1; y < rect.Y2; y++ {
		for x := rect.X1; x < rect.X2; x++ {
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
}

// outlinedBox 画一枚描边选项按钮：四周 thickness 宽的白边，圆角处留空。
func outlinedBox(img *image.RGBA, rect geom.Rect, thickness, radius int) {
	for y := rect.Y1; y < rect.Y2; y++ {
		for x := rect.X1; x < rect.X2; x++ {
			onTop := y-rect.Y1 < radius && (x-rect.X1 < radius || rect.X2-x <= radius)
			onBottom := rect.Y2-y <= radius && (x-rect.X1 < radius || rect.X2-x <= radius)
			if onTop || onBottom {
				continue
			}
			if y-rect.Y1 < thickness || rect.Y2-y <= thickness || x-rect.X1 < thickness || rect.X2-x <= thickness {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
}

func TestFindOptionButtons(t *testing.T) {
	area := geom.Rect{X1: 0, Y1: 0, X2: 900, Y2: 500}
	opts := OptionButtonOptions{ColorRange: [2][3]uint8{{170, 170, 170}, {255, 255, 255}}}
	box := geom.Rect{X1: 400, Y1: 300, X2: 600, Y2: 342}

	t.Run("描边按钮带文字与左侧按键提示", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		outlinedBox(img, box, 2, 6)
		// 框内三个"字"
		filledRect(img, geom.Rect{X1: 415, Y1: 310, X2: 435, Y2: 332}, 255)
		filledRect(img, geom.Rect{X1: 445, Y1: 310, X2: 465, Y2: 332}, 255)
		filledRect(img, geom.Rect{X1: 475, Y1: 310, X2: 495, Y2: 332}, 255)
		// 紧贴左侧的实心按键提示：尺寸太小且是实心，不该被当成按钮
		filledRect(img, geom.Rect{X1: 360, Y1: 308, X2: 382, Y2: 334}, 255)
		got := FindOptionButtons(img, area, opts)
		if len(got) != 1 {
			t.Fatalf("识别到 %d 个选项按钮, want 1: %+v", len(got), got)
		}
		if got[0] != box {
			t.Fatalf("按钮矩形 = %+v, want %+v", got[0], box)
		}
		if !box.Contains(got[0].Center()) {
			t.Fatalf("点击中心 %+v 不在按钮内", got[0].Center())
		}
	})

	t.Run("白底实心按钮不是选项", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		filledRect(img, box, 255)
		if got := FindOptionButtons(img, area, opts); len(got) != 0 {
			t.Fatalf("实心按钮被当成选项: %+v", got)
		}
	})

	t.Run("不规则亮斑不是选项", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		for y := box.Y1; y < box.Y2; y++ {
			dy := float64(y-(box.Y1+box.Y2)/2) / float64(box.Y2-box.Y1)
			half := float64(box.X2-box.X1) / 2 * (1 - dy*dy*1.6)
			if half < 2 {
				continue
			}
			cx := float64(box.X1+box.X2) / 2
			for x := int(cx - half); x < int(cx+half); x++ {
				img.SetRGBA(x, y, color.RGBA{230, 235, 255, 255})
			}
		}
		if got := FindOptionButtons(img, area, opts); len(got) != 0 {
			t.Fatalf("亮斑被当成选项: %+v", got)
		}
	})

	t.Run("缺下边框不算按钮", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		outlinedBox(img, box, 2, 0)
		filledRect(img, geom.Rect{X1: box.X1, Y1: box.Y2 - 3, X2: box.X2, Y2: box.Y2}, 0)
		filledRect(img, geom.Rect{X1: 415, Y1: 310, X2: 435, Y2: 332}, 255)
		if got := FindOptionButtons(img, area, opts); len(got) != 0 {
			t.Fatalf("缺下边框被当成选项: %+v", got)
		}
	})

	t.Run("空框没有文字不算按钮", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		outlinedBox(img, box, 2, 6)
		if got := FindOptionButtons(img, area, opts); len(got) != 0 {
			t.Fatalf("空框被当成选项: %+v", got)
		}
	})

	t.Run("单行字幕不算按钮", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		filledRect(img, geom.Rect{X1: 300, Y1: 320, X2: 700, Y2: 340}, 255)
		if got := FindOptionButtons(img, area, opts); len(got) != 0 {
			t.Fatalf("字幕被当成选项: %+v", got)
		}
	})

	t.Run("两个选项各认一个", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 900, 500))
		first := geom.Rect{X1: 400, Y1: 200, X2: 640, Y2: 244}
		second := geom.Rect{X1: 400, Y1: 260, X2: 600, Y2: 302}
		outlinedBox(img, first, 2, 6)
		outlinedBox(img, second, 2, 6)
		filledRect(img, geom.Rect{X1: 420, Y1: 212, X2: 440, Y2: 232}, 255)
		filledRect(img, geom.Rect{X1: 420, Y1: 272, X2: 440, Y2: 292}, 255)
		got := FindOptionButtons(img, area, opts)
		if len(got) != 2 {
			t.Fatalf("识别到 %d 个选项按钮, want 2: %+v", len(got), got)
		}
		if got[0] != first || got[1] != second {
			t.Fatalf("按钮矩形 = %+v, want %+v %+v", got, first, second)
		}
	})
}
