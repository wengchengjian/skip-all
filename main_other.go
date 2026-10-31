//go:build !windows

// 非 Windows 平台默认只提供 CLI 与核心能力；GUI 优先在 Windows 提供。
package main

import (
	"fmt"
	"os"

	"github.com/wengchengjian/skip-all/internal/app"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("SkipAll GUI 当前优先支持 Windows；非 Windows 平台请使用 CLI：")
		fmt.Println("  skip-all list")
		fmt.Println("  skip-all doctor --game zzz")
		os.Exit(0)
	}
	os.Exit(app.RunCLI(os.Args[1:]))
}
