//go:build windows

package windows

var procMessageBeep = user32.NewProc("MessageBeep")

// 系统提示音类型，对应 MB_* 常量。
const (
	BeepInfo    uint32 = 0x40 // MB_ICONASTERISK：启动/正常
	BeepWarning uint32 = 0x30 // MB_ICONEXCLAMATION：暂停/停止
	BeepError   uint32 = 0x10 // MB_ICONHAND：异常
)

// Beep 播放一次系统提示音，不阻塞。
func Beep(kind uint32) {
	go procMessageBeep.Call(uintptr(kind))
}
