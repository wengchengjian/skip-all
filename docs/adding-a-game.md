# 新增一个游戏

目标：新增游戏不修改 `pkg/` 和 `internal/engine`，只增加 `games/<id>` 和资源。

## 1. 建立游戏包

```text
games/<id>/
├─ game.go          // 实现 assist.Game
├─ story_skip.go    // 可选：装配通用 StorySkip Feature
└─ assets/
   ├─ screens/*.yml
   └─ templates/<sub>/<id>/{raw.png,mask.png}
```

`game.go` 至少实现：

```go
type Game struct{}
func (Game) ID() string { return "example" }
func (Game) Name() string { return "示例游戏" }
func (Game) WindowTitles() []string { return []string{"示例游戏"} }
func (Game) ProcessNames() []string { return []string{"Example.exe"} }
func (Game) Features() []assist.Feature { return []assist.Feature{storySkipFeature()} }
func (Game) Assets() fs.FS { return assetsFS }
```

然后在 `games/registry.go` 的 `All` 列表中加入这个 Game。

## 2. 画面 YAML

参考 `assets/games/zzz/screens/commission_assistant.yml`，字段：

- `screen_id` / `screen_name` / `pc_alt`
- `area_list[].area_name`
- `pc_rect: [x1, y1, x2, y2]`（1920×1080 逻辑坐标）
- `area_type: text | template | none`，可省略，由 `text`/`template_id` 推断
- 文本区域：`text`、`lcs_percent`、可选 `color_range`
- 模板区域：`template_sub_dir`、`template_id`、`template_match_threshold`

建议先写一个 `doctor` 能截图的画面，确认坐标后再填模板。

## 3. 模板图片

模板目录结构：

```text
assets/games/<id>/templates/<template_sub_dir>/<template_id>/raw.png
assets/games/<id>/templates/<template_sub_dir>/<template_id>/mask.png
```

`raw.png` 是裁剪截图；`mask.png` 白色区域参与匹配、黑色区域忽略。没有 mask 时可用全白图。

## 4. 复用 StorySkip

如果目标游戏的 UI 也是“右上角菜单/跳过/自动 + 对话框 + 选项”，直接构造 `storyskip.Spec`：

```go
spec := storyskip.Spec{
    Screen: "commission_assistant",
    StoryArea: "文本-剧情右上角",
    ConfirmArea: "对话框确认",
    DialogArea: "对话框内容",
    RightOptionArea: "右侧选项区域",
    MiddleOptionArea: "中间选项区域",
    Keywords: []string{"菜单", "跳过", "自动"},
    SkipKeyword: "跳过",
    MenuKeyword: "菜单",
    AutoKeyword: "自动",
    Guards: []storyskip.Guard{
        {Screen: "commission_assistant", Area: "左上角返回", Kind: storyskip.GuardTemplate},
    },
}
feature := storyskip.NewFeature(spec)
```

更复杂的游戏实现自己的 `assist.Feature`，参考 `pkg/assist/storyskip` 的 Step 结构。

## 5. 测试

- 用 `FakeRuntime` 返回预设 OCR 结果和模板命中，断言 `Step` 返回的 `[]assist.Action`。
- 至少覆盖：命中跳过、命中确认、选项选择、守卫不点击、连续空帧。
- 在 `skip-all doctor --game <id>` 中确认窗口、截图、OCR 能跑通。

## 6. 注册与验收

- 在 `games/registry.go` 注册。
- `go test ./games/... ./pkg/assist/storyskip/...`。
- 真实游戏里逐项验证：跳过/确认、对话、选项、误点守卫、暂停/急停。
