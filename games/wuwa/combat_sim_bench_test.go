package wuwa

import (
	"testing"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// BenchmarkSimFrameBuild 量"造一帧"的代价，用来把测试开销和决策开销分开。
func BenchmarkSimFrameBuild(b *testing.B) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	g.concerto = 0.6
	spec := combatSpec()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = simFrame(g, spec)
	}
}

// BenchmarkSimCombatStepOnly 只量 Step 本身（帧预先造好）。
func BenchmarkSimCombatStepOnly(b *testing.B) {
	g := newSimGame("char_hiyuki", "char_linnai", "char_suisui")
	feature := CombatFeature()
	session, err := feature.NewSession(simRuntime{game: g}, feature.DefaultSettings())
	if err != nil {
		b.Fatal(err)
	}
	defer session.Close()
	spec := combatSpec()
	base := time.Now()
	frames := make([]*assist.Frame, 0, 64)
	for i := 0; i < 64; i++ {
		f := assist.Frame{Image: simFrame(g, spec), At: base.Add(time.Duration(i) * 100 * time.Millisecond)}
		frames = append(frames, &f)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := frames[i%len(frames)]
		result, actions, err := session.Step(nil, *f)
		if err != nil {
			b.Fatal(err)
		}
		for _, a := range actions {
			g.apply(a, result.State)
		}
	}
}
