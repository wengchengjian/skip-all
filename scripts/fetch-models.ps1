# 下载 PP-OCRv6 模型与 onnxruntime.dll 到 assets/，并校验 SHA256。
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$modelDir = Join-Path $root "assets\models\ppocrv6"
$runtimeDir = Join-Path $root "assets\runtime"
$cacheDir = Join-Path $root ".cache\models"
New-Item -ItemType Directory -Force -Path $modelDir, $runtimeDir, $cacheDir | Out-Null

$zipUrl = "https://github.com/OneDragon-Anything/OneDragon-Env/releases/download/ppocrv6/ppocrv6.zip"
$zipHash = "ABE2AD83E8B684F4C905619E2271475D921683526DC3B6AB5448D8012F8E24C9"
$zipPath = Join-Path $cacheDir "ppocrv6.zip"
$dllUrl = "https://modelscope.cn/models/getcharzp/go-ocr/resolve/master/lib/onnxruntime.dll"
$dllHash = "DEC964AB1EE36CC9B0AE247D13B376627992FC57DEC0454354017AB8FD84F1EA"
$dllPath = Join-Path $runtimeDir "onnxruntime.dll"

function Get-CheckedFile([string]$Url, [string]$Path, [string]$Hash) {
    if ((Test-Path $Path) -and ((Get-FileHash $Path -Algorithm SHA256).Hash -eq $Hash)) {
        Write-Host "已存在且校验通过: $Path"
        return
    }
    Write-Host "下载 $Url"
    Invoke-WebRequest -Uri $Url -OutFile $Path
    $actual = (Get-FileHash $Path -Algorithm SHA256).Hash
    if ($actual -ne $Hash) {
        Remove-Item $Path -Force
        throw "SHA256 不匹配: $Path 期望 $Hash 实际 $actual"
    }
}

Get-CheckedFile $zipUrl $zipPath $zipHash
Get-CheckedFile $dllUrl $dllPath $dllHash

$extractDir = Join-Path $cacheDir "ppocrv6"
if (Test-Path $extractDir) { Remove-Item $extractDir -Recurse -Force }
Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force
Get-ChildItem $extractDir -File | Where-Object { $_.Name -ne "simfang.ttf" } | Copy-Item -Destination $modelDir -Force
Copy-Item $dllPath (Join-Path $modelDir "onnxruntime.dll") -Force

Write-Host "模型已就绪: $modelDir"
Write-Host "运行库已就绪: $dllPath"
