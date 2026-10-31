package vision

import "testing"

func TestLongestCommonSubsequenceLength(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"abcde", "ace", 3},
		{"abc", "abc", 3},
		{"abc", "def", 0},
		{"", "abc", 0},
		{"你好世界", "你世界", 3},
		{"Start", "start", 4},
	}
	for _, c := range cases {
		if got := LongestCommonSubsequenceLength(c.a, c.b); got != c.want {
			t.Errorf("LongestCommonSubsequenceLength(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestFindByLCS(t *testing.T) {
	cases := []struct {
		source, target string
		percent        float64
		want           bool
	}{
		{"确认", "请点击确认按钮", 0.5, true},
		{"确认", "确", 0.5, true},
		{"确认", "确", 0.6, false},
		{"开始游戏", "开始", 0.5, true},
		{"Start", "START", 1, true},
		{"", "abc", 0.3, false},
		{"abc", "", 0.3, false},
	}
	for _, c := range cases {
		if got := FindByLCS(c.source, c.target, c.percent); got != c.want {
			t.Errorf("FindByLCS(%q, %q, %.2f) = %v, want %v", c.source, c.target, c.percent, got, c.want)
		}
	}
}
