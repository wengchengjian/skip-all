package wuwa

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/storyskip"
)

// StorySkipFeature 创建鸣潮剧情跳过功能。
func StorySkipFeature() assist.Feature {
	return storyskip.NewFeature(storySkipSpec())
}

func storySkipSpec() storyskip.Spec {
	const storyScreen = "鸣潮剧情"
	return storyskip.Spec{
		Screen:           storyScreen,
		StoryArea:        "剧情跳过",
		ConfirmArea:      "对话框确认",
		DialogArea:       "对话内容",
		RightOptionArea:  "右侧选项区域",
		MiddleOptionArea: "中间选项区域",
		// 鸣潮的“跳过”是左上角的图标，没有文字，只能靠模板；
		// 顶部的“跳过”图标是老版剧情界面，位置不同，一并尝试。
		StorySkipAreas:  []string{"剧情跳过", "剧情跳过-顶部"},
		DialogStyle:     storyskip.DialogText,
		DialogClickArea: "对话内容",
		// 纯模板判定，不为了剧情栏加载 OCR；只有确认框出现时才用 OCR。
		DisableStoryOCR:  true,
		ConfirmText:      "确认",
		ConfirmTexts:     []string{"确认", "確認", "Confirm"},
		SkipClickWait:    300 * time.Millisecond,
		GuardWait:        time.Second,
		RecentClickLimit: 3,
	}
}
