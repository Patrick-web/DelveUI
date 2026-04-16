# DelveUI

A native desktop GUI for the [Delve](https://github.com/go-delve/delve) Go debugger. Built with [Wails v3](https://wails.io) + Svelte.

![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)
![Svelte](https://img.shields.io/badge/Svelte-FF3E00?logo=svelte&logoColor=white)
![macOS](https://img.shields.io/badge/macOS-000?logo=apple&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-0078D4?logo=windows&logoColor=white)
![Linux](https://img.shields.io/badge/Linux-FCC624?logo=linux&logoColor=black)

## Features

- **Full DAP client** — launch, breakpoints, stepping, call stack, variables, evaluate
- **Multiple concurrent sessions** — run several debug targets at once, switch via tabs
- **Auto-detect editor configs** — scans your system for debug configurations from VS Code, Zed, and GoLand
- **Multi-editor support** — imports `.vscode/launch.json`, `.zed/debug.json`, and `.idea/runConfigurations/`
- **Terminal pane** — ANSI-colored program output, theme-aware terminal colors
- **Debug Console** — REPL with command history for evaluating expressions
- **Theme engine** — 8 bundled themes (One Dark, GitHub Dark, Catppuccin, Dracula, Nord, and more), live preview, install custom themes from file
- **System tray** — manage sessions from the menu bar; left-click popup with session controls
- **Command palette** — `⌘⇧P` with fuzzy search, Run/Debug/Theme submodes
- **Settings** — Appearance, Terminal, Panels, Debug Files, General with full keyboard navigation
- **Port-in-use detection** — detects bind errors and offers to kill the blocking process
- **Auto-updates** — checks GitHub Releases for new versions on launch
- **Cross-platform** — macOS (primary), Windows, Linux

## Requirements

- [Go 1.21+](https://go.dev/dl/)
- [Delve](https://github.com/go-delve/delve) (`go install github.com/go-delve/delve/cmd/dlv@latest`)
- [Node.js 18+](https://nodejs.org/) (for building the frontend)
- [Wails v3 CLI](https://wails.io) (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`)

## Quick Start

```bash
# Clone
git clone https://github.com/Patrick-web/DelveUI.git
cd DelveUI

# Run in dev mode (hot reload)
wails3 dev

# Or build for production
wails3 build
```

The app opens maximized. On first launch, it scans your system for existing debug configurations and offers to import them.

## Usage

### Import debug configs

1. Click **Projects** in the title bar → **Auto-detect** to scan for editor configs
2. Or **Add debug.json** to manually pick a file
3. Supports `.vscode/launch.json`, `.zed/debug.json`, and GoLand XML configs

### Start a debug session

1. Click **▶ Run** in the title bar and pick a config
2. Or use the command palette: `⌘⇧P` → "Debug: Run…"
3. Session tab appears in the title bar; terminal output streams live

### Keyboard shortcuts

| Key | Action |
|---|---|
| `⌘⇧P` | Command palette |
| `⌘K ⌘T` | Theme picker |
| `⌘,` | Settings |
| `F5` | Continue |
| `⇧F5` | Stop |
| `F10` | Step over |
| `F11` | Step in |
| `⇧F11` | Step out |

### Themes

8 bundled themes: One Dark, One Light, GitHub Dark, GitHub Light, Catppuccin Mocha, Catppuccin Latte, Dracula, Nord.

- Switch via `⌘K ⌘T` or Settings → Appearance
- Live preview on hover/focus, click to apply
- Install custom themes from `.json` files
- Terminal ANSI colors follow the active theme

## Architecture

```
├── main.go                    # Wails app entry, service registration
├── internal/
│   ├── config/                # Debug config loader (Zed + VS Code format)
│   ├── dap/                   # DAP protocol client (go-dap wrapper)
│   ├── session/               # Debug session lifecycle + process management
│   ├── services/              # Wails-bound services (Workspace, Session, File)
│   ├── detect/                # Auto-detection scanner + parsers
│   ├── themes/                # Theme service (bundled + user themes)
│   ├── settings/              # App settings persistence
│   ├── debugfiles/            # Debug file database
│   ├── updater/               # Auto-update via GitHub Releases
│   ├── tray/                  # System tray (platform-specific)
│   └── workspace/             # Recent workspaces store
├── frontend/src/
│   ├── App.svelte             # Main layout: titlebar + splitpanes + statusbar
│   ├── lib/
│   │   ├── store.ts           # Svelte stores + Wails event bridge
│   │   ├── theme-engine.ts    # Runtime theme application
│   │   ├── CommandPalette.svelte
│   │   ├── SettingsPage.svelte
│   │   ├── WelcomePage.svelte
│   │   ├── ImportWizard.svelte
│   │   ├── ConfigPicker.svelte
│   │   ├── TerminalPane.svelte
│   │   └── ...panels
│   └── lib/panels/            # Dock layout + panel registry
└── assets/themes/             # Bundled theme JSON files
```

## Releasing

See [RELEASING.md](RELEASING.md) for the release process.

## License

MIT
