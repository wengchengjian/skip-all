package vision

import "strings"

// LongestCommonSubsequenceLength 计算两个字符串的最长公共子序列长度（按 Unicode 字符计）。
func LongestCommonSubsequenceLength(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for i := 1; i <= len(ar); i++ {
		for j := 1; j <= len(br); j++ {
			switch {
			case ar[i-1] == br[j-1]:
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
		clear(cur)
	}
	return prev[len(br)]
}

// FindByLCS 判断 OCR 文本 target 是否与期望文本 source 足够相似。
// 相似度按最长公共子序列长度占 source 长度的比例计算，忽略大小写。
func FindByLCS(source, target string, percent float64) bool {
	if source == "" || target == "" {
		return false
	}
	common := LongestCommonSubsequenceLength(strings.ToLower(source), strings.ToLower(target))
	return float64(common) >= float64(len([]rune(source)))*percent
}
