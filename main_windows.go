//go:build windows

package main

import (
	"embed"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/wengchengjian/skip-all/games"
	"github.com/wengchengjian/skip-all/internal/app"
	"github.com/wengchengjian/skip-all/internal/engine"
	"github.com/wengchengjian/skip-all/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

const (
	// windowReleaseDelay 是关闭主窗口后销毁窗口、释放 WebView 内存的延迟：
	// 这段时间内重新打开不用重建窗口，超时后回到纯托盘的轻量模式。
	windowReleaseDelay = 30 * time.Second
	// trayRefreshInterval 是托盘菜单与提示的刷新间隔。
	trayRefreshInterval = 2 * time.Second
)

var (
	wailsApp *application.App
	appSvc   *app.Service

	// windowMu 保护主窗口状态：轻量模式下主窗口可以根本不存在。
	windowMu     sync.Mutex
	mainWindow   *application.WebviewWindow
	unhookClose  func()
	releaseTimer *time.Timer

	quitting atomic.Bool
)

func main() {
	// 带参数时走 CLI，保持与 cmd/skip-all 一致的行为。
	if len(os.Args) > 1 {
		os.Exit(app.RunCLI(os.Args[1:]))
	}

	svc, err := app.NewService(app.Options{
		EventSink: func(name string, data any) {
			if wailsApp != nil {
				wailsApp.Event.Emit(name, data)
			}
		},
		Quit: quitApp,
	})
	if err != nil {
		log.Fatal(err)
	}
	appSvc = svc

	wailsApp = application.New(application.Options{
		Name:        "SkipAll",
		Description: "跳过游戏剧情、CG 与对话",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(appSvc),
		},
		OnShutdown: appSvc.Shutdown,
		Windows: application.WindowsOptions{
			// 轻量模式下一个窗口都不存在，关掉"最后一个窗口关闭即退出"。
			DisableQuitOnLastWindowClosed: true,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	setupTray()
	registerHotkeys()
	appSvc.StartAutoUpdateCheck()

	// 启动即自动执行所有未排除游戏；游戏窗口稍后打开时后台扫描会继续尝试。
	go func() {
		time.Sleep(500 * time.Millisecond)
		startAll()
	}()
	// 后台先把主窗口建好（隐藏），双击托盘时就能立刻显示，不用等 WebView 冷启动。
	go func() {
		time.Sleep(800 * time.Millisecond)
		prewarmMainWindow()
	}()

	log.Printf("轻量模式：仅托盘运行，双击托盘图标打开主界面")
	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// trayUI 保存托盘菜单里需要随状态刷新的条目。
type trayUI struct {
	tray     *application.SystemTray
	games    map[string]*application.MenuItem
	features map[string]*application.MenuItem
	run      *application.MenuItem
	usage    *application.MenuItem

	// 状态相关的展示：只在状态真正变化时更新图标与提示音。
	lastState string
}

// setupTray 建立托盘图标与菜单：轻量模式下这是唯一常驻的界面。
// 菜单里放游戏列表与状态、功能开关、会话控制、性能占用。
func setupTray() *trayUI {
	ui := &trayUI{
		games:    make(map[string]*application.MenuItem),
		features: make(map[string]*application.MenuItem),
	}
	tray := wailsApp.SystemTray.New()
	ui.tray = tray
	// 托盘图标始终是程序图标，运行状态用菜单里的彩色状态点表示。
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetTooltip("SkipAll")

	state := appSvc.GetTrayState()
	menu := wailsApp.NewMenu()
	menu.Add("打开主界面").OnClick(func(*application.Context) { openMainWindow() })
	menu.AddSeparator()

	// 游戏列表：显示进程与会话状态，点一下打开主界面。
	// 状态点用菜单项图标表达，比 ●/○ 这类文字更直观。
	for _, game := range state.Games {
		item := menu.Add(gameLabel(game))
		item.SetBitmap(gameDot(game))
		item.OnClick(func(*application.Context) { openMainWindow() })
		ui.games[game.ID] = item
	}
	menu.AddSeparator()

	// 功能开关：一次只玩一个游戏，开关直接作用于全部游戏；标签里带上各自的快捷键。
	for _, feature := range state.Features {
		featureID := feature.ID
		label := feature.Name
		if key := featureHotkey(feature.ID); key != "" {
			label = label + "  " + key
		}
		item := menu.AddCheckbox(label, feature.Enabled)
		item.OnClick(func(ctx *application.Context) {
			if err := appSvc.SetFeatureEnabledEverywhere(featureID, ctx.IsChecked()); err != nil {
				log.Printf("切换功能 %s 失败: %v", featureID, err)
			}
		})
		ui.features[feature.ID] = item
	}
	menu.AddSeparator()

	// 运行状态栏：一行显示状态 + 下一步动作，左侧彩色状态点，点击等同于按运行开关。
	ui.run = menu.Add(runLabel(state.Status))
	ui.run.SetBitmap(statusDot(stateKey(state.Status)))
	ui.run.OnClick(func(*application.Context) { toggleRun() })
	menu.AddSeparator()

	ui.usage = menu.Add(usageLabel(state))
	ui.usage.SetEnabled(false)
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { quitApp() })
	tray.SetMenu(menu)

	// 左键与双击都直接打开主界面，右键出菜单。
	tray.OnClick(openMainWindow)
	tray.OnDoubleClick(openMainWindow)

	go ui.refreshLoop()
	return ui
}

// refreshLoop 定期把运行状态、功能开关和性能占用刷到托盘上。
func (ui *trayUI) refreshLoop() {
	ticker := time.NewTicker(trayRefreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		if quitting.Load() {
			return
		}
		ui.refresh()
	}
}

func (ui *trayUI) refresh() {
	state := appSvc.GetTrayState()
	for _, game := range state.Games {
		if item, ok := ui.games[game.ID]; ok {
			item.SetLabel(gameLabel(game))
			item.SetBitmap(gameDot(game))
		}
	}
	for _, feature := range state.Features {
		if item, ok := ui.features[feature.ID]; ok {
			item.SetChecked(feature.Enabled)
		}
	}
	ui.run.SetLabel(runLabel(state.Status))
	ui.run.SetBitmap(statusDot(stateKey(state.Status)))
	ui.usage.SetLabel(usageLabel(state))
	ui.tray.SetTooltip(tooltipText(state))
	ui.syncState(state.Status)
}

// syncState 在运行状态真正变化时刷新状态点并播放提示音。
func (ui *trayUI) syncState(status app.StatusView) {
	key := stateKey(status)
	if key == ui.lastState {
		return
	}
	ui.lastState = key
	beepFor(key)
}

// stateKey 把状态归成托盘要区分的几种：idle / running / paused / error。
func stateKey(status app.StatusView) string {
	switch engine.State(status.State) {
	case engine.StateRunning:
		return "running"
	case engine.StatePaused:
		return "paused"
	case engine.StateError:
		return "error"
	}
	return "idle"
}

// beepFor 按状态播放提示音；用户关掉 sound 时静音。
func beepFor(key string) {
	if !appSvc.GetConfig().App.Hotkeys.Sound {
		return
	}
	switch key {
	case "running":
		platform.Beep(platform.BeepInfo)
	case "paused", "idle":
		platform.Beep(platform.BeepWarning)
	case "error":
		platform.Beep(platform.BeepError)
	}
}

// gameLabel 把游戏状态压成菜单里的一行（状态点由菜单图标表达，文字里不再放符号）。
func gameLabel(game app.GameStateView) string {
	text := "未运行"
	if game.ProcessRunning {
		text = "已启动"
	}
	switch engine.State(game.State) {
	case engine.StateRunning:
		text = "运行中"
	case engine.StatePaused:
		text = "已暂停"
	case engine.StateError:
		text = "异常"
	}
	return game.Name + " · " + text
}

// gameDot 返回游戏行左侧的状态点：绿=运行中、琥珀=已暂停、红=异常、
// 蓝=进程在但会话未起、灰=进程不在。
func gameDot(game app.GameStateView) []byte {
	switch engine.State(game.State) {
	case engine.StateRunning:
		return statusDot("running")
	case engine.StatePaused:
		return statusDot("paused")
	case engine.StateError:
		return statusDot("error")
	}
	if game.ProcessRunning {
		return statusDot("ready")
	}
	return statusDot("idle")
}

// statusDot 按状态取状态点（PNG）。
// 配色是低饱和的"信号色"：绿=运行、琥珀=暂停、朱红=异常、靛蓝=待命、
// 中性灰=无游戏。top 比 base 略亮，只用来做纵向光照，不做出高光球。
func statusDot(key string) []byte {
	switch key {
	case "running":
		return platform.StatusDot(rgb(0x34, 0xD0, 0x7F), rgb(0x1B, 0xA8, 0x62), dotSize)
	case "paused":
		return platform.StatusDot(rgb(0xF0, 0xB0, 0x42), rgb(0xCE, 0x8B, 0x24), dotSize)
	case "error":
		return platform.StatusDot(rgb(0xE8, 0x5C, 0x5C), rgb(0xC4, 0x3E, 0x45), dotSize)
	case "ready":
		return platform.StatusDot(rgb(0x5A, 0x9C, 0xE8), rgb(0x3B, 0x7B, 0xC9), dotSize)
	}
	return platform.StatusDot(rgb(0xA8, 0xAE, 0xB8), rgb(0x8C, 0x93, 0x9E), dotSize)
}

// rgb 是构造 color.RGBA 的简写，省掉到处写 A: 255。
func rgb(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// dotSize 是菜单状态点的像素尺寸（含抗锯齿）。
const dotSize = 16

// runLabel 把运行状态压成一行：状态 + 唯一的切换快捷键与下一步动作。
// 点击这一行等同于按该快捷键。
func runLabel(status app.StatusView) string {
	key := toggleKeyLabel()
	switch engine.State(status.State) {
	case engine.StateRunning:
		return "运行中 · " + key + " 暂停"
	case engine.StatePaused:
		return "已暂停 · " + key + " 继续"
	case engine.StateError:
		return "异常 · " + key + " 重新开始"
	}
	return "已停止 · " + key + " 启动"
}

// toggleKeyLabel 返回运行开关的显示名，缺省 Home。
func toggleKeyLabel() string {
	key := strings.TrimSpace(appSvc.GetConfig().App.Hotkeys.Toggle)
	if key == "" {
		key = "Home"
	}
	return key
}

// featureHotkey 返回功能的全局快捷键，未配置返回空串。
func featureHotkey(featureID string) string {
	hotkeys := appSvc.GetConfig().App.Hotkeys
	if key, ok := hotkeys.Features[featureID]; ok {
		return strings.TrimSpace(key)
	}
	return ""
}

// usageLabel 生成性能占用那一行。
func usageLabel(state app.TrayStateView) string {
	return fmt.Sprintf("CPU %.1f%% · 内存 %.0f MB", state.CPU, state.MemoryMB)
}

// tooltipText 生成托盘提示：当前状态 + 游戏 + 帧率 + 内存。
func tooltipText(state app.TrayStateView) string {
	status := state.Status
	text := "SkipAll · 未检测到游戏"
	switch engine.State(status.State) {
	case engine.StateRunning:
		text = "SkipAll · 运行中"
		if name := runningGameName(state); name != "" {
			text = "SkipAll · " + name + " 运行中"
		}
		if status.FPS > 0 {
			text = fmt.Sprintf("%s · %.0f FPS", text, status.FPS)
		}
	case engine.StatePaused:
		text = "SkipAll · 已暂停"
	case engine.StateError:
		text = "SkipAll · 异常"
	default:
		for _, game := range state.Games {
			if game.ProcessRunning {
				text = "SkipAll · " + game.Name + " 已启动，等待识别"
				break
			}
		}
	}
	return fmt.Sprintf("%s · %.0fMB", text, state.MemoryMB)
}

// runningGameName 返回正在跑会话的游戏名。
func runningGameName(state app.TrayStateView) string {
	for _, game := range state.Games {
		if engine.State(game.State) == engine.StateRunning {
			return game.Name
		}
	}
	return ""
}

// prewarmMainWindow 后台提前创建隐藏的主窗口，让第一次双击立刻可见；
// 一直没打开就按轻量模式规则延迟销毁。
func prewarmMainWindow() {
	windowMu.Lock()
	created := false
	if mainWindow == nil && !quitting.Load() {
		var unhook func()
		mainWindow, unhook = newMainWindow()
		unhookClose = unhook
		created = true
	}
	windowMu.Unlock()
	if created {
		scheduleWindowRelease()
	}
}

// openMainWindow 按需创建主窗口并显示；轻量模式下这是唯一创建窗口的入口。
func openMainWindow() {
	cancelWindowRelease()

	windowMu.Lock()
	window := mainWindow
	if window == nil {
		var unhook func()
		window, unhook = newMainWindow()
		mainWindow = window
		unhookClose = unhook
	}
	windowMu.Unlock()

	window.Show()
	window.Focus()
}

// newMainWindow 创建（隐藏的）主窗口，并把关闭行为改成"隐藏 + 延迟释放"。
// 调用方需持有 windowMu。
func newMainWindow() (*application.WebviewWindow, func()) {
	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "SkipAll",
		Width:            1020,
		Height:           760,
		MinWidth:         880,
		MinHeight:        620,
		URL:              "/",
		Hidden:           true,
		InitialPosition:  application.WindowCentered,
		BackgroundColour: application.NewRGB(243, 246, 251),
	})
	unhook := window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() {
			return
		}
		e.Cancel()
		window.Hide()
		scheduleWindowRelease()
	})
	return window, unhook
}

// scheduleWindowRelease 安排主窗口销毁：隐藏满 windowReleaseDelay 后释放 WebView 内存。
func scheduleWindowRelease() {
	windowMu.Lock()
	defer windowMu.Unlock()
	if releaseTimer != nil {
		releaseTimer.Stop()
	}
	releaseTimer = time.AfterFunc(windowReleaseDelay, releaseMainWindow)
}

// cancelWindowRelease 取消尚未执行的销毁（用户又打开了窗口）。
func cancelWindowRelease() {
	windowMu.Lock()
	defer windowMu.Unlock()
	if releaseTimer != nil {
		releaseTimer.Stop()
		releaseTimer = nil
	}
}

// releaseMainWindow 销毁主窗口并释放空闲资源，回到纯托盘的轻量模式。
func releaseMainWindow() {
	windowMu.Lock()
	window := mainWindow
	unhook := unhookClose
	mainWindow = nil
	unhookClose = nil
	releaseTimer = nil
	windowMu.Unlock()
	if window == nil {
		return
	}
	if unhook != nil {
		unhook() // 撤掉拦截，这次关闭要真正销毁窗口
	}
	window.Close()
	appSvc.ReleaseIdleResources()
	log.Printf("主窗口已销毁，回到轻量模式")
}

// quitApp 退出程序：标记退出后不再拦截窗口关闭事件。
func quitApp() {
	quitting.Store(true)
	cancelWindowRelease()
	wailsApp.Quit()
}

// hotkeyManager 用低级键盘钩子实现快捷键：能拦住 F12 这类系统保留键，
// 并且只在游戏窗口处于前台时才吃掉按键。
var hotkeyManager *platform.HotkeyManager

// registerHotkeys 绑定快捷键；配置变化时可重复调用以重新绑定。
func registerHotkeys() {
	hotkeys := appSvc.GetConfig().App.Hotkeys
	toggleKey := strings.TrimSpace(hotkeys.Toggle)
	if toggleKey == "" {
		toggleKey = "F9"
	}
	bindings := []platform.Hotkey{
		{Accelerator: toggleKey, Callback: toggleRun},
	}
	for featureID, key := range hotkeys.Features {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		id := featureID
		bindings = append(bindings, platform.Hotkey{
			Accelerator: key,
			Callback: func() {
				enabled, err := appSvc.ToggleFeatureEverywhere(id)
				if err != nil {
					log.Printf("快捷键 %s 切换 %s 失败: %v", key, id, err)
					return
				}
				log.Printf("快捷键 %s：%s %s", key, id, enabledText(enabled))
			},
		})
	}

	if hotkeyManager == nil {
		hotkeyManager = platform.NewHotkeyManager(hotkeyGate)
		hotkeyManager.Start()
	}
	for _, err := range hotkeyManager.Set(bindings) {
		log.Printf("快捷键配置无效: %v", err)
	}
}

// hotkeyGate 决定快捷键此刻是否生效：默认只在游戏窗口（或本程序窗口）位于
// 前台时拦截按键，避免抢占其它应用的 F9/F11/F12。
func hotkeyGate() bool {
	cfg := appSvc.GetConfig().App
	if !cfg.Hotkeys.GameOnly {
		return true
	}
	foreground := platform.ForegroundProcessName()
	if foreground == "" {
		return false
	}
	// 本程序自己的窗口在前台时也放行，方便开着主界面直接按快捷键。
	if own := ownProcessName(); own != "" && foreground == own {
		return true
	}
	for _, game := range games.All() {
		for _, name := range game.ProcessNames() {
			if strings.EqualFold(foreground, name) {
				return true
			}
		}
	}
	return false
}

// ownProcessName 返回本程序的可执行文件名（小写）。
func ownProcessName() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return strings.ToLower(filepath.Base(exe))
}

// enabledText 把开关状态转成日志用词。
func enabledText(enabled bool) string {
	if enabled {
		return "已启用"
	}
	return "已停用"
}

// startAll 开始全部会话，失败时只记日志。
func startAll() {
	if result := appSvc.StartAll(); !result.OK {
		log.Printf("启动失败: %s", result.Error)
	}
}

// toggleRun 是唯一的运行开关，按一次走一步：
// 已停止/异常 → 启动，运行中 → 暂停，已暂停 → 继续。
func toggleRun() {
	status := appSvc.GetStatus()
	switch {
	case status.Paused:
		appSvc.ResumeAll()
	case status.Running:
		appSvc.PauseAll()
	default:
		startAll()
	}
}
