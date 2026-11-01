// Package zzz 提供绝区零的游戏适配器。
package zzz

import (
	"io/fs"

	zzzassets "github.com/wengchengjian/skip-all/assets/games/zzz"
	"github.com/wengchengjian/skip-all/pkg/assist"
)

// GameID 是绝区零的游戏标识。
const GameID = "zzz"

// Game 是绝区零的游戏适配器。
type Game struct{}

var _ assist.Game = (*Game)(nil)

// New 创建绝区零游戏适配器。
func New() *Game { return &Game{} }

// ID 返回游戏标识。
func (g *Game) ID() string { return GameID }

// Name 返回游戏名称。
func (g *Game) Name() string { return "绝区零" }

// WindowTitles 返回游戏窗口标题。
func (g *Game) WindowTitles() []string { return []string{"绝区零", "ZenlessZoneZero"} }

// ProcessNames 返回游戏进程名。
func (g *Game) ProcessNames() []string { return []string{"ZenlessZoneZero.exe"} }

// Features 返回游戏支持的功能。
func (g *Game) Features() []assist.Feature {
	return []assist.Feature{StorySkipFeature(), CombatFeature()}
}

// Assets 返回内嵌的画面配置与模板资源。
func (g *Game) Assets() fs.FS { return zzzassets.FS() }

// RequiresForeground 报告是否需要前台输入。绝区零的后台消息点击可用，保持后台。
func (g *Game) RequiresForeground() bool { return false }
