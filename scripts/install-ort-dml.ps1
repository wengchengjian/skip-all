# 安装 Windows DirectML 版 ONNX Runtime（供 OCR 的 DirectML 后端使用）。
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$dst = Join-Path $root "assets\runtime"
$tmp = Join-Path $root ".research\ortdml-install"
$url = "https://files.pythonhosted.org/packages/be/90/99566dc6398028e7691a5b12720fd85f757a0901818b84599d28abb3f085/onnxruntime_directml-1.24.4-cp311-cp311-win_amd64.whl"

New-Item -ItemType Directory -Force -Path $dst, $tmp | Out-Null
$whl = Join-Path $tmp "dml.whl"
$zip = Join-Path $tmp "dml.zip"
Invoke-WebRequest -Uri $url -OutFile $whl
Copy-Item $whl $zip -Force
Expand-Archive -Path $zip -DestinationPath (Join-Path $tmp "x") -Force
$capi = Join-Path $tmp "x\onnxruntime\capi"
Copy-Item (Join-Path $capi "onnxruntime.dll") (Join-Path $dst "onnxruntime_dml.dll") -Force
Copy-Item (Join-Path $capi "onnxruntime_providers_shared.dll") $dst -Force
Copy-Item (Join-Path $capi "DirectML.dll") $dst -Force
Remove-Item $tmp -Recurse -Force
Write-Host "已安装 DirectML OCR 运行库到 $dst"
