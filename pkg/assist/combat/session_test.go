package combat

import (
	"context"
	"errors"
	"image"
	"image/color"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

type fakeRuntime struct {
	screens  *screen.Set
	hit      bool
	scores   map[string]float64 // 按模板 id 给分数，决定"是否锁定目标"这类判据
	ocrText  string             // 技能按钮上 OCR 到的文字（冷却数字）
	ultSlots []geom.Rect        // 大招就绪判据的槽位
	ultReady map[int]bool       // 哪些槽位就绪
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
	if f.ultReady != nil && ref.SubDir == "libready" {
		for i, slot := range f.ultSlots {
			if slot == area.Rect && f.ultReady[i] {
				return []vision.Match{{Rect: area.Rect, Confidence: 0.95}}, nil
			}
		}
		return nil, nil
	}
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
			{Name: "中断", RectValues: []int{560, 360, 1360, 720}},
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

func TestIntroStepsAfterSwitch(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleMainDPS, "b": RoleSubDPS, "c": RoleHealer}
	spec.IntroSteps = []Step{{Kind: StepSkill}, {Kind: StepAttack}}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	// 第 0 帧：协奏满 -> 切人
	if result, _ := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base}); result.State != "协奏满切人" {
		t.Fatalf("应当先切人: %q", result.State)
	}
	// 第 1 帧：入场技（技能），第 2 帧：入场技（普攻），之后才回主连招
	result, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(300 * time.Millisecond)})
	if result.State != "入场-技能" || actions[0].Key != "e" {
		t.Fatalf("入场技第一步: state=%q actions=%+v", result.State, actions)
	}
	result, _ = step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(600 * time.Millisecond)})
	if result.State != "入场-普攻" {
		t.Fatalf("入场技第二步: %q", result.State)
	}
}

func TestAbortPausesWhenReviveShown(t *testing.T) {
	spec := testSpec()
	spec.AbortArea = "中断"
	spec.AbortTexts = []string{"复活"}
	rt := testRuntime(true)
	rt.ocrText = "复活确认"
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	result, actions := step(t, session, frameAt(time.Now()))
	if result.State != "战斗中断" || len(actions) != 0 {
		t.Fatalf("出现复活提示时应暂停: state=%q actions=%+v", result.State, actions)
	}
}

func TestSwitchInvalidatesCooldownCache(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"a", "b", "c"}}
	spec.Roles = map[string]Role{"a": RoleMainDPS, "b": RoleSubDPS, "c": RoleHealer}
	spec.Plan = []Step{{Kind: StepSkill}, {Kind: StepAttack}}
	spec.Cooldowns = map[StepKind]geom.Rect{StepSkill: {X1: 10, Y1: 10, X2: 60, Y2: 60}}

	rt := testRuntime(true)
	rt.ocrText = "9"
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	base := time.Now()

	// 先读一次冷却（"9" 秒），技能被跳过。
	result, _ := step(t, session, frameAt(base))
	if result.State != "连招-普攻" {
		t.Fatalf("冷却中应当跳过技能, state = %q", result.State)
	}
	if len(session.cdCache) == 0 {
		t.Fatal("应当写入过冷却缓存")
	}
	// 切人后缓存必须清空，否则新角色的技能会被误判成冷却中。
	session.noteSwitch(assist.Frame{}, base.Add(time.Second))
	if len(session.cdCache) != 0 {
		t.Fatalf("切人后冷却缓存应清空, 还有 %d 条", len(session.cdCache))
	}
	if session.agentAt != (time.Time{}) || session.agentID != "" {
		t.Fatal("切人后角色缓存也要清空")
	}
}

func TestUltReadyBonusAffectsSwitch(t *testing.T) {
	slots := []geom.Rect{
		{X1: 1800, Y1: 210, X2: 1900, Y2: 265},
		{X1: 1800, Y1: 340, X2: 1900, Y2: 400},
		{X1: 1800, Y1: 470, X2: 1900, Y2: 530},
	}
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "other"}}
	spec.Roles = map[string]Role{"dps": RoleMainDPS, "sub": RoleSubDPS}
	spec.UltReady = &UltReadyCheck{Slots: slots, SubDir: "libready", IDs: []string{"lib_ready_fire"}, Threshold: 0.8, Bonus: 25}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	rt := testRuntime(true)
	rt.ultSlots = slots
	rt.ultReady = map[int]bool{2: true}
	session := newSession(rt, spec.withDefaults(), sessionSettings{})
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: time.Now()})
	// 副C(20)+久未出场(20)=40 与 未知(5)+久未出场(20)+大招就绪(25)=50，后者应当胜出。
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("大招就绪的槽位应当被优先选中, actions = %+v", actions)
	}
}

func TestHealerLockoutAfterSwitch(t *testing.T) {
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "healer"}}
	spec.Roles = map[string]Role{"dps": RoleMainDPS, "sub": RoleSubDPS, "healer": RoleHealer}
	spec.Switch = &SwitchPolicy{MinInterval: time.Millisecond, HealerLockout: 16 * time.Second}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.8,
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()

	// 第一次协奏满：主C在场，应当切治疗(3)。
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("第一次应当切治疗位, actions = %+v", actions)
	}
	// 5 秒后再满：治疗位在 16 秒锁内，应当改切副C(2)。
	_, actions = step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(5 * time.Second)})
	if len(actions) != 1 || actions[0].Key != "2" {
		t.Fatalf("治疗位冷却中应当切副C, actions = %+v", actions)
	}
	// 17 秒后：治疗位解锁，又能被选中。
	_, actions = step(t, session, assist.Frame{Image: concertoFullFrame(), At: base.Add(17 * time.Second)})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("锁定结束后应当能再切治疗, actions = %+v", actions)
	}
}

// forteFrame 造一帧：共鸣回路区域要么是白点（满），要么是暗色（未满）。
func forteFrame(full bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	area := geom.Rect{X1: 1125, Y1: 996, X2: 1156, Y2: 1008}
	c := color.RGBA{20, 20, 20, 255}
	if full {
		c = color.RGBA{252, 252, 252, 255}
	}
	for y := area.Y1; y < area.Y2; y++ {
		for x := area.X1; x < area.X2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestForteGateBlocksSkill(t *testing.T) {
	newSpec := func(need ForteNeed) Spec {
		spec := testSpec()
		spec.Keys.Skill = "e"
		spec.Plan = []Step{{Kind: StepSkill}, {Kind: StepAttack}}
		spec.ForteFull = &ForteCheck{
			Area:       geom.Rect{X1: 1125, Y1: 996, X2: 1156, Y2: 1008},
			ColorRange: [2][3]uint8{{244, 246, 250}, {255, 255, 255}},
			MinRatio:   0.08,
		}
		spec.ForteGate = map[string]ForteNeed{"agent-x": need}
		return spec
	}
	team := stubTeam{}

	// 需要"满"：未满时跳过技能，直接普攻。
	spec := newSpec(ForteNeedFull)
	spec.Team = team
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	if result, actions := step(t, session, assist.Frame{Image: forteFrame(false), At: time.Now()}); result.State != "连招-普攻" {
		t.Fatalf("回路未满时应当跳过技能: state=%q actions=%+v", result.State, actions)
	}
	// 满了之后放技能。
	spec2 := newSpec(ForteNeedFull)
	spec2.Team = team
	session2 := newSession(testRuntime(true), spec2.withDefaults(), sessionSettings{})
	if result, _ := step(t, session2, assist.Frame{Image: forteFrame(true), At: time.Now()}); result.State != "连招-技能" {
		t.Fatalf("回路满了应当放技能: state=%q", result.State)
	}

	// 需要"未满"：满时反而跳过。
	spec3 := newSpec(ForteNeedNotFull)
	spec3.Team = team
	session3 := newSession(testRuntime(true), spec3.withDefaults(), sessionSettings{})
	if result, _ := step(t, session3, assist.Frame{Image: forteFrame(true), At: time.Now()}); result.State != "连招-普攻" {
		t.Fatalf("要求未满时，满了应当跳过技能: state=%q", result.State)
	}
}

func TestIntroWaitsForLanding(t *testing.T) {
	spec := testSpec()
	spec.Plan = []Step{{Kind: StepAttack}, {Kind: StepSkill}, {Kind: StepAttack}}
	spec.Keys.Attack = ""
	spec.AttackPoint = geom.Point{X: 960, Y: 540}
	spec.Intro = &IntroSpec{MaxDuration: 1200 * time.Millisecond}
	spec.Ready = map[StepKind]ReadyCheck{
		StepSkill: {Area: geom.Rect{X1: 10, Y1: 10, X2: 60, Y2: 60}, ColorRange: [2][3]uint8{{253, 253, 253}, {255, 255, 255}}, MinRatio: 0.05},
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()

	// 模拟一次切人：进入入场等待。
	session.noteSwitch(assist.Frame{}, base)

	dark := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	result, actions := step(t, session, assist.Frame{Image: dark, At: base.Add(100 * time.Millisecond)})
	if result.State != "入场-落地" || len(actions) != 1 || actions[0].Kind != assist.ActionClick {
		t.Fatalf("入场等待期间应当点普攻: state=%q actions=%+v", result.State, actions)
	}

	// 技能变可用 = 落地，回到连招。
	lit := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 10; y < 60; y++ {
		for x := 10; x < 60; x++ {
			lit.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	result, _ = step(t, session, assist.Frame{Image: lit, At: base.Add(300 * time.Millisecond)})
	if !strings.HasPrefix(result.State, "连招-") {
		t.Fatalf("落地后应当回到连招, state=%q", result.State)
	}
	if !session.introUntil.IsZero() {
		t.Fatal("落地后入场等待应当结束")
	}
}

func TestIntroTimesOut(t *testing.T) {
	spec := testSpec()
	spec.Intro = &IntroSpec{MaxDuration: 500 * time.Millisecond}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.noteSwitch(assist.Frame{}, base)
	dark := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// 超过 MaxDuration 后不再处于入场等待。
	step(t, session, assist.Frame{Image: dark, At: base.Add(600 * time.Millisecond)})
	if !session.introUntil.IsZero() {
		t.Fatal("超时后应当结束入场等待")
	}
}

func TestIntroBurnMode(t *testing.T) {
	spec := testSpec()
	spec.Keys.Skill = "e"
	spec.Keys.Echo = "q"
	spec.Keys.Attack = ""
	spec.Plan = []Step{{Kind: StepAttack}}
	spec.Intro = &IntroSpec{MaxDuration: time.Second}
	spec.IntroByAgent = map[string]IntroSpec{"agent-x": {Mode: IntroBurn, MaxDuration: 1500 * time.Millisecond}}
	spec.Team = stubTeam{}
	spec.Ready = map[StepKind]ReadyCheck{
		StepSkill: {Area: geom.Rect{X1: 10, Y1: 10, X2: 60, Y2: 60}, ColorRange: [2][3]uint8{{253, 253, 253}, {255, 255, 255}}, MinRatio: 0.05},
	}
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	session.agentID = "agent-x" // 固定当前角色，避免测试依赖认人
	session.noteSwitch(assist.Frame{}, base)

	lit := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 10; y < 60; y++ {
		for x := 10; x < 60; x++ {
			lit.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	// 技能可用 -> 入场爆发按技能。
	result, actions := step(t, session, assist.Frame{Image: lit, At: base.Add(100 * time.Millisecond)})
	if result.State != "入场-爆发" || actions[0].Key != "e" {
		t.Fatalf("技能可用时入场应当打技能: state=%q actions=%+v", result.State, actions)
	}

	// 技能进冷却（不亮）-> 声骸收尾（上游 if not click_echo(): click()），入场结束。
	dark := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	result, actions = step(t, session, assist.Frame{Image: dark, At: base.Add(300 * time.Millisecond)})
	if result.State != "入场-收尾" || actions[0].Key != "q" {
		t.Fatalf("技能冷却后应当声骸收尾: state=%q actions=%+v", result.State, actions)
	}
	if !session.introUntil.IsZero() {
		t.Fatal("收尾后入场状态应当清空")
	}
	// 再下一帧回到连招。
	result, _ = step(t, session, assist.Frame{Image: dark, At: base.Add(400 * time.Millisecond)})
	if !strings.HasPrefix(result.State, "连招-") {
		t.Fatalf("收尾之后应当回到连招: state=%q", result.State)
	}
}

func TestBuffExpiredBonusInSwitch(t *testing.T) {
	two := func(buff time.Duration) Spec {
		spec := testSpec()
		spec.Keys.Switch = []string{"1", "2", "3"}
		spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "sub2"}}
		spec.Roles = map[string]Role{"dps": RoleMainDPS, "sub": RoleSubDPS, "sub2": RoleSubDPS}
		spec.BuffTimes = map[string]time.Duration{"sub": buff, "sub2": buff}
		spec.Switch = &SwitchPolicy{MinInterval: time.Millisecond}
		spec.Concerto = &Concerto{
			Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
			Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
			Inner:     0.35119,
			Outer:     0.42261,
			FullRatio: 0.8,
		}
		return spec
	}
	// 3 号位的增益已过期：应当优先选它（同为副C，靠 buffBonus 拉开）。
	spec := two(4 * time.Second)
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	base := time.Now()
	// 让 1 号位先出场过（记录切人时间），3 号位从未出场。
	session.switchInAt[1] = base.Add(-1 * time.Second)
	_, actions := step(t, session, assist.Frame{Image: concertoFullFrame(), At: base})
	if len(actions) != 1 || actions[0].Key != "3" {
		t.Fatalf("增益已过期的槽位应当被优先选中, actions=%+v", actions)
	}
}

// TestConcertoUsesStrongestElement 验证按"最强元素色"判满：
// 两种颜色各覆盖一半环（合计够 0.8 但单个都不够）时不应触发切人。
func TestConcertoUsesStrongestElement(t *testing.T) {
	ring := func(colorA, colorB color.RGBA, split bool) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		for y := 0; y < 63; y++ {
			for x := 0; x < 63; x++ {
				d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
				if d < 22.1 || d > 26.6 {
					continue
				}
				useA := !split || x < 31
				c := colorB
				if useA {
					c = colorA
				}
				img.SetRGBA(700+x, 960+y, c)
			}
		}
		return img
	}
	base := func() Spec {
		spec := testSpec()
		spec.Keys.Switch = []string{"1", "2", "3"}
		spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "healer"}}
		spec.Concerto = &Concerto{
			Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
			Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}, {{60, 150, 210}, {95, 180, 245}}},
			Inner:     0.35119,
			Outer:     0.42261,
			FullRatio: 0.8,
		}
		return spec
	}
	yellow := color.RGBA{220, 205, 110, 255}
	blue := color.RGBA{80, 165, 230, 255}

	// 单一颜色铺满整环 -> 满。
	spec := base()
	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	if _, ok := session.concertoSwitch(assist.Frame{Image: ring(yellow, yellow, false), At: time.Now()}, time.Now()); !ok {
		t.Fatal("单色铺满环应当判为满")
	}
	// 两种颜色各占一半 -> 单个色都不到门限，不应切人。
	spec2 := base()
	session2 := newSession(testRuntime(true), spec2.withDefaults(), sessionSettings{})
	if _, ok := session2.concertoSwitch(assist.Frame{Image: ring(yellow, blue, true), At: time.Now()}, time.Now()); ok {
		t.Fatal("两色各占一半不应算满（上游只用当前角色那一色）")
	}
}

// TestConcertoRequiresFullRing 验证"差一点满不切"：覆盖率 0.85 不切，0.96 才切。
func TestConcertoRequiresFullRing(t *testing.T) {
	// 按"覆盖弧长比例"造环：frac 表示环上有多少比例被点亮。
	makeRing := func(frac float64) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		for y := 0; y < 63; y++ {
			for x := 0; x < 63; x++ {
				d := math.Hypot(float64(x)+0.5-31.5, float64(y)+0.5-31.5)
				if d < 22.1 || d > 26.6 {
					continue
				}
				// 用角度决定这一段是否点亮
				ang := math.Atan2(float64(y)+0.5-31.5, float64(x)+0.5-31.5)
				if (ang+math.Pi)/(2*math.Pi) <= frac {
					img.SetRGBA(700+x, 960+y, color.RGBA{220, 205, 110, 255})
				}
			}
		}
		return img
	}
	spec := testSpec()
	spec.Keys.Switch = []string{"1", "2", "3"}
	spec.Team = slotTeam{current: 0, ids: []string{"dps", "sub", "healer"}}
	spec.Concerto = &Concerto{
		Rect:      geom.Rect{X1: 700, Y1: 960, X2: 763, Y2: 1023},
		Colors:    [][2][3]uint8{{{205, 190, 90}, {235, 222, 130}}},
		Inner:     0.35119,
		Outer:     0.42261,
		FullRatio: 0.95,
	}

	session := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	if _, ok := session.concertoSwitch(assist.Frame{Image: makeRing(0.85), At: time.Now()}, time.Now()); ok {
		t.Fatal("86% 的环不该判为满")
	}
	session2 := newSession(testRuntime(true), spec.withDefaults(), sessionSettings{})
	if _, ok := session2.concertoSwitch(assist.Frame{Image: makeRing(0.98), At: time.Now()}, time.Now()); !ok {
		t.Fatal("满环应当触发切人")
	}
}
