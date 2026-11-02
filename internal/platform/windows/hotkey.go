//go:build windows

package windows

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL  = 13    // WH_KEYBOARD_LL
	wmKeyDown     = 0x100 // WM_KEYDOWN
	wmSysKeyDown  = 0x104 // WM_SYSKEYDOWN
	wmQuit        = 0x012 // WM_QUIT
	llkhfInjected = 0x10  // KBDLLHOOKSTRUCT.flags：事件由 SendInput 等注入
)

var (
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
)

// kbdLLHookStruct 对应 Win32 KBDLLHOOKSTRUCT。
type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// winMsg 对应 Win32 MSG；字段顺序与尺寸必须完整，GetMessageW 会整体写入。
type winMsg struct {
	HWnd    uintptr
	Message uint32
	Pad     uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pad2    uint32
	PtX     int32
	PtY     int32
	Pad3    uint32
}

// Hotkey 是一条快捷键绑定；Accelerator 形如 "F9"、"Ctrl+F11"。
type Hotkey struct {
	Accelerator string
	Callback    func()
}

// HotkeyBinding 是解析后的快捷键。
type HotkeyBinding struct {
	Source   string
	vk       uint32
	ctrl     bool
	alt      bool
	shift    bool
	win      bool
	callback func()
}

var (
	// hotkeyMu 保护绑定表与钩子句柄；钩子回调在独立线程上执行。
	hotkeyMu       sync.Mutex
	hotkeyBindings []*HotkeyBinding
	hotkeyHook     uintptr
	hotkeyThreadID uint32
	hotkeyGate     func() bool
	hotkeyCallback = windows.NewCallback(hotkeyProc)
	hotkeyRunning  atomic.Bool
	hotkeyGateAt   atomic.Int64 // 上次 gate 判定为真的时间（UnixNano）
)

// ParseAccelerator 解析快捷键字符串，非法时返回可读错误供配置校验。
func ParseAccelerator(s string) (*HotkeyBinding, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, fmt.Errorf("快捷键不能为空")
	}
	parts := strings.Split(trimmed, "+")
	binding := &HotkeyBinding{Source: trimmed}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("快捷键 %q 格式错误", trimmed)
		}
		if i == len(parts)-1 {
			// 最后一段必须是真的按键，不能只写修饰键（如 "Ctrl+Shift"）。
			if isModifierName(part) {
				return nil, fmt.Errorf("快捷键 %q 缺少主键", trimmed)
			}
			vk, err := hotkeyVirtualKey(part)
			if err != nil {
				return nil, fmt.Errorf("快捷键 %q: %w", trimmed, err)
			}
			binding.vk = vk
			continue
		}
		switch strings.ToLower(part) {
		case "ctrl", "control", "cmdorctrl":
			binding.ctrl = true
		case "alt", "option":
			binding.alt = true
		case "shift":
			binding.shift = true
		case "win", "super", "meta":
			binding.win = true
		default:
			return nil, fmt.Errorf("快捷键 %q 里的修饰键 %q 不支持", trimmed, part)
		}
	}
	return binding, nil
}

// isModifierName 判断一个名字是不是修饰键。
func isModifierName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ctrl", "control", "cmdorctrl", "alt", "option", "shift", "win", "super", "meta":
		return true
	}
	return false
}

// hotkeyVirtualKey 把按键名转成虚拟键码；复用 input_sendinput.go 里的 keySpecs。
// 额外支持单个字母/数字与 F13–F24（keySpecs 只到 F12），热键常拿这些键避让游戏。
func hotkeyVirtualKey(name string) (uint32, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if spec, ok := keySpecs[key]; ok {
		return uint32(spec.vk), nil
	}
	if len(key) == 1 {
		switch c := key[0]; {
		case c >= 'a' && c <= 'z':
			return uint32(c - 'a' + 'A'), nil // VK 用大写字母
		case c >= '0' && c <= '9':
			return uint32(c), nil // 数字的 ASCII 就是 VK
		}
	}
	if n, err := parseKeyIndex(key); err == nil && n >= 13 && n <= 24 {
		return uint32(0x6F + n), nil
	}
	return 0, fmt.Errorf("不支持的按键 %q", name)
}

// parseKeyIndex 从 "f13" 这类名字里取出序号。
func parseKeyIndex(key string) (int, error) {
	if len(key) < 2 || key[0] != 'f' {
		return 0, fmt.Errorf("不是 F 键")
	}
	n := 0
	for _, c := range key[1:] {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("不是 F 键")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// HotkeyManager 用 WH_KEYBOARD_LL 钩子实现全局快捷键。
// 相比 RegisterHotKey：F12 这类系统保留键也能用，并且能在按键到达其它程序
// 之前判断该不该拦截——只有前台是目标游戏时才吃掉按键。
type HotkeyManager struct {
	gate func() bool
}

// NewHotkeyManager 创建快捷键管理器；gate 返回 false 时快捷键整体失效，
// 按键原样透传给其它程序。
func NewHotkeyManager(gate func() bool) *HotkeyManager {
	return &HotkeyManager{gate: gate}
}

// Start 安装钩子并开始消息循环，重复调用无副作用。
func (m *HotkeyManager) Start() {
	if hotkeyRunning.Swap(true) {
		return
	}
	hotkeyMu.Lock()
	hotkeyGate = m.gate
	hotkeyMu.Unlock()
	go hotkeyLoop()
}

// Set 重新绑定快捷键，返回每条非法绑定的错误；非法项被跳过。
func (m *HotkeyManager) Set(hotkeys []Hotkey) []error {
	var bindings []*HotkeyBinding
	var errs []error
	for _, hk := range hotkeys {
		if strings.TrimSpace(hk.Accelerator) == "" {
			continue
		}
		binding, err := ParseAccelerator(hk.Accelerator)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		binding.callback = hk.Callback
		bindings = append(bindings, binding)
	}
	hotkeyMu.Lock()
	hotkeyBindings = bindings
	// 未 Start 时也记住 gate，方便单元测试与提前绑定。
	if m.gate != nil {
		hotkeyGate = m.gate
	}
	hotkeyMu.Unlock()
	return errs
}

// Stop 卸载钩子并结束消息循环。
func (m *HotkeyManager) Stop() {
	if !hotkeyRunning.Swap(false) {
		return
	}
	hotkeyMu.Lock()
	threadID := hotkeyThreadID
	hook := hotkeyHook
	hotkeyHook = 0
	hotkeyThreadID = 0
	hotkeyMu.Unlock()
	if hook != 0 {
		procUnhookWindowsHookEx.Call(hook)
	}
	if threadID != 0 {
		procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
	}
}

// hotkeyLoop 在独立线程上安装钩子并跑消息循环：低级键盘钩子只在安装它的线程收到回调。
func hotkeyLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	threadID := windows.GetCurrentThreadId()
	hook, _, err := procSetWindowsHookExW.Call(whKeyboardLL, hotkeyCallback, 0, 0)
	if hook == 0 {
		log.Printf("安装键盘钩子失败: %v", err)
		hotkeyRunning.Store(false)
		return
	}
	hotkeyMu.Lock()
	hotkeyHook = hook
	hotkeyThreadID = threadID
	hotkeyMu.Unlock()
	log.Printf("键盘钩子已安装（线程 %d）", threadID)

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0)
		if int32(ret) == 0 || msg.Message == wmQuit {
			return
		}
	}
}

// hotkeyProc 是低级键盘钩子回调：命中绑定时返回 1 吃掉按键，否则放行。
// lParam 直接声明成结构体指针，交给 syscall.NewCallback 做转换，避免 uintptr 转指针。
func hotkeyProc(nCode uintptr, wParam uintptr, info *kbdLLHookStruct) uintptr {
	if int32(nCode) < 0 || info == nil {
		return callNextHook(nCode, wParam, info)
	}
	if wParam != wmKeyDown && wParam != wmSysKeyDown {
		return callNextHook(nCode, wParam, info)
	}
	// 自己注入的按键（前台模式的 SendInput）不参与快捷键判断。
	if info.Flags&llkhfInjected != 0 {
		return callNextHook(nCode, wParam, info)
	}

	hotkeyMu.Lock()
	bindings := hotkeyBindings
	gate := hotkeyGate
	hotkeyMu.Unlock()
	if len(bindings) == 0 || gate == nil {
		return callNextHook(nCode, wParam, info)
	}
	if !gateCached(gate) {
		return callNextHook(nCode, wParam, info)
	}
	for _, binding := range bindings {
		if !binding.matches(info.VkCode) {
			continue
		}
		if binding.callback != nil {
			go binding.callback()
		}
		return 1 // 吃掉这次按键，游戏不会同时收到
	}
	return callNextHook(nCode, wParam, info)
}

// gateCached 缓存 gate 的肯定结果，避免每次按键都枚举进程；否定结果不缓存，
// 这样切回游戏立刻生效。
func gateCached(gate func() bool) bool {
	if time.Since(time.Unix(0, hotkeyGateAt.Load())) < 300*time.Millisecond {
		return true
	}
	if !gate() {
		return false
	}
	hotkeyGateAt.Store(time.Now().UnixNano())
	return true
}

// matches 判断按键是否命中这条绑定（修饰键需精确匹配）。
func (b *HotkeyBinding) matches(vk uint32) bool {
	if b.vk != vk {
		return false
	}
	return b.ctrl == keyDown(0x11) && b.alt == keyDown(0x12) &&
		b.shift == keyDown(0x10) && b.win == keyDown(0x5B, 0x5C)
}

// keyDown 报告任一虚拟键当前是否按下。
func keyDown(vks ...uint32) bool {
	for _, vk := range vks {
		if state, _, _ := procGetAsyncKeyState.Call(uintptr(vk)); state&0x8000 != 0 {
			return true
		}
	}
	return false
}

// callNextHook 把事件交给后续钩子与正常处理。
func callNextHook(nCode, wParam uintptr, info *kbdLLHookStruct) uintptr {
	var raw uintptr
	if info != nil {
		raw = uintptr(unsafe.Pointer(info))
	}
	ret, _, _ := procCallNextHookEx.Call(0, nCode, wParam, raw)
	return ret
}

// ForegroundProcessName 返回前台窗口所属进程的文件名（小写），失败返回空串。
func ForegroundProcessName() string {
	fg := windows.GetForegroundWindow()
	if fg == 0 {
		return ""
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(fg, &pid); err != nil || pid == 0 {
		return ""
	}
	return processNameByPID(pid)
}

// processNameByPID 用 QueryFullProcessImageName 取进程文件名。
func processNameByPID(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, 260)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	full := windows.UTF16ToString(buf[:size])
	if idx := strings.LastIndexAny(full, "\\/"); idx >= 0 {
		full = full[idx+1:]
	}
	return strings.ToLower(full)
}
