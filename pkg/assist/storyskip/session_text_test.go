package storyskip

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// 鸣潮这类没有对话框底板、只用字幕行的界面。
func textSpec() Spec {
	return Spec{
		Screen:           "鸣潮剧情",
		StoryArea:        "剧情跳过",
		ConfirmArea:      "对话框确认",
		DialogArea:       "对话内容",
		RightOptionArea:  "右侧选项区域",
		MiddleOptionArea: "中间选项区域",
		StorySkipAreas:   []string{"剧情跳过"},
		DialogStyle:      DialogText,
		DialogClickArea:  "对话内容",
		DisableStoryOCR:  true,
		ConfirmTexts:     []string{"确认", "確認"},
		SkipClickWait:    300 * time.Millisecond,
		RecentClickLimit: 3,
	}
}

func textScreens() *screen.Set {
	set := screen.NewSet()
	set.Add(&screen.Screen{
		ID: "wuwa_story", Name: "鸣潮剧情",
		AreaList: []*screen.Area{
			{Name: "剧情跳过", RectValues: []int{24, 132, 150, 236}, TemplateSubDir: "story", TemplateID: "skip"},
			{Name: "对话框确认", RectValues: []int{980, 595, 1420, 700}, Text: "确认"},
			{Name: "对话内容", RectValues: []int{480, 780, 1440, 935}},
			{Name: "右侧选项区域", RectValues: []int{1150, 300, 1900, 760}, ColorRange: &[2][3]uint8{{240, 240, 240}, {255, 255, 255}}},
			{Name: "中间选项区域", RectValues: []int{520, 250, 1400, 780}},
		},
	})
	return set
}

func newTextSession(t *testing.T, scripts []frameScript) (*Session, *fakeRuntime) {
	t.Helper()
	rt := &fakeRuntime{screens: textScreens(), scripts: scripts}
	session, err := NewFeature(textSpec()).NewSession(rt, assist.Settings{"mode": "skip"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return session.(*Session), rt
}

func TestTextDialogClicksDialogArea(t *testing.T) {
	subtitle := block("你总算来啦。", geom.Rect{X1: 860, Y1: 800, X2: 1040, Y2: 822})
	session, rt := newTextSession(t, []frameScript{{
		at:  time.Now(),
		ocr: map[string][]vision.TextBlock{"对话内容": {subtitle}},
	}})
	result, actions := step(t, session, rt, 0)
	if result.State != "对话中点击" {
		t.Fatalf("state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 960, Y: 857}}})
}

func TestTextDialogWaitsOnEmptyScreen(t *testing.T) {
	session, rt := newTextSession(t, []frameScript{{at: time.Now()}})
	result, actions := step(t, session, rt, 0)
	if result.State != "等待剧情画面" {
		t.Fatalf("state = %q", result.State)
	}
	checkActions(t, actions, nil)
}

func TestTextDialogSkipsWithoutStoryOCR(t *testing.T) {
	// 剧情栏里就算 OCR 出“跳过”，关闭剧情栏 OCR 后也不该点它。
	skipText := block("跳过", geom.Rect{X1: 40, Y1: 140, X2: 120, Y2: 190})
	session, rt := newTextSession(t, []frameScript{{
		at:  time.Now(),
		ocr: map[string][]vision.TextBlock{"剧情跳过": {skipText}},
	}})
	result, actions := step(t, session, rt, 0)
	if result.State != "等待剧情画面" {
		t.Fatalf("state = %q", result.State)
	}
	checkActions(t, actions, nil)
}

func TestTextDialogClicksSkipTemplateThenConfirm(t *testing.T) {
	session, rt := newTextSession(t, []frameScript{
		{
			at:      time.Now(),
			matches: map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 58, Y1: 165, X2: 100, Y2: 205}, Confidence: 0.95}}},
		},
		{
			at: time.Now(),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("確認", geom.Rect{X1: 1130, Y1: 620, X2: 1230, Y2: 660})},
			},
		},
	})
	if result, actions := step(t, session, rt, 0); result.State != "模板跳过" {
		t.Fatalf("第一帧 state = %q, actions = %+v", result.State, actions)
	}
	result, actions := step(t, session, rt, 1)
	if result.State != "确认跳过" {
		t.Fatalf("第二帧 state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1180, Y: 640}}})
}

func TestMaskStyleStillUsesGrayMask(t *testing.T) {
	// 默认样式（绝区零）不看字幕行，只认灰底遮罩。
	spec := testSpec()
	if spec.DialogStyle != "" {
		t.Fatalf("默认 DialogStyle 应为空，实际 %q", spec.DialogStyle)
	}
	if got := spec.withDefaults().DialogStyle; got != DialogMask {
		t.Fatalf("默认 DialogStyle = %q", got)
	}
}
