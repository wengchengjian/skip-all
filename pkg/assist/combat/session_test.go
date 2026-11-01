package combat

import (
	"context"
	"errors"
	"image"
	"io"
	"log/slog"
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
}

func (f *fakeRuntime) Screen(name string) (*screen.Screen, error) {
	sc, ok := f.screens.Get(name)
	if !ok {
		return nil, errors.New("未找到画面 " + name)
	}
	return sc, nil
}

func (f *fakeRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	return nil, nil
}

func (f *fakeRuntime) MatchTemplate(assist.Frame, screen.Area, vision.TemplateRef, float64) ([]vision.Match, error) {
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
		},
	})
	return &fakeRuntime{screens: set, hit: hit}
}

func testSpec() Spec {
	return Spec{
		Screen:      "测试战斗",
		DetectAreas: []string{"锁定"},
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
