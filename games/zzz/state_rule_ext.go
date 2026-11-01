package zzz

import (
	"strings"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
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

	queue   []combat.Step
	agentID string
	agentAt time.Time
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
	if e.agentAt.IsZero() || now.Sub(e.agentAt) >= 2*time.Second {
		e.agentAt = now
		if id, ok, err := e.Team.Agent(rt, frame); err == nil && ok {
			e.agentID = id
		} else {
			e.agentID = ""
		}
	}
	rules := agentRules[e.agentID]
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
	return false
}

func (e *zzzStateRuleExt) lit(frame assist.Frame, rect geom.Rect) bool {
	threshold := e.Threshold
	if threshold <= 0 {
		threshold = 0.05
	}
	return vision.ColorRatio(frame.Image, rect, brightRange) >= threshold
}

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
	}
	return "动作"
}
