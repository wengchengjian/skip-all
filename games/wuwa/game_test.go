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
	if len(game.Features()) != 1 || game.Features()[0].ID() != "story_skip" {
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
	for _, area := range []string{"剧情跳过", "对话框确认", "对话内容", "右侧选项区域", "中间选项区域"} {
		if _, ok := story.Area(area); !ok {
			t.Fatalf("鸣潮剧情 缺少区域 %s", area)
		}
	}
	skip, ok := story.Area("剧情跳过")
	if !ok || skip.TemplateID == "" || skip.TemplateSubDir != "story" {
		t.Fatalf("剧情跳过 应是模板区域: %+v", skip)
	}
}

func TestStorySkipSpecUsesTextDialog(t *testing.T) {
	spec := storySkipSpec()
	if spec.DialogStyle != storyskip.DialogText {
		t.Fatalf("DialogStyle = %v", spec.DialogStyle)
	}
	if len(spec.StorySkipAreas) == 0 {
		t.Fatal("缺少剧情跳过模板区域")
	}
}
