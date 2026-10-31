// Package wuwaassets 内嵌鸣潮的画面配置与模板图片。
package wuwaassets

import (
	"embed"
	"io/fs"
)

//go:embed screens templates
var files embed.FS

// FS 返回游戏资源文件系统，根目录包含 screens/ 与 templates/。
func FS() fs.FS { return files }
