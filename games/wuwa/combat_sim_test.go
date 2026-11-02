package wuwa

import (
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"math"
	"os"
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/combat"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// simGame 是自动战斗的模拟对局：记录队伍、当前角色、协奏值和技能可用状态，
// 收到会话发出的动作后按"游戏规则"推进状态。
// 这样测试只依赖我们自己写的识别与决策，不依赖真实游戏或真实模板匹配。
type simGame struct {
	team     []string // 三个槽位的角色 id
	current  int
	concerto float64 // 0..1
	skill    bool
	echo     bool
	lib      bool
	forte    bool
	targeted bool
	// cd 是各技能剩余冷却帧数。
	cdSkill, cdEcho, cdLib int
	// 每帧递增，冷却与观察用。
	frame int
	// 动作记录：每帧会话给出的动作类型与按键。
	actions []simAction
	// 当前角色的技能被放过几次，用来发现"一直普攻不出招"。
	skillUses  int
	attackUses int
	switches   int
}

type simAction struct {
	frame  int
	state  string
	kind   assist.ActionKind
	key    string
	slot   int
	filled float64
}

func newSimGame(team ...string) *simGame {
	return &simGame{team: team, skill: true, echo: true, lib: true, targeted: true}
}

func (g *simGame) agent() string {
	if g.current < len(g.team) {
		return g.team[g.current]
	}
	return ""
}

// apply 按会话给出的动作推进模拟对局。
func (g *simGame) apply(action assist.Action, state string) {
	g.frame++
	entry := simAction{frame: g.frame, state: state, kind: action.Kind, key: action.Key, slot: g.current, filled: g.concerto}
	g.actions = append(g.actions, entry)
	switch action.Kind {
	case assist.ActionKeyTap:
		switch action.Key {
		case "e":
			if g.skill {
				g.skillUses++
				g.skill = false
				g.cdSkill = 80 // 8 秒
				g.concerto = clamp(g.concerto+0.35, 0, 1)
			}
		case "q":
			if g.echo {
				g.echo = false
				g.cdEcho = 200
				g.concerto = clamp(g.concerto+0.15, 0, 1)
			}
		case "r":
			if g.lib {
				g.lib = false
				g.cdLib = 250
				g.concerto = clamp(g.concerto+0.25, 0, 1)
			}
		case "1", "2", "3":
			g.switchTo(int(action.Key[0]-'1') - 0)
		}
	case assist.ActionClick, assist.ActionClickHold:
		g.attackUses++
		g.concerto = clamp(g.concerto+0.05, 0, 1)
	case assist.ActionMiddleClick:
		g.targeted = true
	}
	g.tick()
}

func (g *simGame) switchTo(slot int) {
	if slot < 0 || slot >= len(g.team) || slot == g.current {
		return
	}
	g.switches++
	g.current = slot
	// 切人后协奏值清零，新角色技能全可用。
	g.concerto = 0
	g.skill, g.echo, g.lib = true, true, true
	g.cdSkill, g.cdEcho, g.cdLib = 0, 0, 0
	g.forte = false
}

func (g *simGame) tick() {
	if g.cdSkill > 0 {
		g.cdSkill--
		if g.cdSkill == 0 {
			g.skill = true
		}
	}
	if g.cdEcho > 0 {
		g.cdEcho--
		if g.cdEcho == 0 {
			g.echo = true
		}
	}
	if g.cdLib > 0 {
		g.cdLib--
		if g.cdLib == 0 {
			g.lib = true
		}
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// simRuntime 把模拟对局翻译成会话要的画面与模板命中。
// 模板匹配的答案来自模拟对局的状态（头像/键位数字/目标锁定），
// 颜色判据走真实像素，和线上同一套视觉代码。
type simRuntime struct {
	game *simGame
}

func (r simRuntime) Screen(name string) (*screen.Screen, error) {
	area := &screen.Area{Name: "中断提示", Rect: geom.Rect{X1: 520, Y1: 840, X2: 1850, Y2: 1060}}
	return screen.NewScreen(name, area), nil
}

func (r simRuntime) OCR(assist.Frame, screen.Area, vision.OCROptions) ([]vision.TextBlock, error) {
	return nil, nil
}

func (r simRuntime) MatchTemplate(_ assist.Frame, area screen.Area, ref vision.TemplateRef, _ float64) ([]vision.Match, error) {
	g := r.game
	hit := func() ([]vision.Match, error) {
		return []vision.Match{{Rect: area.Rect, Confidence: 0.95}}, nil
	}
	miss := func() ([]vision.Match, error) { return nil, nil }
	if ref.SubDir == "team" {
		switch ref.ID {
		case "char_1_text", "char_2_text", "char_3_text":
			slot := int(ref.ID[5] - '1')
			if slot != g.current {
				return hit()
			}
			return miss()
		}
		for i, s := range newWuwaTeam().avatarSlots {
			if area.Rect == s.Rect {
				if g.team[i] == ref.ID {
					return hit()
				}
				return miss()
			}
		}
		return miss()
	}
	switch ref.ID {
	case "has_target":
		if g.targeted {
			return hit()
		}
		return miss()
	case "no_target":
		if !g.targeted {
			return hit()
		}
		return miss()
	case "e_forte":
		if g.forte {
			return hit()
		}
		return miss()
	}
	return miss()
}

func (r simRuntime) ColorStats(assist.Frame, screen.Area) (vision.ColorStats, error) {
	return vision.ColorStats{}, nil
}

func (r simRuntime) Log() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// simFrame 按模拟对局状态画一帧：敌人血条、协奏环、技能按钮高亮。
func simFrame(g *simGame, spec combat.Spec) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	// 敌人血条：宽 200、高 8，填充率 1.0。
	for y := 100; y < 108; y++ {
		for x := 800; x < 1000; x++ {
			img.SetRGBA(x, y, color.RGBA{200, 70, 65, 255})
		}
	}
	// 协奏环：按 concerto 比例画弧。
	if spec.Concerto != nil {
		ring := spec.Concerto.Rect
		h := float64(ring.Y2 - ring.Y1)
		cx, cy := float64(ring.X2-ring.X1)/2, h/2
		for y := 0; y < ring.Y2-ring.Y1; y++ {
			for x := 0; x < ring.X2-ring.X1; x++ {
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
				d := math.Hypot(dx, dy)
				if d < spec.Concerto.Inner*h || d > spec.Concerto.Outer*h {
					continue
				}
				frac := (math.Atan2(dy, dx) + math.Pi) / (2 * math.Pi)
				if frac <= g.concerto {
					img.SetRGBA(ring.X1+x, ring.Y1+y, color.RGBA{80, 165, 230, 255})
				}
			}
		}
	}
	// 技能按钮高亮。
	if check, ok := spec.Ready[combat.StepSkill]; ok && g.skill {
		fillRect(img, check.Area, color.RGBA{255, 255, 255, 255})
	}
	if check, ok := spec.Ready[combat.StepEcho]; ok && g.echo {
		fillRect(img, check.Area, color.RGBA{255, 255, 255, 255})
	}
	if check, ok := spec.Ready[combat.StepLiberation]; ok && g.lib {
		fillRect(img, check.Area, color.RGBA{255, 255, 255, 255})
	}
	if spec.ForteFull != nil && g.forte {
		fillRect(img, spec.ForteFull.Area, color.RGBA{252, 252, 252, 255})
	}
	return img
}

func fillRect(img *image.RGBA, r geom.Rect, c color.RGBA) {
	for y := r.Y1; y < r.Y2; y++ {
		for x := r.X1; x < r.X2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// runSim 用真实鸣潮配置跑一段模拟对局，返回记录。
func runSim(t *testing.T, g *simGame, frames int) {
	t.Helper()
	feature := CombatFeature()
	session, err := feature.NewSession(simRuntime{game: g}, feature.DefaultSettings())
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	defer session.Close()
	spec := combatSpec()
	base := time.Now()
	interval := 100 * time.Millisecond // 自动战斗的 MinFPS 是 10
	for i := 0; i < frames; i++ {
		frame := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * interval)}
		result, actions, err := session.Step(nil, frame)
		if err != nil {
			t.Fatalf("第 %d 帧出错: %v", i, err)
		}
		if len(actions) == 0 {
			g.frame++
			g.actions = append(g.actions, simAction{frame: g.frame, state: result.State, slot: g.current, filled: g.concerto})
			g.tick()
			continue
		}
		for _, action := range actions {
			g.apply(action, result.State)
		}
	}
}

// summarise 把一段模拟对局压缩成"状态变化 + 动作计数"，失败时打印。
func summarise(g *simGame) string {
	var b []string
	last := ""
	for _, a := range g.actions {
		if a.state == last {
			continue
		}
		last = a.state
		b = append(b, fmt.Sprintf("f%03d slot%d con=%.2f %s", a.frame, a.slot, a.filled, a.state))
	}
	keys := map[string]int{}
	for _, a := range g.actions {
		if a.kind == assist.ActionKeyTap {
			keys[a.key]++
		}
	}
	return fmt.Sprintf("切人=%d 技能=%d 普攻=%d 按键=%v\n%s",
		g.switches, g.skillUses, g.attackUses, keys, joinAll(b))
}

func joinAll(s []string) string {
	out := ""
	for i, line := range s {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}
