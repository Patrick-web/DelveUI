//go:build !darwin

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
	}
}

func configureTray(c *Controller) {
	// Windows/Linux: toggle tray window on left-click
	c.tray.OnClick(func() {
		if c.trayWin != nil {
			c.tray.PositionWindow(c.trayWin, 4)
			c.trayWin.Show()
			c.trayWin.Focus()
		}
	})
}
