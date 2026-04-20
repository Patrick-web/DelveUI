package main

import (
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/jp/DelveUI/internal/debugfiles"
	"github.com/jp/DelveUI/internal/detect"
	"github.com/jp/DelveUI/internal/services"
	"github.com/jp/DelveUI/internal/session"
	"github.com/jp/DelveUI/internal/settings"
	"github.com/jp/DelveUI/internal/themes"
	"github.com/jp/DelveUI/internal/tray"
	"github.com/jp/DelveUI/internal/updater"
	"github.com/jp/DelveUI/internal/workspace"
)

var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	store, err := workspace.NewStore()
	if err != nil {
		log.Fatal(err)
	}
	mgr, err := session.NewManager()
	if err != nil {
		log.Printf("warning: %v", err)
		mgr = &session.Manager{}
	}

	themeSvc, err := themes.NewService()
	if err != nil {
		log.Fatal(err)
	}
	settingsSvc, err := settings.NewService()
	if err != nil {
		log.Fatal(err)
	}
	dbgFiles, err := debugfiles.NewStore()
	if err != nil {
		log.Fatal(err)
	}
	_ = dbgFiles.ReloadAll()

	// If user configured a custom dlv path in settings, use it
	if s := settingsSvc.Get(); s.DlvPath != "" {
		mgr.SetDlvPath(s.DlvPath)
	}

	updateSvc := updater.NewService(version)
	detectSvc := detect.NewService(dbgFiles)

	wsSvc := services.NewWorkspaceService(store)
	sessSvc := services.NewSessionService(mgr, wsSvc)
	fileSvc := services.NewFileService()

	// Auto-load default debug file on startup
	if def := dbgFiles.GetDefault(); def != nil {
		_, _ = wsSvc.OpenDebugFile(def.Path)
	}

	app := application.New(application.Options{
		Name:        "DelveUI",
		Description: "Delve debugger GUI for Go",
		Services: []application.Service{
			application.NewService(wsSvc),
			application.NewService(sessSvc),
			application.NewService(fileSvc),
			application.NewService(themeSvc),
			application.NewService(settingsSvc),
			application.NewService(dbgFiles),
			application.NewService(updateSvc),
			application.NewService(detectSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	installAppMenu(app)

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "DelveUI",
		Width:            1280,
		Height:           820,
		BackgroundColour: application.NewRGB(18, 19, 23),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	win.Maximise()

	win.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
	})

	wsSvc.SetApp(app)
	detectSvc.SetApp(app)
	updateSvc.SetApp(app)

	trayCtrl := tray.New(app, win, wsSvc, sessSvc, mgr)

	// Handle tray "open session in main window"
	app.Event.On("tray:open-session", func(e *application.CustomEvent) {
		if sid, ok := e.Data.(string); ok && sid != "" {
			app.Event.Emit("switch-session", sid)
			win.Show()
			win.Focus()
		}
	})
	_ = trayCtrl

	sub := mgr.Subscribe()
	go func() {
		for ev := range sub {
			app.Event.Emit("session:event", ev)
		}
	}()

	// Background update check 30s after launch
	updater.BackgroundCheck(app, version, 30*time.Second)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	mgr.StopAll()
}

// installAppMenu wires the native macOS menu bar so keyboard shortcuts like
// Cmd+Q / Cmd+W / Cmd+C get proper Mac behavior and the app feels native.
func installAppMenu(app *application.App) {
	menu := app.NewMenu()
	menu.AddRole(application.AppMenu)

	file := menu.AddSubmenu("File")
	file.Add("Open debug.json…").SetAccelerator("CmdOrCtrl+Shift+O").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:open-debug-file", nil)
	})
	file.Add("Quick Open File…").SetAccelerator("CmdOrCtrl+O").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:quick-open", nil)
	})
	file.AddSeparator()
	file.AddRole(application.CloseWindow)

	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)

	debug := menu.AddSubmenu("Debug")
	debug.Add("Continue").SetAccelerator("F5").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:debug-control", "Continue")
	})
	debug.Add("Pause").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:debug-control", "Pause")
	})
	debug.Add("Step Over").SetAccelerator("F10").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:debug-control", "StepOver")
	})
	debug.Add("Step In").SetAccelerator("F11").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:debug-control", "StepIn")
	})
	debug.Add("Step Out").SetAccelerator("Shift+F11").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:debug-control", "StepOut")
	})
	debug.AddSeparator()
	debug.Add("Stop").SetAccelerator("Shift+F5").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:debug-control", "Stop")
	})
	debug.Add("Command Palette…").SetAccelerator("CmdOrCtrl+Shift+P").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:command-palette", nil)
	})

	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)

	app.Menu.Set(menu)
}
