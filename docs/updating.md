# 自动更新

启动后默认延迟约 4 秒检查 GitHub latest release；设置页的“更新”区域也可以手动检查。

配置项（`config.yaml`）：

```yaml
update:
  auto_check: true
  repo: wengchengjian/skip-all
```

发布流程：`.github/workflows/release.yml` 在推送 `v*` tag 时构建 Windows exe，生成 `skip-all-windows-amd64.exe`、zip 和 `checksums.txt`。

真实发布链路可以用 manual tag 测试：

```powershell
$env:SKIP_ALL_UPDATE_LIVE='1'; $env:SKIP_ALL_UPDATE_DOWNLOAD='1'; $env:SKIP_ALL_UPDATE_RESUME='1'
go test -tags manual ./internal/update -run TestLiveRelease -v
```

检查器默认使用当前运行平台（runtime.GOOS/runtime.GOARCH）选择 release 附件；下载使用固定 .part 文件和 HTTP Range 断点续传，网络中断后下一次从已下载字节继续。

客户端会下载 exe、校验 sha256，然后通过隐藏的 PowerShell 辅助脚本等待当前进程退出、替换 exe 并重启。更新检查失败只写日志，不阻塞启动。