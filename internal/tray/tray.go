package tray

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jp/DelveUI/internal/services"
	"github.com/jp/DelveUI/internal/session"
)

type Controller struct {
	app      *application.App
	tray     *application.SystemTray
	mainWin  application.Window
	trayWin  application.Window
	ws       *services.WorkspaceService
	sess     *services.SessionService
	mgr      *session.Manager
	mu       sync.Mutex
}

func New(app *application.App, mainWin application.Window, ws *services.WorkspaceService, sess *services.SessionService, mgr *session.Manager) *Controller {
	c := &Controller{app: app, mainWin: mainWin, ws: ws, sess: sess, mgr: mgr}

	// Create hidden tray popup window
	c.trayWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
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
	})

	c.tray = app.SystemTray.New()
	c.tray.SetLabel("DUI")
	c.tray.SetTooltip("DelveUI")

	// Left-click: toggle attached tray window
	c.tray.AttachWindow(c.trayWin).WindowOffset(4)

	// Right-click: show the config menu (start/stop sessions)
	c.tray.OnRightClick(func() {
		c.tray.OpenMenu()
	})

	c.Rebuild()

	ch := mgr.Subscribe()
	go func() {
		for range ch {
			application.InvokeAsync(c.Rebuild)
			// Push session updates to tray window
			c.app.Event.Emit("tray:sessions-changed", true)
		}
	}()
	return c
}

func (c *Controller) MainWin() application.Window { return c.mainWin }

// Rebuild constructs the right-click menu from current workspace + sessions state.
func (c *Controller) Rebuild() {
	c.mu.Lock()
	defer c.mu.Unlock()

	menu := application.NewMenu()

	cfgs := c.ws.Configs()
	if len(cfgs) == 0 {
		menu.Add("(no debug configs)").SetEnabled(false)
	}
	for _, cfg := range cfgs {
		cfg := cfg
		running := c.mgr.FindByCfg(cfg.ID)
		item := menu.Add(cfg.Label)
		if running != nil {
			item.SetChecked(true)
			item.OnClick(func(*application.Context) {
				_ = c.sess.StopByCfg(cfg.ID)
			})
		} else {
			item.OnClick(func(*application.Context) {
				go func() { _, _ = c.sess.Start(cfg.ID) }()
			})
		}
	}

	menu.AddSeparator()
	menu.Add("Open debug.json…").OnClick(func(*application.Context) {
		go c.pickFile()
	})
	menu.Add("Show Window").OnClick(func(*application.Context) {
		if c.mainWin != nil {
			c.mainWin.Show()
			c.mainWin.Focus()
		}
	})
	menu.Add("Stop All").OnClick(func(*application.Context) {
		c.mgr.StopAll()
	})
	menu.Add("Quit").OnClick(func(*application.Context) {
		c.app.Quit()
	})

	c.tray.SetMenu(menu)

	anyRunning := false
	for _, s := range c.mgr.List() {
		if s.State() == session.StateRunning || s.State() == session.StateStopped {
			anyRunning = true
			break
		}
	}
	if anyRunning {
		c.tray.SetLabel("● DUI")
	} else {
		c.tray.SetLabel("DUI")
	}
}

func (c *Controller) pickFile() {
	if _, err := c.ws.PickDebugFile(); err != nil {
		c.app.Logger.Error("pick debug.json: " + err.Error())
	}
	application.InvokeAsync(c.Rebuild)
	c.app.Event.Emit("workspace:changed", c.ws.Info())
}
