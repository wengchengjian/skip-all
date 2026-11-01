package wuwa

import (
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/storyskip"
	"github.com/wengchengjian/skip-all/pkg/screen"
)

func TestGameMetadata(t *testing.T) {
	game := New()
	if game.ID() != "wuwa" {
		t.Fatalf("ID = %q", game.ID())
	}
	if game.Name() != "鸣潮" {
		t.Fatalf("Name = %q", game.Name())
	}
	if len(game.WindowTitles()) == 0 || game.WindowTitles()[0] != "鸣潮" {
		t.Fatalf("WindowTitles = %v", game.WindowTitles())
	}
	if len(game.ProcessNames()) != 1 || game.ProcessNames()[0] != "Client-Win64-Shipping.exe" {
		t.Fatalf("ProcessNames = %v", game.ProcessNames())
	}
	ids := map[string]bool{}
	for _, feature := range game.Features() {
		ids[feature.ID()] = true
	}
	if !ids["story_skip"] || !ids["auto_combat"] {
		t.Fatalf("Features = %v", game.Features())
	}
}

func TestAssetsLoadStoryScreen(t *testing.T) {
	game := New()
	set, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面失败: %v", err)
	}
	story, ok := set.Get("鸣潮剧情")
	if !ok {
		t.Fatal("缺少 鸣潮剧情 画面")
	}
	for _, area := range []string{"剧情跳过", "剧情跳过-对话", "对话框确认", "对话内容", "右侧选项区域", "中间选项区域"} {
		if _, ok := story.Area(area); !ok {
			t.Fatalf("鸣潮剧情 缺少区域 %s", area)
		}
	}
	for name, id := range map[string]string{"剧情跳过": "skip", "剧情跳过-对话": "skip_dialog"} {
		area, ok := story.Area(name)
		if !ok || area.TemplateID != id || area.TemplateSubDir != "story" {
			t.Fatalf("%s 应是模板区域 %s: %+v", name, id, area)
		}
	}
}

func TestStorySkipSpecUsesTextDialog(t *testing.T) {
	spec := storySkipSpec()
	if _, ok := spec.Mode.(storyskip.TextMode); !ok {
		t.Fatalf("Mode = %T, want storyskip.TextMode", spec.Mode)
	}
	if len(spec.StorySkipAreas) == 0 {
		t.Fatal("缺少剧情跳过模板区域")
	}
}

func TestAssetsLoadCombatScreen(t *testing.T) {
	game := New()
	set, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面失败: %v", err)
	}
	combat, ok := set.Get("鸣潮战斗")
	if !ok {
		t.Fatal("缺少 鸣潮战斗 画面")
	}
	area, ok := combat.Area("敌人血条")
	if !ok {
		t.Fatal("缺少 敌人血条 区域")
	}
	if area.Rect.X2 <= area.Rect.X1 || area.Rect.Y2 <= area.Rect.Y1 {
		t.Fatalf("敌人血条 区域未解析: %+v", area.Rect)
	}
}

func TestCombatSpecUsesUpstreamKeys(t *testing.T) {
	spec := combatSpec()
	if spec.Keys.Skill != "e" || spec.Keys.Echo != "q" || spec.Keys.Liberation != "r" || spec.Keys.Dodge != "lshift" {
		t.Fatalf("按键绑定 = %+v", spec.Keys)
	}
	if spec.HealthBarArea == "" || spec.HealthBarRange != ([2][3]uint8{{174, 55, 55}, {225, 85, 76}}) {
		t.Fatalf("血条证据 = %q %v", spec.HealthBarArea, spec.HealthBarRange)
	}
}
