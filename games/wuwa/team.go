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
}

func newWuwaTeam() *wuwaTeam {
	return &wuwaTeam{
		slots: []screen.Area{
			{Name: "角色1数字", Rect: geom.Rect{X1: 1728, Y1: 204, X2: 1762, Y2: 240}},
			{Name: "角色2数字", Rect: geom.Rect{X1: 1728, Y1: 335, X2: 1762, Y2: 371}},
			{Name: "角色3数字", Rect: geom.Rect{X1: 1727, Y1: 467, X2: 1761, Y2: 503}},
		},
		templates: []string{"char_1_text", "char_2_text", "char_3_text"},
		threshold: 0.8,
	}
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
