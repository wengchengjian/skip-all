# OCR 模型与 GPU

## 原绝区零自动化项目

- 默认模型：**PP-OCRv6**（`DEFAULT_OCR_MODEL_NAME = "ppocrv6"`）。
- 模型文件：`assets/models/onnx_ocr/ppocrv6/{det.onnx,rec.onnx,ppocrv6_dict.txt}`。
- 推理参数：`det_limit_side_len=960`、`rec_image_shape=3,48,320`、`det_db_box_thresh=0.45`。
- 推理运行时：Windows 使用 **onnxruntime-directml**（`onnxruntime-directml==1.24.4`）。
- GPU 开关：`ocr_use_gpu`，**默认 false**；在资源管理界面切换，切换时会重载 OCR。
- DirectML 的优点：Intel / NVIDIA / AMD 都能用；不要求 CUDA Toolkit。上游把 CUDA 作为可选 provider，但 Windows 默认装 DirectML。

## 当前 skip-all

- 模型：同样是 **PP-OCRv6**，文件在 `assets/models/ppocrv6`。
- 运行时：当前 `go-ocr` 使用 CPU 版 `onnxruntime.dll`。
- `go-ocr` 的 Go 配置只暴露了 `UseCuda`，没有 `DmlExecutionProvider`；所以目前不能直接复刻上游的 DirectML。
- 跳过/点击模式已经改成**白色文字行投影**定位按钮、选项、确认框、对话框，不再逐帧 OCR；默认 5 FPS 下 `Step` 约 20ms，模型和 GPU 对这条路径基本没有影响。
- `auto` 模式仍需要 OCR 识别“自动/菜单”，是唯一还会持续跑 OCR 的模式。

## 模型选择

| 模型 | 速度 | 中文/游戏 UI 准确率 | 建议 |
| --- | --- | --- | --- |
| PP-OCRv6 small（当前/上游） | 快 | 好 | 保持，适合实时 |
| PP-OCRv5 mobile | 略快 | 接近，稍弱 | 可做后备，不必要 |
| PP-OCRv6 server | 慢很多 | 最好 | 不适合逐帧 |
| RapidOCR / PP-OCRv4 mobile | 很快 | 一般 | 只适合简单短文本 |
| Windows 内置 OCR | 快 | 对游戏字体一般 | 不推荐做选项识别 |

结论：**不要为了速度换模型**。当前瓶颈不是模型，而是每帧多次调用；文字行检测已经把跳过路径的 OCR 去掉了。`auto` 模式才值得上 GPU。

## 加 GPU 的两条路

### 1. DirectML（推荐，和上游一致）

需要：

- `onnxruntime-directml.dll` 或带 DirectML provider 的 ONNX Runtime。
- 给 `onnxruntime_purego` 增加 `SessionOptionsAppendExecutionProvider("DmlExecutionProvider", ...)` 的封装（当前绑定没有暴露这个通用接口）。
- 启动时先探测 DML provider，再创建 OCR；失败回退 CPU。ONNX Runtime 的 engine 在进程内是全局单例，不能在同一进程里先 CPU 再切 GPU，自动回退需要在创建 OCR 前完成探测。

优点：Intel/NVIDIA/AMD 通用，不依赖 CUDA。  
代价：需要 fork/替换 Go 的 ORT 绑定，或改用 Python/独立 OCR 进程。

### 2. CUDA（当前 Go 绑定已有入口）

`go-ocr` 的 `paddle.Config` 已有 `UseCuda`，但需要：

- 带 CUDA provider 的 `onnxruntime.dll`（不是现在的 CPU DLL）。
- 匹配版本的 CUDA / cuDNN 运行库。
- NVIDIA GPU；当前机器是 CMP 40HX，`nvidia-smi` 可用，但 GPU 已被其他程序占用。

优点：改动小，Go 绑定直接支持。  
代价：NVIDIA only，部署依赖重；用户机器没有 CUDA 时必须回退 CPU。

## 建议配置

- 默认 `ocr.gpu=auto` 只应在 **auto 模式** 或 doctor 中启用；skip/click 模式不需要 OCR。
- 探测顺序：DirectML -> CUDA -> CPU。
- 模型保持 `ppocrv6`，不换 server。
- 如果 GPU 初始化失败，记录原因并回退 CPU；不要静默卡住启动。
