package zzz

import (
	"io/fs"
	"sort"
	"sync"
	"time"

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
	// frontLearned/backLearned 是本场认出来的代理人模板白名单，认出来后只比这几张。
	mu           sync.Mutex
	frontLearned []string
	backLearned  []string
	learnedAt    time.Time
	// scanCursor 让"全量认人"分摊到多帧，避免一次扫 71 张头像卡住战斗循环。
	scanCursor int
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

// zzzTeamRefreshTTL 之后重新全量认一次。
const zzzTeamRefreshTTL = 10 * time.Second

// scanBatch 是一次最多比对的模板数量。71 张头像全扫约 225ms，
// 摊到 8 张/帧后每帧约 25ms，1 秒内能扫完一轮。
const scanBatch = 8

// candidates 取某个模板族这一轮要比对的 id；learning=true 表示这次是全量扫描的一部分。
func (t *zzzTeam) candidates(now time.Time, subDir string) ([]string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fresh := len(t.frontLearned) > 0 && now.Sub(t.learnedAt) < zzzTeamRefreshTTL
	if fresh {
		if subDir == "avatar1" {
			return t.frontLearned, false
		}
		if len(t.backLearned) > 0 {
			return t.backLearned, false
		}
	}
	// 还没认全：本帧只比一批，剩下的下一帧继续。
	all := t.frontIDs
	if subDir == "avatar2" {
		all = t.backIDs
	}
	if len(all) == 0 {
		return nil, false
	}
	if t.scanCursor >= len(all) {
		t.scanCursor = 0
	}
	end := t.scanCursor + scanBatch
	if end > len(all) {
		end = len(all)
	}
	batch := all[t.scanCursor:end]
	t.scanCursor = end
	if t.scanCursor >= len(all) {
		// 一轮扫完：用认出来的角色当白名单。
		t.learnedAt = now
	}
	return batch, true
}

// LearnedIDs 返回已经认出来的代理人模板（供连携选边等场景复用，避免再扫 71 张）。
func (t *zzzTeam) LearnedIDs() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := make([]string, 0, len(t.frontLearned)+len(t.backLearned))
	ids = append(ids, t.frontLearned...)
	ids = append(ids, t.backLearned...)
	return ids
}

// learn 记住某个模板族本轮认出来的 id。
func (t *zzzTeam) learn(now time.Time, subDir, id string) {
	if id == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if subDir == "avatar1" {
		t.frontLearned = []string{id}
	} else {
		found := append([]string(nil), t.backLearned...)
		exists := false
		for _, existing := range found {
			if existing == id {
				exists = true
				break
			}
		}
		if !exists {
			found = append(found, id)
		}
		t.backLearned = found
	}
	t.learnedAt = now
}

// matchIn 在一个区域里逐个模板比对，返回分数最高的模板 id。
func (t *zzzTeam) matchIn(rt combat.Runtime, frame assist.Frame, area screen.Area, ids []string, subDir string) (string, error) {
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
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
	if bestID != "" {
		t.learn(now, subDir, bestID)
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
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	ids, learning := t.candidates(now, "avatar1")
	id, err := t.matchIn(rt, frame, t.front, ids, "avatar1")
	if err != nil {
		return "", false, err
	}
	if id == "" && !learning {
		// 白名单里没有：换人了，作废白名单，从当前游标继续分批认。
		t.invalidate()
		ids, _ = t.candidates(now, "avatar1")
		if id, err = t.matchIn(rt, frame, t.front, ids, "avatar1"); err != nil {
			return "", false, err
		}
	}
	return id, id != "", nil
}

// invalidate 作废白名单，让下一次 candidates 重新分批扫描。
func (t *zzzTeam) invalidate() {
	t.mu.Lock()
	t.learnedAt = time.Time{}
	t.scanCursor = 0
	t.mu.Unlock()
}

// Slots 列出前后台槽位上的代理人 id（认不出留空），切人打分用。
