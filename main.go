package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/jp/DelveUI/internal/adapter"
	"github.com/jp/DelveUI/internal/debugfiles"
	"github.com/jp/DelveUI/internal/detect"
	"github.com/jp/DelveUI/internal/discovery"
	"github.com/jp/DelveUI/internal/discovery/goprovider"
	"github.com/jp/DelveUI/internal/discovery/jsprovider"
	"github.com/jp/DelveUI/internal/discovery/pythonprovider"
	"github.com/jp/DelveUI/internal/search"
	"github.com/jp/DelveUI/internal/services"
	"github.com/jp/DelveUI/internal/session"
	"github.com/jp/DelveUI/internal/settings"
	"github.com/jp/DelveUI/internal/themes"
	"github.com/jp/DelveUI/internal/tray"
	"github.com/jp/DelveUI/internal/updater"
)

var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	var initialProject string
	flag.StringVar(&initialProject, "project", "", "path to debug.json or project root to open on launch")
	flag.Parse()

	// Adapters: maps language tags to DAP server process specs.
	// Adding a new language is one Register() call.
	adapterReg := adapter.NewRegistry()
	adapterReg.Register(adapter.ProcessSpec{
		Language:    "go",
		Label:       "Go (Delve)",
		Description: "Debug Go programs using the Delve debugger",
		AdapterID:   "delve",
		DAPType:     "go",
		Binary:      findDlv(),
		BinaryArgs:  []string{"dap"},
		BinaryName:  "dlv",
		ExtraPath:   goToolchainPaths(),
		InstallCmd:  "go install github.com/go-delve/delve/cmd/dlv@latest",
		InstallURL:  "https://github.com/go-delve/delve",
	})
	adapterReg.Register(adapter.ProcessSpec{
		Language:    "python",
		Label:       "Python (debugpy)",
		Description: "Debug Python programs using debugpy",
		AdapterID:   "debugpy",
		DAPType:     "python",
		Binary:      adapter.FindBinary("debugpy", pythonScriptPaths()),
		BinaryName:  "debugpy",
		BinaryArgs:  nil,
		ExtraPath:   pythonScriptPaths(),
		InstallCmd:  "pip3 install --user debugpy 2>/dev/null || pip3 install --break-system-packages debugpy 2>/dev/null || pip install --user debugpy",
		InstallURL:  "https://github.com/microsoft/debugpy",
	})
	// Node.js: Zed downloads vscode-js-debug from GitHub Releases and runs it
	// via node. We replicate that here: the adapter binary is node, with the
	// DAP server JS file as the first argument.
	adapterReg.Register(adapter.ProcessSpec{
		Language:    "node",
		Label:       "JavaScript / TypeScript (js-debug)",
		Description: "Debug JS/TS programs using vscode-js-debug",
		AdapterID:   "js-debug",
		DAPType:     "pwa-node",
		Binary:      adapter.FindBinary("node", nil),
		BinaryName:  "node",
		BinaryArgs:  jsDebugArgs(),
		PortFlag:    "$PORT $HOST",
		ExtraPath:   nil,
		InstallCmd:  `V=$(curl -fsSL https://api.github.com/repos/microsoft/vscode-js-debug/releases/latest 2>/dev/null | tr ',' '\n' | grep '"tag_name"' | head -1 | cut -d'"' -f4) && DIR="$HOME/.delveui/adapters/js-debug" && mkdir -p "$DIR" && echo "Downloading js-debug $V…" && curl -fsSL "https://github.com/microsoft/vscode-js-debug/releases/download/${V}/js-debug-dap-${V}.tar.gz" | tar xz -C "$DIR" && echo "Installed to $DIR"`,
		InstallURL:  "https://github.com/microsoft/vscode-js-debug",
	})

	mgr := session.NewManager(adapterReg)

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
		if err := adapterReg.SetBinary("go", s.DlvPath); err != nil {
			log.Printf("warning: invalid adapter path in settings: %v", err)
		}
	}

	updateSvc := updater.NewService(version)
	detectSvc := detect.NewService(dbgFiles)

	wsSvc := services.NewWorkspaceService(dbgFiles)
	sessSvc := services.NewSessionService(mgr, wsSvc)
	fileSvc := services.NewFileService()
	searchSvc := search.New(wsSvc)
	adapterSvc := adapter.NewService(adapterReg)

	// Discovery: pluggable run/test/attach target detection. Providers register
	// themselves explicitly here so the import graph stays self-documenting —
	// adding a new language is one Register() call.
	discoveryReg := discovery.NewRegistry()
	discoveryReg.Register(goprovider.New())
	discoveryReg.Register(pythonprovider.New())
	discoveryReg.Register(jsprovider.New())
	discoverySvc := discovery.NewService(discoveryReg, wsSvc, mgr)

	// Initial workspace priority:
	//   1. --project flag       (explicit override, used when spawning a new window)
	//   2. most-recent project  (when the restoreLastProject setting is on)
	//
	// Without a restored or explicit project, the welcome page handles
	// onboarding so we leave wsSvc untouched.
	if initialProject != "" {
		if _, err := wsSvc.OpenDebugFile(initialProject); err != nil {
			log.Printf("warning: --project %q: %v", initialProject, err)
		}
	} else if wsSvc.Root() == "" {
		restore := true
		if cur := settingsSvc.Get(); cur.RestoreLastProject != nil {
			restore = *cur.RestoreLastProject
		}
		if restore {
			if recent := dbgFiles.MostRecent(); recent != nil && !recent.Stale {
				_, _ = wsSvc.OpenWorkspace(recent.Path)
			}
		}
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
			application.NewService(discoverySvc),
			application.NewService(searchSvc),
			application.NewService(adapterSvc),
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
	discoverySvc.SetApp(app)
	updateSvc.SetApp(app)
	searchSvc.SetApp(app)
	adapterSvc.SetApp(app)

	trayCtrl := tray.New(app, win, wsSvc, sessSvc, mgr)

	// Handle tray "open session in main window"
	app.Event.On("tray:open-session", func(e *application.CustomEvent) {
		if sid, ok := e.Data.(string); ok && sid != "" {
			app.Event.Emit("switch-session", sid)
			win.Show()
			win.Focus()
		}
	})

	// Spawn a fresh DelveUI process pointed at a different project.
	app.Event.On("project:open-new-window", func(e *application.CustomEvent) {
		path, ok := e.Data.(string)
		if !ok || path == "" {
			return
		}
		exe, err := os.Executable()
		if err != nil {
			log.Printf("project:open-new-window: %v", err)
			return
		}
		cmd := exec.Command(exe, "--project", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Printf("project:open-new-window: %v", err)
			return
		}
		go func() { _ = cmd.Wait() }()
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

	// Ensure debug adapter child processes are killed on SIGINT/SIGTERM,
	// since those bypass the normal Wails shutdown path.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		mgr.StopAll()
		app.Quit()
	}()

	runErr := app.Run()
	mgr.StopAll()
	if runErr != nil {
		log.Fatal(runErr)
	}
}

// installAppMenu wires the native macOS menu bar so keyboard shortcuts like
// Cmd+Q / Cmd+W / Cmd+C get proper Mac behavior and the app feels native.
func installAppMenu(app *application.App) {
	menu := app.NewMenu()
	menu.AddRole(application.AppMenu)

	file := menu.AddSubmenu("File")
	file.Add("Open Folder…").SetAccelerator("CmdOrCtrl+Shift+O").OnClick(func(_ *application.Context) {
		app.Event.Emit("menu:open-folder", nil)
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

// findDlv searches PATH and common install locations for the dlv binary.
func findDlv() string {
	if p, err := exec.LookPath("dlv"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		"/usr/local/bin/dlv",
		"/opt/homebrew/bin/dlv",
		"/usr/local/go/bin/dlv",
	}
	if home != "" {
		candidates = append(candidates,
			home+"/go/bin/dlv",
			home+"/.local/bin/dlv",
			home+"/bin/dlv",
		)
		if gp := os.Getenv("GOPATH"); gp != "" {
			candidates = append(candidates, gp+"/bin/dlv")
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// goToolchainPaths returns PATH entries needed to find the Go toolchain
// when launched as a macOS .app (which has a minimal default PATH).
func goToolchainPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{"/usr/local/go/bin"}
	if home != "" {
		paths = append(paths, home+"/go/bin")
	}
	if gp := os.Getenv("GOPATH"); gp != "" {
		paths = append(paths, gp+"/bin")
	}
	return paths
}

// pythonScriptPaths returns common locations where pip installs script
// binaries (like debugpy). These are searched by FindBinary and added to
// the subprocess PATH so the DAP server is reachable.
func pythonScriptPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home != "" {
		paths = append(paths, home+"/.local/bin")
		// pip --user installs to ~/Library/Python/X.Y/bin on macOS.
		if ents, err := os.ReadDir(home + "/Library/Python"); err == nil {
			for _, e := range ents {
				if e.IsDir() && strings.HasPrefix(e.Name(), "3.") {
					paths = append(paths, home+"/Library/Python/"+e.Name()+"/bin")
				}
			}
		}
	}
	return paths
}

// jsDebugArgs returns the path to the vscode-js-debug DAP server JS file,
// assuming it was installed to ~/.delveui/adapters/js-debug.
func jsDebugArgs() []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	return []string{filepath.Join(home, ".delveui", "adapters", "js-debug", "js-debug", "src", "dapDebugServer.js")}
}
