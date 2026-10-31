# 测试与验收

## 自动化

```powershell
go test ./...
go vet ./...
```

重点测试：

- `pkg/vision`：LCS、gray-mask、colorful、模板匹配（合成图）。
- `pkg/screen`：加载上游 `commission_assistant.yml` 后区域坐标/类型正确。
- `pkg/assist/storyskip`：FakeRuntime + 脚本化帧覆盖 8 个状态场景。
- `internal/engine`：开始/暂停/恢复/停止、动作执行、异常退出释放输入。
- `internal/platform/windows`：坐标缩放/客户区换算纯函数；真实 Win32 测试无窗口时跳过。

## 后台输入与手柄

- 后台模式：游戏窗口失焦时仍能截图；`cursor_mode=never` 下点击不改变物理鼠标位置、不抢占前台。
- 精确点击：`cursor_mode=auto/always` 时用独立测试窗口验证目标命中；归位只在用户空闲时验证，避免测试脚本抢前台输入。
- 前台回退：设置 `input.mode=foreground` 后，点击前会激活游戏窗口。
- 手柄：确认 `ViGEmBus` 服务运行、`ViGEmClient.dll` 可加载；虚拟手柄能创建/移除，`xbox_a` 等按键可发送。
- 跨平台：`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` 与 `GOOS=darwin ...` 通过；非 Windows 平台只提供 CLI 与核心能力。

## 真实《绝区零》验收

前置：游戏窗口化或无边框、16:9、关闭 HDR/缩放干扰；以管理员身份运行（如果游戏也是管理员）。

- [ ] 主线剧情出现“跳过”后 0.5 秒内点击，出现“确认”后点击。
- [ ] 纯对话按 0.5 秒间隔推进。
- [ ] 对话选项按“第一个/最后一个”配置选择。
- [ ] CG/过场有跳过按钮时正常跳过。
- [ ] 战斗、勘域、二级菜单、玩法引导中 60 秒内 0 点击。
- [ ] ALT-TAB 暂停、回游戏恢复。
- [ ] F10 急停，鼠标/键盘无残留按下。
- [ ] 移动/缩放窗口后坐标仍准确。
- [ ] 游戏退出后程序安全停止并提示。
- [ ] 删除 OCR 模型后启动会提示下载而不是崩溃。

## 性能观察

开发机记录 benchmark 趋势，不把单次数值当硬承诺：

- 1080p PrintWindow 截图目标 <50ms
- 单区域模板匹配目标 <20ms
- 小区域 OCR 目标 <150ms
- 整体循环目标 10 FPS

## 调试入口

```powershell
skip-all doctor --game zzz
skip-all doctor --game zzz --dump-frame
```

`--dump-frame` 会把当前截图、各区域框、OCR 结果保存到 `%APPDATA%\skip-all\debug\`，用于游戏更新后的画面修复。
