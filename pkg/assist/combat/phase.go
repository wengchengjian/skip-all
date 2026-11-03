package combat

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// introPhase 是"切人后的入场窗口 + 入场技队列"。
//
// 这两件事生命周期不同，但都从 noteSwitch 开始、都需要在脱战/中断时清理：
//   - 窗口（deadline/mode）：只有 spec.Intro != nil 才有，到点或技能就绪就结束
//   - 队列（steps）：每次切人都挂上 spec.IntroSteps，逐拍消费
//
// 抽出来的原因：原先 enter/update/exit 散在 noteSwitch、nextAction、
// Step 的脱战分支、checkAbort 分支、switchHeld、pushCombatState 六处，
// 改一个入场模式要同时找齐这些地方。
type introPhase struct {
	deadline time.Time
	mode     IntroMode
	steps    []Step
}

// windowActive 报告入场窗口是否还开着。
func (p *introPhase) windowActive() bool { return !p.deadline.IsZero() }

// reset 清空入场窗口与队列（脱战用）。
func (p *introPhase) reset() {
	p.deadline = time.Time{}
	p.mode = IntroWait
	p.steps = nil
}

// clearSteps 只清队列，保留窗口。
func (p *introPhase) clearSteps() { p.steps = nil }

// enter 在切人时开启一个新入场。
func (p *introPhase) enter(now time.Time, spec Spec) {
	p.steps = append(p.steps[:0], spec.IntroSteps...)
	p.deadline = time.Time{}
	if spec.Intro != nil {
		limit := spec.Intro.MaxDuration
		if limit <= 0 {
			limit = 1200 * time.Millisecond
		}
		p.deadline = now.Add(limit)
		p.mode = spec.Intro.Mode
	}
}

// stepWindow 推进入场窗口。ok=true 表示本帧由入场接管并给出动作。
//
// 返回 ok=false 时窗口可能已结束（deadline 归零），调用方应继续走后续流程。
func (p *introPhase) stepWindow(s *Session, frame assist.Frame, now time.Time) (assist.Action, string, bool) {
	if p.deadline.IsZero() {
		return assist.Action{}, "", false
	}
	// 角色专属入场（如今汐的入场爆发）：切进来才知道是谁，所以在这里解析。
	if s.spec.Team != nil {
		if id := s.currentAgent(frame); id != "" {
			if override, ok := s.spec.IntroByAgent[id]; ok {
				p.mode = override.Mode
				limit := override.MaxDuration
				if limit <= 0 {
					limit = 1200 * time.Millisecond
				}
				if deadline := s.switchAt.Add(limit); deadline.After(p.deadline) {
					p.deadline = deadline
				}
			}
		}
	}
	if now.After(p.deadline) {
		p.deadline = time.Time{}
		return assist.Action{}, "", false
	}
	if p.mode == IntroBurn {
		// 入场爆发：技能可用就一直按，进冷却后用声骸/普攻收尾并结束入场。
		if s.ready(frame, StepSkill) && s.spec.Keys.Skill != "" {
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Skill}, "入场-爆发", true
		}
		if s.spec.Keys.Echo != "" && s.ready(frame, StepEcho) {
			p.deadline = time.Time{}
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Echo}, "入场-收尾", true
		}
		p.deadline = time.Time{}
		return assist.Action{}, "", false
	}
	if s.ready(frame, StepSkill) {
		p.deadline = time.Time{}
		return assist.Action{}, "", false
	}
	action := assist.Action{Kind: assist.ActionClick, Point: s.spec.AttackPoint}
	if s.spec.Keys.Attack != "" {
		action = assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Attack}
	}
	return action, "入场-落地", true
}

// stepQueue 消费入场技队列，每帧最多消费一步。
func (p *introPhase) stepQueue(s *Session, frame assist.Frame) (assist.Action, string, bool) {
	if len(p.steps) == 0 {
		return assist.Action{}, "", false
	}
	step := p.steps[0]
	p.steps = p.steps[1:]
	if action, state, ok := s.stepAction(frame, step); ok {
		return action, "入场-" + state, true
	}
	return assist.Action{}, "", false
}

// switchPhase 是"切人后确认真的切过去了"的窗口。
//
// 后台输入可能被吞，上游 switch_next_char 里也会复查 in_team：
// 宽限期内没换人就补一次普攻。
type switchPhase struct {
	at   time.Time
	from int
	done bool
}

func (p *switchPhase) active() bool { return !p.at.IsZero() }

func (p *switchPhase) reset() { *p = switchPhase{} }

// enter 在切人时开启确认窗口。from=-1 表示当前槽位未知，直接判定失败。
func (p *switchPhase) enter(now time.Time, from int) {
	p.at = now
	p.from = from
	p.done = false
}

// step 推进一帧。ok=true 表示本帧要补一次普攻（切人没生效）。
func (p *switchPhase) step(s *Session, frame assist.Frame, now time.Time) (assist.Action, string, bool) {
	if p.at.IsZero() || s.spec.Team == nil {
		return assist.Action{}, "", false
	}
	since := now.Sub(p.at)
	if since > switchVerifyTimeout || p.from < 0 {
		p.at = time.Time{}
		return assist.Action{}, "", false
	}
	if cur, ok, err := s.spec.Team.Current(runtimeAdapter{rt: s.rt}, frame); err == nil && ok && cur != p.from {
		p.at = time.Time{} // 切人成功
		return assist.Action{}, "", false
	}
	if since > switchVerifyGrace && !p.done {
		p.done = true
		action := assist.Action{Kind: assist.ActionClick, Point: s.spec.AttackPoint}
		if s.spec.Keys.Attack != "" {
			action = assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Attack}
		}
		return action, "切人未生效", true
	}
	return assist.Action{}, "", false
}
