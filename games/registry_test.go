package games

import "testing"

func TestRegistry(t *testing.T) {
	all := All()
	if len(all) != 2 || all[0].ID() != "zzz" || all[1].ID() != "wuwa" {
		t.Fatalf("All() = %v", all)
	}
	game, ok := Get("zzz")
	if !ok || game.Name() != "绝区零" {
		t.Fatalf("Get(zzz) = %v, %v", game, ok)
	}
	if game, ok := Get("wuwa"); !ok || game.Name() != "鸣潮" {
		t.Fatalf("Get(wuwa) = %v, %v", game, ok)
	}
	if _, ok := Get("unknown"); ok {
		t.Fatal("未知游戏不应存在")
	}
}
