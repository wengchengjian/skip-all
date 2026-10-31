# skip-all

跳过游戏里浪费时间的剧情、CG、对话和重复操作。

当前版本 v0.1.0 只实现《绝区零》的剧情/对话/CG 跳过；核心按“游戏适配器 + 功能（Feature）”分层，后续接入新游戏不需要改运行时框架。

## 现在能做什么

- 检测右上角“菜单 / 跳过 / 自动”，自动点击“跳过”和“确认”
- 普通对话自动推进，按配置选择第一个或最后一个选项
- 支持 skip、auto、click 三种剧情模式
- 进入战斗、勘域、二级菜单、玩法引导时自动避让，不误点
- 默认后台运行：PrintWindow 截图 + PostMessage 点击，不抢焦点、绝不移动前台鼠标
- 精确跳过开关：`input.cursor_mode` 默认 `auto`（用户空闲时短暂移动并归位）；`never` 绝不动鼠标但只能推进对话，`always` 总是移动
- 可选前台模式（SendInput），兼容个别忽略后台消息的游戏
- 支持 ViGEm 虚拟 Xbox 手柄（需 ViGEmBus 驱动），用于手柄按键
- F10 急停；停止时释放所有键鼠状态
- 内置 OCR 模型检查与下载，不依赖 Python、OpenCV 或游戏内存
- 设置保存后立即生效；运行中的会话会用新配置自动重启
- 启动即自动检测并执行全部未排除游戏；设置页可排除游戏
- F9 暂停/继续（空闲时开始全部），F10 全部停止
- 顶栏实时显示 FPS 和 OCR 后端（DirectML / CUDA / CPU）
- 日志打印剧情流程、动作类型和目标坐标
- 跳过/点击模式用白色文字行投影定位按钮、选项和对话框，不逐帧跑 OCR；默认 5 FPS，CPU 明显低于 OCR 方案

v0.1 不包含钓鱼、短信、空洞事件、自动战斗和日常任务。这些会作为后续 Feature 逐步加入。

## 快速开始

环境要求：

- Windows 10/11 x64
- Go 1.26+
- Node.js 24+、pnpm 12+
- Wails v3 CLI：go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

拉取 OCR 模型（约 46MB）：

```powershell
./scripts/fetch-models.ps1
```

开发运行：

```powershell
wails3 dev
```

构建发布包：

```powershell
./scripts/build.ps1
```

构建产物在 bin/，包含主程序、onnxruntime.dll、模型目录和 ViGEmClient.dll。运行前确认游戏为窗口化或无边框窗口、逻辑分辨率 16:9；独占全屏可能无法截图。程序启动后会自动扫描所有未排除游戏的窗口，游戏稍后启动也会在后台继续等待。

需要虚拟手柄时，先安装 [ViGEmBus](https://github.com/nefarius/ViGEmBus)，在设置中打开“启用虚拟 Xbox 手柄”。ViGEmBus 已停止维护，但本机已有可用驱动；没有驱动时后台鼠标模式仍可正常工作。

## 使用

1. 启动《绝区零》，进入游戏窗口。
2. 启动 skip-all，选择“绝区零 → 剧情跳过”。
3. 点击开始（或按 F9）。
4. 播放剧情时程序会自动点击跳过/确认/对话选项；按 F10 或点击停止结束。

CLI 调试：

```powershell
skip-all list
skip-all doctor --game zzz
skip-all run --game zzz --feature story_skip --headless
skip-all models install
```

doctor 会输出窗口、截图、缩放、OCR 模型和识别结果，是排查画面问题的第一入口。

## 架构

```text
cmd/skip-all            GUI / CLI 单一入口
internal/app            应用服务（配置、游戏注册、会话管理）
internal/engine         会话循环、暂停/恢复/急停、动作执行
internal/platform        跨平台门面；Windows 下提供窗口、后台截图、后台输入与 ViGEm 手柄
pkg/assist              公共适配器接口（Game/Feature/Session/Runtime）
pkg/assist/storyskip    通用剧情跳过状态机
pkg/vision              OCR、颜色、模板匹配、LCS
pkg/screen              画面 YAML 模型
games/zzz               绝区零适配器与资源
frontend                Vue 3 + TypeScript 界面
```

OCR 模型与 GPU 方案：阅读 [docs/ocr.md](docs/ocr.md)。

新增游戏：阅读 [docs/adding-a-game.md](docs/adding-a-game.md)。如果游戏也是“右上角菜单/跳过/确认 + 对话框”的同构 UI，通常只需要新增 games/<id> 和画面 YAML，不用改核心。

## 跨平台

- **Windows**：完整功能，后台截图/后台输入/ViGEm 虚拟手柄。
- **macOS / Linux**：核心、CLI、画面/OCR/配置层可编译运行；屏幕与输入平台层目前返回“暂不支持”，后续按平台实现。
- `internal/platform` 是跨平台门面，`internal/platform/windows` 只在 Windows 编译；新增平台只需实现同一组 `Window/Capturer/Input` 接口。
- GUI 当前优先 Windows；非 Windows 构建会落到 CLI 入口（`main_other.go`）。

## 开发

```powershell
go test ./...
go vet ./...
wails3 build
```

测试中的 OCR、截屏、输入均通过接口注入 fake，不依赖真实游戏窗口；真实《绝区零》的验收清单见 [docs/testing.md](docs/testing.md)。

## 许可证

本项目采用 GPL-3.0。部分行为、画面定义和模板资源移植自 [OneDragon-Anything/ZenlessZoneZero-OneDragon](https://github.com/OneDragon-Anything/ZenlessZoneZero-OneDragon)（提交 cc3e80f3eacbbe11df5af06590cff095d95553a6），详见 [NOTICE](NOTICE) 与 [docs/upstream.md](docs/upstream.md)。第三方依赖见 [docs/third-party.md](docs/third-party.md)。
