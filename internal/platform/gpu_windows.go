//go:build windows

package platform

import win "github.com/wengchengjian/skip-all/internal/platform/windows"

// DiscreteGPUIndex 返回 DXGI 枚举中的独立显卡序号。
func DiscreteGPUIndex() (int, string) { return win.DiscreteGPUIndex() }
