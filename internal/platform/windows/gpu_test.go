//go:build windows

package windows

import "testing"

// TestDiscreteGPUIndexSmoke 验证 DXGI 枚举路径不会因为 COM vtable 索引错误而崩溃。
// 没有独显的机器返回 (0, "") 也算通过；有独显时记录实际序号与名称。
func TestDiscreteGPUIndexSmoke(t *testing.T) {
	index, name := DiscreteGPUIndex()
	if index < 0 {
		t.Fatalf("DiscreteGPUIndex index = %d, want >= 0", index)
	}
	t.Logf("DiscreteGPUIndex = %d, %q", index, name)
}
