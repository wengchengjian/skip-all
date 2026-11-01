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
	// StepKey 是直接按某个键（规则里出现"切换角色-下一个/上一个"这类动作时用）。
	StepKey StepKind = "key"
	// StepHeavy 是重击/蓄力：在攻击点按住左键一小段时间再抬起。
	StepHeavy StepKind = "heavy"
	// StepSwitch 是切换角色，Index 指向 Keys.Switch 的下标。
	StepSwitch StepKind = "switch"
)

// Step 是连招序列里的一步。
type Step struct {
	Kind  StepKind
	Index int
	Key   string
	// Hold 是重击按住的时长，0 用默认 600ms（上游 heavy_attack 的 duration）。
	Hold time.Duration
}

// ReadyCheck 是"这个动作现在能不能用"的判据，对应上游的 available()：
// 技能按钮上出现纯白高亮就说明可用（ok-ww 用 text_white_color 的像素占比）。
type ReadyCheck struct {
	Area       geom.Rect // 1080p 逻辑像素
	ColorRange [2][3]uint8
	MinRatio   float64
}

// UltReadyCheck 是"后台角色大招是否就绪"的判据，照上游 update_lib_portrait_icon()：
// 在槽位区域里匹配到就绪标记模板，说明该角色可以放大招，切人时优先。
type UltReadyCheck struct {
	Slots     []geom.Rect
	SubDir    string
	IDs       []string
	Threshold float64
	Bonus     float64
}

// IntroSpec 描述"入场动画期间该做什么"。
// 上游 BaseChar.wait_down()/wait_intro()：有入场技时持续点普攻，直到角色落地
// （落地用"技能可用"判断），超时 1.2 秒。
type IntroSpec struct {
	// MaxDuration 是入场等待的上限，超时就直接进入连招。
	MaxDuration time.Duration
	// Mode 决定入场期间做什么，默认 IntroWait（点普攻等落地）。
	Mode IntroMode
}

// IntroMode 是入场期间的行为。
type IntroMode string

const (
	// IntroWait 是基类行为：点普攻，等技能可用（落地）。对应 wait_down()/wait_intro()。
	IntroWait IntroMode = ""
	// IntroBurn 是"入场就打一轮爆发"的行为：技能可用就一直按技能，
	// 技能进冷却后按声骸（没有再普攻）收尾。对应今汐重写的 handle_intro()。
	IntroBurn IntroMode = "burn"
)

// ForteCheck 是"共鸣回路是否已满"的判据：上游 BaseChar.is_forte_full() 在
// (2251,1993)-(2311,2016)（4K）里数接近纯白的像素占比，超过 0.08 就算满。
type ForteCheck struct {
	Area       geom.Rect
	ColorRange [2][3]uint8
	MinRatio   float64
}

// ForteNeed 表示某个角色的技能要等回路"满"还是"未满"。
// 上游不同角色写法相反（忌炎要未满才放，布兰特要满才放），所以按角色分别配。
type ForteNeed string

const (
	// ForteNeedFull 表示技能要等回路满。
	ForteNeedFull ForteNeed = "full"
	// ForteNeedNotFull 表示技能要等回路未满。
	ForteNeedNotFull ForteNeed = "not_full"
)

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
	Screen       string                   // 画面配置名，只有用模板证据时才需要
	DetectAreas  []string                 // 战斗证据区域（模板），任意一个命中即认为在战斗
	HealthBar    *HealthBar               // 敌人血条证据，留空表示不用它
	BossBar      *BossBar                 // Boss 血条证据，留空表示不用它
	Concerto     *Concerto                // 协奏值能量环，满了优先切人
	HealerKey    string                   // 治疗位按键，战斗开始与结束时各按一次
	Extensions   []Extension              // 游戏专属战斗能力，先于通用连招执行
	Team         Team                     // 当前角色判读，留空表示不识别
	Target       *Target                  // 是否已锁定目标的判据，留空表示不做目标识别
	LockInterval time.Duration            // 没锁住目标时按中键的间隔，0 表示不锁定
	AttackPoint  geom.Point               // 普攻的点击位置
	Keys         Keys                     // 按键绑定
	Plan         []Step                   // 默认连招序列，循环执行
	PlansByAgent map[string][]Step        // 角色专属连招；识别到当前角色时优先用
	Ready        map[StepKind]ReadyCheck  // 技能可用性判据，没配的动作照旧放
	Cooldowns    map[StepKind]geom.Rect   // 技能冷却数字的区域，OCR 到正数就不放
	IntroSteps   []Step                   // 切人后插入的一小段（固定序列，留空则用 Intro）
	Intro        *IntroSpec               // 入场等待：切人后点普攻直到"落地"，对应上游 wait_down/wait_intro
	IntroByAgent map[string]IntroSpec     // 角色专属的入场行为（如今汐的入场爆发）
	AbortArea    string                   // 中断检测的 OCR 区域名（画面配置里），留空不检测
	AbortTexts   []string                 // 命中这些文字说明战斗被打断（复活/失败界面），暂停出招
	UltReady     *UltReadyCheck           // 后台角色大招就绪判据，命中给切人加分
	ForteFull    *ForteCheck              // 共鸣回路是否已满的判据（鸣潮）
	ForteGate    map[string]ForteNeed     // 哪些角色的技能要等回路状态，对应上游 is_forte_full()
	Roles        map[string]Role          // 角色定位，切人时用来打分
	BuffTimes    map[string]time.Duration // 角色增益持续时间，切人时用来判断"增益是否已掉"
	Switch       *SwitchPolicy            // 切人规则，留空用默认间隔
	Interval     time.Duration            // 每步之间的间隔
	Hold         time.Duration            // 命中证据后保持战斗态的时间
	IdleWait     time.Duration            // 不在战斗时的等待
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
