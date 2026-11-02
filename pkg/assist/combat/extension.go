package combat

import (
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// Extension 是游戏专属的战斗能力（例如绝区零的闪光闪避、快速支援）。
// 通用流程每帧先问扩展；扩展处理不了（ok=false）再走通用连招，
// 这样游戏特有的机制不会污染共享状态机。
type Extension interface {
	// Name 是扩展名，同时用作设置项开关后缀（ext_<name>）。
	Name() string
	// Step 返回一步动作；ok=false 表示这一帧不处理。
	Step(rt Runtime, frame assist.Frame) (assist.Action, string, bool, error)
}

// CombatState 是会话算出来、但扩展自己看不到（也没有像素可看）的状态。
// 实现了 StateAware 的扩展每帧会先拿到它。
type CombatState struct {
	// Agent 是当前在场角色的模板 id（识别失败为空）。
	Agent string
	// ConcertoFull 表示协奏环是否已满（满环即拥有入场技）。
	ConcertoFull bool
	// InIntro 表示还在入场等待期间。
	InIntro bool
	// LastOutro 是上一次切走的角色模板 id（上游 check_outro() 的记账：
	// 队里 last_switch_time 最大的那个队友，即刚放完延奏的人）。
	// 只有 InIntro 期间才有意义；没切过人时为空。
	LastOutro string
	// Flying 表示当前是否在空中（上游 BaseChar.flying()）。
	Flying bool
	// InLiberation 表示当前角色的共鸣解放动画是否还在窗口内（上游 still_in_liberation()）。
	InLiberation bool
	// NeedFastPerform 表示队友里有人对当前角色返回 MUST（上游 need_fast_perform()），
	// 当前角色应当尽快收尾让位。
	NeedFastPerform bool
}

// StateAware 是扩展可选实现的状态回调。
type StateAware interface {
	SetCombatState(CombatState)
}

// Runtime 是扩展可用的运行时能力。
type Runtime interface {
	MatchTemplate(frame assist.Frame, area screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error)
}

// runtimeAdapter 把 assist.Runtime 收敛成扩展需要的最小接口。
type runtimeAdapter struct {
	rt assist.Runtime
}

func (r runtimeAdapter) MatchTemplate(frame assist.Frame, area screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error) {
	return r.rt.MatchTemplate(frame, area, ref, threshold)
}
