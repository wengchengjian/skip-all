// Package perf 是整体性能基线的度量入口。
//
// 只放度量代码，不参与运行时。用 scripts/perf_baseline.ps1 跑：
//
//	PERF_BASELINE=1 go test ./internal/perf -run TestPerfBaseline -v
//
// 每个阶段输出一行 PERF|name=...|...，由脚本汇总成 docs/perf-baseline.md。
package perf
