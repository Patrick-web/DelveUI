//go:build darwin

package tray

import "github.com/wailsapp/wails/v3/pkg/application"

func trayWindowOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Title:            "DelveUI Sessions",
		Width:            360,
		Height:           420,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundColour: application.NewRGB(27, 29, 34),
		URL:              "/tray.html",
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBarHidden,
		},
	}
}

func configureTray(c *Controller) {
	// macOS: attach window to tray icon (toggles on left-click)
	c.tray.AttachWindow(c.trayWin).WindowOffset(4)
}
