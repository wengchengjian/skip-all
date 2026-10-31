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
		ConfirmCheckArea: "确认框勾选框",
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
			{Name: "剧情菜单-跳过剧情", RectValues: []int{1140, 725, 1610, 845}},
			{Name: "确认框勾选框", RectValues: []int{818, 542, 1012, 628}, TemplateSubDir: "story", TemplateID: "confirm_check"},
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

func newTextClickSession(t *testing.T, scripts []frameScript) (*Session, *fakeRuntime) {
	t.Helper()
	rt := &fakeRuntime{screens: textScreens(), scripts: scripts}
	session, err := NewFeature(textSpec()).NewSession(rt, assist.Settings{"mode": "click"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return session.(*Session), rt
}

func TestTextDialogClicksDialogArea(t *testing.T) {
	subtitle := block("你总算来啦。", geom.Rect{X1: 860, Y1: 800, X2: 1040, Y2: 822})
	session, rt := newTextClickSession(t, []frameScript{{
		at:      time.Now(),
		ocr:     map[string][]vision.TextBlock{"对话内容": {subtitle}},
		matches: map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 69, Y1: 176, X2: 89, Y2: 194}, Confidence: 0.9}}},
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
				"对话框确认": {block("確認", geom.Rect{X1: 990, Y1: 610, X2: 1090, Y2: 650})},
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
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1040, Y: 630}}})
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

func TestIgnoreGamepadDisablesPadKeys(t *testing.T) {
	spec := textSpec()
	spec.IgnoreGamepad = true
	rt := &fakeRuntime{screens: textScreens(), scripts: []frameScript{{at: time.Now()}}}
	session, err := NewFeature(spec).NewSession(rt, assist.Settings{"mode": "skip", "gamepad": true})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if session.(*Session).cfg.gamepad {
		t.Fatal("IgnoreGamepad 为真时不应启用手柄按键")
	}
}

func TestRetrySkipUntilConfirmShows(t *testing.T) {
	base := time.Date(2026, 11, 1, 6, 0, 0, 0, time.Local)
	skipMatch := map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 58, Y1: 165, X2: 100, Y2: 205}, Confidence: 0.95}}}
	scripts := []frameScript{
		{at: base, matches: skipMatch},
		{at: base.Add(600 * time.Millisecond), matches: skipMatch},
		{at: base.Add(1200 * time.Millisecond), matches: skipMatch},
		{at: base.Add(1800 * time.Millisecond), matches: skipMatch},
		{at: base.Add(2400 * time.Millisecond), matches: skipMatch},
	}
	session, rt := newTextSession(t, scripts)
	if result, _ := step(t, session, rt, 0); result.State != "模板跳过" {
		t.Fatalf("第 0 帧 state = %q", result.State)
	}
	for i := 1; i <= 3; i++ {
		result, actions := step(t, session, rt, i)
		if result.State != "重试跳过" {
			t.Fatalf("第 %d 帧 state = %q, want 重试跳过", i, result.State)
		}
		checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 79, Y: 185}}})
	}
	// 重试次数用完后只等待，不再点。
	if result, actions := step(t, session, rt, 4); result.State != "等待确认框" || len(actions) != 0 {
		t.Fatalf("第 4 帧 state = %q actions = %+v", result.State, actions)
	}
}

func TestRetrySkipThrottled(t *testing.T) {
	base := time.Date(2026, 11, 1, 6, 0, 0, 0, time.Local)
	skipMatch := map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 58, Y1: 165, X2: 100, Y2: 205}, Confidence: 0.95}}}
	scripts := []frameScript{
		{at: base, matches: skipMatch},
		{at: base.Add(100 * time.Millisecond), matches: skipMatch},
	}
	session, rt := newTextSession(t, scripts)
	step(t, session, rt, 0)
	if result, actions := step(t, session, rt, 1); result.State != "等待确认框" || len(actions) != 0 {
		t.Fatalf("间隔不足 500ms 不应补点: state = %q actions = %+v", result.State, actions)
	}
}

func TestConfirmDialogFallbackNeedsCheckbox(t *testing.T) {
	// 只有「确认」没有勾选框时不能点：那是别的弹窗（设置、退出确认等）。
	confirmText := block("确认", geom.Rect{X1: 990, Y1: 610, X2: 1090, Y2: 650})
	session, rt := newTextSession(t, []frameScript{{
		at:  time.Now(),
		ocr: map[string][]vision.TextBlock{"对话框确认": {confirmText}},
	}})
	result, actions := step(t, session, rt, 0)
	if result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("缺勾选框时不应点确认: state = %q actions = %+v", result.State, actions)
	}
}

func TestConfirmDialogFallbackClicksWithCheckbox(t *testing.T) {
	confirmText := block("確認", geom.Rect{X1: 990, Y1: 610, X2: 1090, Y2: 650})
	session, rt := newTextSession(t, []frameScript{{
		at:  time.Now(),
		ocr: map[string][]vision.TextBlock{"对话框确认": {confirmText}},
		matches: map[string][]vision.Match{
			"确认框勾选框": {{Rect: geom.Rect{X1: 858, Y1: 562, X2: 902, Y2: 602}, Confidence: 0.95}},
		},
	}})
	result, actions := step(t, session, rt, 0)
	if result.State != "确认弹窗" {
		t.Fatalf("state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1040, Y: 630}}})
}

func TestConfirmDialogFallbackThrottled(t *testing.T) {
	base := time.Date(2026, 11, 1, 7, 0, 0, 0, time.Local)
	confirmText := block("确认", geom.Rect{X1: 990, Y1: 610, X2: 1090, Y2: 650})
	boxes := map[string][]vision.Match{
		"确认框勾选框": {{Rect: geom.Rect{X1: 858, Y1: 562, X2: 902, Y2: 602}, Confidence: 0.95}},
	}
	scripts := []frameScript{
		{at: base, ocr: map[string][]vision.TextBlock{"对话框确认": {confirmText}}, matches: boxes},
		{at: base.Add(time.Second), ocr: map[string][]vision.TextBlock{"对话框确认": {confirmText}}, matches: boxes},
		{at: base.Add(3 * time.Second), ocr: map[string][]vision.TextBlock{"对话框确认": {confirmText}}, matches: boxes},
	}
	session, rt := newTextSession(t, scripts)
	if result, _ := step(t, session, rt, 0); result.State != "确认弹窗" {
		t.Fatalf("第 0 帧 state = %q", result.State)
	}
	// 1 秒内不再重复检查，避免每帧都在弹窗上点确认。
	if result, actions := step(t, session, rt, 1); result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("节流期内不应再点: state = %q actions = %+v", result.State, actions)
	}
	if result, _ := step(t, session, rt, 2); result.State != "确认弹窗" {
		t.Fatalf("超过间隔后应再次点确认, state = %q", result.State)
	}
}

func TestSubtitleWithoutStoryUIStaysIdle(t *testing.T) {
	// 大世界底部也有白字（交互提示、任务提示），没有剧情跳过图标时不能点。
	subtitle := block("前往目标地点", geom.Rect{X1: 860, Y1: 800, X2: 1040, Y2: 822})
	session, rt := newTextSession(t, []frameScript{{
		at:  time.Now(),
		ocr: map[string][]vision.TextBlock{"对话内容": {subtitle}},
	}})
	result, actions := step(t, session, rt, 0)
	if result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("没有剧情 UI 时不应点击: state = %q actions = %+v", result.State, actions)
	}
}

func TestSubtitleWithStoryUIClicks(t *testing.T) {
	subtitle := block("前往目标地点", geom.Rect{X1: 860, Y1: 800, X2: 1040, Y2: 822})
	session, rt := newTextClickSession(t, []frameScript{{
		at:      time.Now(),
		ocr:     map[string][]vision.TextBlock{"对话内容": {subtitle}},
		matches: map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 69, Y1: 176, X2: 89, Y2: 194}, Confidence: 0.9}}},
	}})
	result, actions := step(t, session, rt, 0)
	if result.State != "对话中点击" {
		t.Fatalf("state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 960, Y: 857}}})
}

// escSpec 是鸣潮的按键流程：检测到剧情 → ESC → 点跳过剧情 → 确认。
func escSpec() Spec {
	spec := textSpec()
	spec.OpenMenuKey = "esc"
	spec.MenuSkipArea = "剧情菜单-跳过剧情"
	spec.MenuSkipKeyword = "跳过"
	spec.MenuWait = 800 * time.Millisecond
	return spec
}

func newEscSession(t *testing.T, scripts []frameScript) (*Session, *fakeRuntime) {
	t.Helper()
	rt := &fakeRuntime{screens: textScreens(), scripts: scripts}
	session, err := NewFeature(escSpec()).NewSession(rt, assist.Settings{"mode": "skip"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return session.(*Session), rt
}

func TestOpenStoryMenuKeyThenSkipThenConfirm(t *testing.T) {
	base := time.Date(2026, 11, 1, 8, 0, 0, 0, time.Local)
	skipMatch := map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 58, Y1: 165, X2: 100, Y2: 205}, Confidence: 0.95}}}
	scripts := []frameScript{
		{at: base, matches: skipMatch},
		{
			at: base.Add(time.Second),
			ocr: map[string][]vision.TextBlock{
				"剧情菜单-跳过剧情": {block("跳过剧情", geom.Rect{X1: 1200, Y1: 760, X2: 1400, Y2: 800})},
			},
		},
		{
			at: base.Add(1500 * time.Millisecond),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("确认", geom.Rect{X1: 1047, Y1: 613, X2: 1310, Y2: 658})},
			},
		},
	}
	session, rt := newEscSession(t, scripts)

	// 1. 看到剧情界面：按 ESC 叫出剧情菜单，不去点图标。
	result, actions := step(t, session, rt, 0)
	if result.State != "按键打开剧情菜单" {
		t.Fatalf("第 0 帧 state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: "esc"}})

	// 2. 菜单出来：点“跳过剧情”。
	result, actions = step(t, session, rt, 1)
	if result.State != "点击跳过剧情" {
		t.Fatalf("第 1 帧 state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1300, Y: 780}}})

	// 3. 弹确认框：点“确认”。
	result, actions = step(t, session, rt, 2)
	if result.State != "确认跳过" {
		t.Fatalf("第 2 帧 state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1178, Y: 635}}})
}

func TestOpenStoryMenuWaitsBeforeClicking(t *testing.T) {
	base := time.Date(2026, 11, 1, 8, 0, 0, 0, time.Local)
	skipMatch := map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 58, Y1: 165, X2: 100, Y2: 205}, Confidence: 0.95}}}
	scripts := []frameScript{
		{at: base, matches: skipMatch},
		{
			at: base.Add(200 * time.Millisecond),
			ocr: map[string][]vision.TextBlock{
				"剧情菜单-跳过剧情": {block("跳过剧情", geom.Rect{X1: 1200, Y1: 760, X2: 1400, Y2: 800})},
			},
		},
	}
	session, rt := newEscSession(t, scripts)
	step(t, session, rt, 0)
	// 菜单还没动画完，不能点。
	if result, actions := step(t, session, rt, 1); result.State != "等待剧情菜单" || len(actions) != 0 {
		t.Fatalf("菜单动画期间不应点击: state = %q actions = %+v", result.State, actions)
	}
}

func TestStoryMenuWithoutSkipButtonGivesUp(t *testing.T) {
	base := time.Date(2026, 11, 1, 8, 0, 0, 0, time.Local)
	skipMatch := map[string][]vision.Match{"剧情跳过": {{Rect: geom.Rect{X1: 58, Y1: 165, X2: 100, Y2: 205}, Confidence: 0.95}}}
	scripts := []frameScript{
		{at: base, matches: skipMatch},
		{at: base.Add(6 * time.Second), matches: skipMatch},
	}
	session, rt := newEscSession(t, scripts)
	step(t, session, rt, 0)
	if result, _ := step(t, session, rt, 1); result.State != "等待剧情画面" {
		t.Fatalf("超时后应放弃, state = %q", result.State)
	}
	if session.menuOpen {
		t.Fatal("超时后 menuOpen 应复位")
	}
}
