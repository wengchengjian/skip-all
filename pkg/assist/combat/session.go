package combat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// Session 是自动战斗会话。
// 状态只有两个：在战斗里按连招出招、不在战斗里等。
// healthBarFillRatio 是上游 find_color_rectangles 的默认填充率阈值。
const healthBarFillRatio = 0.95

// abortOCRTTL 是中断提示的缓存时间。
const abortOCRTTL = 2 * time.Second

// cooldownOCRTTL 是冷却 OCR 的缓存时间。
const cooldownOCRTTL = 500 * time.Millisecond

// agentTTL 是角色识别的缓存时间。
const agentTTL = 2 * time.Second

// concertoSwitchCooldown 是两次协奏切人之间的最小间隔。
const concertoSwitchCooldown = 2 * time.Second

type Session struct {
	rt         assist.Runtime
	spec       Spec
	cfg        sessionSettings
	cursor     int
	lastCombat time.Time
	lockAt     time.Time
	healed     bool
	switchAt   time.Time
	agentID    string
	agentAt    time.Time
	cdCache    map[StepKind]cdEntry
	slotSeen   map[int]time.Time
	introLeft  []Step
	abortAt    time.Time
}

func newSession(rt assist.Runtime, spec Spec, cfg sessionSettings) *Session {
	return &Session{rt: rt, spec: spec, cfg: cfg, cdCache: make(map[StepKind]cdEntry), slotSeen: make(map[int]time.Time)}
}

// cdEntry 是冷却 OCR 的短时缓存，避免每帧都对技能按钮跑 OCR。
type cdEntry struct {
	at      time.Time
	cooling bool
}

// Step 执行一帧。
func (s *Session) Step(_ context.Context, frame assist.Frame) (assist.Result, []assist.Action, error) {
	if frame.Image == nil {
		return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, nil
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	inCombat, err := s.inCombat(frame)
	if err != nil {
		return assist.Result{}, nil, err
	}
	if inCombat {
		s.lastCombat = now
	} else if s.lastCombat.IsZero() || now.Sub(s.lastCombat) > s.spec.Hold {
		// 证据会短暂丢失（转身、切人、特效遮挡），用保持时间把它们连起来。
		s.cursor = 0
		s.lockAt = time.Time{}
		// 脱战了：缓存和入场技都清掉，下一场重新来。
		s.cdCache = make(map[StepKind]cdEntry)
		s.agentAt = time.Time{}
		s.agentID = ""
		s.introLeft = nil
		// 上游在战斗结束后会切回治疗位；这里也是同一件事。
		if s.healed && !s.cfg.noHealer {
			if key := s.healerKey(frame); key != "" {
				s.healed = false
				return assist.Result{State: "战斗结束切治疗", Wait: s.spec.Interval},
					[]assist.Action{{Kind: assist.ActionKeyTap, Key: key}}, nil
			}
		}
		s.healed = false
		return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, nil
	}
	// 上游在开打前先切到治疗位，保证有人能奶。
	if !s.cfg.noHealer && !s.healed {
		if key := s.healerKey(frame); key != "" {
			s.healed = true
			return assist.Result{State: "战斗前切治疗", Wait: s.spec.Interval},
				[]assist.Action{{Kind: assist.ActionKeyTap, Key: key}}, nil
		}
	}
	// 游戏专属能力优先：闪光闪避、快速支援这类要抢在连招前面。
	if action, state, ok, err := s.runExtensions(frame); err != nil {
		return assist.Result{}, nil, err
	} else if ok {
		return assist.Result{State: state, Wait: s.spec.Interval}, []assist.Action{action}, nil
	}
	// 协奏值满了就切人（上游 switch_next_char 的核心判据）。
	if action, ok := s.concertoSwitch(frame, now); ok {
		return assist.Result{State: "协奏满切人", Wait: s.spec.Interval}, []assist.Action{action}, nil
	}
	// 中键锁定：上游 target_enemy() 在没锁定目标时反复中键。
	if s.spec.LockInterval > 0 && (s.lockAt.IsZero() || now.Sub(s.lockAt) >= s.spec.LockInterval) {
		locked, err := s.hasTarget(frame)
		if err != nil {
			return assist.Result{}, nil, err
		}
		if !locked {
			s.lockAt = now
			return assist.Result{State: "锁定目标", Wait: s.spec.Interval},
				[]assist.Action{{Kind: assist.ActionMiddleClick, Point: s.spec.AttackPoint}}, nil
		}
	}

	// 战斗被打断（复活/失败界面）时先别乱按。
	if aborted, err := s.checkAbort(frame); err != nil {
		return assist.Result{}, nil, err
	} else if aborted {
		s.cursor = 0
		s.introLeft = nil
		return assist.Result{State: "战斗中断", Wait: s.spec.IdleWait}, nil, nil
	}

	action, state, ok := s.nextAction(frame)
	if !ok {
		return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, nil
	}
	return assist.Result{State: state, Wait: s.spec.Interval}, []assist.Action{action}, nil
}

// Close 释放会话。
func (s *Session) Close() error { return nil }

// inCombat 依次看模板证据与血条颜色证据，任一命中即认为在战斗。
func (s *Session) inCombat(frame assist.Frame) (bool, error) {
	if bar := s.spec.HealthBar; bar != nil {
		bounds := frame.Image.Bounds()
		rects := vision.FindColorRectangles(frame.Image, geom.Rect{X1: bounds.Min.X, Y1: bounds.Min.Y, X2: bounds.Max.X, Y2: bounds.Max.Y},
			bar.Color, bar.MinWidth, bar.MinHeight, bar.MaxHeight, healthBarFillRatio)
		if len(rects) > 0 {
			return true, nil
		}
	}
	if boss := s.spec.BossBar; boss != nil {
		// 上游 is_boss() 要求这个区域里恰好只有一条，避免把别的红色 UI 当成 Boss 血条。
		if rects := vision.FindColorRectangles(frame.Image, boss.Rect, boss.Color,
			boss.MinWidth, boss.MinHeight, 0, healthBarFillRatio); len(rects) == 1 {
			return true, nil
		}
	}
	for _, name := range s.spec.DetectAreas {
		area, err := s.area(name)
		if err != nil {
			return false, err
		}
		if area.TemplateID == "" {
			continue
		}
		matches, err := s.rt.MatchTemplate(frame, *area, vision.TemplateRef{
			SubDir: area.TemplateSubDir,
			ID:     area.TemplateID,
		}, area.TemplateMatchThreshold)
		if err != nil {
			return false, fmt.Errorf("匹配战斗证据 %s: %w", name, err)
		}
		if len(matches) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// ready 判断某个动作现在能不能用：先看按钮高亮，再看冷却数字；没配判据就当作可用。
func (s *Session) ready(frame assist.Frame, kind StepKind) bool {
	if check, ok := s.spec.Ready[kind]; ok {
		if vision.ColorRatio(frame.Image, check.Area, check.ColorRange) < check.MinRatio {
			return false
		}
	}
	if area, ok := s.spec.Cooldowns[kind]; ok {
		if cooling, err := s.coolingDown(frame, kind, area); err == nil && cooling {
			return false
		}
	}
	return true
}

// coolingDown 读技能按钮上的冷却数字：读到正数说明还在冷却。
// 上游鸣潮用 get_cd、绝区零用 has_cd，都是这条路子。
func (s *Session) coolingDown(frame assist.Frame, kind StepKind, rect geom.Rect) (bool, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if entry, ok := s.cdCache[kind]; ok && now.Sub(entry.at) < cooldownOCRTTL {
		return entry.cooling, nil
	}
	blocks, err := s.rt.OCR(frame, screen.Area{Name: "技能冷却", Rect: rect}, vision.OCROptions{})
	if err != nil {
		return false, err
	}
	cooling := false
	for _, block := range blocks {
		if seconds, ok := parseCooldownSeconds(block.Text); ok && seconds > 0 {
			cooling = true
			break
		}
	}
	s.cdCache[kind] = cdEntry{at: now, cooling: cooling}
	return cooling, nil
}

// parseCooldownSeconds 从 OCR 文本里解析冷却秒数，例如 "12"、"3.5"、" 8 "。
func parseCooldownSeconds(text string) (float64, bool) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimSuffix(cleaned, "s")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// nextAction 按连招序列取下一条动作，跳过被设置关掉的动作。
// currentPlan 返回当前角色专属的连招，没有就用默认连招。
// 角色识别按 agentTTL 缓存，避免每帧都跑一遍模板匹配。
func (s *Session) currentPlan(frame assist.Frame) []Step {
	if len(s.spec.PlansByAgent) == 0 || s.spec.Team == nil {
		return s.spec.Plan
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if s.agentAt.IsZero() || now.Sub(s.agentAt) >= agentTTL {
		s.agentAt = now
		if id, ok, err := s.spec.Team.Agent(runtimeAdapter{rt: s.rt}, frame); err == nil && ok {
			s.agentID = id
		} else {
			s.agentID = ""
		}
	}
	if plan, ok := s.spec.PlansByAgent[s.agentID]; ok && len(plan) > 0 {
		return plan
	}
	return s.spec.Plan
}

// checkAbort 看画面上是否出现中断提示（复活/失败）。
func (s *Session) checkAbort(frame assist.Frame) (bool, error) {
	if s.spec.AbortArea == "" || len(s.spec.AbortTexts) == 0 {
		return false, nil
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	if !s.abortAt.IsZero() && now.Sub(s.abortAt) < abortOCRTTL {
		return true, nil
	}
	area, err := s.area(s.spec.AbortArea)
	if err != nil {
		return false, err
	}
	blocks, err := s.rt.OCR(frame, *area, vision.OCROptions{})
	if err != nil {
		return false, fmt.Errorf("识别中断提示: %w", err)
	}
	for _, block := range blocks {
		for _, text := range s.spec.AbortTexts {
			if text != "" && strings.Contains(block.Text, text) {
				s.abortAt = now
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Session) nextAction(frame assist.Frame) (assist.Action, string, bool) {
	// 刚切过人：先把入场技打完（鸣潮的 handle_intro）。
	if len(s.introLeft) > 0 {
		step := s.introLeft[0]
		s.introLeft = s.introLeft[1:]
		if action, state, ok := s.stepAction(frame, step); ok {
			return action, "入场-" + state, true
		}
	}
	plan := s.currentPlan(frame)
	if len(plan) == 0 {
		return assist.Action{}, "", false
	}
	for i := 0; i < len(plan); i++ {
		step := plan[s.cursor%len(plan)]
		s.cursor++
		if action, state, ok := s.stepAction(frame, step); ok {
			return action, "连招-" + state, true
		}
	}
	return assist.Action{}, "", false
}

// stepAction 把连招里的一步转成实际动作；ok=false 表示这一步被关掉或还没就绪，调用方跳过。
func (s *Session) stepAction(frame assist.Frame, step Step) (assist.Action, string, bool) {
	switch step.Kind {
	case StepAttack:
		if s.spec.Keys.Attack != "" {
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Attack}, "普攻", true
		}
		return assist.Action{Kind: assist.ActionClick, Point: s.spec.AttackPoint}, "普攻", true
	case StepSkill:
		if s.spec.Keys.Skill == "" || !s.ready(frame, StepSkill) {
			return assist.Action{}, "", false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Skill}, "技能", true
	case StepEcho:
		if s.spec.Keys.Echo == "" || s.cfg.noEcho || !s.ready(frame, StepEcho) {
			return assist.Action{}, "", false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Echo}, "装备技能", true
	case StepLiberation:
		if s.spec.Keys.Liberation == "" || s.cfg.noLiberation || !s.ready(frame, StepLiberation) {
			return assist.Action{}, "", false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Liberation}, "爆发", true
	case StepDodge:
		if s.spec.Keys.Dodge == "" || s.cfg.noDodge {
			return assist.Action{}, "", false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Dodge}, "闪避", true
	case StepKey:
		if step.Key == "" {
			return assist.Action{}, "", false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: step.Key}, "按键", true
	case StepSwitch:
		if step.Index < 0 || step.Index >= len(s.spec.Keys.Switch) {
			return assist.Action{}, "", false
		}
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		s.noteSwitch(frame, now)
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Switch[step.Index]}, "切换角色", true
	}
	return assist.Action{}, "", false
}

// runExtensions 依次询问各扩展，第一个认领的生效。
func (s *Session) runExtensions(frame assist.Frame) (assist.Action, string, bool, error) {
	if len(s.spec.Extensions) == 0 {
		return assist.Action{}, "", false, nil
	}
	rt := runtimeAdapter{rt: s.rt}
	for _, ext := range s.spec.Extensions {
		if s.cfg.disabledExt(ext.Name()) {
			continue
		}
		action, state, ok, err := ext.Step(rt, frame)
		if err != nil {
			return assist.Action{}, "", false, fmt.Errorf("扩展 %s: %w", ext.Name(), err)
		}
		if ok {
			return action, state, true, nil
		}
	}
	return assist.Action{}, "", false, nil
}

// hasTarget 判断当前是否已经锁定目标：has_target / no_target 两张模板比分数。
// 没配置目标识别时一律当作已锁定，免得退化成乱按中键。
func (s *Session) hasTarget(frame assist.Frame) (bool, error) {
	t := s.spec.Target
	if t == nil || t.Have == "" || t.Missing == "" {
		return true, nil
	}
	haveScore, err := s.templateScore(frame, t, t.Have)
	if err != nil {
		return false, err
	}
	missScore, err := s.templateScore(frame, t, t.Missing)
	if err != nil {
		return false, err
	}
	threshold := t.Threshold
	if threshold <= 0 {
		threshold = 0.6
	}
	return haveScore >= missScore && haveScore >= threshold, nil
}

// templateScore 返回模板在目标区域里的最高分（多尺度取最好）。
func (s *Session) templateScore(frame assist.Frame, t *Target, id string) (float64, error) {
	area := screen.Area{
		Name:                   "战斗目标",
		Type:                   screen.AreaTemplate,
		Rect:                   t.Rect,
		TemplateSubDir:         t.SubDir,
		TemplateID:             id,
		TemplateMatchThreshold: 0.05,
	}
	matches, err := s.rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: t.SubDir, ID: id}, area.TemplateMatchThreshold)
	if err != nil {
		return 0, fmt.Errorf("匹配目标模板 %s: %w", id, err)
	}
	best := 0.0
	for _, m := range matches {
		if m.Confidence > best {
			best = m.Confidence
		}
	}
	return best, nil
}

// concertoSwitch 在协奏值满时返回一次切人动作。
func (s *Session) concertoSwitch(frame assist.Frame, now time.Time) (assist.Action, bool) {
	if s.cfg.noConcerto || s.spec.Concerto == nil || len(s.spec.Keys.Switch) == 0 {
		return assist.Action{}, false
	}
	// 切人后协奏环会清空，留一点冷却避免连点。
	cooldown := concertoSwitchCooldown
	if s.spec.Switch != nil && s.spec.Switch.MinInterval > 0 {
		cooldown = s.spec.Switch.MinInterval
	}
	if !s.switchAt.IsZero() && now.Sub(s.switchAt) < cooldown {
		return assist.Action{}, false
	}
	c := s.spec.Concerto
	coverage := vision.RingCoverage(frame.Image, c.Rect, c.Colors, c.Inner, c.Outer)
	full := c.FullRatio
	if full <= 0 {
		full = 0.8
	}
	if coverage < full {
		return assist.Action{}, false
	}
	s.noteSwitch(frame, now)
	return assist.Action{Kind: assist.ActionKeyTap, Key: s.chooseSwitchKey(frame)}, true
}

// chooseSwitchKey 选切人按键：能列出各槽位角色时按定位打分，否则退回按顺序切。
func (s *Session) chooseSwitchKey(frame assist.Frame) string {
	keys := s.spec.Keys.Switch
	if len(keys) == 0 {
		return ""
	}
	locator, ok := s.spec.Team.(SlotLocator)
	if !ok || len(s.spec.Roles) == 0 {
		return s.nextSwitchKey(frame)
	}
	ids, err := locator.Slots(runtimeAdapter{rt: s.rt}, frame)
	if err != nil || len(ids) == 0 {
		return s.nextSwitchKey(frame)
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	current := -1
	if s.spec.Team != nil {
		if idx, found, err := s.spec.Team.Current(runtimeAdapter{rt: s.rt}, frame); err == nil && found {
			current = idx
		}
	}
	if current >= 0 && current < len(ids) {
		s.slotSeen[current] = now
	}
	best, bestKey := -1e9, ""
	currentRole := Role("")
	if current >= 0 && current < len(ids) {
		currentRole = s.spec.Roles[ids[current]]
	}
	for i, id := range ids {
		if i == current || i >= len(keys) {
			continue
		}
		score := roleScore(currentRole, s.spec.Roles[id]) + s.staleBonus(i, now) + s.ultBonus(frame, i)
		if score > best {
			best, bestKey = score, keys[i]
		}
	}
	if bestKey == "" {
		return s.nextSwitchKey(frame)
	}
	return bestKey
}

// ultBonus 给"大招已就绪"的槽位加分：上游 update_lib_portrait_icon() 记录后台角色
// 的解放是否可用，切人时优先让能放大招的人上场。
func (s *Session) ultBonus(frame assist.Frame, slot int) float64 {
	check := s.spec.UltReady
	if check == nil || slot >= len(check.Slots) {
		return 0
	}
	threshold := check.Threshold
	if threshold <= 0 {
		threshold = 0.8
	}
	for _, id := range check.IDs {
		matches, err := s.rt.MatchTemplate(frame, screen.Area{Name: "大招就绪", Rect: check.Slots[slot]}, vision.TemplateRef{SubDir: check.SubDir, ID: id}, threshold)
		if err != nil {
			return 0
		}
		if len(matches) > 0 {
			if check.Bonus > 0 {
				return check.Bonus
			}
			return 15
		}
	}
	return 0
}

// staleBonus 给"很久没出场"的槽位加分，对应上游 _oldest_switch_target。
func (s *Session) staleBonus(slot int, now time.Time) float64 {
	seen, ok := s.slotSeen[slot]
	if !ok {
		return 20
	}
	bonus := now.Sub(seen).Seconds()
	if bonus > 20 {
		bonus = 20
	}
	if bonus < 0 {
		bonus = 0
	}
	return bonus
}

// roleScore 决定"从当前定位切到目标定位"该给多少分，思路照上游 swap 规则：
// 主C打完切辅助/治疗，辅助/治疗打完切回主C，同类不优先。
func roleScore(current, target Role) float64 {
	if target == "" {
		return 5
	}
	switch current {
	case RoleMainDPS:
		switch target {
		case RoleHealer:
			return 30
		case RoleSubDPS:
			return 20
		case RoleSupport:
			return 10
		}
	case RoleHealer, RoleSupport:
		switch target {
		case RoleMainDPS:
			return 30
		case RoleSubDPS:
			return 20
		case RoleSupport:
			return 10
		}
	case RoleSubDPS:
		switch target {
		case RoleMainDPS:
			return 30
		case RoleHealer:
			return 20
		case RoleSupport:
			return 10
		}
	default:
		return 10
	}
	return 0
}

// noteSwitch 记录一次切人：清掉与"当前角色"有关的缓存，并挂上入场技。
// 切人之后技能按钮上的冷却数字换了人、头像也换了，缓存必须失效。
func (s *Session) noteSwitch(frame assist.Frame, now time.Time) {
	s.switchAt = now
	s.cdCache = make(map[StepKind]cdEntry)
	s.agentAt = time.Time{}
	s.agentID = ""
	s.introLeft = append([]Step(nil), s.spec.IntroSteps...)
	_ = frame
}

// healerKey 返回治疗位按键：能识别出队伍里的治疗就按它的槽位键，否则用配置的固定键。
func (s *Session) healerKey(frame assist.Frame) string {
	if locator, ok := s.spec.Team.(HealerLocator); ok {
		if index, found, err := locator.HealerSlot(runtimeAdapter{rt: s.rt}, frame); err == nil && found && index < len(s.spec.Keys.Switch) {
			return s.spec.Keys.Switch[index]
		}
	}
	return s.spec.HealerKey
}

// nextSwitchKey 选下一个角色的按键：识别得到当前槽位就按下一个，否则用第一个键。
func (s *Session) nextSwitchKey(frame assist.Frame) string {
	keys := s.spec.Keys.Switch
	if len(keys) == 0 {
		return ""
	}
	if s.spec.Team != nil {
		if idx, ok, err := s.spec.Team.Current(runtimeAdapter{rt: s.rt}, frame); err == nil && ok && len(keys) > 1 {
			return keys[(idx+1)%len(keys)]
		}
	}
	return keys[0]
}

func (s *Session) area(name string) (*screen.Area, error) {
	sc, err := s.rt.Screen(s.spec.Screen)
	if err != nil {
		return nil, fmt.Errorf("读取画面 %s: %w", s.spec.Screen, err)
	}
	area, ok := sc.Area(name)
	if !ok {
		return nil, fmt.Errorf("画面 %s 缺少区域 %s", s.spec.Screen, name)
	}
	return area, nil
}
