package tray

import (
	_ "embed"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jp/DelveUI/internal/services"
	"github.com/jp/DelveUI/internal/session"
)

//go:embed icon.png
var trayIcon []byte

type Controller struct {
	app     *application.App
	tray    *application.SystemTray
	mainWin application.Window
	trayWin application.Window
	ws      *services.WorkspaceService
	sess    *services.SessionService
	mgr     *session.Manager
	mu      sync.Mutex
}

func New(app *application.App, mainWin application.Window, ws *services.WorkspaceService, sess *services.SessionService, mgr *session.Manager) *Controller {
	c := &Controller{app: app, mainWin: mainWin, ws: ws, sess: sess, mgr: mgr}

	c.tray = app.SystemTray.New()
	c.tray.SetTooltip("DelveUI")
	c.tray.SetIcon(trayIcon)

	// Create tray popup window (platform-specific options applied below)
	c.trayWin = app.Window.NewWithOptions(trayWindowOptions())

	// Platform-specific: attach window on macOS, click handler on other platforms
	configureTray(c)

	// Right-click always shows the config menu
	c.tray.OnRightClick(func() {
		c.tray.OpenMenu()
	})

	c.Rebuild()

	ch := mgr.Subscribe()
	go func() {
		for range ch {
			application.InvokeAsync(c.Rebuild)
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
	menu.Add("Open Folder…").OnClick(func(*application.Context) {
		go c.pickFolder()
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
}

func (c *Controller) pickFolder() {
	if _, err := c.ws.PickWorkspaceFolder(); err != nil {
		c.app.Logger.Error("pick folder: " + err.Error())
	}
	application.InvokeAsync(c.Rebuild)
	c.app.Event.Emit("workspace:changed", c.ws.Info())
}
