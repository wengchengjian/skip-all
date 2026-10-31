// Command skip-all 是 skip-all 的 CLI 入口；无参数时请使用根目录的 Wails GUI 二进制。
package main

import (
	"os"

	"github.com/wengchengjian/skip-all/internal/app"
)

func main() {
	os.Exit(app.RunCLI(os.Args[1:]))
}
