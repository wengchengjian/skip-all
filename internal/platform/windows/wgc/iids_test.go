//go:build windows

package wgc

import (
	"fmt"
	"testing"
)

// TestStaticsIIDs 校验硬编码的 statics IID 与系统实际报告的一致。
//
// 这些 IID 是运行时用 GetIids 查出来再固化的；如果哪天 Windows 改了，
// 这个测试会先失败，而不是等到捕获时报 E_NOINTERFACE。
func TestStaticsIIDs(t *testing.T) {
	if err := RoInitialize(); err != nil {
		t.Fatal(err)
	}
	defer RoUninitialize()

	const iidInspectable = "AF86E2E0-B12D-4C6A-9C5A-D7AA65101E90"
	f, err := activationFactory("Windows.Graphics.Capture.Direct3D11CaptureFramePool", iidInspectable)
	if err != nil {
		t.Fatalf("取帧池激活工厂失败: %v", err)
	}
	defer f.release()

	ids, err := getIids(f)
	if err != nil {
		t.Fatalf("GetIids: %v", err)
	}
	found := false
	for _, id := range ids {
		fmt.Printf("IID|%s\n", id.String())
		if id.String() == iidFramePoolStatics {
			found = true
		}
	}
	if !found {
		t.Fatalf("没有找到帧池 statics IID %s", iidFramePoolStatics)
	}
}
