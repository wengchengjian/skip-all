package combat

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// Session 是自动战斗会话。
// 状态只有两个：在战斗里按连招出招、不在战斗里等。
type Session struct {
	rt         assist.Runtime
	spec       Spec
	cfg        sessionSettings
	cursor     int
	lastCombat time.Time
}

func newSession(rt assist.Runtime, spec Spec, cfg sessionSettings) *Session {
	return &Session{rt: rt, spec: spec, cfg: cfg}
}

// Step 执行一帧。
func (s *Session) Step(_ context.Context, frame assist.Frame) (assist.Result, []assist.Action, error) {
	if frame.Image == nil {
		return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, nil
	}
	now := frame.At
	if now.IsZero() {
		now = time.Now()
	}
	inCombat, err := s.inCombat(frame)
	if err != nil {
		return assist.Result{}, nil, err
	}
	if inCombat {
		s.lastCombat = now
	} else if s.lastCombat.IsZero() || now.Sub(s.lastCombat) > s.spec.Hold {
		// 证据会短暂丢失（转身、切人、特效遮挡），用保持时间把它们连起来。
		s.cursor = 0
		return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, nil
	}

	action, state, ok := s.nextAction()
	if !ok {
		return assist.Result{State: "等待战斗", Wait: s.spec.IdleWait}, nil, nil
	}
	return assist.Result{State: "连招-" + state, Wait: s.spec.Interval}, []assist.Action{action}, nil
}

// Close 释放会话。
func (s *Session) Close() error { return nil }

// inCombat 依次看模板证据与血条颜色证据，任一命中即认为在战斗。
func (s *Session) inCombat(frame assist.Frame) (bool, error) {
	if s.spec.HealthBarArea != "" {
		area, err := s.area(s.spec.HealthBarArea)
		if err != nil {
			return false, err
		}
		if ratioInRange(frame.Image, area.Rect, s.spec.HealthBarRange) >= s.spec.HealthBarRatio {
			return true, nil
		}
	}
	for _, name := range s.spec.DetectAreas {
		area, err := s.area(name)
		if err != nil {
			return false, err
		}
		if area.TemplateID == "" {
			continue
		}
		matches, err := s.rt.MatchTemplate(frame, *area, vision.TemplateRef{
			SubDir: area.TemplateSubDir,
			ID:     area.TemplateID,
		}, area.TemplateMatchThreshold)
		if err != nil {
			return false, fmt.Errorf("匹配战斗证据 %s: %w", name, err)
		}
		if len(matches) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// nextAction 按连招序列取下一条动作，跳过被设置关掉的动作。
func (s *Session) nextAction() (assist.Action, string, bool) {
	plan := s.spec.Plan
	if len(plan) == 0 {
		return assist.Action{}, "", false
	}
	for i := 0; i < len(plan); i++ {
		step := plan[s.cursor%len(plan)]
		s.cursor++
		switch step.Kind {
		case StepAttack:
			if s.spec.Keys.Attack != "" {
				return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Attack}, "普攻", true
			}
			return assist.Action{Kind: assist.ActionClick, Point: s.spec.AttackPoint}, "普攻", true
		case StepSkill:
			if s.spec.Keys.Skill == "" {
				continue
			}
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Skill}, "技能", true
		case StepEcho:
			if s.spec.Keys.Echo == "" || s.cfg.noEcho {
				continue
			}
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Echo}, "装备技能", true
		case StepLiberation:
			if s.spec.Keys.Liberation == "" || s.cfg.noLiberation {
				continue
			}
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Liberation}, "爆发", true
		case StepDodge:
			if s.spec.Keys.Dodge == "" || s.cfg.noDodge {
				continue
			}
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Dodge}, "闪避", true
		case StepSwitch:
			if step.Index < 0 || step.Index >= len(s.spec.Keys.Switch) {
				continue
			}
			return assist.Action{Kind: assist.ActionKeyTap, Key: s.spec.Keys.Switch[step.Index]}, "切换角色", true
		}
	}
	return assist.Action{}, "", false
}

func (s *Session) area(name string) (*screen.Area, error) {
	sc, err := s.rt.Screen(s.spec.Screen)
	if err != nil {
		return nil, fmt.Errorf("读取画面 %s: %w", s.spec.Screen, err)
	}
	area, ok := sc.Area(name)
	if !ok {
		return nil, fmt.Errorf("画面 %s 缺少区域 %s", s.spec.Screen, name)
	}
	return area, nil
}

// ratioInRange 统计区域里落在颜色范围内的像素比例。
// 上游用同样的思路判断敌人血条（enemy_health_color_red 的 RGB 区间）。
func ratioInRange(img *image.RGBA, rect geom.Rect, colorRange [2][3]uint8) float64 {
	if img == nil {
		return 0
	}
	b := img.Bounds()
	r := image.Rect(rect.X1, rect.Y1, rect.X2, rect.Y2).Intersect(b)
	if r.Empty() {
		return 0
	}
	hit, total := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			total++
			if c.R >= colorRange[0][0] && c.R <= colorRange[1][0] &&
				c.G >= colorRange[0][1] && c.G <= colorRange[1][1] &&
				c.B >= colorRange[0][2] && c.B <= colorRange[1][2] {
				hit++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total)
}
