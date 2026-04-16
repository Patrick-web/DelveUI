package session

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/google/uuid"

	"github.com/jp/DelveUI/internal/config"
)

type Manager struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	subscribers []chan Event
	dlvPath     string
}

func NewManager() (*Manager, error) {
	dlv, err := exec.LookPath("dlv")
	if err != nil {
		return nil, fmt.Errorf("dlv not found on PATH: %w", err)
	}
	return &Manager{sessions: make(map[string]*Session), dlvPath: dlv}, nil
}

func (m *Manager) DlvPath() string { return m.dlvPath }

func (m *Manager) SetDlvPath(path string) { m.dlvPath = path }

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

	if err := s.start(ctx, m.dlvPath); err != nil {
		s.emit(Event{Kind: "error", Message: err.Error()})
		s.killProcess()
		s.setState(StateError)
		return s, err
	}
	return s, nil
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
