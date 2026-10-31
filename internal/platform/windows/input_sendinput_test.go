//go:build windows

package windows

import (
	"testing"
	"unsafe"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

func TestVirtualKey(t *testing.T) {
	cases := map[string]uint16{
		"a":        0x41,
		"Z":        0x5A,
		"1":        0x31,
		"0":        0x30,
		"return":   0x0D,
		"enter":    0x0D,
		"escape":   0x1B,
		"spacebar": 0x20,
		"space":    0x20,
		"left":     0x25,
		"f1":       0x70,
		"f12":      0x7B,
	}
	for name, want := range cases {
		vk, _, err := virtualKey(name)
		if err != nil {
			t.Fatalf("virtualKey(%q): %v", name, err)
		}
		if vk != want {
			t.Fatalf("virtualKey(%q) = 0x%02X, want 0x%02X", name, vk, want)
		}
	}
	if _, _, err := virtualKey("不存在的键"); err == nil {
		t.Fatal("未知按键应返回错误")
	}
}

func TestScalePoint(t *testing.T) {
	cases := []struct {
		point         geom.Point
		width, height int
		want          geom.Point
	}{
		{geom.Point{X: 0, Y: 0}, 1920, 1080, geom.Point{X: 0, Y: 0}},
		{geom.Point{X: 1919, Y: 1079}, 1920, 1080, geom.Point{X: 1919, Y: 1079}},
		{geom.Point{X: 100, Y: 200}, 2560, 1440, geom.Point{X: 133, Y: 267}},
		{geom.Point{X: 960, Y: 540}, 1280, 720, geom.Point{X: 640, Y: 360}},
	}
	for _, c := range cases {
		got, err := scalePoint(c.point, c.width, c.height)
		if err != nil {
			t.Fatalf("scalePoint(%v, %d, %d): %v", c.point, c.width, c.height, err)
		}
		if got != c.want {
			t.Fatalf("scalePoint(%v, %d, %d) = %v, want %v", c.point, c.width, c.height, got, c.want)
		}
	}
	if _, err := scalePoint(geom.Point{X: -1, Y: 0}, 1920, 1080); err == nil {
		t.Fatal("负坐标应返回错误")
	}
	if _, err := scalePoint(geom.Point{X: 1920, Y: 0}, 1920, 1080); err == nil {
		t.Fatal("超出逻辑分辨率的坐标应返回错误")
	}
	if _, err := scalePoint(geom.Point{X: 0, Y: 0}, 0, 0); err == nil {
		t.Fatal("无效客户区尺寸应返回错误")
	}
}

func TestInputEventLayout(t *testing.T) {
	// Win32 INPUT 在 64 位上为 40 字节、32 位上为 28 字节，SendInput 的 cbSize 必须精确匹配。
	ptrSize := unsafe.Sizeof(uintptr(0))
	wantSize := uintptr(28)
	wantOffset := uintptr(4)
	if ptrSize == 8 {
		wantSize = 40
		wantOffset = 8
	}
	if got := unsafe.Sizeof(inputEvent{}); got != wantSize {
		t.Fatalf("inputEvent 大小 = %d, want %d", got, wantSize)
	}
	if got := unsafe.Offsetof(inputEvent{}.data); got != wantOffset {
		t.Fatalf("inputEvent.data 偏移 = %d, want %d", got, wantOffset)
	}
	if got := unsafe.Sizeof(mouseInput{}); got != wantSize-8 {
		t.Fatalf("mouseInput 大小 = %d, want %d", got, wantSize-8)
	}
}

func TestCursorMode(t *testing.T) {
	in := NewInputWithOptions(nil, InputOptions{})
	if in.cursorMode != CursorNever {
		t.Fatalf("默认 cursorMode = %q, want %q", in.cursorMode, CursorNever)
	}
	if in.shouldMoveCursor() {
		t.Fatal("never 模式不应移动光标")
	}
	in = &Input{cursorMode: CursorAlways}
	if !in.shouldMoveCursor() {
		t.Fatal("always 模式应移动光标")
	}
}

func TestInputNilWindow(t *testing.T) {
	in := NewInput(nil)
	if err := in.Click(geom.Point{}); err != ErrWindowNotFound {
		t.Fatalf("Click err = %v, want ErrWindowNotFound", err)
	}
	if err := in.KeyTap("a"); err != ErrWindowNotFound {
		t.Fatalf("KeyTap err = %v, want ErrWindowNotFound", err)
	}
	if err := in.ReleaseAll(); err != nil {
		t.Fatalf("ReleaseAll: %v", err)
	}
}
