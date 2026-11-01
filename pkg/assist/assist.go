// Package assist 定义游戏自动化适配器接口。
package assist

import (
	"context"
	"image"
	"io/fs"
	"log/slog"
	"time"

	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// Settings 是功能的原始配置（来自 YAML）。
type Settings map[string]any

// Frame 是一帧标准化后的游戏画面。
type Frame struct {
	Image *image.RGBA
	At    time.Time
	Scale float64
}

// ActionKind 是动作类型。
type ActionKind int

const (
	ActionClick ActionKind = iota
	ActionClickCurrent
	ActionKeyTap
	ActionWait
)

// Action 是由 engine 执行的动作。
type Action struct {
	Kind     ActionKind
	Point    geom.Point
	Key      string
	Duration time.Duration
}

// Result 是一轮 Feature 执行的元信息。
type Result struct {
	State   string
	Wait    time.Duration
	Message string
}

// Session 是一个可执行的功能会话。
type Session interface {
	Step(ctx context.Context, frame Frame) (Result, []Action, error)
	Close() error
}

// Runtime 提供给 Feature 的只读运行时服务。
type Runtime interface {
	Screen(name string) (*screen.Screen, error)
	OCR(frame Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error)
	MatchTemplate(frame Frame, area screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error)
	ColorStats(frame Frame, area screen.Area) (vision.ColorStats, error)
	Log() *slog.Logger
}

// Feature 是一个可插拔功能。
type Feature interface {
	ID() string
	Name() string
	Description() string
	DefaultSettings() Settings
	Validate(Settings) error
	NewSession(Runtime, Settings) (Session, error)
}

// FrameRateFeature 是可选接口：功能需要比全局更高的截图帧率时实现它。
// 自动战斗这类要连续出招的场合靠它抬高帧率，其它功能不受影响。
type FrameRateFeature interface {
	MinFPS() int
}

// Game 是一个游戏适配器。
type Game interface {
	ID() string
	Name() string
	WindowTitles() []string
	ProcessNames() []string
	Features() []Feature
	Assets() fs.FS
	// RequiresForeground 报告该游戏的后台消息输入是否无效。
	// 返回 true 时运行时会把游戏窗口切到前台、使用真实键鼠输入。
	RequiresForeground() bool
}
