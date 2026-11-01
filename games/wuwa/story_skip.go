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
		ConfirmCheckArea: "确认框勾选框",
		// 按 ESC 或剧情暂停时会出现"剧情梗概"面板，里面有"跳过剧情"。
		PanelSkipArea:    "剧情面板-跳过",
		PanelSkipKeyword: "跳过",
		RightOptionArea:  "右侧选项区域",
		MiddleOptionArea: "中间选项区域",
		// 鸣潮的“跳过”是左上角的图标，没有文字，只能靠模板：
		// 过场剧情在左侧偏下，普通剧情对话在左上角、图标更小。
		StorySkipAreas: []string{"剧情跳过", "剧情跳过-对话"},
		// 字幕式界面：没有对话框底板，靠白色字幕行 + 剧情 UI 模板判断。
		Mode:            storyskip.TextMode{},
		DialogClickArea: "对话内容",
		// 鸣潮的手柄跳过按键没有实测过，先固定用鼠标/后台点击。
		IgnoreGamepad: true,
		// 字幕是带描边的白字，亮度到不了纯白，白色文字行要放宽下限。
		ButtonColorRange: [2][3]uint8{{170, 170, 170}, {255, 255, 255}},
		ConfirmText:      "确认",
		ConfirmTexts:     []string{"确认", "確認", "Confirm"},
		SkipClickWait:    300 * time.Millisecond,
		GuardWait:        time.Second,
		RecentClickLimit: 3,
	}
}
