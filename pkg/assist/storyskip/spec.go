// Package storyskip 实现与具体游戏解耦的剧情跳过状态机。
package storyskip

import "time"

// Mode 是剧情推进模式。
type Mode string

const (
	// ModeSkip 跳过剧情：点击跳过按钮并确认。
	ModeSkip Mode = "skip"
	// ModeAuto 等待剧情自动播放。
	ModeAuto Mode = "auto"
	// ModeClick 只自动点击对话。
	ModeClick Mode = "click"
)

// DialogOption 是对话选项的选择策略。
type DialogOption string

const (
	// OptionFirst 选择最上方的选项。
	OptionFirst DialogOption = "first"
	// OptionLast 选择最下方的选项。
	OptionLast DialogOption = "last"
)

// GuardKind 是守卫区域的识别方式。
type GuardKind int

const (
	// GuardTemplate 通过模板匹配识别守卫区域。
	GuardTemplate GuardKind = iota
	// GuardText 通过 OCR 关键词识别守卫区域。
	GuardText
)

// Guard 是命中后只等待、不点击的守卫区域。
type Guard struct {
	Screen string        // 画面名；为空时使用 Spec.Screen
	Area   string        // 区域名
	Kind   GuardKind     // 识别方式，默认模板匹配
	Text   string        // Kind 为 GuardText 时使用的关键词
	Wait   time.Duration // 命中后的等待，默认使用 Spec.GuardWait
}

// Spec 描述一个游戏的剧情跳过界面。
type Spec struct {
	Screen           string // 主画面名
	StoryArea        string // 剧情右上角菜单区域
	ConfirmArea      string // 跳过/确认弹窗的按钮区域
	DialogArea       string // 对话框内容区域
	RightOptionArea  string // 右侧对话选项区域
	MiddleOptionArea string // 中间对话选项区域
	AutoButtonArea   string // 展开菜单后的自动按钮区域，可留空

	Keywords    []string // 剧情右上角可见关键词，默认“菜单/跳过/自动”
	ConfirmText string   // 确认按钮关键词，默认“确认”
	SkipKeyword string   // 跳过关键词，默认“跳过”
	MenuKeyword string   // 菜单关键词，默认“菜单”
	AutoKeyword string   // 自动关键词，默认“自动”
	// ButtonColorRange 是可用按钮/选项的高亮颜色范围，默认白色。
	// 灰色“跳过”是禁用态，必须过滤掉，否则会一直点它。
	ButtonColorRange [2][3]uint8

	Guards []Guard // 命中后等待、不点击的区域

	MenuQuiet         time.Duration // 点击菜单后的静默时间，默认 5s
	SkipClickWait     time.Duration // 点击跳过后的等待，默认 400ms
	GuardWait         time.Duration // 守卫区域的默认等待，默认 1s
	GrayMaskThreshold float64       // 对话框 gray mask 判定阈值，默认 0.9
	RecentClickLimit  int           // 对话消失后的补点帧数，默认 3
}

func (s Spec) withDefaults() Spec {
	if s.ConfirmText == "" {
		s.ConfirmText = "确认"
	}
	if s.SkipKeyword == "" {
		s.SkipKeyword = "跳过"
	}
	if s.MenuKeyword == "" {
		s.MenuKeyword = "菜单"
	}
	if s.AutoKeyword == "" {
		s.AutoKeyword = "自动"
	}
	if len(s.Keywords) == 0 {
		s.Keywords = []string{s.MenuKeyword, s.SkipKeyword, s.AutoKeyword}
	}
	if s.ButtonColorRange == [2][3]uint8{} {
		s.ButtonColorRange = [2][3]uint8{{240, 240, 240}, {255, 255, 255}}
	}
	if s.MenuQuiet <= 0 {
		s.MenuQuiet = 5 * time.Second
	}
	if s.SkipClickWait <= 0 {
		s.SkipClickWait = 400 * time.Millisecond
	}
	if s.GuardWait <= 0 {
		s.GuardWait = time.Second
	}
	if s.GrayMaskThreshold <= 0 {
		s.GrayMaskThreshold = 0.9
	}
	if s.RecentClickLimit <= 0 {
		s.RecentClickLimit = 3
	}
	return s
}

func (s Spec) validate() error {
	for _, item := range []struct {
		name  string
		value string
	}{
		{"Screen", s.Screen},
		{"StoryArea", s.StoryArea},
		{"ConfirmArea", s.ConfirmArea},
		{"DialogArea", s.DialogArea},
		{"RightOptionArea", s.RightOptionArea},
		{"MiddleOptionArea", s.MiddleOptionArea},
	} {
		if item.value == "" {
			return &SpecError{Field: item.name}
		}
	}
	return nil
}

// SpecError 表示剧情跳过配置缺少必要字段。
type SpecError struct {
	Field string
}

func (e *SpecError) Error() string { return "剧情跳过配置缺少 " + e.Field }
