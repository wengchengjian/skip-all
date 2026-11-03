//go:build windows

package windows

import (
	"errors"
	"image"
	"math"
	"testing"
)

func TestConvertBGRA(t *testing.T) {
	src := []byte{
		1, 2, 3, 0, // B G R A -> R=3 G=2 B=1
		5, 6, 7, 0,
	}
	dst := image.NewRGBA(image.Rect(0, 0, 2, 1))
	convertBGRA(dst, src, 2, 1)
	want := []byte{3, 2, 1, 255, 7, 6, 5, 255}
	for i := range want {
		if dst.Pix[i] != want[i] {
			t.Fatalf("pix[%d] = %d, want %d (全部: %v)", i, dst.Pix[i], want[i], dst.Pix[:8])
		}
	}
}

func TestIsBlackFrame(t *testing.T) {
	black := make([]byte, 16*16*4)
	for i := 3; i < len(black); i += 4 {
		black[i] = 255
	}
	if !isBlackFrame(black) {
		t.Fatal("全黑帧应当判定为黑")
	}
	black[4*100] = 200
	if isBlackFrame(black) {
		t.Fatal("存在亮像素时不应判定为全黑")
	}
}

// TestResizeRGBA 验证缩放后的颜色通道顺序正确（源 BGRA -> 目标 RGBA）。
func TestResizeRGBA(t *testing.T) {
	// 2x2 BGRA：左上红、右上绿、左下蓝、右下白
	src := []byte{
		0, 0, 255, 255, // 红：B=0 G=0 R=255
		0, 255, 0, 255, // 绿
		255, 0, 0, 255, // 蓝
		255, 255, 255, 255, // 白
	}
	dst := image.NewRGBA(image.Rect(0, 0, 4, 4))
	scaleBGRAtoRGBA(dst, src, 2, 2)
	if dst.Rect.Dx() != 4 || dst.Rect.Dy() != 4 {
		t.Fatalf("尺寸 = %v, want 4x4", dst.Rect)
	}
	if got := dst.RGBAAt(0, 0); got.R < 200 || got.G > 50 {
		t.Fatalf("左上角应接近红色，实际 %v", got)
	}
	if got := dst.RGBAAt(3, 3); got.R < 200 || got.G < 200 || got.B < 200 {
		t.Fatalf("右下角应接近白色，实际 %v", got)
	}
}

// TestCaptureRealWindow 在真实游戏窗口上验证 GDI 截图流程，没有窗口时跳过。
func TestCaptureRealWindow(t *testing.T) {
	w, err := Find([]string{"绝区零", "ZenlessZoneZero"}, []string{"ZenlessZoneZero.exe"})
	if errors.Is(err, ErrWindowNotFound) {
		t.Skip("本机未运行绝区零")
	}
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	rect, err := w.ClientRect()
	if err != nil {
		t.Fatalf("ClientRect: %v", err)
	}

	c := NewCapturer(w)
	defer c.Close()
	img, scale, err := c.Capture()
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got := img.Bounds().Dx(); got != standardWidth {
		t.Fatalf("宽度 = %d, want %d", got, standardWidth)
	}
	if got := img.Bounds().Dy(); got != standardHeight {
		t.Fatalf("高度 = %d, want %d", got, standardHeight)
	}
	wantScale := float64(rect.Width()) / standardWidth
	if math.Abs(scale-wantScale) > 1e-9 {
		t.Fatalf("scale = %f, want %f", scale, wantScale)
	}

	// 输出平均亮度，便于判断截图是否全黑（不强制失败，游戏可能正处在黑屏加载）。
	var sum, count uint64
	for i := 0; i+3 < len(img.Pix); i += 4 * 97 {
		sum += uint64(img.Pix[i]) + uint64(img.Pix[i+1]) + uint64(img.Pix[i+2])
		count += 3
	}
	if count > 0 {
		t.Logf("截图尺寸=%dx%d scale=%.3f 平均亮度=%.1f", img.Bounds().Dx(), img.Bounds().Dy(), scale, float64(sum)/float64(count))
	}
}
