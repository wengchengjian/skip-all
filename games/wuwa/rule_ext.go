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

	lastAt  time.Time
	queue   []combat.Step
	agentID string

	// state 是会话推送进来的战斗状态（协奏是否满、是否还在入场、延奏来源）。
	state combat.CombatState
}

// SetCombatState 实现 combat.StateAware。
func (e *wuwaRuleExt) SetCombatState(s combat.CombatState) { e.state = s }

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
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	// 规则队列：上一条规则的后续动作还没打完就接着打。
	if len(e.queue) > 0 {
		step := e.queue[0]
		e.queue = e.queue[1:]
		if action, ok := e.actionFor(step); ok {
			return action, "规则-队列", true, nil
		}
	}
	// 没有专属规则的角色沿用上游 BaseChar.do_perform 的保守动作（"__base__"）。
	rules := e.Rules[agent]
	if len(rules) == 0 {
		rules = e.Rules[baseRuleKey]
	}
	for _, rule := range rules {
		if !e.eval(rule.Cond, frame) || len(rule.Ops) == 0 {
			continue
		}
		e.queue = append([]combat.Step(nil), rule.Ops[1:]...)
		if action, ok := e.actionFor(rule.Ops[0]); ok {
			return action, "规则-" + rule.Cond, true, nil
		}
	}
	return assist.Action{}, "", false, nil
}

// actionFor 把一步动作翻译成输入。
func (e *wuwaRuleExt) actionFor(step combat.Step) (assist.Action, bool) {
	switch step.Kind {
	case combat.StepAttack:
		return assist.Action{Kind: assist.ActionClick, Point: geom.Point{X: 960, Y: 540}}, true
	case combat.StepHeavy:
		return assist.Action{Kind: assist.ActionClickHold, Point: geom.Point{X: 960, Y: 540}, Duration: 600 * time.Millisecond}, true
	case combat.StepSkill:
		return assist.Action{Kind: assist.ActionKeyTap, Key: "e"}, true
	case combat.StepEcho:
		return assist.Action{Kind: assist.ActionKeyTap, Key: "q"}, true
	case combat.StepLiberation:
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
