package session

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	godap "github.com/google/go-dap"

	"github.com/jp/DelveUI/internal/config"
	"github.com/jp/DelveUI/internal/dap"
	"github.com/jp/DelveUI/internal/debugclean"
)

type State string

const (
	StateIdle     State = "idle"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopped  State = "stopped" // paused at breakpoint
	StateExited   State = "exited"
	StateError    State = "error"
)

type Event struct {
	SessionID string         `json:"sessionId"`
	CfgID     string         `json:"cfgId,omitempty"`
	Kind      string         `json:"kind"` // state | output | stopped | exited | threads | error
	State     State          `json:"state,omitempty"`
	Output    string         `json:"output,omitempty"`
	Category  string         `json:"category,omitempty"`
	ThreadID  int            `json:"threadId,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Message   string         `json:"message,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

type Session struct {
	ID      string              `json:"id"`
	CfgID   string              `json:"cfgId"`
	Label   string              `json:"label"`
	Cfg     config.LaunchConfig `json:"cfg"`
	Port    int                 `json:"port"`
	PID     int                 `json:"pid"`
	mu      sync.Mutex
	state   State
	cmd     *exec.Cmd
	client  *dap.Client
	stopped struct {
		ThreadID int
		Reason   string
	}
	bus         func(Event)
	initialized chan struct{} // closed when DAP InitializedEvent arrives
	cfgDoneOnce sync.Once
}

func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) setState(st State) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
	s.emit(Event{Kind: "state", State: st})
}

func (s *Session) emit(e Event) {
	e.SessionID = s.ID
	e.CfgID = s.CfgID
	if s.bus != nil {
		s.bus(e)
	}
}

func (s *Session) Client() *dap.Client { return s.client }

func (s *Session) StoppedThread() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped.ThreadID
}

func (s *Session) start(ctx context.Context, dlvPath string) error {
	s.setState(StateStarting)
	s.initialized = make(chan struct{})

	port, err := freePort()
	if err != nil {
		return err
	}
	s.Port = port

	args := []string{"dap", "--listen", fmt.Sprintf("127.0.0.1:%d", port)}
	cmd := exec.CommandContext(ctx, dlvPath, args...)
	if s.Cfg.Cwd != "" {
		cmd.Dir = s.Cfg.Cwd
	} else if s.Cfg.Program != "" {
		cmd.Dir = s.Cfg.Program
	}
	cmd.SysProcAttr = processSysProcAttr()
	cmd.Env = enrichedEnv()
	cmd.Stdout = logWriter{s: s, cat: "dlv-stdout"}
	cmd.Stderr = logWriter{s: s, cat: "dlv-stderr"}
	if err := cmd.Start(); err != nil {
		s.setState(StateError)
		return fmt.Errorf("start dlv: %w", err)
	}
	s.cmd = cmd
	s.PID = cmd.Process.Pid

	workDir := cmd.Dir
	go func() {
		err := cmd.Wait()
		if removed, _ := debugclean.CleanDir(workDir); len(removed) > 0 {
			s.emit(Event{Kind: "output", Category: "console",
				Output: fmt.Sprintf("[delveui] cleaned %d debug binary file(s)\n", len(removed))})
		}
		s.emit(Event{Kind: "exited", Message: fmt.Sprintf("dlv exited: %v", err)})
		s.setState(StateExited)
	}()

	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := dap.Dial(dctx, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		_ = cmd.Process.Kill()
		s.setState(StateError)
		return err
	}
	s.client = client
	go s.eventLoop()

	if _, err := client.Initialize("delveui"); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	launchArgs := map[string]any{
		"request":    s.Cfg.Request,
		"mode":       s.Cfg.Mode,
		"program":    s.Cfg.Program,
		"cwd":        s.Cfg.Cwd,
		"name":       s.Cfg.Label,
		"type":       "go",
		"args":       s.Cfg.Args,
		"buildFlags": strings.Join(s.Cfg.BuildFlags, " "),
	}
	if env, err := config.LoadEnvFile(s.Cfg.EnvFile); err == nil && env != nil {
		launchArgs["env"] = env
	}
	if err := client.Launch(launchArgs); err != nil {
		return err
	}
	// Wait for InitializedEvent before returning so the frontend can set
	// breakpoints during the DAP configuration phase. The frontend explicitly
	// calls ConfigurationDone() after sending its breakpoints.
	select {
	case <-s.initialized:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for DAP InitializedEvent")
	}
	s.emit(Event{Kind: "output", Category: "console", Output: fmt.Sprintf("[delveui] Launched %s (program=%s, mode=%s) on dlv port %d\n", s.Cfg.Label, s.Cfg.Program, s.Cfg.Mode, s.Port)})
	s.setState(StateRunning)
	return nil
}

// ConfigurationDone tells dlv we're finished configuring (breakpoints, etc.)
// so it can resume the program. Idempotent.
func (s *Session) ConfigurationDone() error {
	if s.client == nil {
		return fmt.Errorf("session not running")
	}
	var err error
	s.cfgDoneOnce.Do(func() {
		err = s.client.ConfigurationDone()
	})
	return err
}

func (s *Session) eventLoop() {
	for msg := range s.client.Events() {
		switch ev := msg.(type) {
		case *godap.OutputEvent:
			s.emit(Event{Kind: "output", Output: ev.Body.Output, Category: ev.Body.Category})
		case *godap.StoppedEvent:
			s.mu.Lock()
			s.stopped.ThreadID = ev.Body.ThreadId
			s.stopped.Reason = ev.Body.Reason
			s.state = StateStopped
			s.mu.Unlock()
			s.emit(Event{Kind: "stopped", ThreadID: ev.Body.ThreadId, Reason: ev.Body.Reason, State: StateStopped})
		case *godap.InitializedEvent:
			// Delve is in the configuration phase. Signal start() so it can
			// return; the frontend will send breakpoints and then explicitly
			// call ConfigurationDone via SessionService.
			if s.initialized != nil {
				select {
				case <-s.initialized:
				default:
					close(s.initialized)
				}
			}
		case *godap.BreakpointEvent:
			s.emit(Event{Kind: "breakpoint", Extra: map[string]any{
				"reason":     ev.Body.Reason,
				"breakpoint": ev.Body.Breakpoint,
			}})
		case *godap.TerminatedEvent:
			s.setState(StateExited)
		case *godap.ThreadEvent:
			s.emit(Event{Kind: "threads", Reason: ev.Body.Reason, ThreadID: ev.Body.ThreadId})
		}
	}
}

func (s *Session) stop() {
	if s.client != nil {
		_ = s.client.Disconnect(true)
		s.client.Close()
	}
	s.killProcess()
	s.setState(StateExited)
}

func (s *Session) killProcess() {
	killProcessGroup(s.cmd)
}

type logWriter struct {
	s   *Session
	cat string
}

func (w logWriter) Write(p []byte) (int, error) {
	w.s.emit(Event{Kind: "output", Output: string(p), Category: w.cat})
	return len(p), nil
}

// enrichedEnv returns the current env with common Go/bin paths added to PATH.
// macOS .app bundles have a minimal PATH; this ensures `go`, `dlv`, etc. are reachable.
func enrichedEnv() []string {
	env := os.Environ()
	home, _ := os.UserHomeDir()
	extraPaths := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/usr/local/go/bin",
	}
	if home != "" {
		extraPaths = append(extraPaths, home+"/go/bin", home+"/.local/bin", home+"/bin")
	}
	if gp := os.Getenv("GOPATH"); gp != "" {
		extraPaths = append(extraPaths, gp+"/bin")
	}

	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = e + ":" + strings.Join(extraPaths, ":")
			return env
		}
	}
	env = append(env, "PATH="+strings.Join(extraPaths, ":"))
	return env
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
