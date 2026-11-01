package wuwa

import (
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"image"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// forteFake 按模板 id 返回命中，用来验证回路提示的优先级。
type forteFake struct {
	hit map[string]bool
}

func (f forteFake) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	if f.hit[ref.ID] {
		return []vision.Match{{Rect: area.Rect, Confidence: 0.9}}, nil
	}
	return nil, nil
}

func TestForteExtensionPriority(t *testing.T) {
	ext := &wuwaForteExt{
		Area:        screen.Area{Name: "回路提示"},
		SubDir:      "forte",
		EID:         "e_forte",
		MouseID:     "mouse_forte",
		SkillKey:    "e",
		Threshold:   0.6,
		Interval:    500 * time.Millisecond,
		AttackPoint: geom.Point{X: 960, Y: 540},
	}
	frame := assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()}

	// 两个图标都在 -> 先按 E（强化共鸣技能）。
	action, state, ok, err := ext.Step(forteFake{hit: map[string]bool{"e_forte": true, "mouse_forte": true}}, frame)
	if err != nil || !ok || state != "回路-E" || action.Key != "e" {
		t.Fatalf("应当先打强化E: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 只有鼠标图标 -> 重击。
	ext2 := *ext
	ext2.last = time.Time{}
	action, state, ok, err = ext2.Step(forteFake{hit: map[string]bool{"mouse_forte": true}}, frame)
	if err != nil || !ok || state != "回路-重击" || action.Kind != assist.ActionClick {
		t.Fatalf("应当重击: action=%+v state=%q ok=%v err=%v", action, state, ok, err)
	}

	// 都没有 -> 不出手。
	ext3 := *ext
	ext3.last = time.Time{}
	if _, _, ok, _ := ext3.Step(forteFake{hit: map[string]bool{}}, frame); ok {
		t.Fatal("没有回路提示时不该出手")
	}
}

// jinhsiTeam 模拟"当前角色是今汐"的队伍；offField 时认不出当前角色（动画期间）。
type jinhsiTeam struct {
	agent    string
	offField bool
}

func (j *jinhsiTeam) Agent(combat.Runtime, assist.Frame) (string, bool, error) {
	return j.agent, j.agent != "", nil
}

func (j *jinhsiTeam) Current(combat.Runtime, assist.Frame) (int, bool, error) {
	if j.offField {
		return -1, false, nil
	}
	return 0, true, nil
}

func TestJinhsiAlternatesAndFinishes(t *testing.T) {
	team := &jinhsiTeam{agent: "char_jinhsi"}
	ext := &wuwaJinhsiExt{team: team, skillKey: "e", echoKey: "q"}
	rt := &teamFakeRuntime{}
	base := time.Now()

	// 第一次出手：普攻。
	action, state, ok, err := ext.Step(rt, assist.Frame{Image: benchFrame(), At: base})
	if err != nil || !ok || state != "今汐-普攻" || action.Kind != assist.ActionClick {
		t.Fatalf("首次应当普攻: state=%q action=%+v ok=%v err=%v", state, action, ok, err)
	}
	// 200ms 节流内不出手。
	if _, _, ok, _ := ext.Step(rt, assist.Frame{Image: benchFrame(), At: base.Add(100 * time.Millisecond)}); ok {
		t.Fatal("节流窗口内不该再出手")
	}
	// 300ms 后：共鸣技能。
	action, state, ok, _ = ext.Step(rt, assist.Frame{Image: benchFrame(), At: base.Add(300 * time.Millisecond)})
	if !ok || state != "今汐-共鸣" || action.Key != "e" {
		t.Fatalf("应当交替到共鸣技能: state=%q action=%+v", state, action)
	}

	// 动画期间（认不出当前角色）：不出手，但被记下。
	team.offField = true
	if _, _, ok, _ := ext.Step(rt, assist.Frame{Image: benchFrame(), At: base.Add(400 * time.Millisecond)}); ok {
		t.Fatal("动画期间不该出手")
	}
	// 重新出场：声骸收尾。
	team.offField = false
	action, state, ok, _ = ext.Step(rt, assist.Frame{Image: benchFrame(), At: base.Add(700 * time.Millisecond)})
	if !ok || state != "今汐-收尾" || action.Key != "q" {
		t.Fatalf("动画结束后应当声骸收尾: state=%q action=%+v", state, action)
	}
}

func TestJinhsiExtensionIgnoresOtherCharacters(t *testing.T) {
	team := &jinhsiTeam{agent: "char_aalto"}
	ext := &wuwaJinhsiExt{team: team, skillKey: "e", echoKey: "q"}
	if _, _, ok, _ := ext.Step(&teamFakeRuntime{}, assist.Frame{Image: benchFrame(), At: time.Now()}); ok {
		t.Fatal("不是今汐时不该接管")
	}
}
