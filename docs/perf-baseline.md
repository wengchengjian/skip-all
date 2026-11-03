# 整体性能基线

> 本文件由 `scripts/perf_baseline.ps1` 生成，不要手改。

| 项 | 值 |
| --- | --- |
| CPU | 13th Gen Intel(R) Core(TM) i7-13700K (24 逻辑核) |
| OS | Microsoft Windows 11 专业工作站版 |
| Go | go1.26.1 windows/amd64 |
| 生成时间 | 2026-11-03 13:15:08 |

## 分阶段单次代价

每个阶段先热身若干次，再取 n 次；分配量用 runtime.MemStats 差分后除以 n。

| 阶段 | n | mean | p50 | p95 | 每次分配字节 | 每次分配次数 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ocr.engine.skill_x3 | 20 | 45.82 ms | 46.28 ms | 50.10 ms | 5,417.9 KB | 2735 |
| ocr.engine.story | 15 | 21.98 ms | 21.29 ms | 27.82 ms | 5,970.9 KB | 487 |
| ocr.matcher.miss_x3 | 12 | 45.41 ms | 45.65 ms | 49.08 ms | 5,467.9 KB | 2752 |
| ocr.matcher.hit | 200 | 161 µs | 0 µs | 627 µs | 0.1 KB | 1 |
| vision.colorstats | 100 | 2.97 ms | 3.00 ms | 3.53 ms | 0.0 KB | 0 |
| vision.colorrect_full | 100 | 1.97 ms | 2.00 ms | 2.51 ms | 40.7 KB | 0 |
| vision.textbands | 300 | 349 µs | 0 µs | 1,010 µs | 0.1 KB | 2 |
| vision.template_match | 100 | 7.93 ms | 8.01 ms | 8.53 ms | 237.5 KB | 5 |
| feature.story_skip.step | 60 | 241 µs | 0 µs | 1,999 µs | 0.2 KB | 4 |
| feature.auto_combat.step | 60 | 2.19 ms | 2.00 ms | 3.00 ms | 0.3 KB | 6 |
| feature.auto_combat.step_boss | 60 | 100 µs | 0 µs | 1,003 µs | 2.7 KB | 16 |
| feature.all.step | 60 | 2.06 ms | 2.01 ms | 3.00 ms | 69.9 KB | 10 |
| async.ocr.submit | 200 | 0 µs | 0 µs | 0 µs | 0.1 KB | 1 |

## 真实截图回放

用 .tmp/ok-ww/tests/images 下的真实截图连续驱动每个功能会话（每帧换一张图），
统计每帧 Step 耗时与该帧内各识别接口的调用次数/总耗时。

| 功能 | 帧数 | mean | p50 | p95 | OCR 次数 | OCR 总耗时 | 模板次数 | 模板总耗时 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| story_skip | 38 | 12.5 ms | 8.5 ms | 29.3 ms | 4 | 139 ms | 324 | 82 ms |
| auto_combat | 38 | 31.4 ms | 9.5 ms | 133.5 ms | 17 | 845 ms | 451 | 270 ms |

## 说明

- 画面坐标统一 1920x1080；OCR 用 assets/models/ppocrv6 的真实模型。
- `ocr.engine.*` 绕过 Matcher 缓存，量纯推理；`ocr.matcher.hit` 是缓存命中路径。
- `feature.*.step` 是整帧 `Session.Step`，含该功能一帧内全部识别与决策。
- 回放用真实模板仓库；空仓库会让 MatchTemplate 立刻报错，读数变成 0ms。
- 回放每帧换图，按帧缓存几乎不命中，因此比真实运行更悲观。
- 绝对值随机器变化；判断优化效果看同一台机器上的前后对比。

