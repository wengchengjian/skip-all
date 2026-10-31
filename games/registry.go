// Package games 提供内置游戏注册表。
package games

import (
	"github.com/wengchengjian/skip-all/games/zzz"
	"github.com/wengchengjian/skip-all/pkg/assist"
)

// All 返回全部内置游戏。
func All() []assist.Game {
	return []assist.Game{zzz.New()}
}

// Get 按游戏标识查找内置游戏。
func Get(id string) (assist.Game, bool) {
	for _, game := range All() {
		if game.ID() == id {
			return game, true
		}
	}
	return nil, false
}
