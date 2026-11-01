package combat

import (
	"context"
	"errors"
	"image"
	"image/color"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

type fakeRuntime struct {
	screens *screen.Set
	hit     bool
	scores  map[string]float64 // 按模板 id 给分数，决定"是否锁定目标"这类判据
	ocrText string             // 技能按钮上 OCR 到的文字（冷却数字）
}

func (f *fakeRuntime) Screen(name string) (*screen.Screen, error) {
	sc, ok := f.screens.Get(name)
	if !ok {
		return nil, errors.New("未找到画面 " + name)
	}
	return sc, nil
}

func (f *fakeRuntime) OCR(_ assist.Frame, area screen.Area, _ vision.OCROptions) ([]vision.TextBlock, error) {
	if f.ocrText == "" {
		return nil, nil
	}
	return []vision.TextBlock{{Text: f.ocrText, Rect: area.Rect, Confidence: 0.9}}, nil
}

func (f *fakeRuntime) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	if f.scores != nil {
		if score, ok := f.scores[ref.ID]; ok {
			if score <= 0 {
				return nil, nil
			}
			return []vision.Match{{Rect: area.Rect, Confidence: score}}, nil
		}
	}
	if !f.hit {
		return nil, nil
	}
	return []vision.Match{{Rect: geom.Rect{X1: 1400, Y1: 900, X2: 1465, Y2: 966}, Confidence: 0.92}}, nil
}

func (f *fakeRuntime) ColorStats(assist.Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}

func (f *fakeRuntime) Log() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testRuntime(hit bool) *fakeRuntime {
	set := screen.NewSet()
	set.Add(&screen.Screen{
		ID:   "combat_test",
		Name: "测试战斗",
		AreaList: []*screen.Area{
			{Name: "锁定", RectValues: []int{1250, 860, 1640, 1040}, TemplateSubDir: "combat", TemplateID: "target"},
			{Name: "血条", RectValues: []int{620, 60, 1300, 170}},
		},
	})
	return &fakeRuntime{screens: set, hit: hit}
}

func testSpec() Spec {
	return Spec{
		Screen:      "测试战斗",
		DetectAreas: []string{"锁定"},
		HealthBar: &HealthBar{
			Color:     [2][3]uint8{{174, 55, 55}, {225, 85, 76}},
			MinWidth:  50,
			MinHeight: 5,
			MaxHeight: 14,
		},
		BossBar: &BossBar{
			Color:     [2][3]uint8{{245, 30, 4}, {255, 185, 75}},
			Rect:      geom.Rect{X1: 634, Y1: 29, X2: 1266, Y2: 100},
			MinWidth:  50,
			MinHeight: 6,
		},
		Keys: Keys{
			Skill: "e", Echo: "q", Liberation: "r", Dodge: "lshift",
			Switch: []string{"1", "2"},
		},
		Plan: []Step{
			{Kind: StepAttack},
			{Kind: StepSkill},
			{Kind: StepEcho},
			{Kind: StepLiberation},
			{Kind: StepDodge},
		},
		Interval: 50 * time.Millisecond,
		Hold:     2 * time.Second,
		IdleWait: 100 * time.Millisecond,
	}
}

func frameAt(at time.Time) assist.Frame {
	return assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 8, 8)), At: at}
}

func step(t *testing.T, session *Session, frame assist.Frame) (assist.Result, []assist.Action) {
	t.Helper()
	result, actions, err := session.Step(context.Background(), frame)
	if err != nil {
		t.Fatalf("Step 出错: %v", err)
	}
	return result, actions
}

func TestIdleOutsideCombat(t *testing.T) {
	session := newSession(testRuntime(false), testSpec().withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "等待战斗" {
		t.Fatalf("state = %q", result.State)
	}
	if len(actions) != 0 {
		t.Fatalf("不在战斗时不应出招: %+v", actions)
	}
}

func TestAttackWhenInCombat(t *testing.T) {
	session := newSession(testRuntime(true), testSpec().withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "连招-普攻" {
		t.Fatalf("state = %q", result.State)
	}
	if len(actions) != 1 || actions[0].Kind != assist.ActionClick {
		t.Fatalf("普攻应该是点击: %+v", actions)
	}
	if actions[0].Point != (geom.Point{X: 960, Y: 540}) {
		t.Fatalf("普攻点击位置 = %+v", actions[0].Point)
	}
}

func TestPlanRotatesThroughSkills(t *testing.T) {
	session := newSession(testRuntime(true), testSpec().withDefaults(), sessionSettings{})
	base := time.Now()
	want := []struct {
		state string
		kind  assist.ActionKind
		key   string
	}{
		{"连招-普攻", assist.ActionClick, ""},
		{"连招-技能", assist.ActionKeyTap, "e"},
		{"连招-装备技能", assist.ActionKeyTap, "q"},
		{"连招-爆发", assist.ActionKeyTap, "r"},
		{"连招-闪避", assist.ActionKeyTap, "lshift"},
		{"连招-普攻", assist.ActionClick, ""},
	}
	for i, w := range want {
		result, actions := step(t, session, frameAt(base.Add(time.Duration(i)*100*time.Millisecond)))
		if result.State != w.state {
			t.Fatalf("第 %d 步 state = %q, want %q", i, result.State, w.state)
		}
		if len(actions) != 1 || actions[0].Kind != w.kind || actions[0].Key != w.key {
			t.Fatalf("第 %d 步动作 = %+v", i, actions)
		}
	}
}

func TestDisabledStepsAreSkipped(t *testing.T) {
	session := newSession(testRuntime(true), testSpec().withDefaults(), sessionSettings{noLiberation: true, noEcho: true})
	base := time.Now()
	steps := []string{}
	for i := 0; i < 2; i++ {
		result, _ := step(t, session, frameAt(base.Add(time.Duration(i)*100*time.Millisecond)))
		steps = append(steps, result.State)
	}
	if steps[0] != "连招-普攻" || steps[1] != "连招-技能" {
		t.Fatalf("关掉装备技能与爆发后仍然出招, states = %v", steps)
	}
}

func TestHoldKeepsCombatBetweenEvidence(t *testing.T) {
	rt := testRuntime(true)
	spec := testSpec().withDefaults()
	session := newSession(rt, spec, sessionSettings{})
	base := time.Now()
	if result, _ := step(t, session, frameAt(base)); result.State != "连招-普攻" {
		t.Fatalf("首帧 = %q", result.State)
	}
	// 证据消失，但还在保持时间内：继续出招。
	rt.hit = false
	if result, _ := step(t, session, frameAt(base.Add(time.Second))); result.State == "等待战斗" {
		t.Fatal("保持时间内不应退出战斗态")
	}
	// 超过保持时间：回到等待。
	if result, _ := step(t, session, frameAt(base.Add(10*time.Second))); result.State != "等待战斗" {
		t.Fatalf("超时后 state = %q", result.State)
	}
}

func TestValidateRejectsBadSwitchIndex(t *testing.T) {
	spec := testSpec()
	spec.Plan = append(spec.Plan, Step{Kind: StepSwitch, Index: 5})
	if err := spec.withDefaults().validate(); err == nil {
		t.Fatal("切换角色下标越界时应当报错")
	}
}

func TestFeatureDefaultSettings(t *testing.T) {
	feature := NewFeature(testSpec())
	if feature.ID() != "auto_combat" || feature.Name() != "自动战斗" {
		t.Fatalf("feature = %s/%s", feature.ID(), feature.Name())
	}
	if err := feature.Validate(feature.DefaultSettings()); err != nil {
		t.Fatalf("默认设置不合法: %v", err)
	}
	if err := feature.Validate(assist.Settings{SettingInterval: -1.0}); err == nil {
		t.Fatal("负间隔应当报错")
	}
}

func TestFeatureRequestsHigherFrameRate(t *testing.T) {
	feature := NewFeature(testSpec())
	needs, ok := assist.Feature(feature).(assist.FrameRateFeature)
	if !ok {
		t.Fatal("自动战斗应当实现 FrameRateFeature")
	}
	if needs.MinFPS() < 10 {
		t.Fatalf("MinFPS = %d", needs.MinFPS())
	}
}

func TestHealthBarEvidence(t *testing.T) {
	session := newSession(testRuntime(false), testSpec().withDefaults(), sessionSettings{})
	// 画一条上游规格的敌人血条（120x8，填充率 1.0）：应当判成战斗。
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 100; y < 108; y++ {
		for x := 700; x < 820; x++ {
			img.SetRGBA(x, y, color.RGBA{200, 70, 60, 255})
		}
	}
	result, actions := step(t, session, assist.Frame{Image: img, At: time.Now()})
	if result.State != "连招-普攻" || len(actions) != 1 {
		t.Fatalf("有血条时应当出招: state = %q actions = %+v", result.State, actions)
	}
	// 换成纯黑画面：不应出招。
	black := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	result, actions = step(t, session, assist.Frame{Image: black, At: time.Now().Add(10 * time.Second)})
	if result.State != "等待战斗" || len(actions) != 0 {
		t.Fatalf("没有血条时不应出招: state = %q actions = %+v", result.State, actions)
	}
}

func TestBossBarEvidence(t *testing.T) {
	session := newSession(testRuntime(false), testSpec().withDefaults(), sessionSettings{})
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// 屏幕上方居中的橙色 Boss 血条：900x30
	for y := 40; y < 70; y++ {
		for x := 700; x < 1600; x++ {
			img.SetRGBA(x, y, color.RGBA{250, 100, 30, 255})
		}
	}
	result, actions := step(t, session, assist.Frame{Image: img, At: time.Now()})
	if result.State != "连招-普攻" || len(actions) != 1 {
		t.Fatalf("有 Boss 血条时应当出招: state = %q actions = %+v", result.State, actions)
	}
}

func TestBossBarRequiresExactlyOne(t *testing.T) {
	// 上游要求区域内恰好一条，两条时不算，避免把红色 UI 误判成 Boss 血条。
	session := newSession(testRuntime(false), testSpec().withDefaults(), sessionSettings{})
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for _, row := range [][2]int{{40, 60}, {70, 90}} {
		for y := row[0]; y < row[1]; y++ {
			for x := 700; x < 1100; x++ {
				img.SetRGBA(x, y, color.RGBA{250, 100, 30, 255})
			}
		}
	}
	result, actions := step(t, session, assist.Frame{Image: img, At: time.Now()})
	if result.State != "等待战斗" || len(actions) != 0 {
		t.Fatalf("两条血条不该判成 Boss 战: state = %q actions = %+v", result.State, actions)
	}
}

func testTargetSpec() Spec {
	spec := testSpec()
	spec.Target = &Target{
		Rect:      geom.Rect{X1: 1400, Y1: 930, X2: 1540, Y2: 1045},
		SubDir:    "combat",
		Have:      "has_target",
		Missing:   "no_target",
		Threshold: 0.6,
	}
	spec.LockInterval = time.Second
	return spec
}

func TestTargetLockedSkipsMiddleClick(t *testing.T) {
	rt := testRuntime(true)
	rt.scores = map[string]float64{"target": 0.9, "has_target": 0.9, "no_target": 0.2}
	session := newSession(rt, testTargetSpec().withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "连招-普攻" {
		t.Fatalf("已锁定目标时应当直接出招, state = %q actions = %+v", result.State, actions)
	}
}

func TestTargetMissingLocksWithMiddleClick(t *testing.T) {
	rt := testRuntime(true)
	rt.scores = map[string]float64{"target": 0.9, "has_target": 0.2, "no_target": 0.9}
	session := newSession(rt, testTargetSpec().withDefaults(), sessionSettings{})
	base := time.Now()
	result, actions := step(t, session, frameAt(base))
	if result.State != "锁定目标" {
		t.Fatalf("没锁定目标时应当按中键, state = %q", result.State)
	}
	if len(actions) != 1 || actions[0].Kind != assist.ActionMiddleClick {
		t.Fatalf("锁定动作 = %+v", actions)
	}
	// 间隔内不重复按中键。
	result, _ = step(t, session, frameAt(base.Add(300*time.Millisecond)))
	if result.State == "锁定目标" {
		t.Fatal("间隔内不该重复按中键")
	}
	// 超过间隔仍然没锁上，再按一次。
	result, _ = step(t, session, frameAt(base.Add(2*time.Second)))
	if result.State != "锁定目标" {
		t.Fatalf("超过间隔应当再锁一次, state = %q", result.State)
	}
}

func TestHealerSwitchAtStartAndEnd(t *testing.T) {
	rt := testRuntime(true)
	spec := testSpec()
	spec.HealerKey = "3"
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	base := time.Now()
	result, actions := step(t, session, frameAt(base))
	if result.State != "战斗前切治疗" || actions[0].Key != "3" {
		t.Fatalf("开局应切治疗: state = %q actions = %+v", result.State, actions)
	}
	// 之后正常出招，不会反复切。
	for i := 1; i < 3; i++ {
		result, _ = step(t, session, frameAt(base.Add(time.Duration(i)*200*time.Millisecond)))
		if result.State == "战斗前切治疗" {
			t.Fatal("切治疗只应在开局做一次")
		}
	}
	// 脱离战斗后切回治疗，然后回到等待。
	rt.hit = false
	result, actions = step(t, session, frameAt(base.Add(10*time.Second)))
	if result.State != "战斗结束切治疗" || actions[0].Key != "3" {
		t.Fatalf("结束应切回治疗: state = %q actions = %+v", result.State, actions)
	}
	result, actions = step(t, session, frameAt(base.Add(11*time.Second)))
	if result.State != "等待战斗" || len(actions) != 0 {
		t.Fatalf("之后应当等待: state = %q actions = %+v", result.State, actions)
	}
}

func TestConcertoFullSwitchesCharacter(t *testing.T) {
	spec := testSpec()
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 63; y++ {
		for x := 0; x < 63; x++ {
			d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
			if d >= 22.1 && d <= 26.6 {
				img.SetRGBA(700+x, 960+y, color.RGBA{220, 205, 110, 255})
			}
		}
	}
	result, actions := step(t, session, assist.Frame{Image: img, At: time.Now()})
	if result.State != "协奏满切人" {
		t.Fatalf("协奏满应切人, state = %q", result.State)
	}
	if len(actions) != 1 || actions[0].Kind != assist.ActionKeyTap || actions[0].Key != "1" {
		t.Fatalf("切人动作 = %+v", actions)
	}
}

type stubExtension struct {
	name  string
	state string
	key   string
	calls int
}

func (s *stubExtension) Name() string { return s.name }

func (s *stubExtension) Step(Runtime, assist.Frame) (assist.Action, string, bool, error) {
	s.calls++
	if s.state == "" {
		return assist.Action{}, "", false, nil
	}
	return assist.Action{Kind: assist.ActionKeyTap, Key: s.key}, s.state, true, nil
}

type combatRuntimeOnly struct{}

func (combatRuntimeOnly) MatchTemplate(assist.Frame, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
	return nil, nil
}

func TestExtensionRunsBeforePlan(t *testing.T) {
	ext := &stubExtension{name: "dodge", state: "扩展-闪避", key: "shift"}
	spec := testSpec()
	spec.Extensions = []Extension{ext}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "扩展-闪避" || len(actions) != 1 || actions[0].Key != "shift" {
		t.Fatalf("扩展应当先于连招生效: state = %q actions = %+v", result.State, actions)
	}
}

func TestExtensionCanBeDisabledBySettings(t *testing.T) {
	ext := &stubExtension{name: "dodge", state: "扩展-闪避", key: "shift"}
	spec := testSpec()
	spec.Extensions = []Extension{ext}
	feature := NewFeature(spec)
	settings := feature.DefaultSettings()
	settings[SettingExtPrefix+"dodge"] = false
	cfg, err := parseSettings(settings, feature.spec)
	if err != nil {
		t.Fatalf("解析设置失败: %v", err)
	}
	session := newSession(testRuntime(true), feature.spec, cfg)
	result, _ := step(t, session, frameAt(time.Now()))
	if result.State == "扩展-闪避" {
		t.Fatal("关掉开关后扩展不该生效")
	}
	if ext.calls != 0 {
		t.Fatalf("关掉后不该被调用, calls = %d", ext.calls)
	}
}

type stubTeam struct{ index int }

func (s stubTeam) Current(Runtime, assist.Frame) (int, bool, error) { return s.index, true, nil }
func (s stubTeam) Agent(Runtime, assist.Frame) (string, bool, error) {
	return "agent-x", true, nil
}

func TestConcertoSwitchUsesCurrentSlot(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = stubTeam{index: 1}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 63; y++ {
		for x := 0; x < 63; x++ {
			d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
			if d >= 22.1 && d <= 26.6 {
				img.SetRGBA(700+x, 960+y, color.RGBA{220, 205, 110, 255})
			}
		}
	}
	_, actions := step(t, session, assist.Frame{Image: img, At: time.Now()})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("2 号位在场上时应当按 3 号键, actions = %+v", actions)
	}
}

func TestAgentPlanOverridesDefault(t *testing.T) {
	spec := testSpec()
	spec.Team = stubTeam{}
	spec.PlansByAgent = map[string][]Step{
		"agent-x": {{Kind: StepLiberation}, {Kind: StepEcho}},
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "连招-爆发" || actions[0].Key != "r" {
		t.Fatalf("应当走角色专属连招: state = %q actions = %+v", result.State, actions)
	}
}

func TestAgentPlanFallsBackWhenUnknown(t *testing.T) {
	spec := testSpec()
	spec.Team = stubTeam{}
	spec.PlansByAgent = map[string][]Step{
		"other-agent": {{Kind: StepLiberation}},
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	result, _ := step(t, session, frameAt(time.Now()))
	if result.State != "连招-普攻" {
		t.Fatalf("认不出角色时应当用默认连招: state = %q", result.State)
	}
}

func TestReadyCheckSkipsUnavailableSkill(t *testing.T) {
	spec := testSpec()
	spec.Plan = []Step{{Kind: StepSkill}, {Kind: StepAttack}}
	spec.Ready = map[StepKind]ReadyCheck{
		StepSkill: {Area: geom.Rect{X1: 10, Y1: 10, X2: 60, Y2: 60}, ColorRange: [2][3]uint8{{253, 253, 253}, {255, 255, 255}}, MinRatio: 0.05},
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})

	// 按钮没高亮 -> 跳过技能，直接普攻。
	empty := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	result, actions := step(t, session, assist.Frame{Image: empty, At: time.Now()})
	if result.State != "连招-普攻" {
		t.Fatalf("技能不可用时应跳过: state = %q actions = %+v", result.State, actions)
	}

	// 按钮高亮 -> 放技能。
	lit := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 10; y < 60; y++ {
		for x := 10; x < 60; x++ {
			lit.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	session2 := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	result, actions = step(t, session2, assist.Frame{Image: lit, At: time.Now()})
	if result.State != "连招-技能" || actions[0].Key != "e" {
		t.Fatalf("技能可用时应放技能: state = %q actions = %+v", result.State, actions)
	}
}

func TestCooldownSkipsSkill(t *testing.T) {
	spec := testSpec()
	spec.Plan = []Step{{Kind: StepSkill}, {Kind: StepAttack}}
	spec.Cooldowns = map[StepKind]geom.Rect{
		StepSkill: {X1: 10, Y1: 10, X2: 60, Y2: 60},
	}

	// 读到冷却数字 -> 跳过技能。
	rt := testRuntime(true)
	rt.ocrText = "8"
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	result, _ := step(t, session, frameAt(time.Now()))
	if result.State != "连招-普攻" {
		t.Fatalf("技能冷却中应跳过: state = %q", result.State)
	}

	// 没有数字 -> 放技能。
	rt2 := testRuntime(true)
	session2 := newSession(rt2, spec.withDefaults(), sessionSettings{})
	result, _ = step(t, session2, frameAt(time.Now().Add(time.Second)))
	if result.State != "连招-技能" {
		t.Fatalf("技能就绪应放技能: state = %q", result.State)
	}
}

func TestParseCooldownSeconds(t *testing.T) {
	cases := map[string]bool{"12": true, "3.5": true, " 8s ": true, "": false, "x": false}
	for text, ok := range cases {
		if _, got := parseCooldownSeconds(text); got != ok {
			t.Fatalf("%q 解析结果 = %v, want %v", text, got, ok)
		}
	}
}

type healerTeam struct{ slot int }

func (h healerTeam) Current(Runtime, assist.Frame) (int, bool, error) { return 0, true, nil }
func (h healerTeam) Agent(Runtime, assist.Frame) (string, bool, error) {
	return "", false, nil
}
func (h healerTeam) HealerSlot(Runtime, assist.Frame) (int, bool, error) {
	return h.slot, true, nil
}

func TestHealerKeyFromTeamRecognition(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.HealerKey = "3"
	spec.Team = healerTeam{slot: 1}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "战斗前切治疗" || actions[0].Key != "2" {
		t.Fatalf("应当按识别到的治疗位按键: state = %q actions = %+v", result.State, actions)
	}
}

type slotTeam struct {
	current int
	ids     []string
}

func (s slotTeam) Current(Runtime, assist.Frame) (int, bool, error) { return s.current, true, nil }
func (s slotTeam) Agent(Runtime, assist.Frame) (string, bool, error) {
	if s.current < len(s.ids) {
		return s.ids[s.current], true, nil
	}
	return "", false, nil
}
func (s slotTeam) Slots(Runtime, assist.Frame) ([]string, error) { return s.ids, nil }

func concertoFullFrame() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 63; y++ {
		for x := 0; x < 63; x++ {
			d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
			if d >= 22.1 && d <= 26.6 {
				img.SetRGBA(700+x, 960+y, color.RGBA{220, 205, 110, 255})
			}
		}
	}
	return img
}

func TestSwitchTargetByRole(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "healer"}}
	spec.Roles = map[string]Role{"dps": RoleMainDPS, "sub": RoleSubDPS, "healer": RoleHealer}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: time.Now()})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("主C在场时应当切给治疗(3), actions = %+v", actions)
	}
}

func TestSwitchTargetFallsBackWithoutRoles(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 1, ids: []string{"a", "b", "c"}}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: time.Now()})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("没有定位信息时按顺序切下一个, actions = %+v", actions)
	}
}
