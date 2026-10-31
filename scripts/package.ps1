# 把 bin/ 打包为可分发 zip（只包含运行所需文件）。
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root "bin"
$zip = Join-Path $bin "skip-all-windows-amd64.zip"
if (-not (Test-Path (Join-Path $bin "skip-all.exe"))) { throw "请先运行 scripts/build.ps1" }

$stage = Join-Path $bin ".package"
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null
Copy-Item (Join-Path $bin "skip-all.exe") $stage -Force
foreach ($name in @("runtime", "models")) {
    $src = Join-Path $bin $name
    if (Test-Path $src) { Copy-Item $src (Join-Path $stage $name) -Recurse -Force }
}
foreach ($name in @("README.md", "LICENSE", "NOTICE")) {
    $src = Join-Path $root $name
    if (Test-Path $src) { Copy-Item $src $stage -Force }
}
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -CompressionLevel Optimal
Remove-Item $stage -Recurse -Force
Write-Host "发布包: $zip"
