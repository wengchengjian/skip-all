package screen

import (
	"testing"
	"testing/fstest"
)

func TestLoadFS(t *testing.T) {
	fsys := fstest.MapFS{
		"screens/normal.yml": &fstest.MapFile{Data: []byte(`
screen_id: normal_world
screen_name: 大世界
pc_alt: true
area_list:
  - area_name: 菜单
    pc_rect: [58, 40, 142, 110]
    template_sub_dir: battle
    template_id: btn_menu
  - area_name: 标题
    pc_rect: [100, 200, 300, 260]
    text: 剧情
    lcs_percent: 0.75
    color_range: [[0, 0, 0], [40, 40, 40]]
  - area_name: 点击区
    area_type: click
    pc_rect: [1, 2, 3, 4]
  - area_name: 旧OCR
    area_type: ocr
    pc_rect: [5, 6, 7, 8]
    text: 旧配置
`)},
		"screens/menu.yaml": &fstest.MapFile{Data: []byte(`
screen_id: menu
screen_name: 菜单
area_list:
  - area_name: 返回
    pc_rect: [0, 0, 10, 10]
    template_sub_dir: menu
    template_id: back
`)},
		"screens/ignore.txt": &fstest.MapFile{Data: []byte("not yaml")},
	}

	set, err := LoadFS(fsys, "screens")
	if err != nil {
		t.Fatalf("LoadFS 失败: %v", err)
	}

	sc, ok := set.Get("大世界")
	if !ok {
		t.Fatal("按名称未找到画面")
	}
	if same, ok := set.Get("normal_world"); !ok || same != sc {
		t.Fatal("按 ID 未找到画面")
	}
	if !sc.PCAlt {
		t.Error("pc_alt 未解析")
	}

	menu, _ := sc.Area("菜单")
	if menu == nil || menu.Type != AreaTemplate {
		t.Fatalf("模板区域类型推断错误: %+v", menu)
	}
	if menu.Rect != (struct{ X1, Y1, X2, Y2 int }{58, 40, 142, 110}) {
		t.Errorf("pc_rect 解析错误: %+v", menu.Rect)
	}
	if menu.TemplateMatchThreshold != 0.7 {
		t.Errorf("模板阈值默认值错误: %v", menu.TemplateMatchThreshold)
	}

	title, _ := sc.Area("标题")
	if title == nil || title.Type != AreaText {
		t.Fatalf("文本区域类型推断错误: %+v", title)
	}
	if title.LCSPercent != 0.75 {
		t.Errorf("lcs_percent 解析错误: %v", title.LCSPercent)
	}
	if title.ColorRange == nil || title.ColorRange[0] != [3]uint8{0, 0, 0} || title.ColorRange[1] != [3]uint8{40, 40, 40} {
		t.Errorf("color_range 解析错误: %+v", title.ColorRange)
	}

	click, _ := sc.Area("点击区")
	if click == nil || click.Type != AreaNone {
		t.Fatalf("click 别名应映射为 none: %+v", click)
	}
	old, _ := sc.Area("旧OCR")
	if old == nil || old.Type != AreaText {
		t.Fatalf("ocr 别名应映射为 text: %+v", old)
	}

	list := set.Screens()
	if len(list) != 2 || list[0].Name != "大世界" || list[1].Name != "菜单" {
		t.Fatalf("Screens 排序或数量错误: %+v", list)
	}
}

func TestLoadFSInvalidRect(t *testing.T) {
	fsys := fstest.MapFS{
		"a.yml": &fstest.MapFile{Data: []byte(`
screen_name: 坏画面
area_list:
  - area_name: 区域
    pc_rect: [1, 2, 3]
`)},
	}
	if _, err := LoadFS(fsys, "."); err == nil {
		t.Fatal("pc_rect 长度错误应返回错误")
	}
}

func TestAreaCanMatch(t *testing.T) {
	if (&Area{Type: AreaText}).CanMatch() {
		t.Error("空文本区域不应可匹配")
	}
	if !(&Area{Type: AreaText, Text: "确认"}).CanMatch() {
		t.Error("文本区域应可匹配")
	}
	if !(&Area{Type: AreaTemplate, TemplateID: "back"}).CanMatch() {
		t.Error("模板区域应可匹配")
	}
	if (&Area{Type: AreaNone}).CanMatch() {
		t.Error("none 区域不应可匹配")
	}
}
