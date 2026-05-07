package session

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/jp/DelveUI/internal/adapter"
	"github.com/jp/DelveUI/internal/config"
)

type Manager struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	subscribers []chan Event
	adapters    *adapter.Registry
}

func NewManager(adapters *adapter.Registry) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		adapters: adapters,
	}
}

// Subscribe returns a channel that receives every session event. Caller must drain.
func (m *Manager) Subscribe() <-chan Event {
	ch := make(chan Event, 256)
	m.mu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.mu.Unlock()
	return ch
}

func (m *Manager) publish(e Event) {
	m.mu.Lock()
	subs := append([]chan Event(nil), m.subscribers...)
	m.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

// FindByCfg returns the first session currently tied to cfgID (running or stopped), or nil.
func (m *Manager) FindByCfg(cfgID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.CfgID == cfgID && s.state != StateExited {
			return s
		}
	}
	return nil
}

func (m *Manager) Start(ctx context.Context, cfg config.LaunchConfig) (*Session, error) {
	spec, err := m.adapters.Resolve(cfg.Language)
	if err != nil {
		// Adapter not installed — try auto-install, matching Zed's
		// pattern where debug adapters are downloaded on first use.
		spec, err = m.tryAutoInstall(ctx, cfg.Language)
	}
	if err != nil {
		return nil, err
	}

	s := &Session{
		ID:    uuid.NewString(),
		CfgID: cfg.ID,
		Label: cfg.Label,
		Cfg:   cfg,
		state: StateIdle,
		bus:   m.publish,
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()

	if err := s.start(ctx, spec); err != nil {
		s.emit(Event{Kind: "error", Message: err.Error()})
		s.killProcess()
		s.setState(StateError)
		return s, err
	}
	return s, nil
}

// tryAutoInstall checks if the adapter is registered (even if not installed),
// runs its install command, and returns the resolved spec. Returns the
// original resolve error if the adapter isn't registered or install fails.
func (m *Manager) tryAutoInstall(ctx context.Context, language string) (adapter.ProcessSpec, error) {
	spec, ok := m.adapters.Get(language)
	if !ok {
		return adapter.ProcessSpec{}, fmt.Errorf("no debug adapter registered for language %q", language)
	}
	if spec.InstallCmd == "" {
		return adapter.ProcessSpec{}, fmt.Errorf("debug adapter for %q is not installed", language)
	}

	m.publish(Event{Kind: "output", Category: "console",
		Output: fmt.Sprintf("[delveui] Installing %s…\n", spec.Label)})

	installErr := adapter.Install(ctx, m.adapters, spec, func(line string) {
		m.publish(Event{Kind: "output", Category: "console", Output: line + "\n"})
	})
	if installErr != nil {
		return adapter.ProcessSpec{}, fmt.Errorf("auto-install %s failed: %w", language, installErr)
	}

	m.publish(Event{Kind: "output", Category: "console",
		Output: fmt.Sprintf("[delveui] %s installed.\n", spec.Label)})

	return m.adapters.Resolve(language)
}

func (m *Manager) Stop(id string) error {
	s := m.Get(id)
	if s == nil {
		return fmt.Errorf("session %s not found", id)
	}
	s.stop()
	return nil
}

func (m *Manager) StopAll() {
	for _, s := range m.List() {
		s.stop()
	}
}
