//go:build windows

package windows

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Xbox 360 虚拟手柄的 ViGEm 错误码。
const vigemErrorNone uintptr = 0x20000000

// XUSB_BUTTON 位。
const (
	xusbDpadUp        = 0x0001
	xusbDpadDown      = 0x0002
	xusbDpadLeft      = 0x0004
	xusbDpadRight     = 0x0008
	xusbStart         = 0x0010
	xusbBack          = 0x0020
	xusbLeftThumb     = 0x0040
	xusbRightThumb    = 0x0080
	xusbLeftShoulder  = 0x0100
	xusbRightShoulder = 0x0200
	xusbGuide         = 0x0400
	xusbA             = 0x1000
	xusbB             = 0x2000
	xusbX             = 0x4000
	xusbY             = 0x8000
)

// xusbReport 对应 ViGEm 的 XUSB_REPORT。
type xusbReport struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

// XboxGamepad 是一个 ViGEmBus 虚拟 Xbox 360 手柄。
type XboxGamepad struct {
	mu     sync.Mutex
	dll    *windows.LazyDLL
	client uintptr
	target uintptr
	report xusbReport
	closed bool

	alloc        *windows.LazyProc
	free         *windows.LazyProc
	connect      *windows.LazyProc
	disconnect   *windows.LazyProc
	targetAlloc  *windows.LazyProc
	targetFree   *windows.LazyProc
	targetAdd    *windows.LazyProc
	targetRemove *windows.LazyProc
	targetUpdate *windows.LazyProc
}

// NewXboxGamepad 加载 ViGEmClient.dll 并创建一个虚拟 Xbox 360 手柄。
func NewXboxGamepad(dllPath string) (*XboxGamepad, error) {
	if dllPath == "" {
		return nil, errors.New("ViGEmClient.dll 路径为空")
	}
	dll := windows.NewLazyDLL(dllPath)
	if err := dll.Load(); err != nil {
		return nil, fmt.Errorf("加载 ViGEmClient.dll: %w", err)
	}
	g := &XboxGamepad{
		dll:          dll,
		alloc:        dll.NewProc("vigem_alloc"),
		free:         dll.NewProc("vigem_free"),
		connect:      dll.NewProc("vigem_connect"),
		disconnect:   dll.NewProc("vigem_disconnect"),
		targetAlloc:  dll.NewProc("vigem_target_x360_alloc"),
		targetFree:   dll.NewProc("vigem_target_free"),
		targetAdd:    dll.NewProc("vigem_target_add"),
		targetRemove: dll.NewProc("vigem_target_remove"),
		targetUpdate: dll.NewProc("vigem_target_x360_update"),
	}
	client, _, _ := g.alloc.Call()
	if client == 0 {
		return nil, errors.New("vigem_alloc 返回空指针")
	}
	g.client = client
	if err := g.call("vigem_connect", g.connect, client); err != nil {
		g.free.Call(client)
		return nil, err
	}
	target, _, _ := g.targetAlloc.Call()
	if target == 0 {
		g.disconnect.Call(client)
		g.free.Call(client)
		return nil, errors.New("vigem_target_x360_alloc 返回空指针")
	}
	g.target = target
	if err := g.call("vigem_target_add", g.targetAdd, client, target); err != nil {
		g.targetFree.Call(target)
		g.disconnect.Call(client)
		g.free.Call(client)
		return nil, err
	}
	return g, nil
}

// Available 报告虚拟手柄是否可用。
func (g *XboxGamepad) Available() bool { return g != nil && !g.closed }

// Tap 按一次手柄按键，例如 xbox_a、xbox_dpad_up、xbox_start。
func (g *XboxGamepad) Tap(key string) error {
	if g == nil || g.closed {
		return errors.New("虚拟手柄未初始化")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.applyKeyLocked(key, true); err != nil {
		return err
	}
	if err := g.updateLocked(); err != nil {
		return err
	}
	time.Sleep(35 * time.Millisecond)
	if err := g.applyKeyLocked(key, false); err != nil {
		return err
	}
	return g.updateLocked()
}

// Reset 把报告恢复为中立状态。
func (g *XboxGamepad) Reset() error {
	if g == nil || g.closed {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.report = xusbReport{}
	return g.updateLocked()
}

// Close 移除虚拟手柄并断开 ViGEmBus。
func (g *XboxGamepad) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	g.closed = true
	var firstErr error
	if g.target != 0 {
		if err := g.call("vigem_target_remove", g.targetRemove, g.client, g.target); err != nil && firstErr == nil {
			firstErr = err
		}
		g.targetFree.Call(g.target)
		g.target = 0
	}
	if g.client != 0 {
		g.disconnect.Call(g.client)
		g.free.Call(g.client)
		g.client = 0
	}
	return firstErr
}

func (g *XboxGamepad) applyKeyLocked(key string, pressed bool) error {
	key = normalizePadKey(key)
	if key == "" {
		return fmt.Errorf("不支持的手柄按键")
	}
	press := func(bit uint16) {
		if pressed {
			g.report.Buttons |= bit
		} else {
			g.report.Buttons &^= bit
		}
	}
	switch key {
	case "xbox_a":
		press(xusbA)
	case "xbox_b":
		press(xusbB)
	case "xbox_x":
		press(xusbX)
	case "xbox_y":
		press(xusbY)
	case "xbox_lb":
		press(xusbLeftShoulder)
	case "xbox_rb":
		press(xusbRightShoulder)
	case "xbox_l_thumb":
		press(xusbLeftThumb)
	case "xbox_r_thumb":
		press(xusbRightThumb)
	case "xbox_dpad_up":
		press(xusbDpadUp)
	case "xbox_dpad_down":
		press(xusbDpadDown)
	case "xbox_dpad_left":
		press(xusbDpadLeft)
	case "xbox_dpad_right":
		press(xusbDpadRight)
	case "xbox_start":
		press(xusbStart)
	case "xbox_back":
		press(xusbBack)
	case "xbox_guide":
		press(xusbGuide)
	case "xbox_lt":
		if pressed {
			g.report.LeftTrigger = 255
		} else {
			g.report.LeftTrigger = 0
		}
	case "xbox_rt":
		if pressed {
			g.report.RightTrigger = 255
		} else {
			g.report.RightTrigger = 0
		}
	case "xbox_ls_up":
		g.setStick(pressed, &g.report.ThumbLX, &g.report.ThumbLY, 0, 32767)
	case "xbox_ls_down":
		g.setStick(pressed, &g.report.ThumbLX, &g.report.ThumbLY, 0, -32768)
	case "xbox_ls_left":
		g.setStick(pressed, &g.report.ThumbLX, &g.report.ThumbLY, -32768, 0)
	case "xbox_ls_right":
		g.setStick(pressed, &g.report.ThumbLX, &g.report.ThumbLY, 32767, 0)
	case "xbox_rs_up":
		g.setStick(pressed, &g.report.ThumbRX, &g.report.ThumbRY, 0, 32767)
	case "xbox_rs_down":
		g.setStick(pressed, &g.report.ThumbRX, &g.report.ThumbRY, 0, -32768)
	case "xbox_rs_left":
		g.setStick(pressed, &g.report.ThumbRX, &g.report.ThumbRY, -32768, 0)
	case "xbox_rs_right":
		g.setStick(pressed, &g.report.ThumbRX, &g.report.ThumbRY, 32767, 0)
	default:
		return fmt.Errorf("不支持的手柄按键 %q", key)
	}
	return nil
}

func (g *XboxGamepad) setStick(pressed bool, x, y *int16, xv, yv int16) {
	if pressed {
		*x, *y = xv, yv
	} else {
		*x, *y = 0, 0
	}
}

func (g *XboxGamepad) updateLocked() error {
	if g.closed || g.client == 0 || g.target == 0 {
		return errors.New("虚拟手柄已关闭")
	}
	return g.call("vigem_target_x360_update", g.targetUpdate, g.client, g.target, uintptr(unsafe.Pointer(&g.report)))
}

func (g *XboxGamepad) call(api string, proc *windows.LazyProc, args ...uintptr) error {
	result, _, _ := proc.Call(args...)
	if uint32(result) == uint32(vigemErrorNone) {
		return nil
	}
	return fmt.Errorf("%s 失败: ViGEm 错误 0x%X", api, uint32(result))
}

func normalizePadKey(key string) string {
	for len(key) > 0 && (key[0] == ' ' || key[0] == '\t') {
		key = key[1:]
	}
	return key
}
