//go:build !windows

package update

import "errors"

// Apply 目前只支持 Windows 自更新。
func Apply(source, destination string) error {
	return errors.New("当前平台不支持自动更新")
}
