// Package wuwa 提供鸣潮的游戏适配器。
package wuwa

import (
	"io/fs"

	wuwaassets "github.com/wengchengjian/skip-all/assets/games/wuwa"
	"github.com/wengchengjian/skip-all/pkg/assist"
)

// GameID 是鸣潮的游戏标识。
const GameID = "wuwa"

// Game 是鸣潮的游戏适配器。
type Game struct{}

var _ assist.Game = (*Game)(nil)

// New 创建鸣潮游戏适配器。
func New() *Game { return &Game{} }

// ID 返回游戏标识。
func (g *Game) ID() string { return GameID }

// Name 返回游戏名称。
func (g *Game) Name() string { return "鸣潮" }

// WindowTitles 返回游戏窗口标题。
func (g *Game) WindowTitles() []string { return []string{"鸣潮", "Wuthering Waves"} }

// ProcessNames 返回游戏进程名。
func (g *Game) ProcessNames() []string { return []string{"Client-Win64-Shipping.exe"} }

// Features 返回游戏支持的功能。
func (g *Game) Features() []assist.Feature {
	return []assist.Feature{StorySkipFeature()}
}

// Assets 返回内嵌的画面配置与模板资源。
func (g *Game) Assets() fs.FS { return wuwaassets.FS() }

// RequiresForeground 报告是否需要前台输入。
// 鸣潮的上游 ok-ww 用 PostMessage 后台点击即可工作，这里同样保持后台。
func (g *Game) RequiresForeground() bool { return false }
