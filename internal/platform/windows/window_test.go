//go:build windows

package windows

import (
	"errors"
	"testing"
)

func TestContainsString(t *testing.T) {
	list := []string{"绝区零", "ZenlessZoneZero"}
	if !containsString(list, "绝区零") {
		t.Fatal("应当精确匹配中文标题")
	}
	if containsString(list, "绝区零 ") {
		t.Fatal("标题匹配不应忽略尾部空格")
	}
	if !containsFold([]string{"ZenlessZoneZero.exe"}, "zenlesszonezero.exe") {
		t.Fatal("进程名应当忽略大小写")
	}
}

func TestFindWindowNotFound(t *testing.T) {
	_, err := Find([]string{"__skip-all-不存在的窗口__"}, nil)
	if !errors.Is(err, ErrWindowNotFound) {
		t.Fatalf("err = %v, want ErrWindowNotFound", err)
	}
}

func TestFindEmptyCriteria(t *testing.T) {
	if _, err := Find(nil, nil); err == nil {
		t.Fatal("标题和进程名同时为空时应返回错误")
	}
}

// TestFindRealGameWindow 在本机没有运行游戏时跳过。
func TestFindRealGameWindow(t *testing.T) {
	w, err := Find([]string{"绝区零", "ZenlessZoneZero"}, []string{"ZenlessZoneZero.exe"})
	if errors.Is(err, ErrWindowNotFound) {
		t.Skip("本机未运行绝区零")
	}
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if w.Handle() == 0 {
		t.Fatal("窗口句柄为 0")
	}
	if !w.IsValid() {
		t.Fatal("刚找到的窗口应当有效")
	}
	rect, err := w.ClientRect()
	if err != nil {
		t.Fatalf("ClientRect: %v", err)
	}
	if rect.Width() <= 0 || rect.Height() <= 0 {
		t.Fatalf("客户区尺寸异常: %+v", rect)
	}
}
