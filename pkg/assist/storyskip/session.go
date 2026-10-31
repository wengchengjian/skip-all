package storyskip

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// 状态机内部等待时长。
const (
	confirmWait      = 100 * time.Millisecond
	storyButtonWait  = 100 * time.Millisecond
	autoPlayingWait  = time.Second
	recentDialogWait = 200 * time.Millisecond
	chosenOptionHold = 500 * time.Millisecond
	chosenOptionMax  = 2 * time.Second
)

// Session 是剧情跳过状态机。
type Session struct {
	rt   assist.Runtime
	spec Spec
	cfg  sessionSettings

	menuClickedAt  time.Time
	chosenOption   string
	chosenOptionAt time.Time
	chosenPoint    geom.Point
	lastOptions    []string
	recentClicks   int
}

var _ assist.Session = (*Session)(nil)

func newSession(rt assist.Runtime, spec Spec, cfg sessionSettings) *Session {
	return &Session{rt: rt, spec: spec, cfg: cfg}
}

// Close 释放会话资源。
func (s *Session) Close() error { return nil }

// Step 处理一帧画面，返回本帧应执行的动作。
func (s *Session) Step(_ context.Context, frame assist.Frame) (assist.Result, []assist.Action, error) {
	if frame.Image == nil {
		return assist.Result{State: "等待剧情画面", Wait: s.cfg.sleepAfterEmpty}, nil, nil
	}

	// 1. 确认框：跳过按钮弹出的确认优先处理。
	actions, handled, err := s.clickConfirm(frame)
	if err != nil {
		return assist.Result{}, nil, err
	}
	if handled {
		s.resetAfterConfirm()
		return assist.Result{State: "确认跳过", Wait: confirmWait}, actions, nil
	}

	// 2. 剧情右上角：跳过 / 菜单 / 自动。
	storyVisible := false
	if s.cfg.mode == ModeClick {
		visible, err := s.storyBarVisible(frame)
		if err != nil {
			return assist.Result{}, nil, err
		}
		storyVisible = visible
	} else {
		step, visible, err := s.handleStoryBar(frame)
		if err != nil {
			return assist.Result{}, nil, err
		}
		storyVisible = visible
		if step != nil {
			result, stepActions := step.unpack()
			return result, stepActions, nil
		}
	}

	// 3. 点击菜单后的 5s 静默：等跳过键和确认框，不处理对话框。
	if s.inMenuQuiet(frame.At) {
		return assist.Result{State: "等待剧情菜单动画", Wait: storyButtonWait}, nil, nil
	}

	// 4. 守卫：返回/战斗菜单/勘域菜单/交互键命中时只等待，不点击。
	if name, wait, ok, err := s.hitGuard(frame); err != nil {
		return assist.Result{}, nil, err
	} else if ok {
		s.recentClicks = 0
		return assist.Result{State: "等待:" + name, Wait: wait}, nil, nil
	}

	// 5. 对话框：gray mask + 中文。
	inDialog, err := s.inDialog(frame)
	if err != nil {
		return assist.Result{}, nil, err
	}
	// 自动/点击模式只要还在剧情界面就继续点，跳过模式必须有对话框特征。
	if !inDialog && s.cfg.mode != ModeSkip && storyVisible {
		inDialog = true
	}

	if inDialog {
		// 6. 选项选择优先于正文点击。
		actions, ok, err := s.clickOption(frame, frame.At)
		if err != nil {
			return assist.Result{}, nil, err
		}
		if ok {
			return assist.Result{State: "点击对话选项", Wait: s.cfg.optionClickInterval}, actions, nil
		}
		// 7. 点击对话框推进。
		s.recentClicks = s.spec.RecentClickLimit
		return assist.Result{State: "对话中点击", Wait: s.cfg.dialogClickInterval}, clickCurrent(), nil
	}

	// 8. 最近对话连续点击：内容为“……”时 OCR 识别不到，最多补点 N 帧。
	if s.recentClicks > 0 {
		s.recentClicks--
		return assist.Result{State: "点击最近对话", Wait: recentDialogWait}, clickCurrent(), nil
	}

	// 9. 没有匹配到剧情画面，等待后重新识别。
	return assist.Result{State: "等待剧情画面", Wait: s.cfg.sleepAfterEmpty}, nil, nil
}

// actionStep 是一次已命中的状态转换。
type actionStep struct {
	state   string
	wait    time.Duration
	actions []assist.Action
}

func (s *actionStep) unpack() (assist.Result, []assist.Action) {
	return assist.Result{State: s.state, Wait: s.wait}, s.actions
}

func (s *Session) clickConfirm(frame assist.Frame) ([]assist.Action, bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.ConfirmArea)
	if err != nil {
		return nil, false, err
	}
	blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{})
	if err != nil {
		return nil, false, fmt.Errorf("识别确认框: %w", err)
	}
	block, ok := findBlock(blocks, s.spec.ConfirmText)
	if !ok {
		return nil, false, nil
	}
	return []assist.Action{clickBlock(block)}, true, nil
}

func (s *Session) storyBarVisible(frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.StoryArea)
	if err != nil {
		return false, err
	}
	blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{})
	if err != nil {
		return false, fmt.Errorf("识别剧情右上角: %w", err)
	}
	return hasKeyword(blocks, s.spec.Keywords), nil
}

func (s *Session) handleStoryBar(frame assist.Frame) (*actionStep, bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.StoryArea)
	if err != nil {
		return nil, false, err
	}
	blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{})
	if err != nil {
		return nil, false, fmt.Errorf("识别剧情右上角: %w", err)
	}
	visible := hasKeyword(blocks, s.spec.Keywords)

	if s.cfg.mode == ModeAuto {
		if s.spec.MiddleOptionArea != "" {
			match, ok, err := s.matchFirst(frame, s.spec.Screen, s.spec.MiddleOptionArea)
			if err != nil {
				return nil, visible, err
			}
			if ok {
				return &actionStep{state: "点击中间选项", wait: s.cfg.optionClickInterval, actions: []assist.Action{{Kind: assist.ActionClick, Point: match.Rect.Center()}}}, visible, nil
			}
		}
		if _, ok := findBlock(blocks, s.spec.AutoKeyword); ok {
			return &actionStep{state: "剧情自动播放中", wait: autoPlayingWait}, visible, nil
		}
		if block, ok := findBlock(blocks, s.spec.MenuKeyword); ok {
			return &actionStep{state: "展开剧情菜单", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, visible, nil
		}
		if s.spec.AutoButtonArea != "" {
			autoArea, err := s.area(s.spec.Screen, s.spec.AutoButtonArea)
			if err != nil {
				return nil, visible, err
			}
			autoBlocks, err := s.rt.OCR(frame, *autoArea, vision.OCROptions{})
			if err != nil {
				return nil, visible, fmt.Errorf("识别自动按钮: %w", err)
			}
			if block, ok := findBlock(autoBlocks, s.spec.AutoKeyword); ok {
				return &actionStep{state: "切换自动模式", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, visible, nil
			}
		}
		return nil, visible, nil
	}

	// 跳过模式：跳过按钮优先级最高。
	if block, ok := findBlock(blocks, s.spec.SkipKeyword); ok {
		return &actionStep{state: "点击跳过", wait: s.spec.SkipClickWait, actions: []assist.Action{clickBlock(block)}}, visible, nil
	}
	if block, ok := findBlock(blocks, s.spec.MenuKeyword); ok {
		s.menuClickedAt = frame.At
		s.clearChosenOption()
		return &actionStep{state: "展开剧情菜单", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, visible, nil
	}
	if block, ok := findBlock(blocks, s.spec.AutoKeyword); ok {
		s.menuClickedAt = frame.At
		s.clearChosenOption()
		return &actionStep{state: "点击自动", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, visible, nil
	}
	return nil, visible, nil
}

func (s *Session) inMenuQuiet(at time.Time) bool {
	if s.menuClickedAt.IsZero() {
		return false
	}
	if at.Sub(s.menuClickedAt) <= s.spec.MenuQuiet {
		return true
	}
	s.menuClickedAt = time.Time{}
	return false
}

func (s *Session) hitGuard(frame assist.Frame) (string, time.Duration, bool, error) {
	for _, guard := range s.spec.Guards {
		screenName := guard.Screen
		if screenName == "" {
			screenName = s.spec.Screen
		}
		area, err := s.area(screenName, guard.Area)
		if err != nil {
			return "", 0, false, err
		}
		matched := false
		switch guard.Kind {
		case GuardText:
			blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{})
			if err != nil {
				return "", 0, false, fmt.Errorf("识别守卫区域 %s: %w", area.Name, err)
			}
			matched = guard.Text == "" || hasBlock(blocks, guard.Text)
		default:
			if area.TemplateID == "" {
				continue
			}
			matches, err := s.rt.MatchTemplate(frame, *area, vision.TemplateRef{
				SubDir: area.TemplateSubDir,
				ID:     area.TemplateID,
			}, area.TemplateMatchThreshold)
			if err != nil {
				return "", 0, false, fmt.Errorf("匹配守卫区域 %s: %w", area.Name, err)
			}
			matched = len(matches) > 0
		}
		if !matched {
			continue
		}
		wait := guard.Wait
		if wait <= 0 {
			wait = s.spec.GuardWait
		}
		name := area.Name
		if guard.Screen != "" {
			name = guard.Screen + "/" + area.Name
		}
		return name, wait, true, nil
	}
	return "", 0, false, nil
}

func (s *Session) inDialog(frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.DialogArea)
	if err != nil {
		return false, err
	}
	stats, err := s.rt.ColorStats(frame, *area)
	if err != nil {
		return false, fmt.Errorf("统计对话框颜色: %w", err)
	}
	if stats.GrayMaskPercent <= s.spec.GrayMaskThreshold {
		return false, nil
	}
	blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{ColorRange: area.ColorRange})
	if err != nil {
		return false, fmt.Errorf("识别对话框内容: %w", err)
	}
	for _, block := range blocks {
		if containsChinese(block.Text) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Session) clickOption(frame assist.Frame, now time.Time) ([]assist.Action, bool, error) {
	for _, areaName := range []string{s.spec.RightOptionArea, s.spec.MiddleOptionArea} {
		if areaName == "" {
			continue
		}
		area, err := s.area(s.spec.Screen, areaName)
		if err != nil {
			return nil, false, err
		}
		blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{ColorRange: area.ColorRange})
		if err != nil {
			return nil, false, fmt.Errorf("识别对话选项: %w", err)
		}
		if len(blocks) == 0 {
			continue
		}

		// 点击后的保护时间：按钮可能变透明且 OCR 识别不到，继续点上一次的位置。
		if !s.chosenOptionAt.IsZero() && now.Sub(s.chosenOptionAt) < chosenOptionHold {
			return []assist.Action{{Kind: assist.ActionClick, Point: s.chosenPoint}}, true, nil
		}

		texts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			texts = append(texts, block.Text)
		}
		same := sameOptions(s.lastOptions, texts)
		s.lastOptions = texts

		var chosen *vision.TextBlock
		for i := range blocks {
			block := &blocks[i]
			// 同一选项持续超过 2s 且选项集合没变，大概率是误识别，跳过。
			if s.chosenOption != "" && !s.chosenOptionAt.IsZero() &&
				now.Sub(s.chosenOptionAt) > chosenOptionMax && block.Text == s.chosenOption && same {
				continue
			}
			if chosen == nil {
				chosen = block
				continue
			}
			if s.cfg.dialogOption == OptionFirst {
				if block.Rect.Center().Y < chosen.Rect.Center().Y {
					chosen = block
				}
			} else if block.Rect.Center().Y > chosen.Rect.Center().Y {
				chosen = block
			}
		}
		if chosen == nil {
			return nil, false, nil
		}
		s.chosenOption = chosen.Text
		s.chosenOptionAt = now
		s.chosenPoint = chosen.Rect.Center()
		return []assist.Action{{Kind: assist.ActionClick, Point: s.chosenPoint}}, true, nil
	}
	return nil, false, nil
}

func (s *Session) matchFirst(frame assist.Frame, screenName, areaName string) (vision.Match, bool, error) {
	area, err := s.area(screenName, areaName)
	if err != nil {
		return vision.Match{}, false, err
	}
	if area.TemplateID == "" {
		return vision.Match{}, false, nil
	}
	matches, err := s.rt.MatchTemplate(frame, *area, vision.TemplateRef{
		SubDir: area.TemplateSubDir,
		ID:     area.TemplateID,
	}, area.TemplateMatchThreshold)
	if err != nil {
		return vision.Match{}, false, fmt.Errorf("匹配区域 %s: %w", area.Name, err)
	}
	if len(matches) == 0 {
		return vision.Match{}, false, nil
	}
	return matches[0], true, nil
}

func (s *Session) area(screenName, areaName string) (*screen.Area, error) {
	sc, err := s.rt.Screen(screenName)
	if err != nil {
		return nil, fmt.Errorf("读取画面 %s: %w", screenName, err)
	}
	area, ok := sc.Area(areaName)
	if !ok {
		return nil, fmt.Errorf("画面 %s 缺少区域 %s", screenName, areaName)
	}
	return area, nil
}

func (s *Session) resetAfterConfirm() {
	s.menuClickedAt = time.Time{}
	s.clearChosenOption()
	s.recentClicks = 0
}

func (s *Session) clearChosenOption() {
	s.chosenOption = ""
	s.chosenOptionAt = time.Time{}
	s.chosenPoint = geom.Point{}
	s.lastOptions = nil
}

func clickBlock(block vision.TextBlock) assist.Action {
	return assist.Action{Kind: assist.ActionClick, Point: block.Rect.Center()}
}

func clickCurrent() []assist.Action {
	return []assist.Action{{Kind: assist.ActionClickCurrent}}
}

func findBlock(blocks []vision.TextBlock, keyword string) (vision.TextBlock, bool) {
	for _, block := range blocks {
		if strings.Contains(block.Text, keyword) {
			return block, true
		}
	}
	return vision.TextBlock{}, false
}

func hasBlock(blocks []vision.TextBlock, keyword string) bool {
	_, ok := findBlock(blocks, keyword)
	return ok
}

func hasKeyword(blocks []vision.TextBlock, keywords []string) bool {
	for _, keyword := range keywords {
		if hasBlock(blocks, keyword) {
			return true
		}
	}
	return false
}

func containsChinese(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func sameOptions(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, text := range a {
		counts[text]++
	}
	for _, text := range b {
		if counts[text] == 0 {
			return false
		}
		counts[text]--
	}
	return true
}
