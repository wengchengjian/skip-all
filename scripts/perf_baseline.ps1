# 生成整体性能基线。用法：pwsh -File scripts/perf_baseline.ps1
#
# 跑 internal/perf 的度量并写成 docs/perf-baseline.md。
# 只做度量，不改代码；结果与机器相关，表头会记下 CPU 与 Go 版本。
$ErrorActionPreference = 'Stop'

$repo = Split-Path -Parent $PSScriptRoot
Push-Location $repo
try {
    $env:PERF_BASELINE = '1'

    $cpu = (Get-CimInstance Win32_Processor | Select-Object -First 1).Name
    $cores = [Environment]::ProcessorCount
    $goVer = (& go version) -replace 'go version ',''
    $os = (Get-CimInstance Win32_OperatingSystem).Caption

    Write-Host '跑基线中（约 30 秒）...'
    $raw = & go test ./internal/perf -run 'TestPerfBaseline|TestPerfReplay' -count=1 -v -timeout 1800s 2>&1

    $rows = @()
    foreach ($line in $raw) {
        if ($line -match '^PERF\|(.+)$') {
            $kv = @{}
            foreach ($p in $Matches[1].Split('|')) {
                $i = $p.IndexOf('=')
                if ($i -gt 0) { $kv[$p.Substring(0, $i)] = $p.Substring($i + 1) }
            }
            $rows += [pscustomobject]@{
                Stage   = $kv['name']
                MeanUS  = [int]$kv['mean_us']
                P50US   = [int]$kv['p50_us']
                P95US   = [int]$kv['p95_us']
                Bytes   = [int64]$kv['alloc_bytes']
                Allocs  = [int64]$kv['alloc_ops']
                N       = [int]$kv['n']
            }
        }
    }
    if ($rows.Count -eq 0) { throw '没有拿到任何 PERF| 读数' }



    # 引擎整拍开销：用 Wait=0 的空功能隔离功能成本。
    $engRaw = & go test ./internal/perf -run TestEngineTickOverhead -count=1 -v -timeout 600s 2>&1
    $engs = @()
    foreach ($line in $engRaw) {
        if ($line -match '^ENGINE\|(.+)$') {
            $kv = @{}
            foreach ($p in $Matches[1].Split('|')) {
                $i = $p.IndexOf('=')
                if ($i -gt 0) { $kv[$p.Substring(0, $i)] = $p.Substring($i + 1) }
            }
            $engs += [pscustomobject]@{ Case = $kv['case']; Fps = [double]$kv['fps']; PeriodMs = [double]$kv['period_ms'] }
        }
    }

    # 截图路径：只能对着真实 HWND 量，单独跑一个包。
    $capRaw = & go test ./internal/platform/windows -run TestCaptureBreakdown -count=1 -v -timeout 600s 2>&1
    $caps = @()
    foreach ($line in $capRaw) {
        if ($line -match '^CAPTURE\|(.+)$') {
            $kv = @{}
            foreach ($p in $Matches[1].Split('|')) {
                $i = $p.IndexOf('=')
                if ($i -gt 0) { $kv[$p.Substring(0, $i)] = $p.Substring($i + 1) }
            }
            $caps += [pscustomobject]@{
                Stage = $kv['stage']
                W     = [int]$kv['w']
                H     = [int]$kv['h']
                Us    = [int]$kv['us']
            }
        }
    }

    # 回放：用真实截图连续驱动功能，比单帧重复更接近真实运行。
    $replays = @()
    foreach ($line in $raw) {
        if ($line -match '^REPLAY\|(.+)$') {
            $kv = @{}
            foreach ($p in $Matches[1].Split('|')) {
                $i = $p.IndexOf('=')
                if ($i -gt 0) { $kv[$p.Substring(0, $i)] = $p.Substring($i + 1) }
            }
            $replays += [pscustomobject]@{
                Feature = $kv['feature']
                Frames  = [int]$kv['frames']
                MeanUS  = [int]$kv['mean_us']
                P50US   = [int]$kv['p50_us']
                P95US   = [int]$kv['p95_us']
                OcrN    = [int]$kv['ocr_calls']
                OcrMs   = [int]$kv['ocr_ms']
                TplN    = [int]$kv['tpl_calls']
                TplMs   = [int]$kv['tpl_ms']
            }
        }
    }

    $md = New-Object System.Text.StringBuilder
    [void]$md.AppendLine('# 整体性能基线')
    [void]$md.AppendLine('')
    [void]$md.AppendLine('> 本文件由 `scripts/perf_baseline.ps1` 生成，不要手改。')
    [void]$md.AppendLine('')
    [void]$md.AppendLine('| 项 | 值 |')
    [void]$md.AppendLine('| --- | --- |')
    [void]$md.AppendLine("| CPU | $cpu ($cores 逻辑核) |")
    [void]$md.AppendLine("| OS | $os |")
    [void]$md.AppendLine("| Go | $goVer |")
    [void]$md.AppendLine("| 生成时间 | $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') |")
    [void]$md.AppendLine('')
    [void]$md.AppendLine('## 分阶段单次代价')
    [void]$md.AppendLine('')
    [void]$md.AppendLine('每个阶段先热身若干次，再取 n 次；分配量用 runtime.MemStats 差分后除以 n。')
    [void]$md.AppendLine('')
    [void]$md.AppendLine('| 阶段 | n | mean | p50 | p95 | 每次分配字节 | 每次分配次数 |')
    [void]$md.AppendLine('| --- | ---: | ---: | ---: | ---: | ---: | ---: |')
    foreach ($r in $rows) {
        $mean = '{0:N2} ms' -f ($r.MeanUS / 1000.0)
        $p50  = '{0:N2} ms' -f ($r.P50US / 1000.0)
        $p95  = '{0:N2} ms' -f ($r.P95US / 1000.0)
        if ($r.MeanUS -lt 1000) {
            $mean = '{0:N0} µs' -f $r.MeanUS
            $p50  = '{0:N0} µs' -f $r.P50US
            $p95  = '{0:N0} µs' -f $r.P95US
        }
        $kb = '{0:N1} KB' -f ($r.Bytes / 1024.0)
        [void]$md.AppendLine("| $($r.Stage) | $($r.N) | $mean | $p50 | $p95 | $kb | $($r.Allocs) |")
    }
    if ($engs.Count -gt 0) {
        [void]$md.AppendLine('')
        [void]$md.AppendLine('## 引擎整拍')
        [void]$md.AppendLine('')
        [void]$md.AppendLine('用一个 Wait=0 的空功能隔离功能成本，量实际周期是否等于 TargetFPS 间隔。')
        [void]$md.AppendLine('')
        [void]$md.AppendLine('| 配置 | 实际 FPS | 实际周期 |')
        [void]$md.AppendLine('| --- | ---: | ---: |')
        foreach ($e in $engs) {
            [void]$md.AppendLine(('| {0} | {1:N2} | {2:N2} ms |' -f $e.Case, $e.Fps, $e.PeriodMs))
        }
        [void]$md.AppendLine('')
    }
    if ($caps.Count -gt 0) {
        [void]$md.AppendLine('')
        [void]$md.AppendLine('## 截图路径拆分')
        [void]$md.AppendLine('')
        [void]$md.AppendLine('对着本机一个真实窗口量（可用环境变量 CAPTURE_WINDOW 指定进程名）。')
        [void]$md.AppendLine('')
        [void]$md.AppendLine('| 阶段 | 窗口 | 耗时 |')
        [void]$md.AppendLine('| --- | --- | ---: |')
        foreach ($c in $caps) {
            [void]$md.AppendLine(('| {0} | {1}x{2} | {3:N2} ms |' -f $c.Stage, $c.W, $c.H, ($c.Us / 1000.0)))
        }
        [void]$md.AppendLine('')
    }
    if ($replays.Count -gt 0) {
        [void]$md.AppendLine('')
        [void]$md.AppendLine('## 真实截图回放')
        [void]$md.AppendLine('')
        [void]$md.AppendLine('用 .tmp/ok-ww/tests/images 下的真实截图连续驱动每个功能会话（每帧换一张图），')
        [void]$md.AppendLine('统计每帧 Step 耗时与该帧内各识别接口的调用次数/总耗时。')
        [void]$md.AppendLine('')
        [void]$md.AppendLine('| 功能 | 帧数 | mean | p50 | p95 | OCR 次数 | OCR 总耗时 | 模板次数 | 模板总耗时 |')
        [void]$md.AppendLine('| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |')
        foreach ($r in $replays) {
            $mean = '{0:N1} ms' -f ($r.MeanUS / 1000.0)
            $p50 = '{0:N1} ms' -f ($r.P50US / 1000.0)
            $p95 = '{0:N1} ms' -f ($r.P95US / 1000.0)
            $cells = @($r.Feature, $r.Frames, $mean, $p50, $p95, $r.OcrN, ($r.OcrMs.ToString() + ' ms'), $r.TplN, ($r.TplMs.ToString() + ' ms'))
            [void]$md.AppendLine('| ' + ($cells -join ' | ') + ' |')
        }
        [void]$md.AppendLine('')
    }
    [void]$md.AppendLine('## 说明')
    [void]$md.AppendLine('')
    [void]$md.AppendLine('- 画面坐标统一 1920x1080；OCR 用 assets/models/ppocrv6 的真实模型。')
    [void]$md.AppendLine('- `ocr.engine.*` 绕过 Matcher 缓存，量纯推理；`ocr.matcher.hit` 是缓存命中路径。')
    [void]$md.AppendLine('- `feature.*.step` 是整帧 `Session.Step`，含该功能一帧内全部识别与决策。')
    [void]$md.AppendLine('- 回放用真实模板仓库；空仓库会让 MatchTemplate 立刻报错，读数变成 0ms。')
    [void]$md.AppendLine('- 回放每帧换图，按帧缓存几乎不命中，因此比真实运行更悲观。')
    [void]$md.AppendLine('- 绝对值随机器变化；判断优化效果看同一台机器上的前后对比。')
    [void]$md.AppendLine('')

    $out = Join-Path $repo 'docs/perf-baseline.md'
    [System.IO.File]::WriteAllText($out, $md.ToString(), (New-Object System.Text.UTF8Encoding $false))
    Write-Host "已写入 $out（$($rows.Count) 个阶段）"
}
finally {
    Pop-Location
}
