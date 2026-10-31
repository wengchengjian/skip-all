# AGENTS.md

本文件是 skip-all 的 AI 协作入口。只保留每次会话都要知道的约束。

## 项目

- 桌面游戏助手，Windows 优先、核心跨平台；Go 1.26 + Wails v3.0.0 + Vue 3 + TypeScript。
- v0.1 只做《绝区零》剧情/对话/CG 跳过；架构必须支持新增游戏不改核心。
- 默认后台模式：PrintWindow 截图 + PostMessage 输入，不抢焦点、不移动物理鼠标；foreground 是回退选项。
- 源码布局：cmd/、internal/、pkg/、games/、assets/games/、frontend/、docs/。

## 常用命令

```powershell
go test ./...
go vet ./...
wails3 dev
wails3 build
```

## 架构约束

- `pkg/assist` 是公共适配器接口；`internal/engine` 只依赖接口，不 import 具体游戏。
- 新游戏放 `games/<id>`，在 `games/registry.go` 注册；新功能实现 `assist.Feature`。
- 画面坐标统一按 1920×1080 逻辑分辨率；平台层负责缩放。
- 不读游戏内存、不注入进程、不修改游戏文件；只做屏幕识别与正常键鼠输入。
- 视觉算法/运行参数改 `pkg/vision` 或 `internal/platform/windows`；不要在 Feature 里直接调 Win32。

## 代码风格

- 中文注释，函数/导出类型写清楚用途；简洁优先，不为不可能场景加抽象。
- Go 用 `gofmt`；错误用 `%w` 包装；不要吞掉错误。
- 新增上游移植代码时更新 `docs/upstream.md` 与 `NOTICE`。

## 许可

- 本项目 GPL-3.0。上游移植部分必须保留归属；不要复制非 GPL/MIT/Apache 兼容代码或素材。
