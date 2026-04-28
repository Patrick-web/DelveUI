package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jp/DelveUI/internal/config"
	"github.com/jp/DelveUI/internal/debugclean"
	"github.com/jp/DelveUI/internal/session"
	"github.com/jp/DelveUI/internal/workspace"
)

// WorkspaceService is exposed to the frontend: open workspace, list configs, recents.
type WorkspaceService struct {
	store     *workspace.Store
	configs   []config.LaunchConfig
	root      string
	debugFile string
	app       *application.App
}

func NewWorkspaceService(store *workspace.Store) *WorkspaceService {
	s := &WorkspaceService{store: store}
	if p := store.ActivePath(); p != "" {
		_ = s.loadPath(p)
	}
	return s
}

// SetApp injects the running application so the service can open native dialogs.
func (s *WorkspaceService) SetApp(app *application.App) { s.app = app }

type WorkspaceInfo struct {
	Root      string                `json:"root"`
	DebugFile string                `json:"debugFile"`
	Configs   []config.LaunchConfig `json:"configs"`
	Recents   []workspace.Recent    `json:"recents"`
	LoadedOK  bool                  `json:"loadedOk"`
	LoadError string                `json:"loadError,omitempty"`
}

func (s *WorkspaceService) Configs() []config.LaunchConfig { return s.configs }
func (s *WorkspaceService) Root() string                   { return s.root }
func (s *WorkspaceService) DebugFile() string              { return s.debugFile }

// ClearWorkspace resets the workspace to empty state (used after reset).
func (s *WorkspaceService) ClearWorkspace() {
	s.root = ""
	s.debugFile = ""
	s.configs = nil
}

func (s *WorkspaceService) Info() WorkspaceInfo {
	info := WorkspaceInfo{Root: s.root, DebugFile: s.debugFile, Configs: s.configs, Recents: s.store.List()}
	info.LoadedOK = len(s.configs) > 0 || (s.root == "" && s.debugFile == "")
	return info
}

// OpenWorkspace accepts either a directory (auto-discovers .zed/debug.json)
// or a path to a debug.json file directly.
func (s *WorkspaceService) OpenWorkspace(path string) (WorkspaceInfo, error) {
	if err := s.loadPath(path); err != nil {
		return s.Info(), err
	}
	remember := s.root
	if remember == "" {
		remember = s.debugFile
	}
	if err := s.store.SetActive(remember); err != nil {
		return s.Info(), err
	}
	return s.Info(), nil
}

// OpenDebugFile loads an arbitrary debug.json path directly.
func (s *WorkspaceService) OpenDebugFile(path string) (WorkspaceInfo, error) {
	return s.OpenWorkspace(path)
}

// PickDebugFile opens a native file dialog for the user to choose a debug.json.
func (s *WorkspaceService) PickDebugFile() (WorkspaceInfo, error) {
	if s.app == nil {
		return s.Info(), errors.New("app not initialized")
	}
	opts := &application.OpenFileDialogOptions{
		Title:                "Choose debug configuration file",
		CanChooseFiles:       true,
		CanChooseDirectories: false,
		Filters: []application.FileFilter{
			{DisplayName: "Debug Config (*.json)", Pattern: "*.json"},
		},
	}
	if s.debugFile != "" {
		opts.Directory = filepath.Dir(s.debugFile)
	} else if s.root != "" {
		opts.Directory = s.root
	}
	dialog := s.app.Dialog.OpenFileWithOptions(opts)
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return s.Info(), err
	}
	if path == "" {
		return s.Info(), nil
	}
	return s.OpenWorkspace(path)
}

// PickWorkspaceFolder opens a native dialog for a directory (auto-detect .zed/debug.json).
func (s *WorkspaceService) PickWorkspaceFolder() (WorkspaceInfo, error) {
	if s.app == nil {
		return s.Info(), errors.New("app not initialized")
	}
	dialog := s.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Choose workspace folder",
		CanChooseFiles:       false,
		CanChooseDirectories: true,
	})
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return s.Info(), err
	}
	if path == "" {
		return s.Info(), nil
	}
	return s.OpenWorkspace(path)
}

// loadPath accepts a directory (→ looks for .zed/debug.json) or a debug.json file path.
func (s *WorkspaceService) loadPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		dbg, cfgs, err := config.LoadFromWorkspace(path)
		if err != nil {
			s.root = path
			s.debugFile = dbg
			s.configs = nil
			return err
		}
		s.root = path
		s.debugFile = dbg
		s.configs = cfgs
		return nil
	}
	// direct file
	cfgs, err := config.LoadFile(path)
	if err != nil {
		s.root = filepath.Dir(path)
		s.debugFile = path
		s.configs = nil
		return err
	}
	s.root = filepath.Dir(path)
	s.debugFile = path
	s.configs = cfgs
	return nil
}

// SessionService is exposed to the frontend: start/stop/control debug sessions.
type SessionService struct {
	mgr *session.Manager
	ws  *WorkspaceService
}

func NewSessionService(mgr *session.Manager, ws *WorkspaceService) *SessionService {
	return &SessionService{mgr: mgr, ws: ws}
}

type SessionInfo struct {
	ID    string              `json:"id"`
	CfgID string              `json:"cfgId"`
	Label string              `json:"label"`
	State session.State       `json:"state"`
	Port  int                 `json:"port"`
	PID   int                 `json:"pid"`
	Cfg   config.LaunchConfig `json:"cfg"`
}

func toInfo(s *session.Session) SessionInfo {
	return SessionInfo{ID: s.ID, CfgID: s.CfgID, Label: s.Label, State: s.State(), Port: s.Port, PID: s.PID, Cfg: s.Cfg}
}

func (s *SessionService) List() []SessionInfo {
	xs := s.mgr.List()
	out := make([]SessionInfo, len(xs))
	for i, x := range xs {
		out[i] = toInfo(x)
	}
	return out
}

// StartResult always carries session info when a session was created, even on
// launch error. If Error is non-empty, the launch failed; the session may still
// have captured stderr/stdout events worth displaying.
type StartResult struct {
	Session SessionInfo `json:"session"`
	Error   string      `json:"error,omitempty"`
}

func (s *SessionService) Start(cfgID string) (StartResult, error) {
	var cfg config.LaunchConfig
	found := false
	for _, c := range s.ws.Configs() {
		if c.ID == cfgID {
			cfg = c
			found = true
			break
		}
	}
	if !found {
		return StartResult{}, fmt.Errorf("config %s not found", cfgID)
	}
	sess, err := s.mgr.Start(context.Background(), cfg)
	result := StartResult{}
	if sess != nil {
		result.Session = toInfo(sess)
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result, nil
}

func (s *SessionService) Stop(id string) error { return s.mgr.Stop(id) }

func (s *SessionService) Restart(id string) (StartResult, error) {
	sess := s.mgr.Get(id)
	if sess == nil {
		return StartResult{}, errors.New("session not found")
	}
	cfgID := sess.CfgID
	_ = s.mgr.Stop(id)
	return s.Start(cfgID)
}

func (s *SessionService) SetExceptionBreakpoints(id string, filters []string) error {
	sess := s.liveSession(id)
	if sess == nil {
		return nil
	}
	return sess.Client().SetExceptionBreakpoints(filters)
}

func (s *SessionService) StopByCfg(cfgID string) error {
	sess := s.mgr.FindByCfg(cfgID)
	if sess == nil {
		return errors.New("no running session for that config")
	}
	return s.mgr.Stop(sess.ID)
}

func (s *SessionService) liveSession(id string) *session.Session {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil {
		return nil
	}
	st := sess.State()
	if st == session.StateExited || st == session.StateError {
		return nil
	}
	return sess
}

func (s *SessionService) Continue(id string) error {
	sess := s.liveSession(id)
	if sess == nil {
		return nil
	}
	return sess.Client().Continue(sess.StoppedThread())
}

func (s *SessionService) StepOver(id string) error {
	sess := s.liveSession(id)
	if sess == nil {
		return nil
	}
	return sess.Client().Next(sess.StoppedThread())
}

func (s *SessionService) StepIn(id string) error {
	sess := s.liveSession(id)
	if sess == nil {
		return nil
	}
	return sess.Client().StepIn(sess.StoppedThread())
}

func (s *SessionService) StepOut(id string) error {
	sess := s.liveSession(id)
	if sess == nil {
		return nil
	}
	return sess.Client().StepOut(sess.StoppedThread())
}

func (s *SessionService) Pause(id string) error {
	sess := s.liveSession(id)
	if sess == nil {
		return nil
	}
	tid := sess.StoppedThread()
	if tid == 0 {
		resp, err := sess.Client().Threads()
		if err == nil && len(resp.Body.Threads) > 0 {
			tid = resp.Body.Threads[0].Id
		}
	}
	return sess.Client().Pause(tid)
}

func (s *SessionService) SetBreakpoints(id, sourcePath string, lines []int) (any, error) {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil {
		return nil, errors.New("session not running")
	}
	resp, err := sess.Client().SetBreakpoints(sourcePath, lines)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ConfigurationDone signals the DAP server that the client is finished with
// configuration (e.g. setting breakpoints) so the program can resume.
// Called by the frontend after start(), once breakpoints have been pushed.
func (s *SessionService) ConfigurationDone(id string) error {
	sess := s.mgr.Get(id)
	if sess == nil {
		return errors.New("session not found")
	}
	return sess.ConfigurationDone()
}

func (s *SessionService) StackTrace(id string, threadID int) (any, error) {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil {
		return nil, errors.New("session not running")
	}
	if threadID == 0 {
		threadID = sess.StoppedThread()
	}
	resp, err := sess.Client().StackTrace(threadID)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *SessionService) Threads(id string) (any, error) {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil || sess.State() == session.StateExited || sess.State() == session.StateError {
		// no-op for terminated sessions
		return map[string]any{"threads": []any{}}, nil
	}
	resp, err := sess.Client().Threads()
	if err != nil {
		return map[string]any{"threads": []any{}}, nil
	}
	return resp.Body, nil
}

func (s *SessionService) Scopes(id string, frameID int) (any, error) {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil {
		return nil, errors.New("session not running")
	}
	resp, err := sess.Client().Scopes(frameID)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *SessionService) Variables(id string, ref int) (any, error) {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil {
		return nil, errors.New("session not running")
	}
	resp, err := sess.Client().Variables(ref)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *SessionService) Evaluate(id string, expr string, frameID int) (any, error) {
	sess := s.mgr.Get(id)
	if sess == nil || sess.Client() == nil {
		return nil, errors.New("session not running")
	}
	resp, err := sess.Client().Evaluate(expr, frameID)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ResourceInfo holds process resource stats.
type ResourceInfo struct {
	PID     int     `json:"pid"`
	Label   string  `json:"label"`
	State   string  `json:"state"`
	Port    int     `json:"port"`
	RSSMb   float64 `json:"rssMb"`
	CPU     string  `json:"cpu"`
	Elapsed string  `json:"elapsed"`
}

func (s *SessionService) Resources(id string) (ResourceInfo, error) {
	sess := s.mgr.Get(id)
	if sess == nil {
		return ResourceInfo{}, errors.New("not found")
	}
	info := ResourceInfo{
		PID:   sess.PID,
		Label: sess.Label,
		State: string(sess.State()),
		Port:  sess.Port,
	}
	if sess.PID > 0 {
		r, c, e := psStats(sess.PID)
		info.RSSMb = r
		info.CPU = c
		info.Elapsed = e
	}
	return info, nil
}

func (s *SessionService) AllResources() []ResourceInfo {
	var out []ResourceInfo
	for _, sess := range s.mgr.List() {
		info := ResourceInfo{PID: sess.PID, Label: sess.Label, State: string(sess.State()), Port: sess.Port}
		if sess.PID > 0 {
			r, c, e := psStats(sess.PID)
			info.RSSMb = r
			info.CPU = c
			info.Elapsed = e
		}
		out = append(out, info)
	}
	return out
}

func (s *SessionService) AppResources() ResourceInfo {
	pid := os.Getpid()
	r, c, e := psStats(pid)
	return ResourceInfo{PID: pid, Label: "DelveUI", State: "running", RSSMb: r, CPU: c, Elapsed: e}
}

func psStats(pid int) (rssMb float64, cpu string, elapsed string) {
	out, err := exec.Command("ps", "-o", "rss=,pcpu=,etime=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "-", "-"
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 3 {
		if rss, err := strconv.ParseFloat(fields[0], 64); err == nil {
			rssMb = rss / 1024.0
		}
		cpu = fields[1] + "%"
		elapsed = fields[2]
	}
	return
}

// CleanResult is returned by CleanDebugBinaries.
type CleanResult struct {
	Dir     string   `json:"dir"`
	Removed []string `json:"removed"`
	Count   int      `json:"count"`
}

// CleanDebugBinaries removes Delve's auto-generated __debug_bin* files from
// the current workspace root (recursively). Also sweeps each launch config's
// cwd/program dir, since those may sit outside the workspace root.
func (s *SessionService) CleanDebugBinaries() (CleanResult, error) {
	root := s.ws.Root()
	seen := map[string]bool{}
	var all []string
	if root != "" {
		xs, _ := debugclean.CleanRecursive(root)
		for _, p := range xs {
			if !seen[p] {
				seen[p] = true
				all = append(all, p)
			}
		}
	}
	for _, cfg := range s.ws.Configs() {
		for _, d := range []string{cfg.Cwd, cfg.Program} {
			if d == "" {
				continue
			}
			if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
				continue
			}
			xs, _ := debugclean.CleanDir(d)
			for _, p := range xs {
				if !seen[p] {
					seen[p] = true
					all = append(all, p)
				}
			}
		}
	}
	if root == "" && len(all) == 0 {
		return CleanResult{}, errors.New("no workspace open")
	}
	return CleanResult{Dir: root, Removed: all, Count: len(all)}, nil
}

// KillPort finds and kills the process listening on the given TCP port.
func (s *SessionService) KillPort(port int) error {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port)).Output()
	if err != nil {
		return fmt.Errorf("no process found on port %d", port)
	}
	pids := strings.Fields(strings.TrimSpace(string(out)))
	for _, pid := range pids {
		_ = exec.Command("kill", "-9", pid).Run()
	}
	return nil
}

// FileService reads source files for the editor view.
type FileService struct{}

func NewFileService() *FileService { return &FileService{} }

type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

var allowedExts = map[string]bool{
	".go": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".mod": true, ".sum": true, ".md": true, ".txt": true, ".env": true,
	".sh": true, ".bash": true, ".zsh": true, ".cfg": true, ".conf": true,
	".proto": true, ".sql": true, ".graphql": true, ".html": true, ".css": true,
}

var hiddenDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
	".cache": true, ".idea": true, ".vscode": true, ".zed": true, ".delveui": true,
	"dist": true, "build": true,
}

func (f *FileService) ListDir(dirPath string) ([]DirEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var dirs, files []DirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !e.IsDir() {
			continue
		}
		if e.IsDir() {
			if hiddenDirs[name] {
				continue
			}
			dirs = append(dirs, DirEntry{Name: name, Path: filepath.Join(dirPath, name), IsDir: true})
		} else {
			ext := filepath.Ext(name)
			if allowedExts[ext] || ext == "" {
				files = append(files, DirEntry{Name: name, Path: filepath.Join(dirPath, name), IsDir: false})
			}
		}
	}
	return append(dirs, files...), nil
}

func (f *FileService) ListGoFiles(root string) ([]string, error) {
	var results []string
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if d.IsDir() {
			if hiddenDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) == ".go" {
			rel, _ := filepath.Rel(root, path)
			results = append(results, rel)
			count++
			if count >= 5000 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return results, err
	}
	return results, nil
}

func (f *FileService) ReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (f *FileService) WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
