// Package combat 提供通用的自动战斗流程，游戏侧只提供按键与画面证据。
package combat

import (
	"errors"
	"fmt"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

// StepKind 是连招里一个动作的类型。
type StepKind string

const (
	// StepAttack 是普通攻击，向战斗区域点一下。
	StepAttack StepKind = "attack"
	// StepSkill 是核心技能（鸣潮的共鸣技能、绝区零的强化特殊技）。
	StepSkill StepKind = "skill"
	// StepEcho 是装备技能（鸣潮的声骸、绝区零的邦布）。
	StepEcho StepKind = "echo"
	// StepLiberation 是爆发技能（共鸣解放 / 终结技）。
	StepLiberation StepKind = "liberation"
	// StepDodge 是闪避。
	StepDodge StepKind = "dodge"
	// StepSwitch 是切换角色，Index 指向 Keys.Switch 的下标。
	StepSwitch StepKind = "switch"
)

// Step 是连招序列里的一步。
type Step struct {
	Kind  StepKind
	Index int
}

// Keys 是技能按键绑定，留空的按键表示该游戏没有这个动作。
type Keys struct {
	Attack     string // 普通攻击的按键；留空表示用鼠标点击
	Skill      string
	Echo       string
	Liberation string
	Dodge      string
	Switch     []string
}

// Spec 是自动战斗的通用配置。
// 共享流程（检测战斗 -> 按连招序列出招 -> 脱离战斗后复位）只有一份，
// 游戏之间的差异只体现在这里的数据上。
type Spec struct {
	Screen         string        // 画面配置名，例如 鸣潮战斗
	DetectAreas    []string      // 战斗证据区域（模板），任意一个命中即认为在战斗
	HealthBarArea  string        // 敌人血条区域名；和 HealthBarRange 一起按颜色比例判断
	HealthBarRange [2][3]uint8   // 血条颜色范围（含端点）
	HealthBarRatio float64       // 区域内落在颜色范围里的像素比例达到它就算在战斗
	AttackPoint    geom.Point    // 普攻的点击位置
	Keys           Keys          // 按键绑定
	Plan           []Step        // 连招序列，循环执行
	Interval       time.Duration // 每步之间的间隔
	Hold           time.Duration // 命中证据后保持战斗态的时间
	IdleWait       time.Duration // 不在战斗时的等待
}

// DefaultPlan 是一套通用的连招节奏：普攻起手、夹技能、中间接爆发与闪避。
var DefaultPlan = []Step{
	{Kind: StepAttack},
	{Kind: StepSkill},
	{Kind: StepAttack},
	{Kind: StepAttack},
	{Kind: StepEcho},
	{Kind: StepAttack},
	{Kind: StepLiberation},
	{Kind: StepAttack},
	{Kind: StepDodge},
}

func (s Spec) withDefaults() Spec {
	if s.Interval <= 0 {
		s.Interval = 150 * time.Millisecond
	}
	if s.Hold <= 0 {
		s.Hold = 3 * time.Second
	}
	if s.IdleWait <= 0 {
		s.IdleWait = time.Second
	}
	if len(s.Plan) == 0 {
		s.Plan = DefaultPlan
	}
	if s.AttackPoint == (geom.Point{}) {
		s.AttackPoint = geom.Point{X: 960, Y: 540}
	}
	return s
}

func (s Spec) validate() error {
	if s.Screen == "" {
		return errors.New("自动战斗缺少画面配置名")
	}
	if len(s.DetectAreas) == 0 && s.HealthBarArea == "" {
		return errors.New("自动战斗缺少战斗证据区域")
	}
	if s.HealthBarArea != "" && s.HealthBarRatio <= 0 {
		return errors.New("自动战斗的血条证据缺少比例阈值")
	}
	if s.Keys.Skill == "" && s.Keys.Liberation == "" && s.Keys.Attack == "" {
		return errors.New("自动战斗缺少按键绑定")
	}
	for _, step := range s.Plan {
		if step.Kind == StepSwitch && (step.Index < 0 || step.Index >= len(s.Keys.Switch)) {
			return fmt.Errorf("连招里的切换角色下标 %d 超出按键绑定范围", step.Index)
		}
	}
	return nil
}
