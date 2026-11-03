package combat

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// stepRule 是决策链上的一条规则。按 combatRules 的顺序匹配，第一条接管本帧。
//
// 原先 Step 里是一串 if/else 提前 return，优先级只能靠读代码顺序理解，
// 也没法在测试里逐条枚举。改成表之后：
//   - 优先级是显式的（切片顺序）
//   - 每条形如 "条件 -> 动作"，可以单独构造输入断言是否命中
//   - 新增规则只往表里插一行，不动流程
type stepRule struct {
	name string
	run  func(s *Session, frame assist.Frame, now time.Time) (assist.Result, []assist.Action, bool, error)
}

// combatRules 是战斗中的决策优先级，从上到下。
//
// 顺序有实际含义：
//   - 切治疗排在扩展前：开打前先保证有人能奶
//   - 扩展（闪避/支援）排在协奏前：要抢在连招和切人前面
//   - 中断排在锁定前：战斗已经结束时不该再按中键
var combatRules = []stepRule{
	{"战斗前切治疗", (*Session).rulePreHeal},
	{"扩展能力", (*Session).ruleExtensions},
	{"协奏满切人", (*Session).ruleConcertoSwitch},
	{"战斗中断", (*Session).ruleAbort},
	{"锁定目标", (*Session).ruleLockTarget},
	{"连招", (*Session).ruleRotation},
}

// healCooldown 是两次"切治疗"之间的最小间隔。
//
// 脱战判定会被短暂打断（敌人转到身后、血条被特效挡住）触发，
// 于是"战斗结束切治疗"之后两三秒又判成"战斗前切治疗"，
// 表现就是玩家手动切到别人、程序立刻又切回治疗位。加冷却压掉这种抖动。
const healCooldown = 20 * time.Second

// rulePreHeal 在开打前先切到治疗位，保证有人能奶。
func (s *Session) rulePreHeal(frame assist.Frame, now time.Time) (assist.Result, []assist.Action, bool, error) {
	if s.cfg.noHealer || s.healed {
		return assist.Result{}, nil, false, nil
	}
	if !s.healAt.IsZero() && now.Sub(s.healAt) < healCooldown {
		s.healed = true
		return assist.Result{}, nil, false, nil
	}
	key := s.healerKey(frame)
	if key == "" {
		return assist.Result{}, nil, false, nil
	}
	s.healed = true
	s.healAt = now
	return assist.Result{State: "战斗前切治疗", Wait: s.spec.Interval},
		[]assist.Action{{Kind: assist.ActionKeyTap, Key: key}}, true, nil
}

// ruleExtensions 让游戏专属能力优先：闪光闪避、快速支援这类要抢在连招前面。
// 调用扩展前先把会话状态推给它（协奏是否满、是否还在入场、当前角色）。
func (s *Session) ruleExtensions(frame assist.Frame, now time.Time) (assist.Result, []assist.Action, bool, error) {
	s.pushCombatState(frame, now)
	action, state, ok, err := s.runExtensions(frame)
	if err != nil {
		return assist.Result{}, nil, false, err
	}
	if !ok {
		return assist.Result{}, nil, false, nil
	}
	return assist.Result{State: state, Wait: s.spec.Interval}, []assist.Action{action}, true, nil
}

// ruleConcertoSwitch 在协奏值满时切人（上游 switch_next_char 的核心判据）。
func (s *Session) ruleConcertoSwitch(frame assist.Frame, now time.Time) (assist.Result, []assist.Action, bool, error) {
	action, ok := s.concertoSwitch(frame, now)
	if !ok {
		return assist.Result{}, nil, false, nil
	}
	return assist.Result{State: "协奏满切人", Wait: s.spec.Interval}, []assist.Action{action}, true, nil
}

// ruleAbort 处理战斗被打断（复活/失败界面）：先别乱按，清掉连招游标与入场队列。
func (s *Session) ruleAbort(frame assist.Frame, _ time.Time) (assist.Result, []assist.Action, bool, error) {
	aborted, err := s.checkAbort(frame)
	if err != nil {
		return assist.Result{}, nil, false, err
	}
	if !aborted {
		return assist.Result{}, nil, false, nil
	}
	s.cursor = 0
	s.intro.clearSteps()
	return assist.Result{State: "战斗中断", Wait: s.spec.IdleWait}, nil, true, nil
}

// ruleLockTarget 在没锁定目标时反复中键（上游 target_enemy()），按 LockInterval 节流。
func (s *Session) ruleLockTarget(frame assist.Frame, now time.Time) (assist.Result, []assist.Action, bool, error) {
	if s.spec.LockInterval <= 0 {
		return assist.Result{}, nil, false, nil
	}
	if !s.lockAt.IsZero() && now.Sub(s.lockAt) < s.spec.LockInterval {
		return assist.Result{}, nil, false, nil
	}
	locked, err := s.hasTarget(frame)
	if err != nil {
		return assist.Result{}, nil, false, err
	}
	if locked {
		return assist.Result{}, nil, false, nil
	}
	s.lockAt = now
	return assist.Result{State: "锁定目标", Wait: s.spec.Interval},
		[]assist.Action{{Kind: assist.ActionMiddleClick, Point: s.spec.AttackPoint}}, true, nil
}

// ruleRotation 走连招：切人确认、入场、轮转。
func (s *Session) ruleRotation(frame assist.Frame, _ time.Time) (assist.Result, []assist.Action, bool, error) {
	action, state, ok := s.nextAction(frame)
	if !ok {
		return assist.Result{}, nil, false, nil
	}
	return assist.Result{State: state, Wait: s.spec.Interval}, []assist.Action{action}, true, nil
}

// gateCombat 处理"在不在战斗"这一层。
//
// 脱战要清空所有战斗内状态，并按需切回治疗位。handled=true 表示本帧已处理完。
// 证据会短暂丢失（转身、切人、特效遮挡），Hold 时间内仍算在战斗中。
func (s *Session) gateCombat(frame assist.Frame, now time.Time) (assist.Result, []assist.Action, bool, error) {
	inCombat, err := s.inCombat(frame)
	if err != nil {
		return assist.Result{}, nil, false, err
	}
	if inCombat {
		s.lastCombat = now
		return assist.Result{}, nil, false, nil
	}
	if !s.lastCombat.IsZero() && now.Sub(s.lastCombat) <= s.spec.Hold {
		return assist.Result{}, nil, false, nil
	}
	s.resetOutOfCombat()
	// 上游在战斗结束后会切回治疗位；这里也是同一件事。
	if s.healed && !s.cfg.noHealer {
		if key := s.healerKey(frame); key != "" {
			s.healed = false
			return assist.Result{State: "战斗结束切治疗", Wait: s.spec.Interval},
				[]assist.Action{{Kind: assist.ActionKeyTap, Key: key}}, true, nil
		}
	}
	s.healed = false
	return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, true, nil
}

// resetOutOfCombat 清空所有跨战斗状态，下一场重新来。
func (s *Session) resetOutOfCombat() {
	s.cursor = 0
	s.lockAt = time.Time{}
	s.cdCache = make(map[StepKind]cdEntry)
	s.agentAt = time.Time{}
	s.agentID = ""
	s.outroAgent = ""
	s.lastLiberation = make(map[string]time.Time)
	s.lastHeavy = make(map[string]time.Time)
	s.lastSkill = make(map[string]time.Time)
	s.lastPerform = make(map[string]time.Time)
	s.lastEcho = make(map[string]time.Time)
	s.flyingChecked = false
	s.hasLavitator = false
	s.intro.clearSteps()
}
