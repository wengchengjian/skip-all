package combat

import (
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// Role 是角色定位，取值和上游对齐（鸣潮 CharType / 绝区零 AgentTypeEnum）。
type Role string

const (
	RoleMainDPS Role = "main_dps"
	RoleSubDPS  Role = "sub_dps"
	RoleHealer  Role = "healer"
	RoleSupport Role = "support"
)

// SlotLocator 是可选接口：能列出每个槽位上是哪个角色时实现它，
// 切人时就能按定位挑目标，而不是简单地切下一个。
type SlotLocator interface {
	Slots(rt Runtime, frame assist.Frame) ([]string, error)
}

// SwitchPolicy 是切人打分的规则，对应上游 get_switch_priority / _choose_switch_target。
type SwitchPolicy struct {
	// MinInterval 是两次切人的最小间隔。
	MinInterval time.Duration
	// HealerLockout 是治疗位被协奏切进来之后的冷却：这段时间内不再优先切它，
	// 对应上游 BaseChar.HEALER_FULL_CON_SWITCH_LOCKOUT = 16 秒。
	HealerLockout time.Duration
}

// DefaultHealerLockout 对齐上游 ok-ww 的 16 秒。
const DefaultHealerLockout = 16 * time.Second
