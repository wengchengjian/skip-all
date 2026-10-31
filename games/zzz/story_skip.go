package zzz

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/storyskip"
)

// StorySkipFeature 创建绝区零剧情跳过功能。
func StorySkipFeature() assist.Feature {
	return storyskip.NewFeature(storySkipSpec())
}

func storySkipSpec() storyskip.Spec {
	const commissionScreen = "委托助手"
	return storyskip.Spec{
		Screen:              commissionScreen,
		StoryArea:           "文本-剧情右上角",
		ConfirmArea:         "对话框确认",
		DialogArea:          "对话框内容",
		RightOptionArea:     "右侧选项区域",
		MiddleOptionArea:    "中间选项区域",
		AutoButtonArea:      "按钮-自动",
		StoryMenuAreas:      []string{"剧情菜单-键盘", "剧情菜单-手柄"},
		StorySkipAreas:      []string{"剧情跳过-手柄"},
		ConfirmTemplateArea: "剧情确认-模板",
		Keywords:            []string{"菜单", "跳过", "自动"},
		ConfirmText:         "确认",
		SkipKeyword:         "跳过",
		MenuKeyword:         "菜单",
		AutoKeyword:         "自动",
		Guards: []storyskip.Guard{
			{Area: "左上角返回", Kind: storyskip.GuardTemplate},
			{Area: "战斗-菜单", Kind: storyskip.GuardTemplate},
			{Screen: "大世界-勘域", Area: "勘域-菜单", Kind: storyskip.GuardTemplate},
			{Screen: "战斗画面", Area: "按键-交互", Kind: storyskip.GuardTemplate},
			{Area: "玩法引导", Kind: storyskip.GuardBand},
		},
		MenuQuiet:         5 * time.Second,
		SkipClickWait:     400 * time.Millisecond,
		GuardWait:         time.Second,
		GrayMaskThreshold: 0.9,
		RecentClickLimit:  3,
	}
}
