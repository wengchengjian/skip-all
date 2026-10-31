//go:build !windows

package platform

// DiscreteGPUIndex 在非 Windows 平台返回默认序号。
func DiscreteGPUIndex() (int, string) { return 0, "" }
