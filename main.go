package main

import (
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/jp/DelveUI/internal/debugfiles"
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
		log.Printf("warning: %v (tray will still start; install dlv to use)", err)
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

	updateSvc := updater.NewService(version)

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
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "DelveUI",
		Width:            1280,
		Height:           820,
		BackgroundColour: application.NewRGB(27, 29, 34),
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
	updater.BackgroundCheck(version, 30*time.Second)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	mgr.StopAll()
}
