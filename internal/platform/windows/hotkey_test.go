//go:build windows

package windows

import "testing"

func TestParseAccelerator(t *testing.T) {
	type mods struct{ ctrl, alt, shift, win bool }
	cases := []struct {
		name  string
		input string
		want  uint32
		mods  mods
	}{
		{"单键 F9", "F9", 0x78, mods{}},
		{"小写 f12", "f12", 0x7B, mods{}},
		{"Home 具名键", "Home", 0x24, mods{}},
		{"字母", "K", 0x4B, mods{}},
		{"数字", "5", 0x35, mods{}},
		{"Ctrl 组合", "Ctrl+F11", 0x7A, mods{ctrl: true}},
		{"多修饰键", "Ctrl+Shift+A", 0x41, mods{ctrl: true, shift: true}},
		{"F13 超出 keySpecs", "F13", 0x7C, mods{}},
		{"Ctrl 与 Control 等价", "Control+Alt+F8", 0x77, mods{ctrl: true, alt: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binding, err := ParseAccelerator(tc.input)
			if err != nil {
				t.Fatalf("ParseAccelerator(%q): %v", tc.input, err)
			}
			if binding.vk != tc.want {
				t.Fatalf("vk = 0x%X, want 0x%X", binding.vk, tc.want)
			}
			if binding.ctrl != tc.mods.ctrl || binding.alt != tc.mods.alt || binding.shift != tc.mods.shift || binding.win != tc.mods.win {
				t.Fatalf("修饰键 = %+v, want %v", binding, tc.mods)
			}
		})
	}
}

func TestParseAcceleratorInvalid(t *testing.T) {
	for _, input := range []string{"", "F99", "Ctrl+", "Hyper+A", "Ctrl+Shift"} {
		if _, err := ParseAccelerator(input); err == nil {
			t.Fatalf("ParseAccelerator(%q) 应报错", input)
		}
	}
}

func TestHotkeyProcMatchesAndSwallows(t *testing.T) {
	var fired int
	manager := NewHotkeyManager(func() bool { return true })
	if errs := manager.Set([]Hotkey{{Accelerator: "F9", Callback: func() { fired++ }}}); len(errs) != 0 {
		t.Fatalf("Set 报错: %v", errs)
	}

	// 命中：返回 1（吃掉按键）并触发回调。
	info := &kbdLLHookStruct{VkCode: 0x78, Flags: 0}
	if got := hotkeyProc(0, wmKeyDown, info); got != 1 {
		t.Fatalf("命中 F9 应返回 1，得到 %d", got)
	}

	// 未命中：返回 0（放行）。
	other := &kbdLLHookStruct{VkCode: 0x41, Flags: 0}
	if got := hotkeyProc(0, wmKeyDown, other); got != 0 {
		t.Fatalf("未命中应返回 0，得到 %d", got)
	}

	// 注入的按键不参与判断，避免自己的模拟输入误触快捷键。
	injected := &kbdLLHookStruct{VkCode: 0x78, Flags: llkhfInjected}
	if got := hotkeyProc(0, wmKeyDown, injected); got != 0 {
		t.Fatalf("注入事件应放行，得到 %d", got)
	}

	// 非按下事件（例如抬起）不触发。
	if got := hotkeyProc(0, 0x101, info); got != 0 {
		t.Fatalf("WM_KEYUP 应放行，得到 %d", got)
	}
}

func TestHotkeyProcRespectsGate(t *testing.T) {
	allowed := false
	manager := NewHotkeyManager(func() bool { return allowed })
	if errs := manager.Set([]Hotkey{{Accelerator: "F9", Callback: func() {}}}); len(errs) != 0 {
		t.Fatalf("Set 报错: %v", errs)
	}
	hotkeyGateAt.Store(0) // 清掉缓存，确保这次真的调用 gate
	info := &kbdLLHookStruct{VkCode: 0x78}
	if got := hotkeyProc(0, wmKeyDown, info); got != 0 {
		t.Fatalf("gate 为假时应放行，得到 %d", got)
	}

	allowed = true
	hotkeyGateAt.Store(0)
	if got := hotkeyProc(0, wmKeyDown, info); got != 1 {
		t.Fatalf("gate 为真时应吃掉按键，得到 %d", got)
	}
}

func TestHotkeyManagerSetSkipsInvalid(t *testing.T) {
	manager := NewHotkeyManager(func() bool { return true })
	errs := manager.Set([]Hotkey{
		{Accelerator: "F9", Callback: func() {}},
		{Accelerator: "Bad+Key", Callback: func() {}},
		{Accelerator: "  ", Callback: func() {}},
	})
	if len(errs) != 1 {
		t.Fatalf("应只有一条非法绑定报错，得到 %v", errs)
	}
	hotkeyMu.Lock()
	count := len(hotkeyBindings)
	hotkeyMu.Unlock()
	if count != 1 {
		t.Fatalf("合法绑定数 = %d, want 1", count)
	}
}
