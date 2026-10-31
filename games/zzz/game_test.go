package zzz

import (
	"io/fs"
	"testing"

	"github.com/wengchengjian/skip-all/pkg/assist/storyskip"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

func TestGameMetadata(t *testing.T) {
	game := New()
	if game.ID() != "zzz" {
		t.Fatalf("ID = %q", game.ID())
	}
	if game.Name() != "绝区零" {
		t.Fatalf("Name = %q", game.Name())
	}
	if len(game.WindowTitles()) == 0 || game.WindowTitles()[0] != "绝区零" {
		t.Fatalf("WindowTitles = %v", game.WindowTitles())
	}
	if len(game.ProcessNames()) != 1 || game.ProcessNames()[0] != "ZenlessZoneZero.exe" {
		t.Fatalf("ProcessNames = %v", game.ProcessNames())
	}
	if len(game.Features()) != 1 || game.Features()[0].ID() != "story_skip" {
		t.Fatalf("Features = %v", game.Features())
	}
}

func TestAssetsLoadScreens(t *testing.T) {
	game := New()
	set, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		t.Fatalf("加载画面失败: %v", err)
	}
	commission, ok := set.Get("委托助手")
	if !ok {
		t.Fatal("缺少 委托助手 画面")
	}
	for _, area := range []string{"对话框内容", "右侧选项区域", "中间选项区域", "对话框确认", "左上角返回", "战斗-菜单", "文本-剧情右上角", "按钮-自动"} {
		if _, ok := commission.Area(area); !ok {
			t.Fatalf("委托助手 缺少区域 %s", area)
		}
	}
	investigation, ok := set.Get("大世界-勘域")
	if !ok {
		t.Fatal("缺少 大世界-勘域 画面")
	}
	if _, ok := investigation.Area("勘域-菜单"); !ok {
		t.Fatal("大世界-勘域 缺少区域 勘域-菜单")
	}
	battle, ok := set.Get("战斗画面")
	if !ok {
		t.Fatal("缺少 战斗画面")
	}
	if _, ok := battle.Area("按键-交互"); !ok {
		t.Fatal("战斗画面 缺少区域 按键-交互")
	}
}

func TestAssetsTemplates(t *testing.T) {
	game := New()
	paths := []string{
		"templates/menu/back/raw.png",
		"templates/menu/back/mask.png",
		"templates/battle/btn_menu/raw.png",
		"templates/battle/btn_menu/mask.png",
		"templates/battle/btn_interact/raw.png",
		"templates/battle/btn_interact/mask.png",
		"templates/commission_assistant/1/raw.png",
		"templates/commission_assistant/1/mask.png",
		"templates/normal_world_investigation/menu/raw.png",
		"templates/normal_world_investigation/menu/mask.png",
	}
	for _, path := range paths {
		info, err := fs.Stat(game.Assets(), path)
		if err != nil {
			t.Fatalf("缺少模板 %s: %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("模板 %s 为空", path)
		}
	}
}

func TestAssetsTemplatesLoadable(t *testing.T) {
	store := vision.NewTemplateStoreFS(New().Assets(), "templates")
	refs := []vision.TemplateRef{
		{SubDir: "menu", ID: "back"},
		{SubDir: "battle", ID: "btn_menu"},
		{SubDir: "battle", ID: "btn_interact"},
		{SubDir: "commission_assistant", ID: "1"},
		{SubDir: "normal_world_investigation", ID: "menu"},
	}
	for _, ref := range refs {
		raw, mask, err := store.Load(ref)
		if err != nil {
			t.Fatalf("加载模板 %s/%s 失败: %v", ref.SubDir, ref.ID, err)
		}
		if raw == nil || raw.Bounds().Empty() {
			t.Fatalf("模板 %s/%s 原图为空", ref.SubDir, ref.ID)
		}
		if mask != nil && mask.Bounds().Size() != raw.Bounds().Size() {
			t.Fatalf("模板 %s/%s 掩码尺寸不一致", ref.SubDir, ref.ID)
		}
	}
}

func TestStorySkipFeatureDefaults(t *testing.T) {
	feature := StorySkipFeature()
	settings := feature.DefaultSettings()
	if settings[storyskip.SettingMode] != string(storyskip.ModeSkip) {
		t.Fatalf("mode = %v", settings[storyskip.SettingMode])
	}
	if err := feature.Validate(nil); err != nil {
		t.Fatalf("默认设置校验失败: %v", err)
	}
}
