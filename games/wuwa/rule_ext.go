package wuwa

import (
	"strings"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// wuwaRule 是"条件 -> 动作序列"的一条规则，条件语法与绝区零的规则引擎一致：
// 支持 ! & | ()，原子由 cond 解析函数给出。
// baseRuleKey 是"没有专属规则的角色"共用的回退规则表。
const baseRuleKey = "__base__"

type wuwaRule struct {
	Cond string
	Ops  []combat.Step
}

// wuwaRuleExt 按角色执行规则表，先命中先执行；都不命中则交回通用连招。
// 这是逐角色 do_perform() 条件分支的数据化实现。
type wuwaRuleExt struct {
	Team       combat.Team
	Skill      geom.Rect
	Echo       geom.Rect
	Liberation geom.Rect
	Ready      [2][3]uint8 // 就绪白（上游 text_white_color）
	Forte      geom.Rect   // 回路满判据区域
	ForteRGB   [2][3]uint8
	Rules      map[string][]wuwaRule
	// OutroRules 是 check_outro() 特殊分支的近似多步收尾：agent -> 延奏来源 -> 规则。
	// 只有在 InIntro 且 LastOutro 匹配时才替换该角色的普通规则。
	OutroRules map[string]map[string][]wuwaRule

	lastAt  time.Time
	queue   []combat.Step
	agentID string
	// cursor 是规则轮转下标：规则表里很多条的条件都恒真（"1"），
	// 如果每帧都从第 0 条开始挑，后面的声骸/技能/重击永远轮不到。
	// 所以一帧只走一条，下一帧从下一条开始，相当于把上游 do_perform 的
	// 出手顺序摊到多帧上。
	cursor int
	// lastRule/lastRuleAt 是上一次真正出手的规则，用来防止同一条规则
	// 在别的规则都不可用时每帧重复触发（上游每个动作本身都要花时间）。
	lastRule   int
	lastRuleAt time.Time

	// state 是会话推送进来的战斗状态（协奏是否满、是否还在入场、延奏来源）。
	state combat.CombatState
}

// SetCombatState 实现 combat.StateAware。
func (e *wuwaRuleExt) SetCombatState(s combat.CombatState) { e.state = s }

// ruleRepeatGuard 是同一条规则的最小重复间隔。
// 规则表是从上游 do_perform 的条件分支机械提取的，同一角色往往有多条
// 恒真条件；没有这个 guard 时，冷却中的技能会让轮转绕回同一条规则，
// 表现就是"一直重击/一直普攻不出招"。
const ruleRepeatGuard = 500 * time.Millisecond

// Name 实现 combat.Extension。
func (e *wuwaRuleExt) Name() string { return "rules" }

// Step 实现 combat.Extension。
func (e *wuwaRuleExt) Step(rt combat.Runtime, frame assist.Frame) (assist.Action, string, bool, error) {
	if e.Team == nil || len(e.Rules) == 0 {
		return assist.Action{}, "", false, nil
	}
	agent, ok, err := e.Team.Agent(rt, frame)
	if err != nil || !ok {
		return assist.Action{}, "", false, nil
	}
	if agent != e.agentID {
		e.agentID = agent
		e.queue = nil
		e.cursor = 0
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	// 协奏值满了就让位：上游每个角色的 do_perform 末尾都会 switch_next_char()，
	// 判断依据就是协奏满。规则引擎是每帧都要出招的，如果在这里继续接管，
	// 共享流程里的 concertoSwitch 永远轮不到，表现就是"协奏满了也不切人"。
	if e.state.ConcertoFull {
		return assist.Action{}, "", false, nil
	}
	// 规则队列：上一条规则的后续动作还没打完就接着打。
	if len(e.queue) > 0 {
		step := e.queue[0]
		e.queue = e.queue[1:]
		if action, ok := e.actionFor(frame, step); ok {
			return action, "规则-队列", true, nil
		}
	}
	// 没有专属规则的角色沿用上游 BaseChar.do_perform 的保守动作（"__base__"）。
	rules := e.Rules[agent]
	if e.state.InIntro && e.state.LastOutro != "" {
		if outro := e.OutroRules[agent][e.state.LastOutro]; len(outro) > 0 {
			rules = outro
		}
	}
	if len(rules) == 0 {
		rules = e.Rules[baseRuleKey]
	}
	// 从 cursor 开始轮转：一帧只出一条规则，下一次从下一条继续。
	// 规则表里很多条的条件恒真（"1"），若每帧都从第 0 条挑，后面的
	// 声骸/技能/重击永远轮不到，表现就是"一直普攻不出招"。
	// 条件不成立的规则直接跳过，全部不成立就交回通用连招。
	for i := 0; i < len(rules); i++ {
		index := (e.cursor + i) % len(rules)
		rule := rules[index]
		if len(rule.Ops) == 0 || !e.eval(rule.Cond, frame) {
			continue
		}
		action, ok := e.actionFor(frame, rule.Ops[0])
		if !ok {
			continue
		}
		// 同一条规则在 guard 时间内不重复：重击要按住 600ms、技能有前后摇，
		// 连续两帧都出同一条会把连招压成单动作。
		if index == e.lastRule && !e.lastRuleAt.IsZero() && now.Sub(e.lastRuleAt) < ruleRepeatGuard {
			continue
		}
		e.cursor = (index + 1) % len(rules)
		e.lastRule, e.lastRuleAt = index, now
		e.queue = append([]combat.Step(nil), rule.Ops[1:]...)
		return action, "规则-" + rule.Cond, true, nil
	}
	e.cursor = 0
	return assist.Action{}, "", false, nil
}

// actionFor 把一步动作翻译成输入。
// 技能类动作要先确认按钮可用：上游 click_resonance/click_echo/click_liberation
// 内部都会先判 available()，冷却中直接返回 false。规则表里的条件多数只写"1"，
// 所以这个可用性判断必须在这里补上，否则会对着冷却中的技能狂按 E。
func (e *wuwaRuleExt) actionFor(frame assist.Frame, step combat.Step) (assist.Action, bool) {
	switch step.Kind {
	case combat.StepAttack:
		return assist.Action{Kind: assist.ActionClick, Point: geom.Point{X: 960, Y: 540}}, true
	case combat.StepHeavy:
		return assist.Action{Kind: assist.ActionClickHold, Point: geom.Point{X: 960, Y: 540}, Duration: 600 * time.Millisecond}, true
	case combat.StepSkill:
		if !e.lit(frame, e.Skill) {
			return assist.Action{}, false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: "e"}, true
	case combat.StepEcho:
		if !e.lit(frame, e.Echo) {
			return assist.Action{}, false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: "q"}, true
	case combat.StepLiberation:
		if !e.lit(frame, e.Liberation) {
			return assist.Action{}, false
		}
		return assist.Action{Kind: assist.ActionKeyTap, Key: "r"}, true
	}
	return assist.Action{}, false
}

// lit 判断某个按钮区域是否出现就绪白点。
func (e *wuwaRuleExt) lit(frame assist.Frame, area geom.Rect) bool {
	if area == (geom.Rect{}) {
		return true
	}
	return vision.ColorRatio(frame.Image, area, e.Ready) >= 0.002
}

// forteFull 判断回路是否已满。
func (e *wuwaRuleExt) forteFull(frame assist.Frame) bool {
	if e.Forte == (geom.Rect{}) {
		return false
	}
	return vision.ColorRatio(frame.Image, e.Forte, e.ForteRGB) >= 0.08
}

// eval 求值条件表达式（! & | 括号）。
func (e *wuwaRuleExt) eval(cond string, frame assist.Frame) bool {
	p := &wuwaCondParser{src: cond, ext: e, frame: frame}
	v := p.parseOr()
	return v && p.err == nil
}

type wuwaCondParser struct {
	src   string
	pos   int
	ext   *wuwaRuleExt
	frame assist.Frame
	err   error
}

func (p *wuwaCondParser) skip() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

func (p *wuwaCondParser) parseOr() bool {
	v := p.parseAnd()
	for {
		p.skip()
		if p.pos < len(p.src) && p.src[p.pos] == '|' {
			p.pos++
			r := p.parseAnd()
			v = v || r
			continue
		}
		return v
	}
}

func (p *wuwaCondParser) parseAnd() bool {
	v := p.parseUnary()
	for {
		p.skip()
		if p.pos < len(p.src) && p.src[p.pos] == '&' {
			p.pos++
			r := p.parseUnary()
			v = v && r
			continue
		}
		return v
	}
}

func (p *wuwaCondParser) parseUnary() bool {
	p.skip()
	if p.pos < len(p.src) && p.src[p.pos] == '!' {
		p.pos++
		return !p.parseUnary()
	}
	if p.pos < len(p.src) && p.src[p.pos] == '(' {
		p.pos++
		v := p.parseOr()
		p.skip()
		if p.pos < len(p.src) && p.src[p.pos] == ')' {
			p.pos++
		}
		return v
	}
	return p.parseAtom()
}

func (p *wuwaCondParser) parseAtom() bool {
	p.skip()
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '&' || c == '|' || c == ')' || c == '(' {
			break
		}
		p.pos++
	}
	atom := strings.TrimSpace(p.src[start:p.pos])
	switch atom {
	case "", "1", "true":
		return true
	case "技能可用":
		return p.ext.lit(p.frame, p.ext.Skill)
	case "声骸可用":
		return p.ext.lit(p.frame, p.ext.Echo)
	case "解放可用":
		return p.ext.lit(p.frame, p.ext.Liberation)
	case "回路满":
		return p.ext.forteFull(p.frame)
	case "回路未满":
		return !p.ext.forteFull(p.frame)
	case "协奏满":
		return p.ext.state.ConcertoFull
	case "入场中":
		return p.ext.state.InIntro
	case "飞行中":
		return p.ext.state.Flying
	case "解放中":
		return p.ext.state.InLiberation
	case "需要速切":
		return p.ext.state.NeedFastPerform
	case "共鸣二段":
		return p.ext.state.CanResonanceStep2
	}
	// 延奏=char_xxx 对应上游 check_outro()：入场时判断刚切走的是谁。
	// check_outro() 在 has_intro 为 false 时返回 'null'，所以必须叠加入场窗口。
	if source, ok := strings.CutPrefix(atom, "延奏="); ok {
		return p.ext.state.InIntro && p.ext.state.LastOutro == source
	}
	p.err = errUnknownAtom(atom)
	return false
}

type errUnknownAtom string

func (e errUnknownAtom) Error() string { return "未知条件原子: " + string(e) }
