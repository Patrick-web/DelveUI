package session

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	godap "github.com/google/go-dap"

	"github.com/jp/DelveUI/internal/adapter"
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

func (s *Session) start(ctx context.Context, spec adapter.ProcessSpec) error {
	s.setState(StateStarting)
	s.initialized = make(chan struct{})

	port, err := freePort()
	if err != nil {
		return err
	}
	s.Port = port

	args := append([]string{}, spec.BinaryArgs...)
	flag := spec.PortFlag
	if flag == "" {
		flag = "--listen $HOST:$PORT"
	}
	flag = strings.ReplaceAll(flag, "$HOST", "127.0.0.1")
	flag = strings.ReplaceAll(flag, "$PORT", strconv.Itoa(port))
	args = append(args, strings.Fields(flag)...)
	if spec.TargetViaCLI {
		args = append(args, s.Cfg.Program)
		args = append(args, s.Cfg.Args...)
	}
	cmd := exec.CommandContext(ctx, spec.Binary, args...)
	if s.Cfg.Cwd != "" {
		cmd.Dir = s.Cfg.Cwd
	} else if s.Cfg.Program != "" {
		cmd.Dir = s.Cfg.Program
	}
	cmd.SysProcAttr = processSysProcAttr()
	cmd.Env = enrichedEnv(spec.ExtraPath)
	cmd.Stdout = logWriter{s: s, cat: fmt.Sprintf("%s-stdout", spec.Language)}
	cmd.Stderr = logWriter{s: s, cat: fmt.Sprintf("%s-stderr", spec.Language)}
	if err := cmd.Start(); err != nil {
		s.setState(StateError)
		return fmt.Errorf("start %s adapter: %w", spec.Language, err)
	}
	s.cmd = cmd
	s.PID = cmd.Process.Pid

	workDir := cmd.Dir
	go func() {
		err := cmd.Wait()
		if spec.Language == "go" {
			if removed, _ := debugclean.CleanDir(workDir); len(removed) > 0 {
				s.emit(Event{Kind: "output", Category: "console",
					Output: fmt.Sprintf("[delveui] cleaned %d debug binary file(s)\n", len(removed))})
			}
		}
		s.emit(Event{Kind: "exited", Message: fmt.Sprintf("%s adapter exited: %v", spec.Language, err)})
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

	if _, err := client.Initialize("delveui", spec.AdapterID); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	isAttach := s.Cfg.Request == "attach" || spec.TargetViaCLI
	dapRequest := s.Cfg.Request
	if spec.TargetViaCLI && dapRequest != "attach" {
		dapRequest = "attach"
	}
	launchArgs := map[string]any{
		"request": dapRequest,
		"name":    s.Cfg.Label,
		"type":    spec.DAPType,
	}
	if !spec.TargetViaCLI {
		launchArgs["args"] = s.Cfg.Args
	}
	// Delve-specific: mode changes how the Go toolchain builds the binary.
	if spec.Language == "go" {
		launchArgs["mode"] = s.Cfg.Mode
		if len(s.Cfg.BuildFlags) > 0 {
			launchArgs["buildFlags"] = strings.Join(s.Cfg.BuildFlags, " ")
		}
	}
	if isAttach {
		if !spec.TargetViaCLI {
			launchArgs["processId"] = s.Cfg.ProcessID
		}
		if s.Cfg.Cwd != "" {
			launchArgs["cwd"] = s.Cfg.Cwd
		}
		if spec.Language == "go" && s.Cfg.Mode == "" {
			launchArgs["mode"] = "local"
		}
	} else {
		if !spec.TargetViaCLI {
			launchArgs["program"] = s.Cfg.Program
		}
		launchArgs["cwd"] = s.Cfg.Cwd
	}

	// Env precedence: explicit cfg.Env (already includes merged walk-up env
	// files when set by the discovery service) wins; falls back to legacy
	// EnvFile field for hand-written configs that point at a single file.
	if len(s.Cfg.Env) > 0 {
		launchArgs["env"] = s.Cfg.Env
	} else if env, err := config.LoadEnvFile(s.Cfg.EnvFile); err == nil && env != nil {
		launchArgs["env"] = env
	}

	var launchErr error
	if isAttach {
		launchErr = client.Attach(launchArgs)
	} else {
		launchErr = client.Launch(launchArgs)
	}
	if launchErr != nil {
		return launchErr
	}
	// Wait for InitializedEvent before returning so the frontend can set
	// breakpoints during the DAP configuration phase. The frontend explicitly
	// calls ConfigurationDone() after sending its breakpoints.
	select {
	case <-s.initialized:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for DAP InitializedEvent")
	}
	s.emit(Event{Kind: "output", Category: "console", Output: fmt.Sprintf("[delveui] Launched %s (program=%s, mode=%s, adapter=%s) on port %d\n", s.Cfg.Label, s.Cfg.Program, s.Cfg.Mode, spec.Language, s.Port)})
	s.setState(StateRunning)
	return nil
}

// ConfigurationDone tells the DAP server we're finished configuring
// (breakpoints, etc.) so it can resume the program. Idempotent.
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
			// The adapter is in the configuration phase. Signal start() so it can
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

// enrichedEnv returns the current env with additional paths appended to PATH.
// macOS .app bundles have a minimal PATH; this ensures adapter toolchains
// (go, python, node, …) are reachable.
func enrichedEnv(extraPaths []string) []string {
	env := os.Environ()
	home, _ := os.UserHomeDir()
	paths := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
	}
	if home != "" {
		paths = append(paths, home+"/.local/bin", home+"/bin")
	}
	paths = append(paths, extraPaths...)

	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = e + ":" + strings.Join(paths, ":")
			return env
		}
	}
	env = append(env, "PATH="+strings.Join(paths, ":"))
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
