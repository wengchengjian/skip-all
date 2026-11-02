//go:build windows

package main

import (
	"embed"
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

// windowReleaseDelay 是关闭主窗口后销毁窗口、释放 WebView 内存的延迟：
// 这段时间内重新打开不用重建窗口，超时后回到纯托盘的轻量模式。
const windowReleaseDelay = 30 * time.Second

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

	log.Printf("轻量模式：仅托盘运行，双击托盘图标打开主界面")
	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupTray 建立托盘图标与右键菜单：轻量模式下这是唯一常驻的界面。
func setupTray() {
	tray := wailsApp.SystemTray.New()
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetTooltip("SkipAll · 待命")

	menu := wailsApp.NewMenu()
	menu.Add("打开主界面").OnClick(func(*application.Context) { openMainWindow() })
	menu.AddSeparator()
	menu.Add("开始").OnClick(func(*application.Context) { startAll() })
	menu.Add("暂停 / 继续").OnClick(func(*application.Context) { togglePause() })
	menu.Add("停止").OnClick(func(*application.Context) { appSvc.StopAll() })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { quitApp() })
	tray.SetMenu(menu)

	// 左键与双击都直接打开主界面，右键出菜单。
	tray.OnClick(openMainWindow)
	tray.OnDoubleClick(openMainWindow)

	go refreshTrayTooltip(tray)
}

// refreshTrayTooltip 定时把运行状态写进托盘提示，不开窗口也能看出在干什么。
func refreshTrayTooltip(tray *application.SystemTray) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if quitting.Load() {
			return
		}
		tray.SetTooltip("SkipAll · " + statusText(appSvc.GetStatus()))
	}
}

// statusText 把状态压成一句中文，给托盘提示用。
func statusText(status app.StatusView) string {
	switch engine.State(status.State) {
	case engine.StateRunning:
		if status.FeatureName != "" {
			return "运行中 · " + status.FeatureName
		}
		return "运行中"
	case engine.StatePaused:
		return "已暂停"
	case engine.StateError:
		if status.Error != "" {
			return "异常 · " + status.Error
		}
		return "异常"
	}
	return "待命"
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

// newMainWindow 创建主窗口，并把关闭行为改成"隐藏 + 延迟释放"。
// 调用方需持有 windowMu。
func newMainWindow() (*application.WebviewWindow, func()) {
	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "SkipAll",
		Width:            1020,
		Height:           760,
		MinWidth:         880,
		MinHeight:        620,
		URL:              "/",
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
