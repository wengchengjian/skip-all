# 整体性能基线

> 本文件由 `scripts/perf_baseline.ps1` 生成，不要手改。

| 项 | 值 |
| --- | --- |
| CPU | 13th Gen Intel(R) Core(TM) i7-13700K (24 逻辑核) |
| OS | Microsoft Windows 11 专业工作站版 |
| Go | go1.26.1 windows/amd64 |
| 生成时间 | 2026-11-03 13:40:48 |

## 分阶段单次代价

每个阶段先热身若干次，再取 n 次；分配量用 runtime.MemStats 差分后除以 n。

| 阶段 | n | mean | p50 | p95 | 每次分配字节 | 每次分配次数 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ocr.engine.skill_x3 | 20 | 48.89 ms | 49.75 ms | 59.88 ms | 5,417.9 KB | 2735 |
| ocr.engine.story | 15 | 19.81 ms | 19.57 ms | 21.47 ms | 5,970.9 KB | 487 |
| ocr.matcher.miss_x3 | 12 | 44.47 ms | 45.05 ms | 46.70 ms | 5,468.0 KB | 2752 |
| ocr.matcher.hit | 200 | 178 µs | 0 µs | 588 µs | 0.1 KB | 1 |
| vision.colorstats | 100 | 2.98 ms | 3.00 ms | 3.51 ms | 0.0 KB | 0 |
| vision.colorrect_full | 100 | 2.01 ms | 2.00 ms | 2.52 ms | 40.7 KB | 0 |
| vision.textbands | 300 | 350 µs | 0 µs | 1,012 µs | 0.1 KB | 2 |
| vision.template_match | 100 | 7.97 ms | 8.02 ms | 8.60 ms | 237.5 KB | 5 |
| feature.story_skip.step | 60 | 248 µs | 0 µs | 1,693 µs | 0.2 KB | 4 |
| feature.auto_combat.step | 60 | 2.21 ms | 2.00 ms | 3.00 ms | 0.3 KB | 6 |
| feature.auto_combat.step_boss | 60 | 75 µs | 0 µs | 1,008 µs | 2.7 KB | 16 |
| feature.all.step | 60 | 2.05 ms | 2.03 ms | 3.10 ms | 69.9 KB | 10 |
| async.ocr.submit | 200 | 0 µs | 0 µs | 0 µs | 0.1 KB | 1 |

## 引擎整拍

用一个 Wait=0 的空功能隔离功能成本，量实际周期是否等于 TargetFPS 间隔。

| 配置 | 实际 FPS | 实际周期 |
| --- | ---: | ---: |
| 截图0ms  FPS=30 | 29.99 | 33.34 ms |
| 截图13ms FPS=30 | 29.93 | 33.41 ms |
| 截图13ms FPS=10 | 10.00 | 100.01 ms |


## 截图路径拆分

对着本机一个真实窗口量（可用环境变量 CAPTURE_WINDOW 指定进程名）。

| 阶段 | 窗口 | 耗时 |
| --- | --- | ---: |
| printwindow | 2080x1210 | 12.74 ms |
| bitblt | 2080x1210 | 12.79 ms |
| scale | 2080x1210 | 1.00 ms |
| pooled_frame | 2080x1210 | 13.74 ms |
| unpooled_capture | 2080x1210 | 19.09 ms |


## 真实截图回放

用 .tmp/ok-ww/tests/images 下的真实截图连续驱动每个功能会话（每帧换一张图），
统计每帧 Step 耗时与该帧内各识别接口的调用次数/总耗时。

| 功能 | 帧数 | mean | p50 | p95 | OCR 次数 | OCR 总耗时 | 模板次数 | 模板总耗时 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| story_skip | 38 | 9.8 ms | 5.5 ms | 31.8 ms | 4 | 140 ms | 324 | 86 ms |
| auto_combat | 38 | 30.8 ms | 10.0 ms | 132.6 ms | 17 | 835 ms | 451 | 259 ms |

## 说明

- 画面坐标统一 1920x1080；OCR 用 assets/models/ppocrv6 的真实模型。
- `ocr.engine.*` 绕过 Matcher 缓存，量纯推理；`ocr.matcher.hit` 是缓存命中路径。
- `feature.*.step` 是整帧 `Session.Step`，含该功能一帧内全部识别与决策。
- 回放用真实模板仓库；空仓库会让 MatchTemplate 立刻报错，读数变成 0ms。
- 回放每帧换图，按帧缓存几乎不命中，因此比真实运行更悲观。
- 绝对值随机器变化；判断优化效果看同一台机器上的前后对比。

