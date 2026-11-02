# 自动更新

启动后默认延迟约 4 秒检查 GitHub latest release，发现新版本会自动下载；是否安装由用户在设置页确认。

配置项（`config.yaml`）：

```yaml
update:
  auto_check: true      # 启动时检查
  auto_download: true   # 发现新版本后后台下载，安装仍需点击
  repo: wengchengjian/skip-all
```

设置页的“更新”区域可以改这两项开关，也能手动检查、下载、安装。

## 版本缓存

下载目录是 `%APPDATA%\skip-all\updates\<版本>\`，每个版本一个子目录：

- 下载中的文件叫 `skip-all-windows-amd64.exe.part`，校验通过后改名。
- 本地存在多个版本时，安装永远挑版本号最大的那个；离线也能安装已下载的包。
- 检查结果里的 `downloaded` 表示该版本已经下载完成。

## 下载

检查器使用当前运行平台（runtime.GOOS/runtime.GOARCH）选择 release 附件。下载用固定 `.part` 文件 + HTTP Range 断点续传：网络中断会保留已下载字节，下次从断点继续；只有 sha256 校验失败才删除。

## 安装

客户端通过隐藏的 PowerShell 辅助脚本等待当前进程退出，替换 exe 并重启，日志写到 `%TEMP%\skip-all-update.log`。更新检查失败只写日志，不阻塞启动。

## 发布

`.github/workflows/release.yml` 在推送 `v*` tag 时构建 Windows exe，生成 `skip-all-windows-amd64.exe`、zip 和 `checksums.txt`。

真实发布链路可以用 manual tag 测试：

```powershell
$env:SKIP_ALL_UPDATE_LIVE='1'; $env:SKIP_ALL_UPDATE_DOWNLOAD='1'; $env:SKIP_ALL_UPDATE_RESUME='1'
go test -tags manual ./internal/update -run TestLiveRelease -v
```
