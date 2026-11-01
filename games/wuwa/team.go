package wuwa

import (
	"sync"
	"time"

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
	// learned 是本场战斗认出来的角色模板白名单：认出来之后只比对这几张，
	// 不必每帧扫几十张头像。
	mu        sync.Mutex
	learned   []string
	learnedAt time.Time
	// scanCursor 让全量认人分摊到多帧：一次要比 3 个槽位 × 62 张模板，
	// 一帧全扫会明显卡顿。
	scanCursor int
}

func newWuwaTeam() *wuwaTeam {
	return &wuwaTeam{
		// 槽位区域按模板实际尺寸收紧：上游头像模板约 70x65，记录位置在
		// 4K (3554,464)/(3537,723)/(3545,986) —— 折半后左上角在 x≈1770、y≈232/361/493。
		// 区域比模板留一点余量即可，别用整条右边栏（那会让匹配慢一倍、也更容易误匹配）。
		avatarSlots: []screen.Area{
			{Name: "角色1头像", Rect: geom.Rect{X1: 1760, Y1: 225, X2: 1850, Y2: 305}},
			{Name: "角色2头像", Rect: geom.Rect{X1: 1760, Y1: 355, X2: 1850, Y2: 435}},
			{Name: "角色3头像", Rect: geom.Rect{X1: 1760, Y1: 487, X2: 1850, Y2: 567}},
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
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	candidates, learning := t.candidates(now)
	area := t.avatarSlots[index]
	best, bestID := 0.0, ""
	if !learning && len(candidates) > 0 {
		// 白名单里认不出来说明换人了，作废白名单重新分批认。
		bestID = matchBest(rt, frame, area, candidates, t.avatarThreshold)
		if bestID == "" {
			t.invalidate()
			candidates, _ = t.candidates(now)
		}
	}
	for _, id := range candidates {
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
	if learning {
		t.learn([]string{bestID}, now)
	}
	return bestID, true, nil
}

// Slots 列出三个槽位上的角色 id（认不出留空），切人打分用。
// teamRefreshTTL 之后会重新全量认一次，队伍变了也能跟上。
const teamRefreshTTL = 10 * time.Second

// wuwaScanBatch 是一次最多比对的模板数量。鸣潮的头像模板来自 4K 裁剪、比绝区零的大，
// 单张匹配约 9ms，所以一帧只比 1 张（3 个槽位 ≈ 28ms），62 张约 2 秒扫完一轮。
const wuwaScanBatch = 1

// candidates 返回这一轮要比对的模板：白名单还新鲜就只比那几张，
// 否则分批扫描，每帧只比一小批。
func (t *wuwaTeam) candidates(now time.Time) (ids []string, learning bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.learned) > 0 && now.Sub(t.learnedAt) < teamRefreshTTL {
		return t.learned, false
	}
	if len(t.avatarIDs) == 0 {
		return nil, false
	}
	if t.scanCursor >= len(t.avatarIDs) {
		t.scanCursor = 0
	}
	end := t.scanCursor + wuwaScanBatch
	if end > len(t.avatarIDs) {
		end = len(t.avatarIDs)
	}
	batch := t.avatarIDs[t.scanCursor:end]
	t.scanCursor = end
	if t.scanCursor >= len(t.avatarIDs) {
		t.learnedAt = now
	}
	return batch, true
}

// matchBest 在一个区域里比对若干模板，返回分数最高的 id。
func matchBest(rt combat.Runtime, frame assist.Frame, area screen.Area, ids []string, threshold float64) string {
	best, bestID := 0.0, ""
	for _, id := range ids {
		matches, err := rt.MatchTemplate(frame, area, vision.TemplateRef{SubDir: "team", ID: id}, threshold)
		if err != nil || len(matches) == 0 {
			continue
		}
		if matches[0].Confidence > best {
			best, bestID = matches[0].Confidence, id
		}
	}
	return bestID
}

// invalidate 作废白名单，让下一次 candidates 重新分批扫描。
func (t *wuwaTeam) invalidate() {
	t.mu.Lock()
	t.learnedAt = time.Time{}
	t.scanCursor = 0
	t.learned = nil
	t.mu.Unlock()
}

// learn 记住本轮认出来的角色。
func (t *wuwaTeam) learn(ids []string, now time.Time) {
	found := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		found = append(found, id)
	}
	if len(found) == 0 {
		return
	}
	t.mu.Lock()
	t.learned = found
	t.learnedAt = now
	t.mu.Unlock()
}

func (t *wuwaTeam) Slots(rt combat.Runtime, frame assist.Frame) ([]string, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	candidates, learning := t.candidates(now)
	ids := make([]string, 0, len(t.avatarSlots))
	for _, area := range t.avatarSlots {
		best, bestID := 0.0, ""
		for _, id := range candidates {
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
	if learning {
		t.learn(ids, now)
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
