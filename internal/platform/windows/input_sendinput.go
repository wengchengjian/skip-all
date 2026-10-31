//go:build windows

package windows

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/wengchengjian/skip-all/pkg/geom"
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	wmActivate    = 0x0006
	waActive      = 1
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	mkLButton     = 0x0001

	mouseEventLeftDown   = 0x0002
	mouseEventLeftUp     = 0x0004
	mouseEventRightDown  = 0x0008
	mouseEventRightUp    = 0x0010
	mouseEventMiddleDown = 0x0020
	mouseEventMiddleUp   = 0x0040

	keyEventExtended = 0x0001
	keyEventKeyUp    = 0x0002

	clickHoldTime = 30 * time.Millisecond
	keyHoldTime   = 15 * time.Millisecond
)

var (
	procSendInput    = user32.NewProc("SendInput")
	procSetCursorPos = user32.NewProc("SetCursorPos")
	procGetCursorPos = user32.NewProc("GetCursorPos")
	procPostMessage  = user32.NewProc("PostMessageW")
	procSendMessage  = user32.NewProc("SendMessageW")
)

// mouseInput 对应 Win32 MOUSEINPUT。
type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	_           uint32 // 对齐 dwExtraInfo
	dwExtraInfo uintptr
}

// keyboardInput 对应 Win32 KEYBDINPUT。
type keyboardInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// inputEvent 对应 Win32 INPUT。data 与 mouseInput/keyboardInput 共用同一段内存。
// 用 uint64 数组保证 64 位下按 8 字节对齐，32 位下与 Win32 结构体保持一致。
const mouseInputSize = unsafe.Sizeof(mouseInput{})

type inputEvent struct {
	typ  uint32
	data [mouseInputSize / 8]uint64
}

// keySpec 是一个按键的虚拟键码及是否为扩展键。
type keySpec struct {
	vk       uint16
	extended bool
}

var keySpecs = func() map[string]keySpec {
	m := map[string]keySpec{
		"return":    {0x0D, false},
		"enter":     {0x0D, false},
		"escape":    {0x1B, false},
		"esc":       {0x1B, false},
		"backspace": {0x08, false},
		"tab":       {0x09, false},
		"spacebar":  {0x20, false},
		"space":     {0x20, false},
		"left":      {0x25, true},
		"up":        {0x26, true},
		"right":     {0x27, true},
		"down":      {0x28, true},
		"home":      {0x24, true},
		"end":       {0x23, true},
		"pageup":    {0x21, true},
		"pagedown":  {0x22, true},
		"insert":    {0x2D, true},
		"delete":    {0x2E, true},
		"shift":     {0x10, false},
		"lshift":    {0xA0, false},
		"rshift":    {0xA1, false},
		"ctrl":      {0x11, false},
		"control":   {0x11, false},
		"lctrl":     {0xA2, false},
		"rctrl":     {0xA3, true},
		"alt":       {0x12, false},
		"lalt":      {0xA4, false},
		"ralt":      {0xA5, true},
		"win":       {0x5B, true},
		"lwin":      {0x5B, true},
		"rwin":      {0x5C, true},
	}
	for i := 1; i <= 12; i++ {
		m[fmt.Sprintf("f%d", i)] = keySpec{vk: uint16(0x6F + i)}
	}
	return m
}()

// 精确点击时的光标策略。
const (
	CursorAuto   = "auto"   // 用户空闲时短暂移动光标，点击后尽量归位
	CursorAlways = "always" // 总是移动光标
	CursorNever  = "never"  // 只发 PostMessage，不移动光标
)

// InputOptions 控制输入方式。
type InputOptions struct {
	// Background 为 true 时使用 PostMessage 向窗口后台发送输入，不激活窗口。
	Background bool
	// Gamepad 为 true 时尝试创建 ViGEm 虚拟 Xbox 手柄。
	Gamepad bool
	// GamepadDLL 是 ViGEmClient.dll 的路径；为空时使用 assets/runtime/ViGEmClient.dll。
	GamepadDLL string
	// CursorMode 控制精确点击是否短暂移动物理光标；空值等同 auto。
	CursorMode string
}

// Input 通过前台 SendInput 或后台 PostMessage 向游戏窗口发送输入。
type Input struct {
	mu         sync.Mutex
	win        *Window
	activate   func() error
	pressed    map[uint16]bool // 记录按下未释放的键，值为是否扩展键
	background bool
	cursorMode string
	gamepad    *XboxGamepad
	gamepadErr error
}

// 编译期断言：Input 可直接作为 engine.Input 使用。
var _ interface {
	Click(geom.Point) error
	ClickCurrent() error
	KeyTap(string) error
	ReleaseAll() error
} = (*Input)(nil)

// NewInput 创建前台输入控制器。点击前会通过窗口的 Activate 确保游戏在前台。
func NewInput(w *Window) *Input {
	return NewInputWithOptions(w, InputOptions{})
}

// NewInputWithOptions 创建输入控制器。默认构造为前台模式；后台模式请传 Background=true。
func NewInputWithOptions(w *Window, opts InputOptions) *Input {
	mode := strings.ToLower(opts.CursorMode)
	switch mode {
	case CursorAuto, CursorAlways, CursorNever:
	default:
		mode = CursorAuto
	}
	in := &Input{win: w, pressed: make(map[uint16]bool), background: opts.Background, cursorMode: mode}
	if w != nil {
		in.activate = w.Activate
	}
	if opts.Gamepad {
		dll := opts.GamepadDLL
		if dll == "" && w != nil {
			dll = "ViGEmClient.dll"
		}
		g, err := NewXboxGamepad(dll)
		if err != nil {
			in.gamepadErr = err
		} else {
			in.gamepad = g
		}
	}
	return in
}

// GamepadAvailable 报告虚拟手柄是否可用。
func (i *Input) GamepadAvailable() bool {
	return i != nil && i.gamepad != nil && i.gamepad.Available()
}

// GamepadError 返回虚拟手柄初始化错误。
func (i *Input) GamepadError() error {
	if i == nil {
		return nil
	}
	return i.gamepadErr
}

// SetActivateFunc 替换点击前的窗口激活动作，主要供测试使用。
func (i *Input) SetActivateFunc(fn func() error) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.activate = fn
}

// Click 把 1920x1080 逻辑坐标换算成客户区坐标后点击。
func (i *Input) Click(p geom.Point) error {
	if i == nil || i.win == nil {
		return ErrWindowNotFound
	}
	if i.background && !i.win.IsActive() {
		// 游戏不在前台时不做任何光标/点击动作，避免把点击发到覆盖它的其他应用。
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	rect, err := i.win.ClientRect()
	if err != nil {
		return err
	}
	pt, err := scalePoint(p, rect.Width(), rect.Height())
	if err != nil {
		return err
	}
	if i.background {
		return i.backgroundClickAtLocked(pt, i.shouldMoveCursor())
	}
	if err := i.ensureActiveLocked(); err != nil {
		return err
	}
	return i.clickLocked(rect.X1+pt.X, rect.Y1+pt.Y)
}

// ClickCurrent 在鼠标当前位置点击，不移动光标。
func (i *Input) ClickCurrent() error {
	if i == nil || i.win == nil {
		return ErrWindowNotFound
	}
	if i.background && !i.win.IsActive() {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.background {
		rect, err := i.win.ClientRect()
		if err != nil {
			return err
		}
		// 对话点击也走真实光标：绝区零忽略 lParam，只有光标在窗口内才生效。
		return i.backgroundClickAtLocked(geom.Point{X: rect.Width() / 2, Y: rect.Height() / 2}, i.shouldMoveCursor())
	}
	if err := i.ensureActiveLocked(); err != nil {
		return err
	}
	var pt winPoint
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return fmt.Errorf("读取鼠标位置失败: %w", lastErr("GetCursorPos"))
	}
	return i.clickLocked(int(pt.X), int(pt.Y))
}

// KeyTap 发送一次按键（按下并释放）。
func (i *Input) KeyTap(key string) error {
	if i == nil || i.win == nil {
		return ErrWindowNotFound
	}
	if i.background && !i.win.IsActive() {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(key) > 5 && key[:5] == "xbox_" {
		if i.gamepad == nil {
			if i.gamepadErr != nil {
				return i.gamepadErr
			}
			return errors.New("虚拟手柄未启用")
		}
		return i.gamepad.Tap(key)
	}
	if i.background {
		vk, _, err := virtualKey(key)
		if err != nil {
			return err
		}
		return i.backgroundKeyTapLocked(vk)
	}
	if err := i.ensureActiveLocked(); err != nil {
		return err
	}
	vk, extended, err := virtualKey(key)
	if err != nil {
		return err
	}
	if err := sendKeyEvent(vk, extended, false); err != nil {
		return err
	}
	time.Sleep(keyHoldTime)
	if err := sendKeyEvent(vk, extended, true); err != nil {
		i.pressed[vk] = extended
		return err
	}
	delete(i.pressed, vk)
	return nil
}

// ReleaseAll 释放鼠标左右键和仍被按住的按键，会话结束时调用。
func (i *Input) ReleaseAll() error {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	var firstErr error
	if i.gamepad != nil {
		if err := i.gamepad.Reset(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if i.background {
		for vk := range i.pressed {
			_ = i.backgroundKeyUpLocked(vk)
			delete(i.pressed, vk)
		}
		return firstErr
	}
	for _, flags := range []uint32{mouseEventLeftUp, mouseEventRightUp, mouseEventMiddleUp} {
		if err := sendMouseEvent(flags, 0, 0); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for vk, extended := range i.pressed {
		if err := sendKeyEvent(vk, extended, true); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(i.pressed, vk)
	}
	return firstErr
}

// Close 释放虚拟手柄等资源。
func (i *Input) Close() error {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.gamepad != nil {
		err := i.gamepad.Close()
		i.gamepad = nil
		return err
	}
	return nil
}

func (i *Input) ensureActiveLocked() error {
	if i.win.IsActive() {
		return nil
	}
	if i.activate == nil {
		return errors.New("未设置窗口激活函数")
	}
	if err := i.activate(); err != nil {
		return fmt.Errorf("激活游戏窗口: %w", err)
	}
	return nil
}

func (i *Input) clickLocked(x, y int) error {
	if r, _, _ := procSetCursorPos.Call(uintptr(x), uintptr(y)); r == 0 {
		return fmt.Errorf("移动鼠标失败: %w", lastErr("SetCursorPos"))
	}
	time.Sleep(10 * time.Millisecond)
	if err := sendMouseEvent(mouseEventLeftDown, 0, 0); err != nil {
		return err
	}
	time.Sleep(clickHoldTime)
	if err := sendMouseEvent(mouseEventLeftUp, 0, 0); err != nil {
		_ = sendMouseEvent(mouseEventLeftUp, 0, 0)
		return err
	}
	return nil
}

// backgroundClickAtLocked 在后台点击客户区 (x,y)；moveCursor 为 true 时短暂移动并归位光标。
func (i *Input) backgroundClickAtLocked(p geom.Point, moveCursor bool) error {
	if i == nil || i.win == nil {
		return ErrWindowNotFound
	}
	hwnd := i.win.Handle()
	if hwnd == 0 {
		return ErrWindowNotFound
	}
	if moveCursor {
		return i.cursorClickLocked(hwnd, p)
	}
	return i.postClickLocked(hwnd, p)
}

// postClickLocked 只发窗口消息；部分游戏会忽略 lParam，改用当前光标位置。
func (i *Input) postClickLocked(hwnd uintptr, p geom.Point) error {
	lparam := uintptr(uint16(p.X)) | uintptr(uint16(p.Y))<<16
	if err := postMessage(hwnd, wmActivate, waActive, 0); err != nil {
		return err
	}
	_ = postMessage(hwnd, wmMouseMove, 0, lparam)
	time.Sleep(5 * time.Millisecond)
	if err := postMessage(hwnd, wmLButtonDown, mkLButton, lparam); err != nil {
		return err
	}
	time.Sleep(clickHoldTime)
	if err := postMessage(hwnd, wmLButtonUp, 0, lparam); err != nil {
		_ = postMessage(hwnd, wmLButtonUp, 0, lparam)
		return err
	}
	return nil
}

// cursorClickLocked 把光标移到目标、点击、再尽量归位；ZZZ 的后台点击依赖真实光标位置。
func (i *Input) cursorClickLocked(hwnd uintptr, p geom.Point) error {
	rect, err := i.win.ClientRect()
	if err != nil {
		return err
	}
	targetX, targetY := rect.X1+p.X, rect.Y1+p.Y
	saved, hasSaved := cursorPos()
	if r, _, _ := procSetCursorPos.Call(uintptr(targetX), uintptr(targetY)); r == 0 {
		return fmt.Errorf("移动鼠标失败: %w", lastErr("SetCursorPos"))
	}
	// SetCursorPos 在混合 DPI 下可能被系统微调，归位判断用实际落点。
	actual, hasActual := cursorPos()
	if !hasActual {
		actual = winPoint{X: int32(targetX), Y: int32(targetY)}
	}
	defer func() {
		if !hasSaved {
			return
		}
		if cur, ok := cursorPos(); ok && absInt(int(cur.X)-int(actual.X)) <= 2 && absInt(int(cur.Y)-int(actual.Y)) <= 2 {
			_, _, _ = procSetCursorPos.Call(uintptr(saved.X), uintptr(saved.Y))
		}
	}()
	time.Sleep(10 * time.Millisecond)
	// 与上游一致：先同步激活窗口，再 PostMessage 鼠标事件；
	// 绝区零后台点击会忽略 lParam 坐标，依赖上面 SetCursorPos 的真实位置。
	_, _, _ = procSendMessage.Call(hwnd, wmActivate, waActive, 0)
	time.Sleep(10 * time.Millisecond)
	if err := postMessage(hwnd, wmLButtonDown, mkLButton, 0); err != nil {
		return err
	}
	time.Sleep(clickHoldTime)
	if err := postMessage(hwnd, wmLButtonUp, 0, 0); err != nil {
		return err
	}
	time.Sleep(25 * time.Millisecond)
	return nil
}

func (i *Input) shouldMoveCursor() bool {
	if i == nil {
		return false
	}
	switch i.cursorMode {
	case CursorNever:
		return false
	case CursorAlways:
		return true
	default:
		// 绝区零的后台点击依赖真实光标位置，lParam 会被忽略。
		// auto 也必须每次短暂移光标再归位，否则用户一动鼠标跳过就失效。
		return true
	}
}

func cursorPos() (winPoint, bool) {
	var pt winPoint
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return winPoint{}, false
	}
	return pt, true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (i *Input) backgroundKeyTapLocked(vk uint16) error {
	if i == nil || i.win == nil {
		return ErrWindowNotFound
	}
	if err := postMessage(i.win.Handle(), 0x0100, uintptr(vk), 1); err != nil {
		return err
	}
	i.pressed[vk] = false
	time.Sleep(keyHoldTime)
	return i.backgroundKeyUpLocked(vk)
}

func (i *Input) backgroundKeyUpLocked(vk uint16) error {
	if i == nil || i.win == nil {
		return ErrWindowNotFound
	}
	return postMessage(i.win.Handle(), 0x0101, uintptr(vk), 0xC0000001)
}

func postMessage(hwnd uintptr, msg, wparam, lparam uintptr) error {
	r, _, _ := procPostMessage.Call(hwnd, msg, wparam, lparam)
	if r == 0 {
		return fmt.Errorf("PostMessage(0x%X) 失败: %w", msg, lastErr("PostMessageW"))
	}
	return nil
}

// scalePoint 把 1920x1080 逻辑坐标换算成客户区物理坐标。
func scalePoint(p geom.Point, width, height int) (geom.Point, error) {
	if width <= 0 || height <= 0 {
		return geom.Point{}, fmt.Errorf("客户区尺寸无效: %dx%d", width, height)
	}
	if p.X < 0 || p.X >= standardWidth || p.Y < 0 || p.Y >= standardHeight {
		return geom.Point{}, fmt.Errorf("坐标 (%d,%d) 超出 1920x1080 逻辑范围", p.X, p.Y)
	}
	x := int(math.Round(float64(p.X) * float64(width) / standardWidth))
	y := int(math.Round(float64(p.Y) * float64(height) / standardHeight))
	if x >= width {
		x = width - 1
	}
	if y >= height {
		y = height - 1
	}
	return geom.Point{X: x, Y: y}, nil
}

// virtualKey 解析按键名称为虚拟键码。
func virtualKey(name string) (uint16, bool, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if spec, ok := keySpecs[key]; ok {
		return spec.vk, spec.extended, nil
	}
	if len(key) == 1 {
		c := key[0]
		switch {
		case c >= 'a' && c <= 'z':
			return uint16(c - 'a' + 'A'), false, nil
		case c >= '0' && c <= '9':
			return uint16(c), false, nil
		}
	}
	return 0, false, fmt.Errorf("不支持的按键 %q", name)
}

// sendMouseEvent 发送鼠标事件。
func sendMouseEvent(flags uint32, dx, dy int32) error {
	var event inputEvent
	event.typ = inputMouse
	mi := (*mouseInput)(unsafe.Pointer(&event.data[0]))
	mi.dx = dx
	mi.dy = dy
	mi.dwFlags = flags
	return sendInputEvent(&event)
}

// sendKeyEvent 发送键盘事件，up 为 true 时发送 KEYUP。
func sendKeyEvent(vk uint16, extended, up bool) error {
	var event inputEvent
	event.typ = inputKeyboard
	ki := (*keyboardInput)(unsafe.Pointer(&event.data[0]))
	ki.wVk = vk
	if extended {
		ki.dwFlags |= keyEventExtended
	}
	if up {
		ki.dwFlags |= keyEventKeyUp
	}
	return sendInputEvent(&event)
}

func sendInputEvent(event *inputEvent) error {
	r, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(event)), unsafe.Sizeof(*event))
	if r != 1 {
		return fmt.Errorf("SendInput 失败: %w", lastErr("SendInput"))
	}
	return nil
}
