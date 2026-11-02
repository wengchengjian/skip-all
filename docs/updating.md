# 自动更新

启动后默认延迟约 4 秒检查 GitHub latest release；设置页的“更新”区域也可以手动检查。

配置项（`config.yaml`）：

```yaml
update:
  auto_check: true
  repo: wengchengjian/skip-all
```

发布流程：`.github/workflows/release.yml` 在推送 `v*` tag 时构建 Windows exe，生成 `skip-all-windows-amd64.exe`、zip 和 `checksums.txt`。

客户端会下载 exe、校验 sha256，然后通过隐藏的 PowerShell 辅助脚本等待当前进程退出、替换 exe 并重启。更新检查失败只写日志，不阻塞启动。