# 构建 skip-all Windows 发布目录。
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $goBin = Join-Path (go env GOPATH) "bin"
    $nodeCmd = Get-Command node -ErrorAction SilentlyContinue
    if ($nodeCmd) { $env:PATH = "$(Split-Path $nodeCmd.Source);$env:PATH" }
    if (Test-Path "C:\Program Files\Git\usr\bin") { $env:PATH = "C:\Program Files\Git\usr\bin;$env:PATH" }
    $env:PATH = "$goBin;$env:PATH"
    $wails3 = Get-Command wails3 -ErrorAction SilentlyContinue
    if (-not $wails3) {
        $candidate = Join-Path $env:GOPATH "bin\wails3.exe"
        if (Test-Path $candidate) { $wails3 = $candidate }
    }
    if (-not $wails3) { throw "未找到 wails3，请先 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0" }
    & $wails3 build
    if ($LASTEXITCODE -ne 0) { throw "wails3 build 失败" }

    $out = Join-Path $root "bin"
    New-Item -ItemType Directory -Force -Path (Join-Path $out "models") | Out-Null
    if (Test-Path (Join-Path $root "assets\runtime")) {
        Copy-Item (Join-Path $root "assets\runtime") (Join-Path $out "runtime") -Recurse -Force
    }
    if (Test-Path (Join-Path $root "assets\models\ppocrv6")) {
        Copy-Item (Join-Path $root "assets\models\ppocrv6") (Join-Path $out "models") -Recurse -Force
    }
    Write-Host "构建完成: $out"
} finally {
    Pop-Location
}
