# 整体性能基线

> 本文件由 `scripts/perf_baseline.ps1` 生成，不要手改。

| 项 | 值 |
| --- | --- |
| CPU | 13th Gen Intel(R) Core(TM) i7-13700K (24 逻辑核) |
| OS | Microsoft Windows 11 专业工作站版 |
| Go | go1.26.1 windows/amd64 |
| 生成时间 | 2026-11-03 13:07:12 |

## 分阶段单次代价

每个阶段先热身若干次，再取 n 次；分配量用 runtime.MemStats 差分后除以 n。

| 阶段 | n | mean | p50 | p95 | 每次分配字节 | 每次分配次数 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ocr.engine.skill_x3 | 20 | 45.50 ms | 44.94 ms | 50.08 ms | 5,418.2 KB | 2735 |
| ocr.engine.story | 15 | 19.86 ms | 19.68 ms | 21.28 ms | 5,970.8 KB | 487 |
| ocr.matcher.miss_x3 | 12 | 43.61 ms | 43.70 ms | 45.64 ms | 5,467.9 KB | 2752 |
| ocr.matcher.hit | 200 | 161 µs | 0 µs | 586 µs | 0.1 KB | 1 |
| vision.colorstats | 100 | 2.96 ms | 3.00 ms | 4.00 ms | 0.0 KB | 0 |
| vision.colorrect_full | 100 | 2.12 ms | 2.00 ms | 3.00 ms | 40.7 KB | 0 |
| vision.textbands | 300 | 353 µs | 0 µs | 1,012 µs | 0.1 KB | 2 |
| vision.template_match | 100 | 7.93 ms | 8.00 ms | 8.72 ms | 237.5 KB | 5 |
| feature.story_skip.step | 60 | 304 µs | 0 µs | 2,268 µs | 0.2 KB | 4 |
| feature.auto_combat.step | 60 | 2.18 ms | 2.00 ms | 3.00 ms | 0.3 KB | 6 |
| feature.all.step | 60 | 2.15 ms | 2.00 ms | 3.00 ms | 68.3 KB | 10 |
| async.ocr.submit | 200 | 0 µs | 0 µs | 0 µs | 0.1 KB | 1 |

## 真实截图回放

用 .tmp/ok-ww/tests/images 下的真实截图连续驱动每个功能会话（每帧换一张图），
统计每帧 Step 耗时与该帧内各识别接口的调用次数/总耗时。

| 功能 | 帧数 | mean | p50 | p95 | OCR 次数 | OCR 总耗时 | 模板次数 | 模板总耗时 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| story_skip | 38 | 12.8 ms | 8.5 ms | 30.6 ms | 4 | 142 ms | 324 | 93 ms |
| auto_combat | 38 | 32.5 ms | 11.3 ms | 139.6 ms | 17 | 862 ms | 451 | 274 ms |

## 说明

- 画面坐标统一 1920x1080；OCR 用 assets/models/ppocrv6 的真实模型。
- `ocr.engine.*` 绕过 Matcher 缓存，量纯推理；`ocr.matcher.hit` 是缓存命中路径。
- `feature.*.step` 是整帧 `Session.Step`，含该功能一帧内全部识别与决策。
- 回放用真实模板仓库；空仓库会让 MatchTemplate 立刻报错，读数变成 0ms。
- 回放每帧换图，按帧缓存几乎不命中，因此比真实运行更悲观。
- 绝对值随机器变化；判断优化效果看同一台机器上的前后对比。

