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
- 运行时：已在 `third_party/go-ocr` 和 `third_party/onnxruntime_purego` 增加 DirectML 入口；GPU 后端用 `ocr.gpu: auto` 按 **DirectML → CUDA → CPU** 探测。
- 跳过/点击模式已经改成**白色文字行投影**定位按钮、选项、确认框、对话框，不再逐帧 OCR；默认 5 FPS 下 `Step` 约 20ms，模型和 GPU 对这条路径基本没有影响。
- `auto` 模式仍需要 OCR 识别“自动/菜单”，是唯一还会持续跑 OCR 的模式。
- 顶栏实时显示 FPS 和当前 OCR 后端（DirectML / CUDA / CPU），方便直接测量性能。
- DirectML 运行库可用 `scripts/install-ort-dml.ps1` 安装到 `assets/runtime`；也可在配置里指定 `ocr.runtime_dir`。

## 模型选择

| 模型 | 速度 | 中文/游戏 UI 准确率 | 建议 |
| --- | --- | --- | --- |
| PP-OCRv6 small（当前/上游） | 快 | 好 | 保持，适合实时 |
| PP-OCRv5 mobile | 略快 | 接近，稍弱 | 可做后备，不必要 |
| PP-OCRv6 server | 慢很多 | 最好 | 不适合逐帧 |
| RapidOCR / PP-OCRv4 mobile | 很快 | 一般 | 只适合简单短文本 |
| Windows 内置 OCR | 快 | 对游戏字体一般 | 不推荐做选项识别 |

结论：**不要为了速度换模型**。当前瓶颈不是模型，而是每帧多次调用；文字行检测已经把跳过路径的 OCR 去掉了。`auto` 模式才值得上 GPU。

## GPU 后端

### 1. DirectML（默认优先，和上游一致）

- 运行库：`onnxruntime_dml.dll` + `onnxruntime_providers_shared.dll` + `DirectML.dll`。
- 安装：运行 `scripts/install-ort-dml.ps1`，文件会放到 `assets/runtime`。
- `onnxruntime-directml` 在创建 session 时会自动注册 DML provider，因此本地绑定不再重复 append；重复 append 会报 `already been registered`。
- 优点：Intel/NVIDIA/AMD 通用，不依赖 CUDA Toolkit。

### 2. CUDA（NVIDIA 可选）

- 需要带 CUDA provider 的 `onnxruntime_cuda.dll`，以及匹配版本的 CUDA / cuDNN 运行库。
- 放在 `assets/runtime` 或 `ocr.runtime_dir`；`ocr.gpu: cuda` 强制使用，`auto` 会在没有 DML 时尝试。
- NVIDIA only；当前机器是 CMP 40HX，`nvidia-smi` 可用，但 GPU 已被其他程序占用。

### 自动探测

`ensureOCR` 会按 `DirectML → CUDA → CPU` 依次尝试；某个后端初始化失败会记录日志并把同一个 ORT DLL 当 CPU 运行时再试，最后回退默认 CPU。ONNX Runtime 的 engine 在进程内是全局单例，所以切换 DLL 需要重启进程。

## 建议配置

- 默认 `ocr.gpu=auto` 只应在 **auto 模式** 或 doctor 中启用；skip/click 模式不需要 OCR。
- 探测顺序：DirectML -> CUDA -> CPU。
- 模型保持 `ppocrv6`，不换 server。
- 如果 GPU 初始化失败，记录原因并回退 CPU；不要静默卡住启动。
