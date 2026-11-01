package zzz

import (
	"io/fs"
	"sort"

	zzzassets "github.com/wengchengjian/skip-all/assets/games/zzz"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// zzzTeam 认人，照 OneDragon 的做法：
//   - 前台大头像固定在 (104,40)-(274,110)，用 avatar_1_* 模板族认；
//   - 后台两个小头像在 (559,40)-(662,91) / (741,40)-(844,91)，用 avatar_2_* 模板族认。
//
// 当前角色就是前台槽位（0），切换靠"下一个"键，所以这里只用来选角色专属连招。
type zzzTeam struct {
	front     screen.Area
	backSlots []screen.Area
	frontIDs  []string
	backIDs   []string
	threshold float64
}

func newZZZTeam() *zzzTeam {
	return &zzzTeam{
		front: screen.Area{
			Name:                   "前台头像",
			Type:                   screen.AreaTemplate,
			Rect:                   geom.Rect{X1: 104, Y1: 40, X2: 274, Y2: 110},
			TemplateMatchThreshold: 0.8,
		},
		backSlots: []screen.Area{
			{Name: "后台头像1", Type: screen.AreaTemplate, Rect: geom.Rect{X1: 559, Y1: 40, X2: 662, Y2: 91}, TemplateMatchThreshold: 0.8},
			{Name: "后台头像2", Type: screen.AreaTemplate, Rect: geom.Rect{X1: 741, Y1: 40, X2: 844, Y2: 91}, TemplateMatchThreshold: 0.8},
		},
		frontIDs:  avatarIDs("avatar1"),
		backIDs:   avatarIDs("avatar2"),
		threshold: 0.8,
	}
}

// avatarIDs 列出模板目录里已有的头像模板 id。
func avatarIDs(dir string) []string {
	entries, err := fs.ReadDir(zzzassets.FS(), "templates/"+dir)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Name())
	}
	sort.Strings(ids)
	return ids
}

// Slots 列出前后台槽位上的代理人 id（认不出留空），切人打分用。
func (t *zzzTeam) Slots(rt combat.Runtime, frame assist.Frame) ([]string, error) {
	ids := make([]string, 0, 3)
	front, err := t.matchIn(rt, frame, t.front, t.frontIDs, "avatar1")
	if err != nil {
		return nil, err
	}
	ids = append(ids, front)
	for _, area := range t.backSlots {
		back, err := t.matchIn(rt, frame, area, t.backIDs, "avatar2")
		if err != nil {
			return nil, err
		}
		ids = append(ids, back)
	}
	return ids, nil
}

// matchIn 在一个区域里逐个模板比对，返回分数最高的模板 id。
func (t *zzzTeam) matchIn(rt combat.Runtime, frame assist.Frame, area screen.Area, ids []string, subDir string) (string, error) {
	best, bestID := 0.0, ""
	for _, id := range ids {
		matches, err := rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: subDir, ID: id}, t.threshold)
		if err != nil {
			return "", err
		}
		if len(matches) > 0 && matches[0].Confidence > best {
			best, bestID = matches[0].Confidence, id
		}
	}
	return bestID, nil
}

// Current 返回当前角色槽位：绝区零前台固定在第一个槽位。
func (t *zzzTeam) Current(rt combat.Runtime, frame assist.Frame) (int, bool, error) {
	if _, ok, err := t.Agent(rt, frame); err != nil || !ok {
		return 0, false, err
	}
	return 0, true, nil
}

// Agent 识别前台角色。
func (t *zzzTeam) Agent(rt combat.Runtime, frame assist.Frame) (string, bool, error) {
	best, bestID := 0.0, ""
	for _, id := range t.frontIDs {
		matches, err := rt.MatchTemplate(frame, t.front, vision.TemplateRef{SubDir: "avatar1", ID: id}, t.threshold)
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
