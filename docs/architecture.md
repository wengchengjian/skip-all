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
- `assist.Feature`：一个可执行功能，负责校验配置并创建 `assist.Session`；同一个游戏可以同时启用多个功能（剧情跳过 + 自动战斗），由 engine 用同一帧截图驱动。
- `assist.Session`：每帧执行一次 `Step(ctx, frame)`，返回状态、等待时长和 `[]assist.Action`。
- `assist.Runtime`：Feature 只能通过它访问画面配置和视觉服务，不直接接触 Win32。
- `assist.Action`：`Click`、`ClickCurrent`、`KeyTap`、`Wait`，由 engine 统一执行。

这样做的目的：状态机可以在没有游戏窗口的情况下用 FakeRuntime 做单元测试，也可以把点击/截图替换成任意平台实现。

## 一帧的生命周期

1. `engine.Run` 按目标 FPS 唤醒。
2. 如果游戏进程不在（`Config.ProcessAlive`）、暂停、窗口不在前台且配置了后台暂停，则跳过本帧。
3. `platform/windows.Capturer.Capture()` 截图并缩放到 1920×1080，返回 RGBA 和缩放系数。
4. engine 构造 `assist.Frame`，按功能声明顺序逐个调 `Session.Step`（同一帧复用，OCR/模板结果按帧缓存）。
5. 每个 Session 通过 Runtime 调用 OCR/模板/颜色服务，返回动作；engine 立即按顺序执行，动作天然串行。
6. 单个功能出错只停该功能；全部功能都出错才让会话进入 error。
7. UI 通过事件收到状态和最近日志。

## 多功能并行与进程门控

- 一个游戏一个 engine，engine 内可挂多个 Feature（`engine.FeatureSpec`）；目标帧率取各功能 `MinFPS` 的最大值。
- 每帧只截图一次，避免两个功能各自截图导致重复开销；动作按功能顺序串行执行，输入不会并发。
- 游戏进程退出时 engine 自动暂停（不再截图，避免连续失败把会话打成 error），进程回来后自动恢复；用户手动暂停不会被进程恢复覆盖。
- 进程存活判定 `platform.ProcessAlive` 每 1 秒最多枚举一次进程表（服务层缓存）。

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
| games/wuwa | 鸣潮适配器、StorySkip Spec（字幕行 + 跳过图标）、内嵌资源 |
| assets/games/zzz、assets/games/wuwa | 画面 YAML 和模板图片 |
| frontend | Vue 3 + TypeScript 界面 |

## 关键设计决定

- 不把 OneDragon 的 Operation 图框架整体搬过来：v0.1 只需要一个顺序状态机，复杂流程以后按 Feature 扩展。
- 剧情跳过的"流程"只有一份（`pkg/assist/storyskip` 的 Step 状态机：确认框优先 → 点跳过 → 守卫 → 门控 → 推进），"画面判读"按 `DialogMode` 接口分开实现：`MaskMode` 给有灰底对话框的界面（绝区零），`TextMode` 给只有白色字幕的界面（鸣潮）。游戏侧只提供数据（区域/模板/关键字）和一个 Mode，新增游戏不需要改状态机。
- 画面坐标固定按 1920×1080 逻辑分辨率书写，平台层负责缩放到当前客户区，避免每个游戏重复适配分辨率。
- OCR 只识别小区域，且按帧缓存，避免每帧全屏识别。
- 默认后台：截图用 PrintWindow，点击用 PostMessage，不激活窗口、默认不移动物理鼠标。
- `cursor_mode`：`auto`（默认，用户空闲时短暂移动并归位）、`never`（纯消息）、`always`（总是移动）。部分游戏（如 ZZZ）只认真实光标位置，纯消息只能推进对话。
- 虚拟手柄通过 ViGEmBus + ViGEmClient.dll 创建 Xbox 360 手柄；后台鼠标点击仍走 PostMessage，手柄用于按键类动作。
- 服务按游戏维护独立 engine 会话（会话内含该游戏全部启用的功能）；启动时扫描全部未排除游戏，游戏窗口稍后出现会自动启动。
- 详情日志由 engine 输出“流程 + 动作 + 坐标”，Service 统一加游戏名前缀。
- 跳过/点击模式优先用白色文字行投影（FindTextBands）定位按钮、选项、确认框和对话框，避免逐帧 OCR；auto 模式保留 OCR 语义。
- OCR 后端按 DirectML → CUDA → CPU 探测，engine 每秒统计实际循环帧率并通过 Status.FPS 暴露给 UI。
- 启用虚拟 Xbox 手柄时，剧情跳过使用 `xbox_start`（+）打开菜单/跳过、`xbox_a`（A）选择和确认；该路径完全不移动光标。
- 剧情按钮先做白色高亮过滤；灰色“跳过”是禁用态，直接忽略，不点击。
- 对话流程顺序与上游一致：确认框 -> 白色跳过/菜单/自动 -> 5s 菜单静默 -> 守卫 -> 右侧选项 -> 中间选项或黑屏推进 -> 对话框正文 -> 对话后补点。
- 不修改游戏内存、不注入、不读包；只做屏幕识别和键鼠/手柄输入。

## 轻量模式与窗口生命周期

默认启动是轻量模式：只创建托盘图标，不创建任何窗口，常驻内存里不含 WebView2 渲染进程。

- 托盘菜单就是轻量模式的主界面：最上面是内置游戏列表（●/○ + 游戏名 + 未运行/已启动/运行中/已暂停/异常，数据来自 Service.GetTrayState），中间是功能开关（剧情跳过、自动战斗；一次只玩一个游戏，开关直接作用于全部游戏），下面是**一行运行控制**（状态 + Home 与下一步动作）、性能占用（CPU% 与工作集 MB，platform.ProcessUsage）和退出。菜单每 2 秒刷新一次。
- 打开主界面：双击或左键单击托盘图标，或点菜单里的"打开主界面"。
- 窗口预热：启动约 0.8 秒后在后台把主窗口建好并隐藏（prewarmMainWindow），第一次双击只剩 Show；实测点击到窗口出现从 234ms 降到 9ms。预热窗口 30 秒内没被打开同样按下面的规则回收。
- 关闭窗口：`WindowClosing` 钩子取消关闭、改为隐藏，立即回到托盘；隐藏后开始计时，延迟 30 秒（`windowReleaseDelay`）没有被再次打开就销毁窗口，释放 WebView2 进程和前端内存。
- 再次打开：窗口对象已销毁时重新创建，前端整页重新加载。运行状态、配置、日志都在 Go 侧（`internal/app` 的服务内存），重建窗口不丢任何状态。
- 托盘 tooltip 显示当前状态、游戏、帧率与内存（如 SkipAll · 鸣潮 运行中 · 5 FPS · 52MB），不打开窗口也能确认引擎是否在工作。
- 运行开关只有一个快捷键（默认 `Home`）：已停止/异常 → 启动，运行中 → 暂停，已暂停 → 继续。
- 状态用菜单里的彩色圆点和提示音表达，**不占用托盘图标**（托盘始终是程序图标）：绿=运行中、琥珀=已暂停、红=异常、蓝=游戏进程在但会话没起、灰=游戏没开（`platform.StatusDot` 生成 PNG，挂到菜单项图标）。状态变化时播系统提示音（`platform.Beep`，设置里可关）。
- 快捷键由 `platform/windows/hotkey.go` 的 `WH_KEYBOARD_LL` 键盘钩子实现，不用 `RegisterHotKey`：F12 这类系统保留键也能用，并且能在按键到达其它程序前判断该不该拦截。默认只在游戏窗口（或本程序窗口）位于前台时才吃掉按键，切到别的应用后 Home 照常给其它程序用。
- 功能开关快捷键（默认 F11 剧情跳过、F12 自动战斗）同样走这个钩子，按一次作用于全部游戏。
- 引擎会话与窗口生命周期完全解耦：`AppService` 的会话 goroutine 在后台独立运行，开窗、关窗、销毁窗口都不影响截图识别与点击，剧情跳过照常进行。
- 关窗不是退出；只有托盘菜单"退出"才结束进程（同时停会话、释放资源）。

