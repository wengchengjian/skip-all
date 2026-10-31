package storyskip

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// 同一区域 OCR 结果缓存时间；降低循环里的重复识别开销。
	storyCacheTTL       = 400 * time.Millisecond
	storyButtonCacheTTL = time.Second
	guardCacheTTL       = time.Second

	padPhaseNone   = 0
	padPhaseSkip   = 1 // 已按“+”触发普通跳过
	padPhaseMenu   = 2 // 已按“+”打开菜单
	padPhaseChosen = 3 // 已在菜单里按 A 选择“跳过”
	padStartKey    = "xbox_start"
	padConfirmKey  = "xbox_a"
	padActivateKey = "xbox_a"
	padActionWait  = 400 * time.Millisecond
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
	lastOptionKey  string
	recentClicks   int
	ocrCache       map[string]ocrCacheEntry
	confirmUntil   time.Time
	padReady       bool
	padPhase       int
	padAt          time.Time
}

type ocrCacheEntry struct {
	at     time.Time
	blocks []vision.TextBlock
}

var _ assist.Session = (*Session)(nil)

func newSession(rt assist.Runtime, spec Spec, cfg sessionSettings) *Session {
	return &Session{rt: rt, spec: spec, cfg: cfg, ocrCache: make(map[string]ocrCacheEntry)}
}

func (s *Session) confirmPending(at time.Time) bool {
	if at.IsZero() {
		at = time.Now()
	}
	return !s.confirmUntil.IsZero() && at.Before(s.confirmUntil)
}

func (s *Session) expectConfirm(at time.Time, d time.Duration) {
	if at.IsZero() {
		at = time.Now()
	}
	s.confirmUntil = at.Add(d)
}

// ocrArea 对同一区域做短时缓存，避免每一帧都跑 OCR。
func (s *Session) ocrArea(frame assist.Frame, area screen.Area, opts vision.OCROptions, ttl time.Duration) ([]vision.TextBlock, error) {
	key := area.Name
	if opts.ColorRange != nil {
		key += "/color"
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if ttl > 0 && s.ocrCache != nil {
		if entry, ok := s.ocrCache[key]; ok && !now.Before(entry.at) && now.Sub(entry.at) < ttl {
			return append([]vision.TextBlock(nil), entry.blocks...), nil
		}
	}
	blocks, err := s.rt.OCR(frame, area, opts)
	if err != nil {
		return nil, err
	}
	if ttl > 0 && s.ocrCache != nil {
		s.ocrCache[key] = ocrCacheEntry{at: now, blocks: append([]vision.TextBlock(nil), blocks...)}
	}
	return blocks, nil
}

// Close 释放会话资源。
func (s *Session) Close() error { return nil }

// Step 处理一帧画面，返回本帧应执行的动作。
func (s *Session) Step(_ context.Context, frame assist.Frame) (assist.Result, []assist.Action, error) {
	if frame.Image == nil {
		return assist.Result{State: "等待剧情画面", Wait: s.cfg.sleepAfterEmpty}, nil, nil
	}
	if s.cfg.gamepad && s.cfg.mode == ModeSkip {
		return s.stepGamepad(frame)
	}

	// 1. 确认框：只在刚点过跳过/菜单后检查，避免每帧 OCR。
	if s.confirmPending(frame.At) {
		actions, handled, err := s.clickConfirm(frame)
		if err != nil {
			return assist.Result{}, nil, err
		}
		if handled {
			s.resetAfterConfirm()
			return assist.Result{State: "确认跳过", Wait: confirmWait}, actions, nil
		}
		// 确认框还没出现：先不要继续点剧情栏/菜单，否则会把下一次跳过按钮
		// 当成确认点击，导致整个跳过流程失败。
		return assist.Result{State: "等待确认框", Wait: confirmWait}, nil, nil
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

	// 3. 点击菜单后的 5s 静默：等跳过键和确认框，不重复点菜单。
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

	// 5. 对话流程：先选项，再黑屏推进，最后对话框正文。
	step, err := s.doDialogClick(frame, storyVisible)
	if err != nil {
		return assist.Result{}, nil, err
	}
	if step != nil {
		result, actions := step.unpack()
		return result, actions, nil
	}

	// 6. 没有匹配到剧情画面，等待后重新识别。
	return assist.Result{State: "等待剧情画面", Wait: s.cfg.sleepAfterEmpty}, nil, nil
}

// stepGamepad 是纯手柄剧情跳过流程：+ 打开菜单/跳过，A 选择，A 确认。
// 它不产生任何鼠标动作，适合游戏未聚焦时使用。
func (s *Session) stepGamepad(frame assist.Frame) (assist.Result, []assist.Action, error) {
	if !s.padReady {
		// 游戏当前可能还是键鼠模式，先发一个手柄输入激活手柄模式。
		// 用右摇杆而不是 A，避免激活时误触 UI。
		s.padReady = true
		s.padAt = frame.At
		return assist.Result{State: "激活手柄模式", Wait: padActionWait}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padActivateKey}}, nil
	}
	if s.padPhase != padPhaseNone {
		if frame.At.Sub(s.padAt) < padActionWait {
			return assist.Result{State: "等待手柄操作", Wait: padActionWait}, nil, nil
		}
		if s.confirmPending(frame.At) {
			ok, err := s.confirmVisible(frame)
			if err != nil {
				return assist.Result{}, nil, err
			}
			if ok {
				s.resetAfterConfirm()
				return assist.Result{State: "确认跳过", Wait: confirmWait}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}}, nil
			}
		}
		switch s.padPhase {
		case padPhaseMenu:
			s.padPhase = padPhaseChosen
			s.padAt = frame.At
			s.expectConfirm(frame.At, 5*time.Second)
			return assist.Result{State: "手柄选择跳过", Wait: padActionWait}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}}, nil
		case padPhaseChosen:
			if frame.At.Sub(s.padAt) > time.Second {
				s.expectConfirm(frame.At, time.Second)
				s.padAt = frame.At
				return assist.Result{State: "手柄选择跳过", Wait: padActionWait}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}}, nil
			}
			return assist.Result{State: "等待手柄确认", Wait: confirmWait}, nil, nil
		default:
			if frame.At.Sub(s.padAt) > 2*time.Second {
				s.padPhase = padPhaseNone
				s.confirmUntil = time.Time{}
			}
			return assist.Result{State: "等待手柄确认", Wait: confirmWait}, nil, nil
		}
	}

	if s.inMenuQuiet(frame.At) {
		return assist.Result{State: "等待手柄菜单", Wait: storyButtonWait}, nil, nil
	}

	area, err := s.area(s.spec.Screen, s.spec.StoryArea)
	if err != nil {
		return assist.Result{}, nil, err
	}
	blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, storyButtonCacheTTL)
	if err != nil {
		return assist.Result{}, nil, fmt.Errorf("识别剧情按钮: %w", err)
	}
	if _, ok := findBlock(blocks, s.spec.SkipKeyword); ok {
		s.padPhase = padPhaseSkip
		s.padAt = frame.At
		s.menuClickedAt = frame.At
		s.expectConfirm(frame.At, 5*time.Second)
		return assist.Result{State: "手柄跳过", Wait: padActionWait}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padStartKey}}, nil
	}
	if _, ok := findBlock(blocks, s.spec.MenuKeyword); ok {
		s.padPhase = padPhaseMenu
		s.padAt = frame.At
		s.menuClickedAt = frame.At
		return assist.Result{State: "手柄菜单", Wait: padActionWait}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padStartKey}}, nil
	}

	if name, wait, ok, err := s.hitGuard(frame); err != nil {
		return assist.Result{}, nil, err
	} else if ok {
		s.recentClicks = 0
		return assist.Result{State: "等待:" + name, Wait: wait}, nil, nil
	}

	inDialog, err := s.inDialog(frame)
	if err != nil {
		return assist.Result{}, nil, err
	}
	if inDialog {
		s.recentClicks++
		return assist.Result{State: "手柄对话", Wait: s.cfg.dialogClickInterval}, []assist.Action{{Kind: assist.ActionKeyTap, Key: padConfirmKey}}, nil
	}
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

func (s *Session) confirmVisible(frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.ConfirmArea)
	if err != nil {
		return false, err
	}
	// 确认按钮必须先确认文字是“确认”，不能只凭白色像素就点：
	// 菜单/跳过动画期间确认区域可能短暂出现其他白字，过早点击会导致跳过流程失败。
	blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, 100*time.Millisecond)
	if err != nil {
		return false, fmt.Errorf("识别确认框: %w", err)
	}
	for _, block := range blocks {
		// 上游的 lcs_percent=1.0 是精确匹配；这里也要求整段文字等于“确认”，
		// 避免把“确认跳过”等菜单文字当成确认按钮。
		if strings.TrimSpace(block.Text) == strings.TrimSpace(s.spec.ConfirmText) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Session) clickConfirm(frame assist.Frame) ([]assist.Action, bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.ConfirmArea)
	if err != nil {
		return nil, false, err
	}
	ok, err := s.confirmVisible(frame)
	if err != nil || !ok {
		return nil, false, err
	}
	// 确认按钮区域是固定按钮框，点区域中心比点 OCR 文字框中心更稳，
	// 避免 OCR 框只覆盖“确”或“认”导致点击位置偏移。
	return []assist.Action{{Kind: assist.ActionClick, Point: area.Rect.Center()}}, true, nil
}

func (s *Session) storyBarVisible(frame assist.Frame) (bool, error) {
	area, err := s.area(s.spec.Screen, s.spec.StoryArea)
	if err != nil {
		return false, err
	}
	blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, storyCacheTTL)
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
	if s.cfg.mode == ModeSkip {
		// 先按原始画面 OCR 找“菜单/跳过/自动”，不能只看白色文字行：
		// 菜单和灰色跳过在原始画面里仍然有字，只靠白色行会漏掉整条菜单路径。
		blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, storyButtonCacheTTL)
		if err != nil {
			return nil, false, fmt.Errorf("识别剧情按钮文字: %w", err)
		}
		if len(blocks) == 0 {
			return nil, false, nil
		}
		if block, ok := findBlock(blocks, s.spec.SkipKeyword); ok {
			// “跳过”只点白色可用态，灰色禁用态没有白色文字行。
			white := false
			for _, band := range vision.FindTextBands(frame.Image, area.Rect, vision.TextBandOptions{ColorRange: &s.spec.ButtonColorRange}) {
				if rectsIntersect(band, block.Rect) {
					white = true
					break
				}
			}
			if white {
				s.menuClickedAt = frame.At
				s.expectConfirm(frame.At, 5*time.Second)
				return &actionStep{state: "点击跳过", wait: s.spec.SkipClickWait, actions: []assist.Action{clickBlock(block)}}, true, nil
			}
		}
		if s.inMenuQuiet(frame.At) {
			return nil, false, nil
		}
		if block, ok := findBlock(blocks, s.spec.MenuKeyword); ok {
			s.menuClickedAt = frame.At
			return &actionStep{state: "展开剧情菜单", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, true, nil
		}
		if block, ok := findBlock(blocks, s.spec.AutoKeyword); ok {
			s.menuClickedAt = frame.At
			return &actionStep{state: "点击自动", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, true, nil
		}
		return nil, false, nil
	}

	blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, storyCacheTTL)
	if err != nil {
		return nil, false, fmt.Errorf("识别剧情右上角: %w", err)
	}
	visible := hasKeyword(blocks, s.spec.Keywords)
	if s.cfg.mode != ModeAuto {
		return nil, visible, nil
	}
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
		autoBlocks, err := s.ocrArea(frame, *autoArea, vision.OCROptions{}, storyCacheTTL)
		if err != nil {
			return nil, visible, fmt.Errorf("识别自动按钮: %w", err)
		}
		if block, ok := findBlock(autoBlocks, s.spec.AutoKeyword); ok {
			return &actionStep{state: "切换自动模式", wait: storyButtonWait, actions: []assist.Action{clickBlock(block)}}, visible, nil
		}
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
			blocks, err := s.ocrArea(frame, *area, vision.OCROptions{}, guardCacheTTL)
			if err != nil {
				return "", 0, false, fmt.Errorf("识别守卫区域 %s: %w", area.Name, err)
			}
			matched = guard.Text == "" || hasBlock(blocks, guard.Text)
		case GuardBand:
			matched = len(vision.FindTextBands(frame.Image, area.Rect, vision.TextBandOptions{ColorRange: &s.spec.ButtonColorRange})) > 0
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
	// 跳过模式只要 gray mask 成立就点，兼容“……”这种没有文字行的对话框。
	if s.cfg.mode == ModeSkip {
		return true, nil
	}
	// 点击/自动模式再用白色文字行确认，避免把其他灰底面板当对话。
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

// doDialogClick 对应上游 _do_dialog_click：对话框/剧情可见时先选项，再黑屏/正文。
func (s *Session) doDialogClick(frame assist.Frame, storyVisible bool) (*actionStep, error) {
	now := frame.At
	inDialog, err := s.inDialog(frame)
	if err != nil {
		return nil, err
	}

	if actions, ok, err := s.clickOptionArea(frame, s.spec.RightOptionArea, now); err != nil {
		return nil, err
	} else if ok {
		return &actionStep{state: "点击右方选项", wait: s.cfg.optionClickInterval, actions: actions}, nil
	}

	if s.cfg.mode == ModeSkip {
		// 跳过模式下中间区域不识别文字，只判断是否为黑屏；黑屏点击可推进过场。
		area, err := s.area(s.spec.Screen, s.spec.MiddleOptionArea)
		if err != nil {
			return nil, err
		}
		stats, err := s.rt.ColorStats(frame, *area)
		if err != nil {
			return nil, fmt.Errorf("判断黑屏: %w", err)
		}
		if stats.ColorfulPercent <= 0.01 && stats.MeanSaturation <= 1 {
			return &actionStep{state: "黑屏点击", wait: s.cfg.dialogClickInterval, actions: clickCurrent()}, nil
		}
	} else if actions, ok, err := s.clickOptionArea(frame, s.spec.MiddleOptionArea, now); err != nil {
		return nil, err
	} else if ok {
		return &actionStep{state: "点击中间选项", wait: s.cfg.optionClickInterval, actions: actions}, nil
	}

	if inDialog || (s.cfg.mode != ModeSkip && storyVisible) {
		s.recentClicks = s.spec.RecentClickLimit
		return &actionStep{state: "对话中点击", wait: s.cfg.dialogClickInterval, actions: clickCurrent()}, nil
	}

	if s.recentClicks > 0 {
		s.recentClicks--
		return &actionStep{state: "点击最近对话", wait: recentDialogWait, actions: clickCurrent()}, nil
	}
	return nil, nil
}

// clickOptionArea 用白色文字行定位选项，不跑 OCR；灰色选项不会被选中。
func (s *Session) clickOptionArea(frame assist.Frame, areaName string, now time.Time) ([]assist.Action, bool, error) {
	if areaName == "" {
		return nil, false, nil
	}
	area, err := s.area(s.spec.Screen, areaName)
	if err != nil {
		return nil, false, err
	}
	bands := vision.FindTextBands(frame.Image, area.Rect, vision.TextBandOptions{ColorRange: &s.spec.ButtonColorRange})
	if len(bands) == 0 {
		return nil, false, nil
	}

	// 点击后的保护时间：按钮可能变透明且识别不到，继续点上一次的位置。
	if !s.chosenOptionAt.IsZero() && now.Sub(s.chosenOptionAt) < chosenOptionHold {
		return []assist.Action{{Kind: assist.ActionClick, Point: s.chosenPoint}}, true, nil
	}

	key := bandKey(bands)
	same := key == s.lastOptionKey
	s.lastOptionKey = key
	if s.chosenOption != "" && !s.chosenOptionAt.IsZero() &&
		now.Sub(s.chosenOptionAt) > chosenOptionMax && key == s.chosenOption && same {
		return nil, false, nil
	}

	chosen := bands[0]
	if s.cfg.dialogOption != OptionFirst {
		chosen = bands[len(bands)-1]
	}
	s.chosenOption = key
	s.chosenOptionAt = now
	s.chosenPoint = chosen.Center()
	return []assist.Action{{Kind: assist.ActionClick, Point: s.chosenPoint}}, true, nil
}

func rectsIntersect(a, b geom.Rect) bool {
	return a.X1 < b.X2 && b.X1 < a.X2 && a.Y1 < b.Y2 && b.Y1 < a.Y2
}

func bandKey(bands []geom.Rect) string {
	var b strings.Builder
	for _, band := range bands {
		fmt.Fprintf(&b, "%d-%d-%d-%d;", band.X1, band.Y1, band.X2, band.Y2)
	}
	return b.String()
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
	s.confirmUntil = time.Time{}
	s.padPhase = padPhaseNone
	s.padAt = time.Time{}
	s.clearChosenOption()
	s.recentClicks = 0
}

func (s *Session) clearChosenOption() {
	s.chosenOption = ""
	s.chosenOptionAt = time.Time{}
	s.chosenPoint = geom.Point{}
	s.lastOptionKey = ""
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
