package wuwa

import (
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// wuwaTeam 判读当前角色，照 ok-ww 的 in_team()：
// 每个角色卡上有 1/2/3 键位数字，当前角色的卡面被高亮放大、数字被挡住，
// 所以"找不到数字的那一格"就是当前角色。
// 模板直接取上游 char_1_text / char_2_text / char_3_text（1080p 参考裁剪）。
type wuwaTeam struct {
	slots     []screen.Area
	templates []string
	threshold float64
	// avatarSlots/avatarIDs 用来认人：槽位头像区 + 全部角色头像模板。
	avatarSlots     []screen.Area
	avatarIDs       []string
	healerIDs       []string
	avatarThreshold float64
}

func newWuwaTeam() *wuwaTeam {
	return &wuwaTeam{
		avatarSlots: []screen.Area{
			{Name: "角色1头像", Rect: geom.Rect{X1: 1750, Y1: 205, X2: 1900, Y2: 305}},
			{Name: "角色2头像", Rect: geom.Rect{X1: 1750, Y1: 335, X2: 1900, Y2: 435}},
			{Name: "角色3头像", Rect: geom.Rect{X1: 1750, Y1: 465, X2: 1900, Y2: 565}},
		},
		avatarIDs:       agentPlanIDs(),
		healerIDs:       healerIDs(),
		avatarThreshold: 0.75,
		slots: []screen.Area{
			{Name: "角色1数字", Rect: geom.Rect{X1: 1728, Y1: 204, X2: 1762, Y2: 240}},
			{Name: "角色2数字", Rect: geom.Rect{X1: 1728, Y1: 335, X2: 1762, Y2: 371}},
			{Name: "角色3数字", Rect: geom.Rect{X1: 1727, Y1: 467, X2: 1761, Y2: 503}},
		},
		templates: []string{"char_1_text", "char_2_text", "char_3_text"},
		threshold: 0.8,
	}
}

// Agent 识别当前角色：先按数字缺失定槽位，再在该槽位头像区里逐个头像模板比对。
func (t *wuwaTeam) Agent(rt combat.Runtime, frame assist.Frame) (string, bool, error) {
	index, ok, err := t.Current(rt, frame)
	if err != nil || !ok || index >= len(t.avatarSlots) {
		return "", false, nil
	}
	area := t.avatarSlots[index]
	best, bestID := 0.0, ""
	for _, id := range t.avatarIDs {
		matches, err := rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: "team", ID: id}, t.avatarThreshold)
		if err != nil {
			return "", false, err
		}
		if len(matches) > 0 && matches[0].Confidence > best {
			best, bestID = matches[0].Confidence, id
		}
	}
	if bestID == "" {
		return "", false, nil
	}
	return bestID, true, nil
}

// Slots 列出三个槽位上的角色 id（认不出留空），切人打分用。
func (t *wuwaTeam) Slots(rt combat.Runtime, frame assist.Frame) ([]string, error) {
	ids := make([]string, 0, len(t.avatarSlots))
	for _, area := range t.avatarSlots {
		best, bestID := 0.0, ""
		for _, id := range t.avatarIDs {
			matches, err := rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: "team", ID: id}, t.avatarThreshold)
			if err != nil {
				return nil, err
			}
			if len(matches) > 0 && matches[0].Confidence > best {
				best, bestID = matches[0].Confidence, id
			}
		}
		ids = append(ids, bestID)
	}
	return ids, nil
}

// HealerSlot 找队伍里的治疗位：三个槽位头像分别和上游 CharType.HEALER 的角色模板比对。
func (t *wuwaTeam) HealerSlot(rt combat.Runtime, frame assist.Frame) (int, bool, error) {
	for i, area := range t.avatarSlots {
		for _, id := range t.healerIDs {
			matches, err := rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: "team", ID: id}, t.avatarThreshold)
			if err != nil {
				return 0, false, err
			}
			if len(matches) > 0 {
				return i, true, nil
			}
		}
	}
	return 0, false, nil
}

func (t *wuwaTeam) Current(rt combat.Runtime, frame assist.Frame) (int, bool, error) {
	missing := -1
	found := 0
	for i, id := range t.templates {
		matches, err := rt.MatchTemplate(frame, t.slots[i], vision.TemplateRef{SubDir: "team", ID: id}, t.threshold)
		if err != nil {
			return 0, false, err
		}
		if len(matches) == 0 {
			if missing < 0 {
				missing = i
			}
			continue
		}
		found++
	}
	// 上游要求至少找到 1 个、且有 1 个缺失（三人队正常是 2 个可见）。
	if found >= 1 && missing >= 0 {
		return missing, true, nil
	}
	return 0, false, nil
}

// agentPlanIDs 返回有专属连招的角色模板 id（与 templates 目录一致）。
func agentPlanIDs() []string {
	ids := make([]string, 0, len(agentPlans))
	for id := range agentPlans {
		ids = append(ids, id)
	}
	return ids
}

// healerIDs 是上游 CharType.HEALER 的角色模板 id（维里奈、守岸人、白芷等）。
var healerIDs_ = []string{
	"char_verina", "char_shorekeeper", "char_suisui", "char_taoqi",
	"char_jianxin", "char_baizhi", "char_youhu", "char_brant",
}

func healerIDs() []string { return healerIDs_ }
