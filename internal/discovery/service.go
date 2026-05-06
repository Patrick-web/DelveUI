package discovery

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jp/DelveUI/internal/config"
	"github.com/jp/DelveUI/internal/session"
)

// workspaceProvider is the contract the Service needs from the rest of
// the app: the active workspace root, plus the running session manager
// (so a Launch can return a session straight to the frontend).
type workspaceProvider interface {
	Root() string
}

type sessionStarter interface {
	Start(ctx context.Context, cfg config.LaunchConfig) (*session.Session, error)
}

// Service is the Wails-bound entry point for the run/attach panel. It owns
// the registry, holds a per-workspace cache, and brokers Launch calls back
// into the session manager so the frontend can use one binding for both
// "discover targets" and "run target".
type Service struct {
	reg     *Registry
	ws      workspaceProvider
	starter sessionStarter
	app     *application.App

	mu          sync.Mutex
	cache       map[string][]Target // root → targets
	lastScanned map[string]time.Time
}

func NewService(reg *Registry, ws workspaceProvider, starter sessionStarter) *Service {
	return &Service{
		reg:         reg,
		ws:          ws,
		starter:     starter,
		cache:       make(map[string][]Target),
		lastScanned: make(map[string]time.Time),
	}
}

func (s *Service) SetApp(app *application.App) { s.app = app }

// TargetList is what the frontend renders. We return targets and a `stale`
// flag so the UI can show "results from prior scan, refreshing…" if the
// user opens the panel while a Refresh is in flight.
type TargetList struct {
	Root        string    `json:"root"`
	Targets     []Target  `json:"targets"`
	Stale       bool      `json:"stale"`
	LastScanned time.Time `json:"lastScanned"`
}

// Targets returns whatever's in the cache for the active workspace, without
// kicking off a scan. Use Refresh for that. Empty cache → empty list (the
// UI calls Refresh on first open).
func (s *Service) Targets() TargetList {
	root := s.ws.Root()
	s.mu.Lock()
	defer s.mu.Unlock()
	return TargetList{
		Root:        root,
		Targets:     append([]Target(nil), s.cache[root]...),
		LastScanned: s.lastScanned[root],
	}
}

// Refresh re-runs every provider against the active workspace root and
// streams results via Wails events. The frontend can either rely on the
// streamed events or re-poll Targets() once `discovery:done` fires.
//
// Events emitted (all carry workspace root as `root`):
//   - discovery:start    {root}
//   - discovery:targets  {root, targets}    (full replacement on completion)
//   - discovery:done     {root, count}
func (s *Service) Refresh() ([]Target, error) {
	root := s.ws.Root()
	if root == "" {
		return nil, errors.New("no workspace open")
	}
	s.emit("discovery:start", map[string]any{"root": root})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		all []Target
		mu  sync.Mutex
		wg  sync.WaitGroup
	)
	for _, p := range s.reg.All() {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			targets, err := p.Discover(ctx, root)
			if err == nil && len(targets) > 0 {
				mu.Lock()
				all = append(all, targets...)
				mu.Unlock()
			}
			procs, err := p.DiscoverProcesses(ctx, root)
			if err == nil && len(procs) > 0 {
				mu.Lock()
				all = append(all, procs...)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()

	// Per-target env walk-up. Done after discovery so providers don't each
	// have to know about the env discovery rules — keeps the contract small.
	for i := range all {
		if all[i].Kind == KindAttach {
			continue // attach has no env semantics
		}
		all[i].EnvFiles = FindEnvFiles(all[i].Dir, root)
	}

	s.mu.Lock()
	s.cache[root] = all
	s.lastScanned[root] = time.Now()
	s.mu.Unlock()

	s.emit("discovery:targets", map[string]any{"root": root, "targets": all})
	s.emit("discovery:done", map[string]any{"root": root, "count": len(all)})
	return all, nil
}

// LaunchResult mirrors services.StartResult so the frontend can register the
// session in its store immediately, without waiting for a session:event with
// a cfgId that resolves to a workspace config (discovery targets aren't in
// workspace.configs — they're a separate, virtual list).
type LaunchResult struct {
	SessionID string              `json:"sessionId"`
	CfgID     string              `json:"cfgId"`
	Label     string              `json:"label"`
	State     string              `json:"state"`
	Port      int                 `json:"port"`
	PID       int                 `json:"pid"`
	Cfg       config.LaunchConfig `json:"cfg"`
	Error     string              `json:"error,omitempty"`
}

func toLaunchResult(sess *session.Session, cfg config.LaunchConfig, err error) LaunchResult {
	r := LaunchResult{Cfg: cfg, CfgID: cfg.ID, Label: cfg.Label}
	if sess != nil {
		r.SessionID = sess.ID
		r.State = string(sess.State())
		r.Port = sess.Port
		r.PID = sess.PID
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

// Launch starts a session for the given target ID. Env files attached to
// the target are merged into the LaunchConfig.Env in walk-up order (the
// outermost file's vars are written first, then each subsequent file
// overrides) — matching dotenv-cli precedence.
func (s *Service) Launch(targetID string) (LaunchResult, error) {
	t, ok := s.find(targetID)
	if !ok {
		return LaunchResult{}, fmt.Errorf("target %s not found", targetID)
	}
	prov := s.reg.Find(t.Provider)
	if prov == nil {
		return LaunchResult{}, fmt.Errorf("provider %s not registered", t.Provider)
	}
	cfg := prov.ToLaunchConfig(t)
	// Carry the discovered file list through to the LaunchConfig so the env
	// inspector can list the sources without re-running discovery.
	cfg.EnvFiles = append([]string(nil), t.EnvFiles...)

	// Merge envFiles into cfg.Env (innermost wins). We do this here rather
	// than at session start so the user can see exactly which keys came
	// from which file in a future "Env" inspector view.
	if len(t.EnvFiles) > 0 {
		merged := map[string]string{}
		maps.Copy(merged, cfg.Env)
		for _, path := range t.EnvFiles {
			env, err := config.LoadEnvFile(path)
			if err != nil || env == nil {
				continue
			}
			maps.Copy(merged, env)
		}
		cfg.Env = merged
	}

	if cfg.Request == "attach" {
		cfg.ProcessID = t.PID
	}

	sess, err := s.starter.Start(context.Background(), cfg)
	return toLaunchResult(sess, cfg, err), nil
}

// LaunchProcess starts an attach session for an arbitrary PID supplied by
// the user (manual picker fallback). The caller provides the workspace
// root via ws.Root(); we synthesize a Target on the fly.
func (s *Service) LaunchProcess(pid int) (LaunchResult, error) {
	if pid <= 0 {
		return LaunchResult{}, errors.New("invalid pid")
	}
	t := Target{
		ID:       fmt.Sprintf("manual-attach-%d", pid),
		Provider: "go",
		Kind:     KindAttach,
		Label:    fmt.Sprintf("attach: PID %d", pid),
		PID:      pid,
		Dir:      s.ws.Root(),
	}
	prov := s.reg.Find("go")
	if prov == nil {
		return LaunchResult{}, errors.New("go provider not registered")
	}
	cfg := prov.ToLaunchConfig(t)
	cfg.ProcessID = pid
	sess, err := s.starter.Start(context.Background(), cfg)
	return toLaunchResult(sess, cfg, err), nil
}

func (s *Service) find(id string) (Target, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, list := range s.cache {
		for _, t := range list {
			if t.ID == id {
				return t, true
			}
		}
	}
	return Target{}, false
}

func (s *Service) emit(name string, data any) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit(name, data)
}
