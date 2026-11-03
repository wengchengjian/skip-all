# 性能审计

对象：整个仓库的每帧热路径。基线数字见 [perf-baseline.md](perf-baseline.md)，
由 `scripts/perf_baseline.ps1` 在同一台机器上生成。

## 方法

1. 先建可复现基线：每个阶段单独计时，并统计每次调用的分配字节与**分配次数**。
2. 分配次数比字节数更重要：字节多只是堆大，次数多才是 GC 压力的来源。
3. 单帧重复测不够——换成**真实截图序列回放**，能暴露"几十帧才跑一次"的开销。
4. 每项改动用同一脚本复测，看同机前后差。

## 一帧到底花在哪

同机（i7-13700K）实测，1920x1080，38 张真实截图连续回放（每帧换图）：

| 功能 | 每帧 mean | p50 | p95 | OCR 次数/耗时 | 模板次数/耗时 |
| --- | ---: | ---: | ---: | ---: | ---: |
| story_skip | 12.8 ms | 8.5 ms | 30.6 ms | 4 / 142 ms | 324 / 93 ms |
| auto_combat | 32.5 ms | 11.3 ms | 139.6 ms | 17 / 862 ms | 451 / 274 ms |

结论：**OCR 与模板匹配两者合计占掉几乎全部时间**，其余（颜色统计、文字行投影、
决策）都在 1ms 量级以下。

注意回放每帧换图，按帧缓存几乎不命中，因此比真实运行更悲观。

## 已修复

### 1. 逐像素接口装箱（pkg/vision）

`image.Image.At()` 每次返回 `color.Color` 接口。对 `*image.RGBA` 来说，
4 字节的 `color.RGBA` 会被装箱到堆上——**每个像素一次分配**。

| 文件 | 函数 | 修前每次分配 |
| --- | --- | ---: |
| color.go | analyzeColorWith | 330,000 次 / 1.3 MB |
| textbands.go | FindTextBands | 161,202 次 / 630 KB |
| template.go | WhiteMask / grayValueAt | 22,306 次 / 303 KB |
| color.go | filterByColorRange | 随区域像素数 |

修法：`pkg/vision/pixel.go` 的 `pixelReader`，对 `*image.RGBA` 直接读 Pix。

| 阶段 | 修前 | 修后 |
| --- | --- | --- |
| vision.colorstats | 6.22 ms / 330,000 次 | 2.71 ms / 0 次 |
| vision.textbands | 1.44 ms / 161,202 次 | 0.35 ms / 2 次 |
| feature.story_skip.step | 0.94 ms / 97,174 次 | 0.21 ms / 4 次 |

### 2. 模板匹配改用列和滑动窗口

`MatchTemplate` 原先对每个候选位置重算三项累加：

    for _, p := range points {
        v := gray[(y+p.dy)*sw + x+p.dx]   // 每点一次乘法
        sumI += v; sumI2 += v*v; sumIT += v*p.v
    }

85 个内置模板里 **只有 1 个带掩码**，所以无掩码是主路径：此时 `sumI`/`sumI2`
就是矩形窗口和，可以用列和滑动窗口 O(1) 推进，内层只剩 `sumIT` 一项。
同时把 `dy*sw+dx` 预计算成偏移，去掉内层乘法。

另外加了一条：窗口方差过小时（纯色背景）直接跳过，省掉整趟 `sumIT`。

| 指标 | 修前 | 修后 |
| --- | ---: | ---: |
| vision.template_match（合成 48x48） | 16.75 ms | **7.97 ms**（2.1x） |
| story_skip 模板总耗时 | 224 ms | **93 ms**（-58%） |
| auto_combat 模板总耗时 | 485 ms | **274 ms**（-43%） |
| auto_combat 每帧 p50 | 18.1 ms | **11.3 ms**（-38%） |

正确性：`TestMatchTemplateEquivalence` 用 24 个随机画面逐例对照优化前的
朴素实现，位置完全一致、置信度差 < 1e-9。

## 待办（按影响排序）

### A. OCR 的 det 占 60~80%

实测（先按 Recognize 的做法设置自适应 det 边长）：

| 区域 | detSide | det | 全部 | rec |
| --- | ---: | ---: | ---: | ---: |
| 技能区 68x57 | 272 | 8.85 ms | 11.51 ms | 2.66 ms |
| 字幕区 1240x130 | 960 | 13.83 ms | 18.77 ms | 4.94 ms |

"已知区域内只有一行字"的场景，det 只是为了找出一个框却要付 8.8ms。
改成直接喂 rec 理论能省掉这一半，但历史数据显示这条路径对预处理敏感
（`8.4` 读成 `18.4`），**必须先用 43 张真实截图验准确率**。

### B. OCR 每次调用分配 5.4 MB / 2,736 次

全部发生在 `third_party/go-ocr`：`preprocessDetImage` 7.2MB、
`draw.CatmullRom` 临时缓冲 3.1MB、`postprocessHeatmap` 2MB。
该库是本地 replace 的 fork，可以改，但要同步更新 NOTICE 与 docs/upstream.md。

### C. 模板调用次数偏多

auto_combat 每帧 11.9 次模板匹配、story_skip 每帧 8.5 次。
目前靠算法提速扛住了；要进一步降只能减少调用次数（缓存识别结果、
收窄判据触发频率），都会改变行为，需要单独评估。

### D. 全屏颜色矩形 1.98 ms / 帧

`FindColorRectangles` 每帧扫全屏 200 万像素，计算受限（每像素 6 次比较，
8MB 读 / 1.9ms ≈ 4.2 GB/s，比内存带宽低一个数量级）。
再快只能减少扫描量（行级跳采样、收窄区域），都会改语义，暂缓。

### E. 还没进基线覆盖的部分

- `internal/platform/windows` 的截图路径（PrintWindow + 缩放）有独立 benchmark，
  还没并进脚本。
- `internal/engine` 的整拍开销（截图之外）没有单独量。
- ZZZ 的功能没有真实截图，无法测。

## 口径提醒（踩过的坑）

- **模板仓库不能为空**：`NewMatcher(ocr, nil)` 会让 `MatchTemplate` 立刻报错，
  读数变成 0ms，看起来"模板匹配不要钱"。必须用 `NewTemplateStoreFS` 接真实模板。
- **量 OCR 拆分前必须设自适应 det 边长**，否则 68x57 会被放大到 960，
  det 读数 116ms（真实 8.1ms）。
- 绝对值随机器变化，只做同机前后对比。
- `feature.*.step` 包含该功能一帧内全部识别，会和视觉阶段重叠，不要相加。
