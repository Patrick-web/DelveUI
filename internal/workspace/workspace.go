package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Recent struct {
	Path     string    `json:"path"`
	LastUsed time.Time `json:"lastUsed"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	Recents []Recent `json:"recents"`
	Active  string   `json:"active"`
}

func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(dir, "DelveUI")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(appDir, "workspaces.json")}
	s.load()
	return s, nil
}

func (s *Store) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, s)
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

func (s *Store) List() []Recent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Recent, len(s.Recents))
	copy(out, s.Recents)
	return out
}

func (s *Store) ActivePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Active
}

func (s *Store) SetActive(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = path
	found := false
	for i := range s.Recents {
		if s.Recents[i].Path == path {
			s.Recents[i].LastUsed = time.Now()
			found = true
			break
		}
	}
	if !found {
		s.Recents = append(s.Recents, Recent{Path: path, LastUsed: time.Now()})
	}
	// sort newest first, cap 10
	for i := 1; i < len(s.Recents); i++ {
		for j := i; j > 0 && s.Recents[j].LastUsed.After(s.Recents[j-1].LastUsed); j-- {
			s.Recents[j], s.Recents[j-1] = s.Recents[j-1], s.Recents[j]
		}
	}
	if len(s.Recents) > 10 {
		s.Recents = s.Recents[:10]
	}
	return s.save()
}
