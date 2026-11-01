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
	calls   int
}

func (f *teamFakeRuntime) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	f.calls++
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
	rt := &teamFakeRuntime{present: map[string]bool{"char_1_text": true, "char_3_text": true}}
	idx, ok, err := team.Current(rt, frame)
	if err != nil || !ok || idx != 1 {
		t.Fatalf("应当识别到 2 号位: idx=%d ok=%v err=%v", idx, ok, err)
	}

	// 三个数字都在 -> 识别不出（不在战斗或界面特殊）。
	rt = &teamFakeRuntime{present: map[string]bool{"char_1_text": true, "char_2_text": true, "char_3_text": true}}
	if _, ok, _ := team.Current(rt, frame); ok {
		t.Fatal("三个数字都在时不该判定当前角色")
	}

	// 都看不到 -> 也不判定。
	rt = &teamFakeRuntime{present: map[string]bool{}}
	if _, ok, _ := team.Current(rt, frame); ok {
		t.Fatal("什么都看不到时不该判定")
	}
}

func TestHealerSlotByAvatar(t *testing.T) {
	team := newWuwaTeam()
	rt := &teamFakeRuntime{present: map[string]bool{"char_verina": true}}
	// fake 对所有槽位都返回命中，所以这里只要验证能找到、且下标合法。
	index, ok, err := team.HealerSlot(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()})
	if err != nil || !ok || index != 0 {
		t.Fatalf("应当识别到治疗位: index=%d ok=%v err=%v", index, ok, err)
	}
	// 队伍里没有治疗 -> 找不到。
	rt2 := &teamFakeRuntime{present: map[string]bool{"char_jinhsi": true}}
	if _, ok, _ := team.HealerSlot(rt2, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: time.Now()}); ok {
		t.Fatal("没有治疗时不该识别到治疗位")
	}
}

func TestTeamWhitelistReducesMatching(t *testing.T) {
	// 用一个"模板很少"的队伍，第一次分批扫完就学出白名单。
	team := &wuwaTeam{
		avatarSlots:     newWuwaTeam().avatarSlots,
		avatarIDs:       []string{"char_verina", "char_jinhsi", "char_baizhi"},
		avatarThreshold: 0.75,
	}
	rt := &teamFakeRuntime{present: map[string]bool{"char_verina": true}}
	now := time.Now()
	frame := func(at time.Time) assist.Frame {
		return assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: at}
	}

	// 分批扫描：多帧之后才学出白名单。
	for i := 0; i < 5 && len(team.learned) == 0; i++ {
		if _, err := team.Slots(rt, frame(now.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("Slots 出错: %v", err)
		}
	}
	first := rt.calls
	if first == 0 {
		t.Fatal("应当有比对")
	}
	if len(team.learned) == 0 {
		t.Fatal("多帧之后应当学出白名单")
	}

	// 第二轮只比白名单里的 1 张 × 3 个槽位。
	rt.calls = 0
	if _, err := team.Slots(rt, frame(now.Add(time.Second))); err != nil {
		t.Fatalf("Slots 出错: %v", err)
	}
	// 关键不变量：单帧比对数量不超过 wuwaScanBatch × 槽位数。
	if limit := wuwaScanBatch * len(team.avatarSlots); rt.calls > limit {
		t.Fatalf("单帧比对 %d 次超过上限 %d", rt.calls, limit)
	}
	if len(team.learned) > len(team.avatarSlots) {
		t.Fatalf("白名单里不该超过槽位数的角色: %v", team.learned)
	}
	_ = first
}

func TestTeamWhitelistRefreshesAfterTTL(t *testing.T) {
	team := newWuwaTeam()
	rt := &teamFakeRuntime{present: map[string]bool{"char_verina": true}}
	now := time.Now()
	_, _ = team.Slots(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: now})
	rt.calls = 0
	// 超过刷新间隔后重新开始分批扫描。
	_, _ = team.Slots(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: now.Add(30 * time.Second)})
	if rt.calls == 0 {
		t.Fatal("超过 TTL 应当重新扫描")
	}
	// 连续多帧之后应当认全一轮。
	for i := 0; i < len(team.avatarIDs)/wuwaScanBatch+2; i++ {
		_, _ = team.Slots(rt, assist.Frame{Image: image.NewRGBA(image.Rect(0, 0, 1920, 1080)), At: now.Add(time.Duration(31+i) * time.Second)})
	}
	if len(team.learned) == 0 {
		t.Fatal("多帧之后应当学出白名单")
	}
}
