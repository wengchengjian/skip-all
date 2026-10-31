package storyskip

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

type frameScript struct {
	at       time.Time
	ocr      map[string][]vision.TextBlock
	ocrColor map[string][]vision.TextBlock // 带高亮颜色过滤时的结果
	matches  map[string][]vision.Match
	gray     map[string]float64
	colorful map[string]float64
	sat      map[string]float64
}

type fakeRuntime struct {
	screens *screen.Set
	scripts []frameScript
	current int
}

func (f *fakeRuntime) Screen(name string) (*screen.Screen, error) {
	sc, ok := f.screens.Get(name)
	if !ok {
		return nil, fmt.Errorf("未找到画面 %q", name)
	}
	return sc, nil
}

func (f *fakeRuntime) script() frameScript {
	if f.current < 0 || f.current >= len(f.scripts) {
		return frameScript{}
	}
	return f.scripts[f.current]
}

func (f *fakeRuntime) OCR(_ assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	if opts.ColorRange != nil {
		if blocks, ok := f.script().ocrColor[area.Name]; ok {
			return blocks, nil
		}
	}
	return f.script().ocr[area.Name], nil
}

func (f *fakeRuntime) MatchTemplate(_ assist.Frame, area screen.Area, _ vision.TemplateRef, _ float64) ([]vision.Match, error) {
	return f.script().matches[area.Name], nil
}

func (f *fakeRuntime) ColorStats(_ assist.Frame, area screen.Area) (vision.ColorStats, error) {
	colorful, ok := f.script().colorful[area.Name]
	if !ok {
		colorful = 1
	}
	sat, ok := f.script().sat[area.Name]
	if !ok {
		sat = 255
	}
	return vision.ColorStats{GrayMaskPercent: f.script().gray[area.Name], ColorfulPercent: colorful, MeanSaturation: sat}, nil
}

func (f *fakeRuntime) Log() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testScreens() *screen.Set {
	set := screen.NewSet()
	set.Add(&screen.Screen{
		ID: "commission_assistant", Name: "委托助手",
		AreaList: []*screen.Area{
			{Name: "对话框内容", RectValues: []int{418, 831, 1500, 980}},
			{Name: "右侧选项区域", RectValues: []int{1438, 536, 1914, 830}, ColorRange: &[2][3]uint8{{240, 240, 240}, {255, 255, 255}}},
			{Name: "中间选项区域", RectValues: []int{520, 245, 1365, 770}, TemplateSubDir: "commission_assistant", TemplateID: "1"},
			{Name: "对话框确认", RectValues: []int{1022, 544, 1238, 750}, Text: "确认", LCSPercent: 1},
			{Name: "左上角返回", RectValues: []int{82, 13, 150, 90}, TemplateSubDir: "menu", TemplateID: "back"},
			{Name: "战斗-菜单", RectValues: []int{58, 40, 142, 110}, TemplateSubDir: "battle", TemplateID: "btn_menu"},
			{Name: "文本-剧情右上角", RectValues: []int{1680, 20, 1840, 165}},
			{Name: "按钮-自动", RectValues: []int{1686, 186, 1838, 242}, Text: "自动"},
		},
	})
	set.Add(&screen.Screen{
		ID: "battle", Name: "战斗画面",
		AreaList: []*screen.Area{
			{Name: "按键-交互", RectValues: []int{1403, 916, 1505, 1018}, TemplateSubDir: "battle", TemplateID: "btn_interact"},
		},
	})
	set.Add(&screen.Screen{
		ID: "normal_world_investigation", Name: "大世界-勘域",
		AreaList: []*screen.Area{
			{Name: "勘域-菜单", RectValues: []int{62, 40, 139, 110}, TemplateSubDir: "normal_world_investigation", TemplateID: "menu"},
		},
	})
	return set
}

func testSpec() Spec {
	return Spec{
		Screen:           "委托助手",
		StoryArea:        "文本-剧情右上角",
		ConfirmArea:      "对话框确认",
		DialogArea:       "对话框内容",
		RightOptionArea:  "右侧选项区域",
		MiddleOptionArea: "中间选项区域",
		AutoButtonArea:   "按钮-自动",
		ConfirmText:      "确认",
		SkipKeyword:      "跳过",
		MenuKeyword:      "菜单",
		AutoKeyword:      "自动",
		Guards: []Guard{
			{Area: "左上角返回", Kind: GuardTemplate},
			{Area: "战斗-菜单", Kind: GuardTemplate},
			{Screen: "大世界-勘域", Area: "勘域-菜单", Kind: GuardTemplate},
			{Screen: "战斗画面", Area: "按键-交互", Kind: GuardTemplate},
		},
		MenuQuiet:         5 * time.Second,
		SkipClickWait:     400 * time.Millisecond,
		GuardWait:         time.Second,
		GrayMaskThreshold: 0.9,
		RecentClickLimit:  3,
	}
}

func newTestSession(t *testing.T, settings assist.Settings, scripts []frameScript) (*Session, *fakeRuntime) {
	t.Helper()
	rt := &fakeRuntime{screens: testScreens(), scripts: scripts}
	session, err := NewFeature(testSpec()).NewSession(rt, settings)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return session.(*Session), rt
}

func testFrame() assist.Frame {
	return assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080))}
}

// frameForScript 把脚本里的 OCR 文字块画成白色矩形，供文字行检测使用。
func frameForScript(script frameScript) assist.Frame {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for areaName, blocks := range script.ocr {
		if script.ocrColor != nil {
			if colored, ok := script.ocrColor[areaName]; ok {
				blocks = colored
			}
		}
		for _, block := range blocks {
			for y := block.Rect.Y1; y < block.Rect.Y2; y++ {
				for x := block.Rect.X1; x < block.Rect.X2; x++ {
					img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
				}
			}
		}
	}
	return assist.Frame{Image: img, At: script.at}
}

func step(t *testing.T, session *Session, rt *fakeRuntime, index int) (assist.Result, []assist.Action) {
	t.Helper()
	if index < 0 || index >= len(rt.scripts) {
		t.Fatalf("帧序号 %d 超出脚本范围 %d", index, len(rt.scripts))
	}
	rt.current = index
	frame := frameForScript(rt.scripts[index])
	result, actions, err := session.Step(context.Background(), frame)
	if err != nil {
		t.Fatalf("第 %d 帧执行失败: %v", index, err)
	}
	return result, actions
}

func block(text string, rect geom.Rect) vision.TextBlock {
	return vision.TextBlock{Text: text, Rect: rect}
}

func checkActions(t *testing.T, got, want []assist.Action) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("动作数量 = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("动作[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func checkWait(t *testing.T, got, want time.Duration) {
	t.Helper()
	if got != want {
		t.Fatalf("等待时间 = %v, want %v", got, want)
	}
}

func TestDefaultSettings(t *testing.T) {
	feature := NewFeature(testSpec())
	if feature.ID() != "story_skip" {
		t.Fatalf("ID = %q", feature.ID())
	}
	settings := feature.DefaultSettings()
	if settings[SettingMode] != string(ModeSkip) {
		t.Fatalf("mode = %v", settings[SettingMode])
	}
	if settings[SettingDialogOption] != string(OptionLast) {
		t.Fatalf("dialog_option = %v", settings[SettingDialogOption])
	}
	if settings[SettingDialogClickInterval] != 0.5 || settings[SettingOptionClickInterval] != 0.1 || settings[SettingSleepAfterEmpty] != 0.5 {
		t.Fatalf("默认间隔不正确: %+v", settings)
	}
	if err := feature.Validate(nil); err != nil {
		t.Fatalf("默认设置校验失败: %v", err)
	}
	if err := feature.Validate(assist.Settings{SettingMode: "bad"}); err == nil {
		t.Fatal("非法模式应校验失败")
	}
	if err := feature.Validate(assist.Settings{SettingDialogClickInterval: -1}); err == nil {
		t.Fatal("负间隔应校验失败")
	}
}

func TestSkipAndConfirm(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	scripts := []frameScript{
		{
			at: base,
			ocr: map[string][]vision.TextBlock{
				"文本-剧情右上角": {block("跳过", geom.Rect{X1: 1680, Y1: 50, X2: 1760, Y2: 100})},
				"对话框内容":    {block("你好", geom.Rect{X1: 500, Y1: 850, X2: 600, Y2: 900})},
			},
			gray: map[string]float64{"对话框内容": 0.95},
		},
		{
			at: base.Add(time.Second),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("确认", geom.Rect{X1: 1022, Y1: 544, X2: 1238, Y2: 750})},
			},
		},
	}
	session, rt := newTestSession(t, nil, scripts)

	result, actions := step(t, session, rt, 0)
	checkWait(t, result.Wait, 400*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1720, Y: 75}}})

	result, actions = step(t, session, rt, 1)
	checkWait(t, result.Wait, 100*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1130, Y: 647}}})
}

func TestConfirmWaitsForRealButton(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	scripts := []frameScript{
		{
			at: base,
			ocr: map[string][]vision.TextBlock{
				"文本-剧情右上角": {block("跳过", geom.Rect{X1: 1680, Y1: 50, X2: 1760, Y2: 100})},
			},
		},
		{
			at: base.Add(time.Second),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("取消", geom.Rect{X1: 1022, Y1: 544, X2: 1238, Y2: 750})},
			},
		},
		{
			at: base.Add(2 * time.Second),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("确认", geom.Rect{X1: 1022, Y1: 544, X2: 1238, Y2: 750})},
			},
		},
	}
	session, rt := newTestSession(t, nil, scripts)

	_, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1720, Y: 75}}})

	result, actions := step(t, session, rt, 1)
	if result.State != "等待确认框" || len(actions) != 0 {
		t.Fatalf("确认框未出现时不应点击: state=%q actions=%+v", result.State, actions)
	}

	result, actions = step(t, session, rt, 2)
	if result.State != "确认跳过" {
		t.Fatalf("状态 = %q", result.State)
	}
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1130, Y: 647}}})
}

func TestMenuThenSkipThenConfirm(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	scripts := []frameScript{
		{
			at: base,
			ocr: map[string][]vision.TextBlock{
				"文本-剧情右上角": {block("菜单", geom.Rect{X1: 1700, Y1: 40, X2: 1760, Y2: 80})},
			},
		},
		{
			at: base.Add(time.Second),
			ocr: map[string][]vision.TextBlock{
				"文本-剧情右上角": {block("跳过", geom.Rect{X1: 1700, Y1: 120, X2: 1800, Y2: 160})},
			},
		},
		{
			at: base.Add(2 * time.Second),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("确认", geom.Rect{X1: 1022, Y1: 544, X2: 1238, Y2: 750})},
			},
		},
	}
	session, rt := newTestSession(t, nil, scripts)

	result, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1730, Y: 60}}})
	if result.State != "展开剧情菜单" {
		t.Fatalf("菜单状态 = %q", result.State)
	}

	result, actions = step(t, session, rt, 1)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1750, Y: 140}}})
	if result.State != "点击跳过" {
		t.Fatalf("跳过状态 = %q", result.State)
	}

	result, actions = step(t, session, rt, 2)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1130, Y: 647}}})
	if result.State != "确认跳过" {
		t.Fatalf("确认状态 = %q", result.State)
	}
}

func TestGamepadMenuSkipConfirm(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	scripts := []frameScript{
		{
			at: base,
			ocr: map[string][]vision.TextBlock{
				"文本-剧情右上角": {block("菜单", geom.Rect{X1: 1700, Y1: 40, X2: 1760, Y2: 80})},
			},
		},
		{at: base.Add(500 * time.Millisecond)},
		{
			at: base.Add(time.Second),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("确认", geom.Rect{X1: 1022, Y1: 544, X2: 1238, Y2: 750})},
			},
		},
		{
			at: base.Add(1500 * time.Millisecond),
			ocr: map[string][]vision.TextBlock{
				"文本-剧情右上角": {block("跳过", geom.Rect{X1: 1720, Y1: 40, X2: 1800, Y2: 80})},
			},
		},
		{
			at: base.Add(2 * time.Second),
			ocr: map[string][]vision.TextBlock{
				"对话框确认": {block("确认", geom.Rect{X1: 1022, Y1: 544, X2: 1238, Y2: 750})},
			},
		},
	}
	session, rt := newTestSession(t, assist.Settings{SettingMode: "skip", SettingGamepad: true}, scripts)
	// 单独测激活流程，避免首帧 A 打乱菜单时序。
	result, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: padActivateKey}})
	if result.State != "激活手柄模式" {
		t.Fatalf("激活状态 = %q", result.State)
	}
	session.padReady = true

	_, actions = step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: padStartKey}})

	_, actions = step(t, session, rt, 1)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}})

	result, actions = step(t, session, rt, 2)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}})
	if result.State != "确认跳过" {
		t.Fatalf("菜单确认状态 = %q", result.State)
	}

	_, actions = step(t, session, rt, 3)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: padStartKey}})

	result, actions = step(t, session, rt, 4)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}})
	if result.State != "确认跳过" {
		t.Fatalf("普通跳过确认状态 = %q", result.State)
	}
}

func TestMenuQuiet(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	dialog := frameScript{
		ocr: map[string][]vision.TextBlock{
			"对话框内容": {block("你好", geom.Rect{X1: 500, Y1: 850, X2: 600, Y2: 900})},
		},
		gray: map[string]float64{"对话框内容": 0.95},
	}
	first := dialog
	first.at = base
	first.ocr = map[string][]vision.TextBlock{
		"文本-剧情右上角": {block("菜单", geom.Rect{X1: 1700, Y1: 40, X2: 1760, Y2: 80})},
		"对话框内容":    {block("你好", geom.Rect{X1: 500, Y1: 850, X2: 600, Y2: 900})},
	}
	quiet := dialog
	quiet.at = base.Add(2 * time.Second)
	after := dialog
	after.at = base.Add(6 * time.Second)
	scripts := []frameScript{first, quiet, after}
	session, rt := newTestSession(t, nil, scripts)

	_, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1730, Y: 60}}})

	result, actions := step(t, session, rt, 1)
	if len(actions) != 0 {
		t.Fatalf("菜单静默期不应点击，实际 %+v", actions)
	}
	if result.State != "等待剧情菜单动画" {
		t.Fatalf("状态 = %q", result.State)
	}
	checkWait(t, result.Wait, 100*time.Millisecond)

	result, actions = step(t, session, rt, 2)
	checkWait(t, result.Wait, 500*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClickCurrent}})
}

func TestGuardsDoNotClick(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		area string
	}{
		{"左上角返回", "左上角返回"},
		{"战斗-菜单", "战斗-菜单"},
		{"勘域-菜单", "勘域-菜单"},
		{"按键-交互", "按键-交互"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := frameScript{
				at: base,
				ocr: map[string][]vision.TextBlock{
					"对话框内容": {block("你好", geom.Rect{X1: 500, Y1: 850, X2: 600, Y2: 900})},
				},
				matches: map[string][]vision.Match{
					tc.area: {{Rect: geom.Rect{X1: 60, Y1: 40, X2: 140, Y2: 110}}},
				},
				gray: map[string]float64{"对话框内容": 0.95},
			}
			session, rt := newTestSession(t, nil, []frameScript{script})
			result, actions := step(t, session, rt, 0)
			if len(actions) != 0 {
				t.Fatalf("守卫命中不应点击，实际 %+v", actions)
			}
			checkWait(t, result.Wait, time.Second)
		})
	}
}

func TestTextGuard(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	spec := testSpec()
	spec.Guards = []Guard{{Area: "左上角返回", Kind: GuardText, Text: "返回"}}
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"左上角返回": {block("返回", geom.Rect{X1: 90, Y1: 30, X2: 140, Y2: 70})},
		},
	}
	rt := &fakeRuntime{screens: testScreens(), scripts: []frameScript{script}}
	session, err := NewFeature(spec).NewSession(rt, nil)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	result, actions := step(t, session.(*Session), rt, 0)
	if len(actions) != 0 {
		t.Fatalf("文本守卫命中不应点击，实际 %+v", actions)
	}
	checkWait(t, result.Wait, time.Second)
}

func TestDialogClick(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"对话框内容": {block("这是对话", geom.Rect{X1: 500, Y1: 850, X2: 700, Y2: 900})},
		},
		gray: map[string]float64{"对话框内容": 0.95},
	}
	session, rt := newTestSession(t, nil, []frameScript{script})
	result, actions := step(t, session, rt, 0)
	checkWait(t, result.Wait, 500*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClickCurrent}})
}

func TestGraySkipIsIgnored(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"文本-剧情右上角": {block("跳过", geom.Rect{X1: 1720, Y1: 40, X2: 1800, Y2: 80})},
		},
		ocrColor: map[string][]vision.TextBlock{"文本-剧情右上角": nil},
	}
	session, rt := newTestSession(t, nil, []frameScript{script})
	rt.current = 0
	frame := frameForScript(script)
	step, _, err := session.handleStoryBar(frame)
	if err != nil {
		t.Fatal(err)
	}
	if step != nil {
		t.Fatalf("灰色跳过不应产生点击: %+v", step)
	}
}

func TestWhiteSkipClicks(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	white := block("跳过", geom.Rect{X1: 1720, Y1: 40, X2: 1800, Y2: 80})
	script := frameScript{
		at:       base,
		ocr:      map[string][]vision.TextBlock{"文本-剧情右上角": {white}},
		ocrColor: map[string][]vision.TextBlock{"文本-剧情右上角": {white}},
	}
	session, rt := newTestSession(t, nil, []frameScript{script})
	rt.current = 0
	frame := frameForScript(script)
	step, _, err := session.handleStoryBar(frame)
	if err != nil {
		t.Fatal(err)
	}
	if step == nil || len(step.actions) != 1 || step.actions[0].Point != white.Rect.Center() {
		t.Fatalf("白色跳过应点击: %+v", step)
	}
}

func TestDialogOptions(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	options := []vision.TextBlock{
		block("第一个", geom.Rect{X1: 1500, Y1: 580, X2: 1800, Y2: 620}),
		block("第二个", geom.Rect{X1: 1500, Y1: 680, X2: 1800, Y2: 720}),
		block("第三个", geom.Rect{X1: 1500, Y1: 780, X2: 1800, Y2: 820}),
	}
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"对话框内容":  {block("选择吧", geom.Rect{X1: 500, Y1: 850, X2: 700, Y2: 900})},
			"右侧选项区域": options,
		},
		gray: map[string]float64{"对话框内容": 0.95},
	}

	t.Run("默认最后一个", func(t *testing.T) {
		session, rt := newTestSession(t, nil, []frameScript{script})
		result, actions := step(t, session, rt, 0)
		checkWait(t, result.Wait, 100*time.Millisecond)
		checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1650, Y: 800}}})
	})

	t.Run("配置第一个", func(t *testing.T) {
		session, rt := newTestSession(t, assist.Settings{SettingDialogOption: "first"}, []frameScript{script})
		_, actions := step(t, session, rt, 0)
		checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1650, Y: 600}}})
	})

	t.Run("中间选项", func(t *testing.T) {
		centerScript := frameScript{
			at: base,
			ocr: map[string][]vision.TextBlock{
				"对话框内容":  {block("选择吧", geom.Rect{X1: 500, Y1: 850, X2: 700, Y2: 900})},
				"中间选项区域": {block("甲", geom.Rect{X1: 700, Y1: 400, X2: 900, Y2: 440}), block("乙", geom.Rect{X1: 700, Y1: 500, X2: 900, Y2: 540})},
			},
			gray: map[string]float64{"对话框内容": 0.95},
		}
		session, rt := newTestSession(t, assist.Settings{SettingMode: "auto"}, []frameScript{centerScript})
		_, actions := step(t, session, rt, 0)
		checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 800, Y: 520}}})
	})
}

func TestGraySkipFallsThroughToOptions(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"文本-剧情右上角": {block("跳过", geom.Rect{X1: 1720, Y1: 40, X2: 1800, Y2: 80})},
			"右侧选项区域": {
				block("第一个", geom.Rect{X1: 1500, Y1: 580, X2: 1800, Y2: 620}),
				block("第二个", geom.Rect{X1: 1500, Y1: 680, X2: 1800, Y2: 720}),
			},
		},
		ocrColor: map[string][]vision.TextBlock{"文本-剧情右上角": nil},
	}
	session, rt := newTestSession(t, nil, []frameScript{script})
	_, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1650, Y: 700}}})
}

func TestOptionsWithoutDialog(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"右侧选项区域": {
				block("第一个", geom.Rect{X1: 1500, Y1: 580, X2: 1800, Y2: 620}),
				block("第二个", geom.Rect{X1: 1500, Y1: 680, X2: 1800, Y2: 720}),
			},
		},
	}
	session, rt := newTestSession(t, nil, []frameScript{script})
	_, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1650, Y: 700}}})
}

func TestRecentDialogClicksStop(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	dialog := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"对话框内容": {block("你好", geom.Rect{X1: 500, Y1: 850, X2: 600, Y2: 900})},
		},
		gray: map[string]float64{"对话框内容": 0.95},
	}
	empty := frameScript{at: base.Add(time.Second)}
	scripts := []frameScript{dialog, empty, empty, empty, empty}
	session, rt := newTestSession(t, nil, scripts)

	_, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClickCurrent}})

	for i := 1; i <= 3; i++ {
		result, actions := step(t, session, rt, i)
		checkWait(t, result.Wait, 200*time.Millisecond)
		checkActions(t, actions, []assist.Action{{Kind: assist.ActionClickCurrent}})
	}

	result, actions := step(t, session, rt, 4)
	if len(actions) != 0 {
		t.Fatalf("超过 3 帧后不应再点击，实际 %+v", actions)
	}
	checkWait(t, result.Wait, 500*time.Millisecond)
}

func TestWaitAfterEmptyScreen(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	session, rt := newTestSession(t, nil, []frameScript{{at: base}})
	result, actions := step(t, session, rt, 0)
	if len(actions) != 0 {
		t.Fatalf("空画面不应点击，实际 %+v", actions)
	}
	if result.State != "等待剧情画面" {
		t.Fatalf("状态 = %q", result.State)
	}
	checkWait(t, result.Wait, 500*time.Millisecond)
}

func TestAutoMode(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	scripts := []frameScript{
		{
			at: base,
			matches: map[string][]vision.Match{
				"中间选项区域": {{Rect: geom.Rect{X1: 700, Y1: 400, X2: 800, Y2: 500}}},
			},
		},
		{
			at:  base.Add(time.Second),
			ocr: map[string][]vision.TextBlock{"文本-剧情右上角": {block("自动", geom.Rect{X1: 1700, Y1: 80, X2: 1760, Y2: 120})}},
		},
		{
			at:  base.Add(2 * time.Second),
			ocr: map[string][]vision.TextBlock{"文本-剧情右上角": {block("菜单", geom.Rect{X1: 1700, Y1: 40, X2: 1760, Y2: 80})}},
		},
		{
			at:  base.Add(3 * time.Second),
			ocr: map[string][]vision.TextBlock{"按钮-自动": {block("自动", geom.Rect{X1: 1700, Y1: 190, X2: 1800, Y2: 230})}},
		},
	}
	session, rt := newTestSession(t, assist.Settings{SettingMode: "auto"}, scripts)

	_, actions := step(t, session, rt, 0)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 750, Y: 450}}})

	result, actions := step(t, session, rt, 1)
	if len(actions) != 0 {
		t.Fatalf("自动播放中不应点击，实际 %+v", actions)
	}
	checkWait(t, result.Wait, time.Second)

	result, actions = step(t, session, rt, 2)
	checkWait(t, result.Wait, 100*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1730, Y: 60}}})

	result, actions = step(t, session, rt, 3)
	checkWait(t, result.Wait, 100*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClick, Point: geom.Point{X: 1750, Y: 210}}})
}

func TestClickMode(t *testing.T) {
	base := time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local)
	script := frameScript{
		at: base,
		ocr: map[string][]vision.TextBlock{
			"对话框内容": {block("你好", geom.Rect{X1: 500, Y1: 850, X2: 600, Y2: 900})},
		},
		gray: map[string]float64{"对话框内容": 0.95},
	}
	session, rt := newTestSession(t, assist.Settings{SettingMode: "click"}, []frameScript{script})
	result, actions := step(t, session, rt, 0)
	checkWait(t, result.Wait, 500*time.Millisecond)
	checkActions(t, actions, []assist.Action{{Kind: assist.ActionClickCurrent}})
}

func TestSpecValidation(t *testing.T) {
	feature := NewFeature(Spec{})
	if _, err := feature.NewSession(&fakeRuntime{screens: testScreens()}, nil); err == nil {
		t.Fatal("缺少区域配置应创建会话失败")
	}
}
