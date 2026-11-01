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

// HealthBar 是敌人血条的判据，对应上游 has_health_bar()：
// 在整帧里找颜色、尺寸、填充率都对得上的实心矩形。
type HealthBar struct {
	Color     [2][3]uint8 // 血条颜色范围（含端点）
	MinWidth  int         // 1080p 逻辑像素
	MinHeight int
	MaxHeight int
}

// BossBar 是 Boss 血条的判据，对应上游 is_boss()：
// 在屏幕上方居中的固定区域里找一条颜色对得上的实心矩形，且只能有一条。
type BossBar struct {
	Color     [2][3]uint8
	Rect      geom.Rect // 1080p 逻辑像素
	MinWidth  int
	MinHeight int
}

// Target 是"有没有锁定目标"的判据，对应上游 has_target()：
// 用 has_target / no_target 两张模板在同一区域里比分数，谁高就算谁。
type Target struct {
	Rect      geom.Rect // 1080p 逻辑像素
	SubDir    string    // 模板目录
	Have      string    // 已锁定时的模板 id
	Missing   string    // 未锁定时的模板 id
	Threshold float64   // 分数下限，上游用 0.6
}

// Concerto 是协奏值能量环的判据，对应上游 get_current_con()：
// 在当前角色头像周围找一圈有颜色的能量环，覆盖率高说明协奏值满、可以切人。
type Concerto struct {
	Rect      geom.Rect // 1080p 逻辑像素
	Colors    [][2][3]uint8
	Inner     float64 // 环内半径，按区域高度的比例（上游 0.35119）
	Outer     float64 // 环外半径（上游 0.42261）
	FullRatio float64 // 覆盖率到此认为协奏值满
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
	Screen       string        // 画面配置名，只有用模板证据时才需要
	DetectAreas  []string      // 战斗证据区域（模板），任意一个命中即认为在战斗
	HealthBar    *HealthBar    // 敌人血条证据，留空表示不用它
	BossBar      *BossBar      // Boss 血条证据，留空表示不用它
	Concerto     *Concerto     // 协奏值能量环，满了优先切人
	HealerKey    string        // 治疗位按键，战斗开始与结束时各按一次
	Extensions   []Extension   // 游戏专属战斗能力，先于通用连招执行
	Team         Team          // 当前角色判读，留空表示不识别
	Target       *Target       // 是否已锁定目标的判据，留空表示不做目标识别
	LockInterval time.Duration // 没锁住目标时按中键的间隔，0 表示不锁定
	AttackPoint  geom.Point    // 普攻的点击位置
	Keys         Keys          // 按键绑定
	Plan         []Step        // 连招序列，循环执行
	Interval     time.Duration // 每步之间的间隔
	Hold         time.Duration // 命中证据后保持战斗态的时间
	IdleWait     time.Duration // 不在战斗时的等待
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
	if len(s.DetectAreas) > 0 && s.Screen == "" {
		return errors.New("自动战斗用模板证据时必须给画面配置名")
	}
	if len(s.DetectAreas) == 0 && s.HealthBar == nil && s.BossBar == nil {
		return errors.New("自动战斗缺少战斗证据")
	}
	if s.BossBar != nil && (s.BossBar.MinWidth <= 0 || s.BossBar.MinHeight <= 0) {
		return errors.New("自动战斗的 Boss 血条证据缺少尺寸阈值")
	}
	if s.HealthBar != nil && (s.HealthBar.MinWidth <= 0 || s.HealthBar.MinHeight <= 0) {
		return errors.New("自动战斗的血条证据缺少尺寸阈值")
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
