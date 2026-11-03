# 性能审计

对象：整个仓库的每帧热路径。基线数字见 [perf-baseline.md](perf-baseline.md)，
由 `scripts/perf_baseline.ps1` 在同一台机器上生成。

## 方法

1. 先建可复现基线：每个阶段单独计时并统计每次调用的分配字节与分配次数。
2. 按"每帧都会跑"排序，只优化热路径；冷路径不做无谓改动。
3. 每项改动都用同一脚本复测，看同一台机器上的前后差。

分配次数比字节数更重要：字节多只是堆大，次数多才是 GC 压力的来源。

## 已修复

### 1. 逐像素接口装箱（pkg/vision）

`image.Image.At()` 每次返回 `color.Color` 接口。对 `*image.RGBA` 来说，
4 字节的 `color.RGBA` 会被装箱到堆上——**每个像素一次分配**。

受影响的位置：

| 文件 | 函数 | 修前每次分配 |
| --- | --- | ---: |
| color.go | analyzeColorWith | 330,000 次 / 1.3 MB |
| textbands.go | FindTextBands | 161,202 次 / 630 KB |
| template.go | WhiteMask / grayValueAt | 22,306 次 / 303 KB |
| color.go | filterByColorRange | 随区域像素数 |

修法：新增 `pkg/vision/pixel.go` 的 `pixelReader`，对 `*image.RGBA` 直接读 Pix，
其它实现回退到 At。5 个调用点全部改用 reader。

复测（同机同比）：

| 阶段 | 修前 | 修后 | 变化 |
| --- | --- | --- | --- |
| vision.colorstats | 6.22 ms / 330,000 次 | **2.71 ms / 0 次** | 时间 2.3x，分配归零 |
| vision.textbands | 1.44 ms / 161,202 次 | **0.35 ms / 2 次** | 时间 4.1x，分配降 8 万倍 |
| feature.story_skip.step | 0.94 ms / 97,174 次 | **0.21 ms / 4 次** | 时间 4.5x，分配降 2.4 万倍 |
| vision.template_match | 14.2 ms / 22,306 次 | 17.0 ms / 2 次 | 分配基本归零，时间受滑窗支配 |

template_match 的耗时没有下降，因为它的瓶颈不是取像素而是 O((W-w)(H-h)wh)
的滑窗搜索——见下面的待办。

## 待办（按每帧影响排序）

### A. OCR 每次调用分配 5.4 MB / 2,736 次

`ocr.engine.skill_x3`（三个 68x57 区域）每次 5.4 MB、2,736 次分配。
按 10Hz 折算约 54 MB/s 的分配量，是目前最大的 GC 压力来源。

推算每区域约 900 次分配、1.8 MB。可疑点：ONNX 输入张量每次新建、
det/rec 之间的中间缓冲没有复用。下一步做分配 profile 定位到具体函数。

### B. 模板匹配 17 ms

`MatchTemplate` 是朴素滑窗：对每个位置重算整个模板的点积。
200x100 的搜索区配 48x48 模板 = 152x52x2304 ≈ 1800 万次乘加。
`sumI`/`sumI2` 可以用积分图 O(1) 拿到，只剩 `sumIT` 需要逐点算，
能把内层乘加砍掉约 2/3。

### C. 全屏颜色矩形 1.9 ms / 帧

`FindColorRectangles` 每帧扫全屏 200 万像素。已经做过缓冲池与提前退出
（分配从 4.18 MB 降到 0），剩下的是扫描本身。
减少扫描量需要收窄搜索区域或做行级跳采样，但都会改变语义，暂缓。

### D. 待查

- `internal/engine` 的整拍开销（截图之外的部分）还没单独量。
- 截图路径（PrintWindow + 缩放）由 `internal/platform/windows` 的
  benchmark 量，还没并入本基线脚本。
- `pkg/assist/async_ocr` 的 worker 池吞吐与陈旧率未量化。

## 口径提醒

- 绝对值随机器变化，只做同机前后对比。
- `feature.*.step` 包含该功能一帧内全部识别，会和视觉阶段重叠，不要相加。
- OCR 有按帧缓存：`ocr.matcher.hit` 是命中路径，`ocr.engine.*` 才是推理成本。
