# 第三方组件

| 组件 | 版本/来源 | 许可证 | 用途 |
| --- | --- | --- | --- |
| Wails | v3.0.0 | MIT | 桌面应用壳、系统托盘、全局快捷键 |
| Vue | 3.x | MIT | 前端界面 |
| Vite | 7.x | MIT | 前端构建 |
| go-ocr（本地 fork） | commit 3abab59da8147c88179b0a394449ce8fafd023bc | MIT | PaddleOCR 的 Go/ONNX 封装；`third_party/go-ocr` 增加 DirectML 配置 |
| onnxruntime_purego（本地 fork） | v0.0.0-20260118041137-401482b32507 | MIT | 纯 Go 调用 onnxruntime；`third_party/onnxruntime_purego` 增加 DirectML 入口 |
| PaddleOCR PP-OCRv6 模型 | OneDragon-Env release ppocrv6 | Apache-2.0 | 中文 OCR 模型 |
| Microsoft onnxruntime | 1.27.x | MIT | ONNX 推理运行时 DLL |
| ViGEmClient | nefarius/ViGEmClient | MIT | 虚拟 Xbox 手柄客户端 DLL |
| ViGEmBus | nefarius/ViGEmBus | BSD-3-Clause | Windows 虚拟手柄驱动（用户自行安装） |
| yaml.v3 | v3.0.1 | MIT / Apache-2.0 | YAML 配置解析 |
| cobra | v1.10.x | Apache-2.0 | CLI 命令 |
| lumberjack | v2.2.1 | MIT | 日志轮转 |
| golang.org/x/image | latest | BSD-3-Clause | 图像缩放 |
| golang.org/x/sys | latest | BSD-3-Clause | Windows syscall |
| Manrope | Google Fonts | SIL OFL 1.1 | 界面字体 |

具体版本以 go.mod/go.sum 为准。若分发二进制，请同时附带对应第三方许可证文本。
