//go:build windows

package windows

import "testing"

// TestMatchProcessName 覆盖进程名比较的常见写法：忽略大小写、带完整路径、无匹配。
func TestMatchProcessName(t *testing.T) {
	names := []string{"ZenlessZoneZero.exe", "ZenlessZoneZeroBeta.exe"}
	tests := []struct {
		name string
		exe  string
		want bool
	}{
		{"完全一致", "ZenlessZoneZero.exe", true},
		{"忽略大小写", "zenlesszonezero.EXE", true},
		{"反斜杠完整路径取文件名", `C:\Games\ZZZ\ZenlessZoneZero.exe`, true},
		{"斜杠完整路径取文件名", "C:/Games/ZZZ/ZenlessZoneZero.exe", true},
		{"命中第二个候选", "ZenlessZoneZeroBeta.exe", true},
		{"无匹配", "notepad.exe", false},
		{"空 exe", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchProcessName(names, tt.exe); got != tt.want {
				t.Fatalf("matchProcessName(%v, %q) = %v, want %v", names, tt.exe, got, tt.want)
			}
		})
	}
}

// TestProcessAliveEmptyNames 验证空名单不做进程门控，始终视为存活。
func TestProcessAliveEmptyNames(t *testing.T) {
	if !ProcessAlive(nil) {
		t.Fatal("ProcessAlive(nil) = false, want true")
	}
	if !ProcessAlive([]string{}) {
		t.Fatal("ProcessAlive([]string{}) = false, want true")
	}
}
