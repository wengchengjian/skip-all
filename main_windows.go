//go:build windows

package main

import (
	"embed"
	"log"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/wengchengjian/skip-all/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

const (
	defaultGameID    = "zzz"
	defaultFeatureID = "story_skip"
)

// wailsApp 由 main 中赋值，供 EventSink 转发服务事件到前端。
var wailsApp *application.App

func main() {
	// 带参数时走 CLI，保持与 cmd/skip-all 一致的行为。
	if len(os.Args) > 1 {
		os.Exit(app.RunCLI(os.Args[1:]))
	}

	var quitting atomic.Bool

	appSvc, err := app.NewService(app.Options{
		EventSink: func(name string, data any) {
			if wailsApp != nil {
				wailsApp.Event.Emit(name, data)
			}
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	wailsApp = application.New(application.Options{
		Name:        "SkipAll",
		Description: "跳过游戏剧情、CG 与对话",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(appSvc),
		},
		OnShutdown: func() {
			appSvc.Shutdown()
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "SkipAll",
		Width:            1020,
		Height:           760,
		URL:              "/",
		InitialPosition:  application.WindowCentered,
		BackgroundColour: application.NewRGB(243, 246, 251),
	})

	// 关闭窗口时隐藏到托盘，退出由托盘菜单触发。
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() {
			return
		}
		window.Hide()
		e.Cancel()
	})

	toggle := func() {
		status := appSvc.GetStatus()
		if status.Running || status.Paused {
			appSvc.Stop()
			return
		}
		if result := appSvc.Start(defaultGameID, defaultFeatureID); !result.OK {
			log.Printf("启动失败: %s", result.Error)
		}
	}

	tray := wailsApp.SystemTray.New()
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetTooltip("SkipAll")

	menu := wailsApp.NewMenu()
	menu.Add("显示").OnClick(func(*application.Context) {
		window.Show()
		window.Focus()
	})
	menu.Add("隐藏").OnClick(func(*application.Context) {
		window.Hide()
	})
	menu.AddSeparator()
	menu.Add("开始").OnClick(func(*application.Context) {
		if result := appSvc.Start(defaultGameID, defaultFeatureID); !result.OK {
			log.Printf("启动失败: %s", result.Error)
		}
	})
	menu.Add("停止").OnClick(func(*application.Context) {
		appSvc.Stop()
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) {
		quitting.Store(true)
		wailsApp.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(func() {
		if window.IsVisible() {
			window.Hide()
		} else {
			window.Show()
			window.Focus()
		}
	})

	hotkeys := appSvc.GetConfig().App.Hotkeys
	toggleKey := hotkeys.Toggle
	if toggleKey == "" {
		toggleKey = "F9"
	}
	panicKey := hotkeys.Panic
	if panicKey == "" {
		panicKey = "F10"
	}
	if err := wailsApp.GlobalShortcut.Register(toggleKey, toggle); err != nil {
		log.Printf("注册快捷键 %s 失败: %v", toggleKey, err)
	}
	if err := wailsApp.GlobalShortcut.Register(panicKey, func() { appSvc.Stop() }); err != nil {
		log.Printf("注册快捷键 %s 失败: %v", panicKey, err)
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
