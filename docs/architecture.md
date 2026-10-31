# 架构

## 分层

```text
┌─────────────────────────────┐
│ Wails UI / Cobra CLI         │
├─────────────────────────────┤
│ internal/app  AppService     │  配置、游戏注册、启动/停止、事件
├─────────────────────────────┤
│ internal/engine              │  循环、暂停/恢复/急停、动作执行
├──────────────┬──────────────┤
│ pkg/assist   │ pkg/vision   │  功能接口 / OCR、模板、颜色
│ pkg/screen   │ internal/... │  画面配置 / Windows 平台
├──────────────┴──────────────┤
│ games/<id>  游戏适配器       │
└─────────────────────────────┘
```

## 核心接口

- `assist.Game`：游戏元数据（ID、名称、窗口标题、进程名、Features、资源 FS）。
- `assist.Feature`：一个可执行功能，负责校验配置并创建 `assist.Session`。
- `assist.Session`：每帧执行一次 `Step(ctx, frame)`，返回状态、等待时长和 `[]assist.Action`。
- `assist.Runtime`：Feature 只能通过它访问画面配置和视觉服务，不直接接触 Win32。
- `assist.Action`：`Click`、`ClickCurrent`、`KeyTap`、`Wait`，由 engine 统一执行。

这样做的目的：状态机可以在没有游戏窗口的情况下用 FakeRuntime 做单元测试，也可以把点击/截图替换成任意平台实现。

## 一帧的生命周期

1. `engine.Run` 按目标 FPS 唤醒。
2. 如果暂停、窗口不在前台且配置了后台暂停，则跳过本帧。
3. `platform/windows.Capturer.Capture()` 截图并缩放到 1920×1080，返回 RGBA 和缩放系数。
4. engine 构造 `assist.Frame` 调 `Session.Step`。
5. Session 通过 Runtime 调用 OCR/模板/颜色服务，返回动作。
6. engine 按顺序执行动作；任何一步失败都进入安全停止并释放键鼠。
7. UI 通过事件收到状态和最近日志。

## 线程模型

- 每个会话一个 goroutine，串行执行截图、识别、点击，保证不会并发操作鼠标。
- 视觉引擎内部有 OCR session 池，可并行识别同一帧内的多个文字框。
- UI/日志/模型下载在各自 goroutine，通过事件总线通信。
- 停止/急停通过 context 取消，engine 的 defer 会调用 `Input.ReleaseAll()`。

## 扩展点

- 新游戏：`games/<id>` 实现 `assist.Game`，在 `games/registry.go` 注册。
- 新功能：实现 `assist.Feature`，可以复用 `pkg/assist/storyskip` 或另写状态机。
- 新画面：在 `assets/games/<id>/screens/*.yml` 增加区域定义，不需要改 Go 代码。
- 新视觉算法：实现 `vision.OCR`/`vision.TemplateStore`，或替换 `vision.Matcher`。
- 新平台：实现 `engine.Device` 和 `engine.Input`，目前只有 Windows。

## 目录职责

| 目录 | 职责 |
| --- | --- |
| cmd/skip-all | CLI 命令与 GUI 启动分流 |
| internal/app | Wails 绑定、配置读写、游戏注册、会话管理 |
| internal/engine | 会话循环与动作执行 |
| internal/platform | 跨平台门面（Windows 完整实现，其他平台返回不支持） |
| internal/platform/windows | Win32 窗口/后台截图/后台输入/ViGEm 手柄 |
| internal/model | OCR 模型下载、校验、解压 |
| pkg/assist | 公共适配器接口 |
| pkg/assist/storyskip | 通用剧情跳过状态机 |
| pkg/screen | 画面 YAML 模型与加载 |
| pkg/vision | OCR/模板/颜色/LCS |
| games/zzz | 绝区零适配器、StorySkip Spec、内嵌资源 |
| assets/games/zzz | 画面 YAML 和模板图片 |
| frontend | Vue 3 + TypeScript 界面 |

## 关键设计决定

- 不把 OneDragon 的 Operation 图框架整体搬过来：v0.1 只需要一个顺序状态机，复杂流程以后按 Feature 扩展。
- 画面坐标固定按 1920×1080 逻辑分辨率书写，平台层负责缩放到当前客户区，避免每个游戏重复适配分辨率。
- OCR 只识别小区域，且按帧缓存，避免每帧全屏识别。
- 默认后台：截图用 PrintWindow，点击用 PostMessage，不激活窗口、不移动物理鼠标；需要前台时切 foreground 模式。
- 虚拟手柄通过 ViGEmBus + ViGEmClient.dll 创建 Xbox 360 手柄；后台鼠标点击仍走 PostMessage，手柄用于按键类动作。
- 不修改游戏内存、不注入、不读包；只做屏幕识别和键鼠/手柄输入。
