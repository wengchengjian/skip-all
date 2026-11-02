package storyskip

import (
	"fmt"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// DialogMode 是"画面判读"的抽象层：状态机只问它几个问题，具体游戏换实现。
//   - MaskMode：绝区零那种有灰底对话框的界面，靠灰底遮罩 + 中文正文判断。
//   - TextMode：鸣潮那种没有对话框底板、只有白色字幕的界面。
//
// 状态机流程（确认框优先 / 点跳过 / 守卫 / 门控 / 推进）对所有游戏是同一份代码。
type DialogMode interface {
	// InDialog 报告当前是否在对话/剧情里。
	InDialog(s *Session, frame assist.Frame) (bool, error)
	// FastDialog 是不跑 OCR 的廉价判断，用于快速触发。
	FastDialog(s *Session, frame assist.Frame) (bool, error)
	// StoryEntry 报告画面里是否真有剧情 UI（跳过按钮等）。
	StoryEntry(s *Session, frame assist.Frame) (bool, error)
	// Advance 返回推进对话的动作。
	Advance(s *Session, frame assist.Frame) ([]assist.Action, error)
	// AllowOptions 报告当前是否允许点对话选项。
	AllowOptions(inDialog, storyVisible bool) bool
	// SkipStoryBarOCR 报告剧情栏是否只用模板、不跑 OCR。
	SkipStoryBarOCR() bool
	// DetectsOptionButtons 报告该游戏的剧情选项是不是“描边按钮”样式（鸣潮过场选项）。
	// 为真时按按钮外形定位点击点，避免把画面里的亮斑当成选项。
	DetectsOptionButtons() bool
}

// MaskMode 是有灰底对话框的界面（绝区零）。
type MaskMode struct{}

var _ DialogMode = MaskMode{}

// InDialog 用灰底遮罩 + 中文正文判断，避免把 CG/过场里的深色区域当对话框。
func (MaskMode) InDialog(s *Session, frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.DialogArea)
	if err != nil {
		return false, err
	}
	hash := areaHash(frame.Image, area.Rect)
	if s.cfg.mode == ModeSkip && s.dialogCached && hash == s.dialogHash && !frame.At.Before(s.dialogAt) {
		return s.dialogIn, nil
	}
	stats, err := s.rt.ColorStats(frame, *area)
	if err != nil {
		return false, fmt.Errorf("统计对话框颜色: %w", err)
	}
	if stats.GrayMaskPercent <= s.spec.GrayMaskThreshold {
		s.storeDialogResult(frame.At, hash, false)
		return false, nil
	}
	blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, 300*time.Millisecond)
	if err != nil {
		return false, fmt.Errorf("识别对话内容: %w", err)
	}
	for _, block := range blocks {
		if containsChinese(block.Text) {
			s.storeDialogResult(frame.At, hash, true)
			return true, nil
		}
	}
	s.storeDialogResult(frame.At, hash, false)
	return false, nil
}

// FastDialog 只看灰底 + 白色正文，不跑 OCR。
func (MaskMode) FastDialog(s *Session, frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.DialogArea)
	if err != nil {
		return false, err
	}
	stats, err := s.rt.ColorStats(frame, *area)
	if err != nil {
		return false, err
	}
	if stats.GrayMaskPercent <= s.spec.GrayMaskThreshold {
		return false, nil
	}
	bands := vision.FindTextBands(frame.Image, area.Rect, vision.TextBandOptions{
		ColorRange: &[2][3]uint8{{170, 170, 170}, {255, 255, 255}},
		MinHeight:  8,
	})
	for _, band := range bands {
		if band.X2-band.X1 >= 20 {
			return true, nil
		}
	}
	return false, nil
}

// StoryEntry 用"菜单或跳过按钮"判断剧情界面；没配菜单区域时按可见处理。
func (MaskMode) StoryEntry(s *Session, frame assist.Frame) (bool, error) {
	return s.storyEntryVisible(frame), nil
}

// Advance 沿用"点当前光标"：绝区零的对话点屏幕任意位置都能推进。
func (MaskMode) Advance(s *Session, frame assist.Frame) ([]assist.Action, error) {
	return clickCurrent(), nil
}

// AllowOptions 绝区零的选项区域只出现在对话框里，不需要额外门控。
func (MaskMode) AllowOptions(inDialog, storyVisible bool) bool { return true }

// SkipStoryBarOCR 绝区零的跳过/菜单是文字，必须 OCR。
func (MaskMode) SkipStoryBarOCR() bool { return false }

// DetectsOptionButtons 绝区零的选项是对话框里的文字行，没有描边按钮。
func (MaskMode) DetectsOptionButtons() bool { return false }

// TextMode 是没有对话框底板、只有白色字幕的界面（鸣潮）。
type TextMode struct{}

var _ DialogMode = TextMode{}

// InDialog 要求同时有字幕行和剧情 UI 证据：大世界底部也有白字（交互/任务提示），
// 只看字幕会把它们当成对话。过场剧情只有描边选项按钮、没有跳过图标，也算在剧情里。
func (TextMode) InDialog(s *Session, frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.DialogArea)
	if err != nil {
		return false, err
	}
	return (s.subtitleVisible(frame, *area) && s.storyEntryStrict(frame)) || s.storyOptionVisible(frame), nil
}

// FastDialog 同 InDialog，但不跑 OCR。
func (TextMode) FastDialog(s *Session, frame assist.Frame) (bool, error) {
	return TextMode{}.InDialog(s, frame)
}

// StoryEntry 要求模板真的命中剧情 UI，不吃"没配置菜单区域就算可见"的默认值；
// 过场选项画面是唯一例外（这种画面根本不画跳过图标）。
func (TextMode) StoryEntry(s *Session, frame assist.Frame) (bool, error) {
	return s.storyEntryStrict(frame) || s.storyOptionVisible(frame), nil
}

// Advance 点字幕区域推进；没配区域时退回点当前光标。
func (TextMode) Advance(s *Session, frame assist.Frame) ([]assist.Action, error) {
	if s.spec.DialogClickArea == "" {
		return clickCurrent(), nil
	}
	area, err := s.area(s.spec.Screen, s.spec.DialogClickArea)
	if err != nil {
		return nil, err
	}
	return []assist.Action{{Kind: assist.ActionClick, Point: area.Rect.Center()}}, nil
}

// AllowOptions 只有确实在剧情里才允许点选项：右侧白字在背包/角色页也有。
func (TextMode) AllowOptions(inDialog, storyVisible bool) bool {
	return inDialog || storyVisible
}

// SkipStoryBarOCR 鸣潮的跳过是图标，只用模板，不为剧情栏加载 OCR。
func (TextMode) SkipStoryBarOCR() bool { return true }

// DetectsOptionButtons 鸣潮的过场选项是描边按钮（「（睁开眼睛）」那种），要按外形找。
func (TextMode) DetectsOptionButtons() bool { return true }
