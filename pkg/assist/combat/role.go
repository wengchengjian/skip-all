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
}
