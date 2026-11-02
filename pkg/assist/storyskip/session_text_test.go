package storyskip

import (
	"context"
	"image"
	"image/color"
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
		PanelSkipArea:    "剧情面板-跳过",
		PanelSkipKeyword: "跳过",
		Mode:             TextMode{},
		DialogClickArea:  "对话内容",

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
			{Name: "剧情面板-跳过", RectValues: []int{1140, 720, 1600, 850}},
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

func TestDefaultModeIsMask(t *testing.T) {
	// 不指定 Mode 时按灰底对话框处理（绝区零）。
	spec := testSpec()
	if spec.Mode != nil {
		t.Fatalf("默认 Mode 应为空，实际 %v", spec.Mode)
	}
	session, _ := newTestSession(t, nil, []frameScript{{at: time.Now()}})
	if _, ok := session.mode().(MaskMode); !ok {
		t.Fatalf("默认判读实现 = %T, want MaskMode", session.mode())
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

func TestConfirmDialogFallbackTicksThenConfirms(t *testing.T) {
	base := time.Date(2026, 11, 1, 9, 10, 0, 0, time.Local)
	box := map[string][]vision.Match{"确认框勾选框": {{Rect: geom.Rect{X1: 858, Y1: 562, X2: 902, Y2: 602}, Confidence: 0.95}}}
	dialog := map[string][]vision.TextBlock{"对话框确认": {block("确认", geom.Rect{X1: 990, Y1: 610, X2: 1090, Y2: 650})}}
	scripts := []frameScript{
		{at: base, ocr: dialog, matches: box},
		{at: base.Add(3 * time.Second), ocr: dialog, matches: box},
	}
	session, rt := newTextSession(t, scripts)
	result, actions := step(t, session, rt, 0)
	if result.State != "勾选不再提示" {
		t.Fatalf("第 0 帧 state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 880, Y: 582}}})

	result, actions = step(t, session, rt, 1)
	if result.State != "确认弹窗" {
		t.Fatalf("第 1 帧 state = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1040, Y: 630}}})
}

func TestConfirmDialogFallbackThrottled(t *testing.T) {
	base := time.Date(2026, 11, 1, 9, 20, 0, 0, time.Local)
	box := map[string][]vision.Match{"确认框勾选框": {{Rect: geom.Rect{X1: 858, Y1: 562, X2: 902, Y2: 602}, Confidence: 0.95}}}
	dialog := map[string][]vision.TextBlock{"对话框确认": {block("确认", geom.Rect{X1: 990, Y1: 610, X2: 1090, Y2: 650})}}
	scripts := []frameScript{
		{at: base, ocr: dialog, matches: box},
		{at: base.Add(time.Second), ocr: dialog, matches: box},
		{at: base.Add(3 * time.Second), ocr: dialog, matches: box},
	}
	session, rt := newTextSession(t, scripts)
	if result, _ := step(t, session, rt, 0); result.State != "勾选不再提示" {
		t.Fatalf("第 0 帧 state = %q", result.State)
	}
	// 2 秒内不再重复检查，避免每帧都在弹窗上乱点。
	if result, actions := step(t, session, rt, 1); result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("节流期内不应再点: state = %q actions = %+v", result.State, actions)
	}
	if result, _ := step(t, session, rt, 2); result.State != "确认弹窗" {
		t.Fatalf("超过间隔后应点确认, state = %q", result.State)
	}
}

// drawOptionButton 画一枚过场剧情的描边选项按钮：空心白框 + 框内"文字" + 紧贴左侧的实心按键提示。
func drawOptionButton(img *image.RGBA, box geom.Rect, keycap geom.Rect) {
	fill := func(r geom.Rect) {
		for y := r.Y1; y < r.Y2; y++ {
			for x := r.X1; x < r.X2; x++ {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	fill(geom.Rect{X1: box.X1, Y1: box.Y1, X2: box.X2, Y2: box.Y1 + 2}) // 上描边
	fill(geom.Rect{X1: box.X1, Y1: box.Y2 - 2, X2: box.X2, Y2: box.Y2}) // 下描边
	fill(geom.Rect{X1: box.X1, Y1: box.Y1, X2: box.X1 + 2, Y2: box.Y2}) // 左描边
	fill(geom.Rect{X1: box.X2 - 2, Y1: box.Y1, X2: box.X2, Y2: box.Y2}) // 右描边
	for i := 0; i < 3; i++ {                                            // 框内三个字
		x := box.X1 + 20 + i*30
		fill(geom.Rect{X1: x, Y1: box.Y1 + 13, X2: x + 20, Y2: box.Y1 + 35})
	}
	fill(keycap) // 按键提示是实心小块
}

func stepFrame(t *testing.T, session *Session, frame assist.Frame) (assist.Result, []assist.Action) {
	t.Helper()
	result, actions, err := session.Step(context.Background(), frame)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	return result, actions
}

func TestTextStoryOptionClickedWithoutSkipIcon(t *testing.T) {
	// 过场选项画面：没有跳过图标、没有字幕，只有一枚描边选项按钮，画面几乎全黑。
	box := geom.Rect{X1: 1400, Y1: 637, X2: 1600, Y2: 679}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	drawOptionButton(img, box, geom.Rect{X1: 1343, Y1: 648, X2: 1365, Y2: 669})
	rt := &fakeRuntime{screens: textScreens()}
	session, err := NewFeature(textSpec()).NewSession(rt, assist.Settings{"mode": "skip"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	result, actions := stepFrame(t, session.(*Session), assist.Frame{Image: img, At: time.Now()})
	if result.State != "点击右方选项" {
		t.Fatalf("state = %q actions = %+v, want 点击右方选项", result.State, actions)
	}
	want := geom.Point{X: (box.X1 + box.X2) / 2, Y: (box.Y1 + box.Y2) / 2}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: want}})
}

func TestTextStoryOptionNeedsDarkBackground(t *testing.T) {
	// 同样的描边按钮出现在亮背景上（菜单/背包那种界面）不算剧情选项，什么都不点。
	box := geom.Rect{X1: 1400, Y1: 637, X2: 1600, Y2: 679}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			img.SetRGBA(x, y, color.RGBA{120, 120, 120, 255})
		}
	}
	drawOptionButton(img, box, geom.Rect{X1: 1343, Y1: 648, X2: 1365, Y2: 669})
	rt := &fakeRuntime{screens: textScreens()}
	session, err := NewFeature(textSpec()).NewSession(rt, assist.Settings{"mode": "skip"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	result, actions := stepFrame(t, session.(*Session), assist.Frame{Image: img, At: time.Now()})
	if result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("亮底面上的描边按钮不该被点: state = %q actions = %+v", result.State, actions)
	}
}

func TestTextStoryOptionNeedsDarkScreen(t *testing.T) {
	// 选项区域虽然是黑的，但整帧是亮的（菜单里也可能有黑面板），不算过场黑屏。
	box := geom.Rect{X1: 1400, Y1: 637, X2: 1600, Y2: 679}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			img.SetRGBA(x, y, color.RGBA{200, 200, 200, 255})
		}
	}
	for y := 300; y < 760; y++ {
		for x := 1150; x < 1900; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	drawOptionButton(img, box, geom.Rect{X1: 1343, Y1: 648, X2: 1365, Y2: 669})
	rt := &fakeRuntime{screens: textScreens()}
	session, err := NewFeature(textSpec()).NewSession(rt, assist.Settings{"mode": "skip"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	result, actions := stepFrame(t, session.(*Session), assist.Frame{Image: img, At: time.Now()})
	if result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("整帧不黑时不该点选项: state = %q actions = %+v", result.State, actions)
	}
}

func TestTextStoryOptionIgnoresBrightBlob(t *testing.T) {
	// 过场特效的亮斑（实心、缺描边）不能当成选项，否则会在大世界里乱点。
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 503; y < 514; y++ {
		for x := 1150; x < 1287; x++ {
			img.SetRGBA(x, y, color.RGBA{240, 240, 240, 255})
		}
	}
	rt := &fakeRuntime{screens: textScreens()}
	session, err := NewFeature(textSpec()).NewSession(rt, assist.Settings{"mode": "skip"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	result, actions := stepFrame(t, session.(*Session), assist.Frame{Image: img, At: time.Now()})
	if result.State != "等待剧情画面" || len(actions) != 0 {
		t.Fatalf("亮斑不该被点: state = %q actions = %+v", result.State, actions)
	}
}
