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
	// StateArea 是角色状态图标所在区域（血条下方的能量/层数条），
	// 用来判定 [<角色>-<状态>] 这类颜色条件。
	StateArea geom.Rect
	// FlashArea 是闪避闪光出现的区域，用来判定 自定义-红光闪避 / 自定义-黄光切人。
	FlashArea geom.Rect

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
	// 闪避闪光：黄光是切人时机，红光是闪避时机。
	switch name {
	case "自定义-黄光切人":
		return e.flash(frame, 0)
	case "自定义-红光闪避":
		return e.flash(frame, 1)
	}
	return false
}

// flash 判定闪避闪光：0 黄光、1 红光。
func (e *zzzStateRuleExt) flash(frame assist.Frame, index int) bool {
	area := e.FlashArea
	if area.X2 <= area.X1 || area.Y2 <= area.Y1 || index >= len(flashColors) {
		return false
	}
	return vision.ColorRatio(frame.Image, area, flashColors[index]) >= flashRatio
}

// matchState 照上游的 COLOR_RANGE_CONNECT：区域里出现足够多的该颜色块就算状态成立。
func (e *zzzStateRuleExt) matchState(frame assist.Frame, check StateCheck) bool {
	area := e.StateArea
	if area.X2 <= area.X1 || area.Y2 <= area.Y1 {
		return false
	}
	required := check.Connect
	if required < 1 {
		required = 1
	}
	rects := vision.FindColorRectangles(frame.Image, area, [2][3]uint8{check.Lower, check.Upper}, 2, 2, 200, 0.9)
	return len(rects) >= required
}

func (e *zzzStateRuleExt) lit(frame assist.Frame, rect geom.Rect) bool {
	threshold := e.Threshold
	if threshold <= 0 {
		threshold = 0.05
	}
	return vision.ColorRatio(frame.Image, rect, brightRange) >= threshold
}

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
