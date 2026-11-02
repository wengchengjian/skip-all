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

// 切人确认的时间窗：300ms 内没换人就认为输入被吞，800ms 后放弃。
const (
	switchVerifyGrace   = 300 * time.Millisecond
	switchVerifyTimeout = 800 * time.Millisecond
)

// defaultConcertoFullRatio 是协奏环算"满"的默认门限：上游要求正好满（1.0），
// 留 5% 余量给抗锯齿与颜色容差。
const defaultConcertoFullRatio = 0.95

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
	// outroAgent 是上一次切走的角色模板 id。上游 check_outro() 在入场时
	// 取队伍里 last_switch_time 最大的队友，就是刚放完延奏的那个人。
	outroAgent string
	// lastLiberation/lastHeavy/lastSkill 记录每个角色最近一次对应动作的时刻，
	// 供 still_in_liberation 与 get_switch_priority 的 NO 锁使用。
	lastLiberation map[string]time.Time
	lastHeavy      map[string]time.Time
	lastSkill      map[string]time.Time
	lastPerform    map[string]time.Time
	// flyingChecked/hasLavitator 是开战后用 edge_levitator 模板确认的一次性状态。
	flyingChecked bool
	hasLavitator  bool
	cdCache       map[StepKind]cdEntry
	slotSeen      map[int]time.Time
	switchInAt    map[int]time.Time
	introLeft     []Step
	introUntil    time.Time
	introMode     IntroMode
	// 切人后要确认真的切过去了：后台输入可能被吞，上游 switch_next_char 里也会复查 in_team。
	pendingFrom int
	pendingAt   time.Time
	pendingDone bool
	abortAt     time.Time
}

func newSession(rt assist.Runtime, spec Spec, cfg sessionSettings) *Session {
	return &Session{rt: rt, spec: spec, cfg: cfg, lastLiberation: make(map[string]time.Time), lastHeavy: make(map[string]time.Time), lastSkill: make(map[string]time.Time), lastPerform: make(map[string]time.Time), cdCache: make(map[StepKind]cdEntry), slotSeen: make(map[int]time.Time), switchInAt: make(map[int]time.Time)}
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
		s.outroAgent = ""
		s.lastLiberation = make(map[string]time.Time)
		s.lastHeavy = make(map[string]time.Time)
		s.lastSkill = make(map[string]time.Time)
		s.lastPerform = make(map[string]time.Time)
		s.flyingChecked = false
		s.hasLavitator = false
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
	// 先把会话算出来的状态推给需要的扩展（协奏是否满、是否还在入场、当前角色）。
	s.pushCombatState(frame, now)
	if action, state, ok, err := s.runExtensions(frame); err != nil {
		return assist.Result{}, nil, err
	} else if ok {
		return assist.Result{State: state, Wait: s.spec.Interval}, []assist.Action{action}, nil
	}
	// 协奏值满了就切人（上游 switch_next_char 的核心判据）。
	if action, ok := s.concertoSwitch(frame, now); ok {
		return assist.Result{State: "协奏满切人", Wait: s.spec.Interval}, []assist.Action{action}, nil
	}
	// 战斗被打断（复活/失败界面）时先别乱按——这一步要排在锁定之前，
	// 否则战斗已经结束时还会去按中键。
	if aborted, err := s.checkAbort(frame); err != nil {
		return assist.Result{}, nil, err
	} else if aborted {
		s.cursor = 0
		s.introLeft = nil
		return assist.Result{State: "战斗中断", Wait: s.spec.IdleWait}, nil, nil
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
	if len(s.spec.PlansByAgent) == 0 {
		return s.spec.Plan
	}
	if plan, ok := s.spec.PlansByAgent[s.currentAgent(frame)]; ok && len(plan) > 0 {
		return plan
	}
	return s.spec.Plan
}

// currentAgent 返回当前角色模板 id（按 agentTTL 缓存）。
func (s *Session) currentAgent(frame assist.Frame) string {
	if s.spec.Team == nil {
		return ""
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
	return s.agentID
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
	// 切人确认：800ms 内发现前台没变，就补一次普攻（上游 switch_next_char 里的兜底）。
	if !s.pendingAt.IsZero() && s.spec.Team != nil {
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		since := now.Sub(s.pendingAt)
		if since > switchVerifyTimeout || s.pendingFrom < 0 {
			s.pendingAt = time.Time{}
		} else if cur, ok, err := s.spec.Team.Current(runtimeAdapter{rt: s.rt}, frame); err == nil && ok && cur != s.pendingFrom {
			s.pendingAt = time.Time{} // 切人成功
		} else if since > switchVerifyGrace && !s.pendingDone {
			s.pendingDone = true
			action := assist.Action{Kind: assist.ActionClick, Point: s.spec.AttackPoint}
			if s.spec.Keys.Attack != "" {
				action = assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Attack}
			}
			return action, "切人未生效", true
		}
	}
	// 入场等待：上游 wait_intro() 期间一直点普攻，直到落地（技能可用）或超时。
	if !s.introUntil.IsZero() {
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		// 角色专属入场（如今汐的入场爆发）：切进来才知道是谁，所以在这里解析。
		if s.spec.Team != nil {
			if id := s.currentAgent(frame); id != "" {
				if override, ok := s.spec.IntroByAgent[id]; ok {
					s.introMode = override.Mode
					limit := override.MaxDuration
					if limit <= 0 {
						limit = 1200 * time.Millisecond
					}
					if deadline := s.switchAt.Add(limit); deadline.After(s.introUntil) {
						s.introUntil = deadline
					}
				}
			}
		}
		if now.After(s.introUntil) {
			s.introUntil = time.Time{}
		} else if s.introMode == IntroBurn {
			// 入场爆发：技能可用就一直按，进冷却后用声骸/普攻收尾并结束入场。
			if s.ready(frame, StepSkill) && s.spec.Keys.Skill != "" {
				return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Skill}, "入场-爆发", true
			}
			if s.spec.Keys.Echo != "" && s.ready(frame, StepEcho) {
				s.introUntil = time.Time{}
				return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Echo}, "入场-收尾", true
			}
			s.introUntil = time.Time{}
		} else if s.ready(frame, StepSkill) {
			s.introUntil = time.Time{}
		} else {
			action := assist.Action{Kind: assist.ActionClick, Point: s.spec.AttackPoint}
			if s.spec.Keys.Attack != "" {
				action = assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Attack}
			}
			return action, "入场-落地", true
		}
	}
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
		if s.spec.Keys.Skill == "" || !s.ready(frame, StepSkill) || !s.forteAllows(frame) {
			return assist.Action{}, "", false
		}
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		s.noteSkill(frame, now)
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
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		s.noteLiberation(frame, now)
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Liberation}, "爆发", true
	case StepDodge:
		if s.spec.Keys.Dodge == "" || s.cfg.noDodge {
			return assist.Action{}, "", false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Dodge}, "闪避", true
	case StepHeavy:
		hold := step.Hold
		if hold <= 0 {
			hold = 600 * time.Millisecond // 上游 heavy_attack(duration=0.6)
		}
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		s.noteHeavy(frame, now)
		return assist.Action{Kind: assist.ActionClickHold, Point: s.spec.AttackPoint, Duration: hold}, "重击", true
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
// pushCombatState 把会话状态交给实现了 StateAware 的扩展。
func (s *Session) pushCombatState(frame assist.Frame, now time.Time) {
	for _, e := range s.spec.Extensions {
		sa, ok := e.(StateAware)
		if !ok {
			continue
		}
		agent := s.currentAgent(frame)
		if agent != "" {
			s.lastPerform[agent] = now
		}
		sa.SetCombatState(CombatState{
			Agent:             agent,
			ConcertoFull:      s.concertoLit(frame),
			InIntro:           !s.introUntil.IsZero(),
			LastOutro:         s.outroAgent,
			Flying:            s.flying(frame),
			InLiberation:      s.inLiberation(frame, now),
			NeedFastPerform:   s.needFastPerform(frame, now),
			CanResonanceStep2: s.canResonanceStep2For(agent, now),
		})
	}
}

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
			s.noteAction(action, frame)
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
// concertoLit 只判断协奏环是否算满，不改状态。
func (s *Session) concertoLit(frame assist.Frame) bool {
	c := s.spec.Concerto
	if c == nil {
		return false
	}
	coverage := 0.0
	for _, ratio := range vision.RingCoverages(frame.Image, c.Rect, c.Colors, c.Inner, c.Outer) {
		if ratio > coverage {
			coverage = ratio
		}
	}
	full := c.FullRatio
	if full <= 0 {
		full = defaultConcertoFullRatio
	}
	return coverage >= full
}

// needFastPerform 对应上游 BaseChar.need_fast_perform()：
// 遍历后台候选，只要有角色对当前角色返回 MUST（协同表或仍在解放窗口），
// 当前角色就应当尽快收尾让位。
func (s *Session) needFastPerform(frame assist.Frame, now time.Time) bool {
	if len(s.spec.SwitchSynergy) == 0 && len(s.spec.LiberationMustSwitch) == 0 {
		return false
	}
	locator, ok := s.spec.Team.(SlotLocator)
	if !ok {
		return false
	}
	ids, err := locator.Slots(runtimeAdapter{rt: s.rt}, frame)
	if err != nil || len(ids) == 0 {
		return false
	}
	current := s.currentAgent(frame)
	if current == "" {
		return false
	}
	for _, id := range ids {
		if id == "" || id == current {
			continue
		}
		if s.synergyBonus(id, current) > 0 || s.liberationBonus(id, now) > 0 || s.resonanceBonus(id, now) > 0 {
			return true
		}
	}
	return false
}

// flying 对应上游 BaseChar.flying()：
// 先确认队伍带滑翔装置（edge_levitator 模板，开战一次），
// 再看 edge_levitator 图标区域白色占比是否 < 0.1。
func (s *Session) flying(frame assist.Frame) bool {
	check := s.spec.Flying
	if check == nil {
		return false
	}
	if !s.flyingChecked {
		s.flyingChecked = true
		if check.TemplateID != "" {
			matches, err := s.rt.MatchTemplate(frame, screen.Area{Name: "滑翔装置", Rect: check.TemplateArea},
				vision.TemplateRef{SubDir: check.TemplateSubDir, ID: check.TemplateID}, check.TemplateThreshold)
			if err == nil && len(matches) > 0 {
				s.hasLavitator = true
			}
		}
	}
	if !s.hasLavitator {
		return false
	}
	max := check.MaxRatio
	if max <= 0 {
		max = 0.1
	}
	return vision.ColorRatio(frame.Image, check.Area, check.ColorRange) < max
}

// noteLiberation 记录当前角色按下了共鸣解放。上游 click_liberation() 成功后
// 会更新 last_liberation（部分角色用 liberation_time），这是 still_in_liberation 的起点。
func (s *Session) noteLiberation(frame assist.Frame, now time.Time) {
	agent := s.currentAgent(frame)
	if agent == "" {
		return
	}
	s.lastLiberation[agent] = now
}

func (s *Session) noteHeavy(frame assist.Frame, now time.Time) {
	agent := s.currentAgent(frame)
	if agent != "" {
		s.lastHeavy[agent] = now
	}
}

func (s *Session) noteSkill(frame assist.Frame, now time.Time) {
	agent := s.currentAgent(frame)
	if agent != "" {
		s.lastSkill[agent] = now
	}
}

// inLiberation 对应上游 BaseChar.still_in_liberation()：
// 当前角色最近一次解放后未超过该角色的窗口（基类 12 秒，Encore 9.5、Xiangliyao 25）。
func (s *Session) inLiberation(frame assist.Frame, now time.Time) bool {
	agent := s.currentAgent(frame)
	if agent == "" {
		return false
	}
	return s.inLiberationFor(agent, now)
}

// inLiberationFor 判断任意角色（含后台候选）是否还在解放窗口内。
func (s *Session) inLiberationFor(agent string, now time.Time) bool {
	at, ok := s.lastLiberation[agent]
	if !ok {
		return false
	}
	window := 12 * time.Second
	if w, ok := s.spec.LiberationWindows[agent]; ok && w > 0 {
		window = w
	}
	return now.Sub(at) < window
}

// canResonanceStep2For 对应上游 BaseChar.can_resonance_step2()：
// 最近一次共鸣技能在 2 秒窗口内（Encore 的 get_switch_priority 用默认 delay=2）。
func (s *Session) canResonanceStep2For(agent string, now time.Time) bool {
	at, ok := s.lastSkill[agent]
	return ok && now.Sub(at) < 2*time.Second
}

// noteAction 在扩展返回动作时补记动作时间（扩展直接按键/重击的情况）。
func (s *Session) noteAction(action assist.Action, frame assist.Frame) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	switch {
	case action.Kind == assist.ActionClickHold:
		s.noteHeavy(frame, now)
	case action.Kind == assist.ActionKeyTap && s.spec.Keys.Liberation != "" && action.Key == s.spec.Keys.Liberation:
		s.noteLiberation(frame, now)
	case action.Kind == assist.ActionKeyTap && s.spec.Keys.Skill != "" && action.Key == s.spec.Keys.Skill:
		s.noteSkill(frame, now)
	}
}

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
	// 上游 _ensure_ring_index()：先看当前角色是哪个元素（哪种颜色占比最高），
	// 再用这一种颜色算覆盖率，避免 6 种颜色混在一起把不该满的环算满。
	coverage := 0.0
	for _, ratio := range vision.RingCoverages(frame.Image, c.Rect, c.Colors, c.Inner, c.Outer) {
		if ratio > coverage {
			coverage = ratio
		}
	}
	// 上游 switch_next_char 里要求 current_con == 1 才切；0.8~0.99 之间会 sleep(0.05)
	// 再查一次，也就是"差一点没满就再等一帧"。这里用 0.95 作为满的门限。
	full := c.FullRatio
	if full <= 0 {
		full = defaultConcertoFullRatio
	}
	if coverage < full {
		return assist.Action{}, false
	}
	// 记下切走之前的槽位，切完要确认真的换了人。
	if s.spec.Team != nil {
		if cur, ok, err := s.spec.Team.Current(runtimeAdapter{rt: s.rt}, frame); err == nil && ok {
			s.pendingFrom = cur
		} else {
			s.pendingFrom = -1
		}
	}
	s.noteSwitch(frame, now)
	s.pendingAt = now
	s.pendingDone = false
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
	bestIndex := -1
	currentRole := Role("")
	agent := s.currentAgent(frame)
	if current >= 0 && current < len(ids) {
		currentRole = s.spec.Roles[ids[current]]
	}
	for i, id := range ids {
		if i == current || i >= len(keys) {
			continue
		}
		// 治疗位刚被切进来过就跳过（上游 HEALER_FULL_CON_SWITCH_LOCKOUT）。
		if s.spec.Roles[id] == RoleHealer && s.healerLocked(i, now) {
			continue
		}
		// 上游 get_switch_priority 的 NO 锁：最近重击/技能还在窗口内时跳过。
		if s.switchLocked(id, now) {
			continue
		}
		score := roleScore(currentRole, s.spec.Roles[id]) + s.staleBonus(i, now) + s.ultBonus(frame, i) + s.buffBonus(i, id, now) + s.synergyBonus(id, agent) + s.liberationBonus(id, now) + s.resonanceBonus(id, now) + s.introMustBonus(id)
		if score > best {
			best, bestKey, bestIndex = score, keys[i], i
		}
	}
	if bestKey == "" {
		return s.nextSwitchKey(frame)
	}
	s.switchInAt[bestIndex] = now
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

// synergyScore 是"上游 MUST 级协同"的分数，压过常规定位打分。
const synergyScore = 1000

// synergyBonus 判断这个候选角色是不是"当前在场角色的必须接手者"。
func (s *Session) synergyBonus(candidate, current string) float64 {
	if current == "" || candidate == current {
		return 0
	}
	for _, trigger := range s.spec.SwitchSynergy[candidate] {
		if trigger == current {
			return synergyScore
		}
	}
	return 0
}

// switchLocked 对应上游 get_switch_priority() 返回 SwitchPriority.NO 的冷却锁：
// 候选角色最近的重击/技能还在各自窗口内时，不参与本次切人打分。
func (s *Session) switchLocked(agent string, now time.Time) bool {
	if d := s.spec.SwitchNoAfterHeavy[agent]; d > 0 {
		if at, ok := s.lastHeavy[agent]; ok && now.Sub(at) < d {
			return true
		}
	}
	if d := s.spec.SwitchNoAfterSkill[agent]; d > 0 {
		if at, ok := s.lastSkill[agent]; ok && now.Sub(at) < d {
			return true
		}
	}
	if d := s.spec.SwitchNoAfterLiberation[agent]; d > 0 {
		if at, ok := s.lastLiberation[agent]; ok && now.Sub(at) < d {
			return true
		}
	}
	if d := s.spec.SwitchNoAfterPerform[agent]; d > 0 {
		if at, ok := s.lastPerform[agent]; ok && now.Sub(at) < d {
			return true
		}
	}
	return false
}

// introMustBonus 给"只要协奏入场就必须切"的候选角色 MUST 级加分：
// 对应上游 Camellya / Jinhsi 的 get_switch_priority() 里 has_intro -> MUST。
func (s *Session) introMustBonus(agent string) float64 {
	if s.spec.SwitchIntroMUST[agent] {
		return synergyScore
	}
	return 0
}

// resonanceBonus 给"最近共鸣技能仍在 can_resonance_step2 窗口内"的候选角色 MUST 级加分：
// 对应上游 Encore 的 get_switch_priority() -> still_in_liberation() or can_resonance_step2() 返回 MUST。
func (s *Session) resonanceBonus(agent string, now time.Time) float64 {
	if !s.spec.ResonanceMustSwitch[agent] {
		return 0
	}
	if s.canResonanceStep2For(agent, now) {
		return synergyScore
	}
	return 0
}

// liberationBonus 给"仍在解放动画窗口内"的候选角色 MUST 级加分：
// 对应上游 Brant/Encore/Lupa 的 get_switch_priority() -> still_in_liberation() 返回 MUST。
func (s *Session) liberationBonus(agent string, now time.Time) float64 {
	if !s.spec.LiberationMustSwitch[agent] {
		return 0
	}
	if s.inLiberationFor(agent, now) {
		return synergyScore
	}
	return 0
}

// buffBonus 给"增益已经掉光"的治疗/副C/辅助加分：上游 _unbuffed_support_target()
// 在候选里优先挑计时增益已过期的治疗与副C，让它们上来补增益。
func (s *Session) buffBonus(slot int, agent string, now time.Time) float64 {
	if len(s.spec.BuffTimes) == 0 {
		return 0
	}
	switch s.spec.Roles[agent] {
	case RoleHealer, RoleSubDPS, RoleSupport:
	default:
		return 0
	}
	buff, ok := s.spec.BuffTimes[agent]
	if !ok || buff <= 0 {
		return 0
	}
	lastIn, seen := s.switchInAt[slot]
	if seen && now.Sub(lastIn) < buff {
		return 0 // 增益还在，不急
	}
	return 8
}

// healerLocked 判断治疗位是否还在切人冷却里。
func (s *Session) healerLocked(slot int, now time.Time) bool {
	lockout := DefaultHealerLockout
	if s.spec.Switch != nil && s.spec.Switch.HealerLockout > 0 {
		lockout = s.spec.Switch.HealerLockout
	}
	at, ok := s.switchInAt[slot]
	return ok && now.Sub(at) < lockout
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
	// 切走之前先记下离场的人：这就是下一次入场的延奏来源（上游 check_outro）。
	if id := s.currentAgent(frame); id != "" {
		s.outroAgent = id
	}
	s.switchAt = now
	s.cdCache = make(map[StepKind]cdEntry)
	s.agentAt = time.Time{}
	s.agentID = ""
	s.introLeft = append([]Step(nil), s.spec.IntroSteps...)
	// 入场行为的角色专属覆盖要等"切进来的是谁"确定后再定，这里先按默认值开启窗口。
	if s.spec.Intro != nil {
		limit := s.spec.Intro.MaxDuration
		if limit <= 0 {
			limit = 1200 * time.Millisecond
		}
		s.introUntil = now.Add(limit)
		s.introMode = s.spec.Intro.Mode
	}
	_ = frame
}

// forteAllows 判断当前角色的技能是否可以放：上游部分角色要等共鸣回路满/未满。
// 没配判据、认不出角色、或该角色不在门控表里都直接放行。
func (s *Session) forteAllows(frame assist.Frame) bool {
	if s.spec.ForteFull == nil || len(s.spec.ForteGate) == 0 {
		return true
	}
	agent := s.currentAgent(frame)
	if agent == "" {
		return true
	}
	need, ok := s.spec.ForteGate[agent]
	if !ok {
		return true
	}
	ratio := vision.ColorRatio(frame.Image, s.spec.ForteFull.Area, s.spec.ForteFull.ColorRange)
	minRatio := s.spec.ForteFull.MinRatio
	if minRatio <= 0 {
		minRatio = 0.08
	}
	full := ratio >= minRatio
	if need == ForteNeedNotFull {
		return !full
	}
	return full
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
