# 整体性能基线

> 本文件由 `scripts/perf_baseline.ps1` 生成，不要手改。

| 项 | 值 |
| --- | --- |
| CPU | 13th Gen Intel(R) Core(TM) i7-13700K (24 逻辑核) |
| OS | Microsoft Windows 11 专业工作站版 |
| Go | go1.26.1 windows/amd64 |
| 生成时间 | 2026-11-03 12:57:49 |

## 分阶段单次代价

每个阶段先热身若干次，再取 n 次；分配量用 runtime.MemStats 差分后除以 n。

| 阶段 | n | mean | p50 | p95 | 每次分配字节 | 每次分配次数 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ocr.engine.skill_x3 | 20 | 45.32 ms | 45.35 ms | 51.47 ms | 5,418.4 KB | 2736 |
| ocr.engine.story | 15 | 20.05 ms | 19.97 ms | 23.05 ms | 5,970.8 KB | 487 |
| ocr.matcher.miss_x3 | 12 | 45.00 ms | 45.29 ms | 47.02 ms | 5,467.9 KB | 2752 |
| ocr.matcher.hit | 200 | 170 µs | 0 µs | 1,005 µs | 0.1 KB | 1 |
| vision.colorstats | 100 | 2.74 ms | 2.50 ms | 3.52 ms | 0.0 KB | 0 |
| vision.colorrect_full | 100 | 1.91 ms | 2.00 ms | 2.53 ms | 40.7 KB | 0 |
| vision.textbands | 300 | 350 µs | 0 µs | 1,017 µs | 0.1 KB | 2 |
| vision.template_match | 100 | 16.75 ms | 16.62 ms | 18.75 ms | 216.0 KB | 2 |
| feature.story_skip.step | 60 | 266 µs | 0 µs | 2,002 µs | 0.2 KB | 4 |
| feature.auto_combat.step | 60 | 2.08 ms | 2.00 ms | 3.00 ms | 0.3 KB | 6 |
| feature.all.step | 60 | 2.02 ms | 2.00 ms | 3.00 ms | 0.5 KB | 10 |
| async.ocr.submit | 200 | 0 µs | 0 µs | 0 µs | 0.1 KB | 1 |

## 说明

- 画面坐标统一 1920x1080；OCR 用 assets/models/ppocrv6 的真实模型。
- `ocr.engine.*` 绕过 Matcher 缓存，量纯推理；`ocr.matcher.hit` 是缓存命中路径。
- `feature.*.step` 是整帧 `Session.Step`，含该功能一帧内全部识别与决策。
- 绝对值随机器变化；判断优化效果看同一台机器上的前后对比。

