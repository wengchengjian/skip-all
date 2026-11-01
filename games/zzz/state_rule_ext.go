package zzz

import (
	"strconv"
	"strings"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// zzzStateRuleExt 按上游的"条件 -> 动作"规则出招。
// 规则表见 state_rules.go（从上游 state_handler 提取）。
// 条件里能识别的原子：
//   - [按键可用-特殊攻击] / [按键可用-终结技]：看按钮是否亮起，同 Ready 判据
//   - [按键可用-普通攻击] / [按键可用-闪避]：恒定可用
//   - 其余（自定义-*、角色状态、切换角色-*）：本项目没有对应状态，按不成立处理
type zzzStateRuleExt struct {
	Team      *zzzTeam
	SkillArea geom.Rect
	UltArea   geom.Rect
	Threshold float64
	// StateArea 是角色状态图标所在区域（血条下方的能量/层数条），
	// 用来判定 [<角色>-<状态>] 这类颜色条件。
	StateArea geom.Rect
	// FlashArea 是闪避闪光出现的区域，用来判定 自定义-红光闪避 / 自定义-黄光切人。
	FlashArea geom.Rect

	queue   []combat.Step
	agentID string
	agentAt time.Time
	// 站场角色变了就说明刚切过人，用来判定 [按键-切换角色-下一个/上一个, 0, N]。
	lastAgent  string
	switchedAt time.Time
	// 连携条出现说明敌人进入失衡（连携只在失衡时可用），
	// 用它来近似 [自定义-失衡时间, 0, N]。
	ChainBar  geom.Rect
	ChainRGB  [2][3]uint8
	ChainRate float64
	stunAt    time.Time
	// energyThreshold 是能量条模板的匹配阈值。
	energyThreshold float64
	// AssistArea/AssistIDs 是快速支援按钮，用来判定 [自定义-快速支援换人]。
	AssistArea geom.Rect
	AssistIDs  []string
	assistAt   time.Time
	parryAt    time.Time
	dodgeAt    time.Time
}

func (e *zzzStateRuleExt) Name() string { return "state_rules" }

func (e *zzzStateRuleExt) Step(rt combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	if len(e.queue) > 0 {
		step := e.queue[0]
		e.queue = e.queue[1:]
		if action, ok := ruleAction(step); ok {
			return action, "规则-" + ruleState(step), true, nil
		}
		return assist.Action{}, "", false, nil
	}
	if e.Team == nil {
		return assist.Action{}, "", false, nil
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	// 支援/格挡/闪避的时间戳：上游这些也是状态机内部变量，这里用画面特征打点。
	if e.AssistArea.X2 > e.AssistArea.X1 && len(e.AssistIDs) > 0 {
		for _, id := range e.AssistIDs {
			matches, err := rt.MatchTemplate(frame, screen.Area{Name: "快速支援", Rect: e.AssistArea}, vision.TemplateRef{SubDir: "combat", ID: id}, 0.85)
			if err == nil && len(matches) > 0 {
				e.assistAt = now
				break
			}
		}
	}
	if e.flash(frame, 0) {
		e.parryAt = now
	}
	if e.flash(frame, 1) {
		e.dodgeAt = now
	}

	// 连携条可见 = 敌人失衡中，刷新失衡时间戳。
	if vision.ColorRatio(frame.Image, e.ChainBar, e.ChainRGB) >= e.chainRate() {
		e.stunAt = now
	}

	// 每帧确认站场角色：白名单学习过之后只比一两张模板，开销很小；
	// 角色变化本身就是"刚切过人"的证据。
	if id, ok, err := e.Team.Agent(rt, frame); err == nil && ok {
		if e.lastAgent != "" && id != e.lastAgent {
			e.switchedAt = now
		}
		e.lastAgent = id
		e.agentID = id
		e.agentAt = now
	} else {
		e.agentID = ""
	}
	// 先跑整队节奏规则（专属配队），再跑这个角色自己的规则。
	rules := make([]StateRule, 0, len(teamRules[e.agentID])+len(agentRules[e.agentID]))
	rules = append(rules, teamRules[e.agentID]...)
	rules = append(rules, agentRules[e.agentID]...)
	for _, rule := range rules {
		if !e.eval(rt, frame, rule.Cond) || len(rule.Ops) == 0 {
			continue
		}
		e.queue = append([]combat.Step(nil), rule.Ops[1:]...)
		if action, ok := ruleAction(rule.Ops[0]); ok {
			return action, "规则-" + ruleState(rule.Ops[0]), true, nil
		}
	}
	return assist.Action{}, "", false, nil
}

// eval 计算条件表达式：支持 ! & | 和括号，原子写在方括号里。
func (e *zzzStateRuleExt) eval(rt combat.Runtime, frame assist.Frame, cond string) bool {
	p := &condParser{src: cond, ext: e, rt: rt, frame: frame}
	ok, _ := p.parseOr()
	return ok
}

type condParser struct {
	src   string
	pos   int
	ext   *zzzStateRuleExt
	rt    combat.Runtime
	frame assist.Frame
}

func (p *condParser) skipSpace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

func (p *condParser) parseOr() (bool, bool) {
	left, ok := p.parseAnd()
	for ok {
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '|' {
			break
		}
		p.pos++
		right, rok := p.parseAnd()
		if !rok {
			break
		}
		left = left || right
	}
	return left, ok
}

func (p *condParser) parseAnd() (bool, bool) {
	left, ok := p.parseUnary()
	for ok {
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '&' {
			break
		}
		p.pos++
		right, rok := p.parseUnary()
		if !rok {
			break
		}
		left = left && right
	}
	return left, ok
}

func (p *condParser) parseUnary() (bool, bool) {
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == '!' {
		p.pos++
		value, ok := p.parseUnary()
		return !value, ok
	}
	if p.pos < len(p.src) && p.src[p.pos] == '(' {
		p.pos++
		value, ok := p.parseOr()
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == ')' {
			p.pos++
		}
		return value, ok
	}
	if p.pos < len(p.src) && p.src[p.pos] == '[' {
		end := strings.IndexByte(p.src[p.pos:], ']')
		if end < 0 {
			return false, false
		}
		atom := p.src[p.pos+1 : p.pos+end]
		p.pos += end + 1
		return p.ext.atom(p.rt, p.frame, atom), true
	}
	return false, false
}

// atom 判定单个条件原子。
func (e *zzzStateRuleExt) atom(rt combat.Runtime, frame assist.Frame, raw string) bool {
	name := strings.TrimSpace(raw)
	if idx := strings.IndexByte(name, ','); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	switch name {
	case "按键可用-普通攻击", "按键可用-闪避", "按键可用-快速支援":
		return true
	case "按键可用-特殊攻击":
		return e.lit(frame, e.SkillArea)
	case "按键可用-终结技":
		return e.lit(frame, e.UltArea)
	}
	// [<角色>-状态>]：用上游 agent.py 的颜色范围 + 连通块数量判定。
	if check, ok := agentStateChecks[name]; ok {
		return e.matchState(frame, check)
	}
	// [<角色>-特殊技可用] / [<角色>-终结技可用]：和按钮就绪是一回事。
	if strings.HasSuffix(name, "-特殊技可用") && strings.Count(name, "-") >= 1 {
		return e.lit(frame, e.SkillArea)
	}
	if strings.HasSuffix(name, "-终结技可用") {
		return e.lit(frame, e.UltArea)
	}
	// [前台-能量] / [后台-1-能量] / [后台-2-能量]：读能量条填充长度（0-120）。
	// 上游用 energy_3_N 模板定位能量条，再量 FOREGROUND_GRAY_RANGE_LENGTH。
	if slot, ok := energySlot(name); ok {
		return e.energyInRange(rt, frame, raw, slot)
	}
	// [后台-1-击破] / [后台-2-击破]：后台第 N 位的代理人是不是"击破"定位。
	if kind, index, ok := parseBackSlot(name); ok {
		return e.backSlotHasKind(rt, frame, index, kind)
	}
	// [自定义-招架支援] / [自定义-闪避] / [自定义-快速支援换人]：用画面特征打点后判时间窗。
	if name == "自定义-招架支援" {
		return e.sinceWithin(e.parryAt, raw, frame)
	}
	if name == "自定义-闪避" {
		return e.sinceWithin(e.dodgeAt, raw, frame)
	}
	if name == "自定义-快速支援换人" {
		return e.sinceWithin(e.assistAt, raw, frame)
	}
	// [自定义-终结技被强制释放]：上游在前台角色终结技可用时打这个标记
	// （auto_battle_agent_context.py 的自动终结技逻辑），这里等价成"终结技可用"。
	if name == "自定义-终结技被强制释放" {
		return e.lit(frame, e.UltArea)
	}
	// [自定义-动作不打断]：规则队列里还有动作没打完，等价于"正在出招"。
	if name == "自定义-动作不打断" {
		return len(e.queue) > 0
	}
	// [自定义-失衡时间, 0, N]：连携条可见期间及之后 N 秒（连携只在失衡时出现）。
	// [自定义-连携换人, 0, N]：同理——连携期间换过人就发生在看到连携条之后。
	if name == "自定义-失衡时间" || name == "自定义-连携换人" {
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		return e.stunWithin(raw, now)
	}
	// [按键-切换角色-下一个/上一个, 0, N]：站场角色刚换过就算成立
	// （本项目分不出是"下一个"还是"上一个"，两者按同一件事处理）。
	if strings.HasPrefix(name, "按键-切换角色") {
		now := frame.At
		if now.IsZero() {
			now = time.Now()
		}
		return e.switchedWithin(raw, now)
	}
	// 闪避闪光：黄光是切人时机，红光是闪避时机。
	switch name {
	case "自定义-黄光切人":
		return e.flash(frame, 0)
	case "自定义-红光闪避":
		return e.flash(frame, 1)
	}
	return false
}

// chainRate 给连携条的金色占比阈值一个默认值。
func (e *zzzStateRuleExt) chainRate() float64 {
	if e.ChainRate > 0 {
		return e.ChainRate
	}
	return 0.5
}

// stunWithin 判断 [自定义-失衡时间, 0, N]：距最近一次"看到连携条"多久。
func (e *zzzStateRuleExt) stunWithin(raw string, now time.Time) bool {
	if e.stunAt.IsZero() {
		return false
	}
	lo, hi, ok := parseWindow(raw)
	if !ok {
		return true
	}
	elapsed := now.Sub(e.stunAt).Seconds()
	return elapsed >= lo && elapsed <= hi
}

// switchedWithin 判断"最近一次切人"是否落在条件给的时间窗内。
func (e *zzzStateRuleExt) switchedWithin(raw string, now time.Time) bool {
	if e.switchedAt.IsZero() {
		return false
	}
	lo, hi, ok := parseWindow(raw)
	if !ok {
		return true
	}
	elapsed := now.Sub(e.switchedAt).Seconds()
	return elapsed >= lo && elapsed <= hi
}

// parseWindow 解析 "[原子, 0, 1.5]" 里的时间窗部分。
func parseWindow(raw string) (float64, float64, bool) {
	parts := strings.Split(raw, ",")
	if len(parts) < 3 {
		return 0, 0, false
	}
	lo, err1 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	hi, err2 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return lo, hi, true
}

// flash 判定闪避闪光：0 黄光、1 红光。
func (e *zzzStateRuleExt) flash(frame assist.Frame, index int) bool {
	area := e.FlashArea
	if area.X2 <= area.X1 || area.Y2 <= area.Y1 || index >= len(flashColors) {
		return false
	}
	return vision.ColorRatio(frame.Image, area, flashColors[index]) >= flashRatio
}

// energySlot 把能量条件映射到槽位：前台=0，后台-1=1，后台-2=2。
// [<角色>-能量] 这类角色专属写法按前台处理。
func energySlot(name string) (int, bool) {
	switch name {
	case "前台-能量":
		return 0, true
	case "后台-1-能量":
		return 1, true
	case "后台-2-能量":
		return 2, true
	}
	if strings.HasSuffix(name, "-能量") {
		return 0, true
	}
	return 0, false
}

// energyInRange 读能量条并判断是否落在条件给的范围里（上游的量程是 0-120）。
func (e *zzzStateRuleExt) energyInRange(rt combat.Runtime, frame assist.Frame, raw string, slot int) bool {
	value, ok := e.energyValue(rt, frame, slot)
	if !ok {
		return false
	}
	lo, hi, hasWindow := parseWindow(raw)
	if !hasWindow {
		return value > 0
	}
	return value >= int(lo) && value <= int(hi)
}

// energyValue 在限定区域里找能量条，量出灰度填充长度。
func (e *zzzStateRuleExt) energyValue(rt combat.Runtime, frame assist.Frame, slot int) (int, bool) {
	if slot >= len(energyTemplateIDs) {
		return 0, false
	}
	threshold := e.energyThreshold
	if threshold <= 0 {
		threshold = 0.6
	}
	area := screen.Area{Name: "能量条", Rect: energySearchArea}
	matches, err := rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: "energy", ID: energyTemplateIDs[slot]}, threshold)
	if err != nil || len(matches) == 0 {
		return 0, false
	}
	return vision.GrayRunLength(frame.Image, matches[0].Rect, energyGrayLower, energyGrayUpper), true
}

// sinceWithin 判断"某个事件是否发生在条件给的时间窗内"。
func (e *zzzStateRuleExt) sinceWithin(at time.Time, raw string, frame assist.Frame) bool {
	if at.IsZero() {
		return false
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	lo, hi, ok := parseWindow(raw)
	if !ok {
		return true
	}
	elapsed := now.Sub(at).Seconds()
	return elapsed >= lo && elapsed <= hi
}

// parseBackSlot 解析 "后台-1-击破" 这类条件。
func parseBackSlot(name string) (kind string, index int, ok bool) {
	parts := strings.Split(name, "-")
	if len(parts) != 3 || parts[0] != "后台" {
		return "", 0, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 1 {
		return "", 0, false
	}
	return parts[2], n, true
}

// backSlotHasKind 判断后台第 index 位（1 起）的代理人是否属于某定位。
func (e *zzzStateRuleExt) backSlotHasKind(rt combat.Runtime, frame assist.Frame, index int, kind string) bool {
	if e.Team == nil {
		return false
	}
	ids, err := e.Team.Slots(rt, frame)
	if err != nil || index >= len(ids) {
		return false
	}
	// 槽位顺序是 前台 + 后台两位，后台第 1 位对应下标 1。
	slot := index
	if slot >= len(ids) || ids[slot] == "" {
		return false
	}
	typeName := map[string]string{
		"击破": AgentTypeStun,
		"强攻": AgentTypeAttack,
		"异常": AgentTypeAnomaly,
		"支援": AgentTypeSupport,
		"防护": AgentTypeDefense,
		"命破": AgentTypeRupture,
	}[kind]
	if typeName == "" {
		return false
	}
	return agentsByType[typeName][ids[slot]]
}

// matchState 照上游的 COLOR_RANGE_CONNECT：区域里出现足够多的该颜色块就算状态成立。
func (e *zzzStateRuleExt) matchState(frame assist.Frame, check StateCheck) bool {
	area := e.StateArea
	if area.X2 <= area.X1 || area.Y2 <= area.Y1 {
		return false
	}
	colors := [2][3]uint8{check.Lower, check.Upper}
	switch check.Way {
	case "COLOR_RANGE_CONNECT", "COLOR_CHANNEL_EQUAL_RANGE_CONNECT":
		required := check.Connect
		if required < 1 {
			required = 1
		}
		return len(vision.FindColorRectangles(frame.Image, area, colors, 2, 2, 200, 0.9)) >= required
	case "COLOR_RANGE_EXIST", "COLOR_CHANNEL_MAX_RANGE_EXIST",
		"FOREGROUND_COLOR_RANGE_LENGTH", "BACKGROUND_COLOR_RANGE_LENGTH",
		"FOREGROUND_GRAY_RANGE_LENGTH", "BACKGROUND_GRAY_RANGE_LENGTH",
		"COLOR_CHANNEL_EQUAL_RANGE_EXIST":
		// 这几类上游量的是"长度/是否存在"，这里统一按颜色占比是否大于 0 近似。
		return vision.ColorRatio(frame.Image, area, colors) > 0
	}
	return false
}

func (e *zzzStateRuleExt) lit(frame assist.Frame, rect geom.Rect) bool {
	threshold := e.Threshold
	if threshold <= 0 {
		threshold = 0.05
	}
	return vision.ColorRatio(frame.Image, rect, brightRange) >= threshold
}

// 能量条的搜索区域与量程（上游 max_length=120，灰度 90-255）。
var (
	energySearchArea  = geom.Rect{X1: 40, Y1: 940, X2: 460, Y2: 1080}
	energyTemplateIDs = []string{"energy_3_1", "energy_3_2", "energy_3_3"}
)

const (
	energyGrayLower = 90
	energyGrayUpper = 255
)

// flashColors / flashRatio 与 zzzDodgeExt 保持一致。
var flashColors = [][2][3]uint8{
	{{225, 195, 60}, {255, 255, 165}}, // 黄光
	{{215, 35, 35}, {255, 110, 110}},  // 红光
}

const flashRatio = 0.03

func ruleAction(step combat.Step) (assist.Action, bool) {
	switch step.Kind {
	case combat.StepAttack:
		return assist.Action{Kind: assist.ActionClick, Point: geom.Point{X: 960, Y: 540}}, true
	case combat.StepSkill:
		return assist.Action{Kind: assist.ActionKeyTap, Key: "e"}, true
	case combat.StepLiberation:
		return assist.Action{Kind: assist.ActionKeyTap, Key: "q"}, true
	case combat.StepDodge:
		return assist.Action{Kind: assist.ActionKeyTap, Key: "shift"}, true
	case combat.StepKey:
		if step.Key == "" {
			return assist.Action{}, false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: step.Key}, true
	}
	return assist.Action{}, false
}

func ruleState(step combat.Step) string {
	switch step.Kind {
	case combat.StepAttack:
		return "普攻"
	case combat.StepSkill:
		return "特殊技"
	case combat.StepLiberation:
		return "终结技"
	case combat.StepDodge:
		return "闪避"
	case combat.StepKey:
		return "按键"
	}
	return "动作"
}
