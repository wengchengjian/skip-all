//go:build windows

package windows

import (
	"bytes"
	"image/color"
	"testing"
)

func TestTrayIconEncodesValidICO(t *testing.T) {
	data := TrayIcon(color.RGBA{R: 64, G: 186, B: 106, A: 255}, 32)
	if len(data) < 6+16+40 {
		t.Fatalf("ICO 太短: %d", len(data))
	}
	// ICONDIR：reserved=0, type=1(icon), count=1
	if data[0] != 0 || data[1] != 0 || data[2] != 1 || data[3] != 0 || data[4] != 1 || data[5] != 0 {
		t.Fatalf("ICONDIR 头部不对: %v", data[:6])
	}
	// 宽高各 32，色深 32
	if data[6] != 32 || data[7] != 32 {
		t.Fatalf("尺寸字段 = %d/%d", data[6], data[7])
	}
	if data[12] != 32 {
		t.Fatalf("bitCount = %d", data[12])
	}
	// 图像偏移应指向 6+16 处，且该处是 BITMAPINFOHEADER（biSize=40）
	offset := int(data[18]) | int(data[19])<<8 | int(data[20])<<16 | int(data[21])<<24
	if offset != 22 {
		t.Fatalf("图像偏移 = %d, want 22", offset)
	}
	if !bytes.Equal(data[offset:offset+4], []byte{40, 0, 0, 0}) {
		t.Fatalf("BMP 头 biSize 不对: %v", data[offset:offset+4])
	}
}

func TestTrayIconCenterIsFilledAndCornerTransparent(t *testing.T) {
	const size = 32
	data := TrayIcon(color.RGBA{R: 64, G: 186, B: 106, A: 255}, size)
	offset := 22 + 40 // ICONDIR+ENTRY + BITMAPINFOHEADER
	// 像素行自下而上，第 y 行第 x 列在 offset + ((size-1-y)*size + x)*4
	pixel := func(x, y int) color.RGBA {
		i := offset + ((size-1-y)*size+x)*4
		return color.RGBA{R: data[i+2], G: data[i+1], B: data[i], A: data[i+3]}
	}
	if a := pixel(size/2, size/2).A; a < 200 {
		t.Fatalf("中心像素应不透明，alpha = %d", a)
	}
	if a := pixel(0, 0).A; a != 0 {
		t.Fatalf("左上角应透明，alpha = %d", a)
	}
}

func TestTrayIconStateColorsDiffer(t *testing.T) {
	idle := TrayIcon(color.RGBA{R: 148, G: 152, B: 160, A: 255}, 32)
	running := TrayIcon(color.RGBA{R: 64, G: 186, B: 106, A: 255}, 32)
	if bytes.Equal(idle, running) {
		t.Fatal("不同状态应生成不同图标")
	}
}
