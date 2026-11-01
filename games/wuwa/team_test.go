package wuwa

import (
	"image"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

type teamFakeRuntime struct {
	present map[string]bool
}

func (f teamFakeRuntime) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	if f.present[ref.ID] {
		return []vision.Match{{Rect: area.Rect, Confidence: 0.95}}, nil
	}
	return nil, nil
}

func TestAgentPlansPresent(t *testing.T) {
	if len(agentPlans) < 10 {
		t.Fatalf("角色专属连招数量太少: %d", len(agentPlans))
	}
	for id, plan := range agentPlans {
		if len(plan) == 0 {
			t.Fatalf("角色 %s 的连招为空", id)
		}
	}
}

func TestTeamCurrentByMissingDigit(t *testing.T) {
	team := newWuwaTeam()
	frame := assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()}

	// 数字 1、3 可见、2 缺失 -> 当前是 2 号位（下标 1）。
	rt := teamFakeRuntime{present: map[string]bool{"char_1_text": true, "char_3_text": true}}
	idx, ok, err := team.Current(rt, frame)
	if err != nil || !ok || idx != 1 {
		t.Fatalf("应当识别到 2 号位: idx=%d ok=%v err=%v", idx, ok, err)
	}

	// 三个数字都在 -> 识别不出（不在战斗或界面特殊）。
	rt = teamFakeRuntime{present: map[string]bool{"char_1_text": true, "char_2_text": true, "char_3_text": true}}
	if _, ok, _ := team.Current(rt, frame); ok {
		t.Fatal("三个数字都在时不该判定当前角色")
	}

	// 都看不到 -> 也不判定。
	rt = teamFakeRuntime{present: map[string]bool{}}
	if _, ok, _ := team.Current(rt, frame); ok {
		t.Fatal("什么都看不到时不该判定")
	}
}

func TestHealerSlotByAvatar(t *testing.T) {
	team := newWuwaTeam()
	rt := teamFakeRuntime{present: map[string]bool{"char_verina": true}}
	// fake 对所有槽位都返回命中，所以这里只要验证能找到、且下标合法。
	index, ok, err := team.HealerSlot(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()})
	if err != nil || !ok || index != 0 {
		t.Fatalf("应当识别到治疗位: index=%d ok=%v err=%v", index, ok, err)
	}
	// 队伍里没有治疗 -> 找不到。
	rt2 := teamFakeRuntime{present: map[string]bool{"char_jinhsi": true}}
	if _, ok, _ := team.HealerSlot(rt2, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()}); ok {
		t.Fatal("没有治疗时不该识别到治疗位")
	}
}
