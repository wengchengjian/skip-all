//go:build windows

package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/wengchengjian/skip-all/internal/app"
	"github.com/wengchengjian/skip-all/internal/engine"
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
	pause    *application.MenuItem
	usage    *application.MenuItem
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
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetTooltip("SkipAll")

	state := appSvc.GetTrayState()
	menu := wailsApp.NewMenu()
	menu.Add("打开主界面").OnClick(func(*application.Context) { openMainWindow() })
	menu.AddSeparator()

	// 游戏列表：只展示进程与会话状态，玩家同一时间只玩一个游戏。
	for _, game := range state.Games {
		item := menu.Add(gameLabel(game))
		item.SetEnabled(false)
		ui.games[game.ID] = item
	}
	menu.AddSeparator()

	// 功能开关：一次只玩一个游戏，开关直接作用于全部游戏。
	for _, feature := range state.Features {
		featureID := feature.ID
		item := menu.AddCheckbox(feature.Name, feature.Enabled)
		item.OnClick(func(ctx *application.Context) {
			if err := appSvc.SetFeatureEnabledEverywhere(featureID, ctx.IsChecked()); err != nil {
				log.Printf("切换功能 %s 失败: %v", featureID, err)
			}
		})
		ui.features[feature.ID] = item
	}
	menu.AddSeparator()

	menu.Add("开始").OnClick(func(*application.Context) { startAll() })
	ui.pause = menu.Add("暂停")
	ui.pause.OnClick(func(*application.Context) { togglePause() })
	menu.Add("停止").OnClick(func(*application.Context) { appSvc.StopAll() })
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
		}
	}
	for _, feature := range state.Features {
		if item, ok := ui.features[feature.ID]; ok {
			item.SetChecked(feature.Enabled)
		}
	}
	if state.Status.Paused {
		ui.pause.SetLabel("继续")
	} else {
		ui.pause.SetLabel("暂停")
	}
	ui.usage.SetLabel(usageLabel(state))
	ui.tray.SetTooltip(tooltipText(state))
}

// gameLabel 把游戏状态压成菜单里的一行。
func gameLabel(game app.GameStateView) string {
	mark := "○"
	text := "未运行"
	if game.ProcessRunning {
		mark = "●"
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
	return mark + " " + game.Name + " · " + text
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

// registerHotkeys 注册全局快捷键，轻量模式下同样有效。
func registerHotkeys() {
	hotkeys := appSvc.GetConfig().App.Hotkeys
	toggleKey := hotkeys.Toggle
	if toggleKey == "" {
		toggleKey = "F9"
	}
	panicKey := hotkeys.Panic
	if panicKey == "" {
		panicKey = "F10"
	}
	if err := wailsApp.GlobalShortcut.Register(toggleKey, togglePauseOrStart); err != nil {
		log.Printf("注册快捷键 %s 失败: %v", toggleKey, err)
	}
	if err := wailsApp.GlobalShortcut.Register(panicKey, func() { appSvc.StopAll() }); err != nil {
		log.Printf("注册快捷键 %s 失败: %v", panicKey, err)
	}
}

// startAll 开始全部会话，失败时只记日志。
func startAll() {
	if result := appSvc.StartAll(); !result.OK {
		log.Printf("启动失败: %s", result.Error)
	}
}

// togglePause 在暂停与继续之间切换。
func togglePause() {
	if appSvc.GetStatus().Paused {
		appSvc.ResumeAll()
		return
	}
	appSvc.PauseAll()
}

// togglePauseOrStart 对应 F9：空闲时开始全部，运行中暂停，暂停中继续。
func togglePauseOrStart() {
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
