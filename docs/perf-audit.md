# 性能审计

对象：整个仓库的每帧热路径。基线数字见 [perf-baseline.md](perf-baseline.md)，
由 `scripts/perf_baseline.ps1` 在同一台机器上生成。

## 方法

1. 先建可复现基线：每个阶段单独计时，并统计每次调用的分配字节与**分配次数**。
2. 分配次数比字节数更重要：字节多只是堆大，次数多才是 GC 压力的来源。
3. 按"每帧都会跑"排序，只优化热路径。
4. 每项改动用同一脚本复测，看同机前后差。

## 一帧到底花在哪

同机（i7-13700K）实测，1920x1080 真实截图：

| 场景 | 每帧代价 | 主要构成 |
| --- | ---: | --- |
| 稳态（OCR 命中缓存 / 走异步） | **2.02 ms** | `colorrect_full` 1.91ms + 剧情跳过 0.27ms |
| OCR 未命中（同步） | **+45 ms** | det 占 60~80%，rec 占 20~40% |

所以整体性能只由两件事决定：

1. **OCR 多久跑一次**（已由按帧缓存 + 异步池控制）
2. **OCR 跑一次多贵**（45ms，其中 det 8.1ms/区域）

稳态那 2ms 在 200ms 的拍预算里只占 1%，不是主要矛盾。

## 已修复

### 逐像素接口装箱（pkg/vision）

`image.Image.At()` 每次返回 `color.Color` 接口。对 `*image.RGBA` 来说，
4 字节的 `color.RGBA` 会被装箱到堆上——**每个像素一次分配**。

| 文件 | 函数 | 修前每次分配 |
| --- | --- | ---: |
| color.go | analyzeColorWith | 330,000 次 / 1.3 MB |
| textbands.go | FindTextBands | 161,202 次 / 630 KB |
| template.go | WhiteMask / grayValueAt | 22,306 次 / 303 KB |
| color.go | filterByColorRange | 随区域像素数 |

修法：新增 `pkg/vision/pixel.go` 的 `pixelReader`，对 `*image.RGBA` 直接读 Pix，
其它实现回退到 At。5 个调用点全部改用 reader。

| 阶段 | 修前 | 修后 |
| --- | --- | --- |
| vision.colorstats | 6.22 ms / 330,000 次 | **2.71 ms / 0 次** |
| vision.textbands | 1.44 ms / 161,202 次 | **0.35 ms / 2 次** |
| feature.story_skip.step | 0.94 ms / 97,174 次 | **0.21 ms / 4 次** |
| vision.template_match | 14.2 ms / 22,306 次 | 16.8 ms / 2 次 |

## 待办（按影响排序）

### A. OCR 的 det 占 60~80%，小区域可以不做 det

实测（先按 Recognize 的做法设置自适应 det 边长）：

| 区域 | detSide | det | 全部 | rec |
| --- | ---: | ---: | ---: | ---: |
| 技能区 68x57 | 272 | 8.85 ms | 11.51 ms | 2.66 ms |
| 字幕区 1240x130 | 960 | 13.83 ms | 18.77 ms | 4.94 ms |

技能冷却这类"已知区域内只有一行字"的场景，det 只是为了找出一个框，
却要付 8.8ms。如果改成直接把整块裁剪喂给 rec，理论能省掉 det 这一半。

**风险**：准确率。历史测量显示 68x57 区域在 detSide<192 时会认错
（`8.4` 读成 `18.4`），说明这条路径对预处理敏感。要做必须先用
43 张真实截图跑一遍准确率对比，不能只看速度。

### B. Service.Template 没有按帧缓存

`Service.OCR` 有按帧缓存（key = 区域 + 颜色范围 + 像素校验和），
`Service.Template` 没有：同一帧同一区域的模板匹配会被完整重算，每次 17ms。

当前基线里没体现出影响，因为 wuwa 的两个功能在这张截图上都不走模板匹配。
但只要某个功能每帧查一次模板图标，就是每帧 17ms。

**下一步**：先数清楚真实运行时每帧到底调几次模板匹配，再决定是加缓存
还是改算法（`sumI`/`sumI2` 在无掩码时可用积分图 O(1)，有掩码时不行）。

### C. 全屏颜色矩形 1.91 ms / 帧

`FindColorRectangles` 每帧扫全屏 200 万像素。已经做过缓冲池与提前退出
（分配从 4.18 MB 降到 0）。剩下的是扫描本身：8MB 读 / 1.9ms ≈ 4.2 GB/s，
比内存带宽低一个数量级，是计算受限（每像素 6 次比较）。

想再快只能减少扫描量（行级跳采样、收窄搜索区域），但都会改变语义
（`minWidth` 可以小到 2），暂时不动。

### D. 还没进基线覆盖的部分

- `internal/platform/windows` 的截图路径（PrintWindow + 缩放）有独立
  benchmark，还没并进 `scripts/perf_baseline.ps1`。
- `internal/engine` 的整拍开销（截图之外）没有单独量。
- ZZZ 的功能没有真实截图，无法测。

## 口径提醒

- 绝对值随机器变化，只做同机前后对比。
- `feature.*.step` 包含该功能一帧内全部识别，会和视觉阶段重叠，不要相加。
- OCR 有按帧缓存：`ocr.matcher.hit` 是命中路径，`ocr.engine.*` 才是推理成本。
- 量 OCR 拆分前必须先设置自适应 det 边长，否则 68x57 会被放大到 960，
  读数虚高十几倍（这一条踩过）。
