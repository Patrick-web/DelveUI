package debugfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jp/DelveUI/internal/config"
)

type Entry struct {
	ID        string                `json:"id"`
	Path      string                `json:"path"`
	Label     string                `json:"label"`
	IsDefault bool                  `json:"isDefault"`
	AddedAt   time.Time             `json:"addedAt"`
	Configs   []config.LaunchConfig `json:"configs"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	Entries []Entry `json:"entries"`
}

func NewStore() (*Store, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cfgDir, "DelveUI")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "debug-files.json")}
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

// Clear removes every registered debug file entry.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = nil
	return s.save()
}

func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.Entries))
	copy(out, s.Entries)
	return out
}

func (s *Store) Add(path string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.Entries {
		if e.Path == path {
			return e, nil
		}
	}
	cfgs, err := config.LoadFile(path)
	if err != nil {
		return Entry{}, err
	}
	label := filepath.Base(filepath.Dir(filepath.Dir(path)))
	if label == "." || label == "/" {
		label = filepath.Base(path)
	}
	entry := Entry{
		ID:      uuid.NewString(),
		Path:    path,
		Label:   label,
		AddedAt: time.Now(),
		Configs: cfgs,
	}
	if len(s.Entries) == 0 {
		entry.IsDefault = true
	}
	s.Entries = append(s.Entries, entry)
	return entry, s.save()
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.Entries {
		if e.ID == id {
			s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("not found")
}

func (s *Store) SetDefault(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i := range s.Entries {
		if s.Entries[i].ID == id {
			s.Entries[i].IsDefault = true
			found = true
		} else {
			s.Entries[i].IsDefault = false
		}
	}
	if !found {
		return fmt.Errorf("not found")
	}
	return s.save()
}

func (s *Store) GetDefault() *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.Entries {
		if e.IsDefault {
			return &e
		}
	}
	if len(s.Entries) > 0 {
		return &s.Entries[0]
	}
	return nil
}

func (s *Store) Reload(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Entries {
		if s.Entries[i].ID == id {
			cfgs, err := config.LoadFile(s.Entries[i].Path)
			if err != nil {
				return err
			}
			s.Entries[i].Configs = cfgs
			return s.save()
		}
	}
	return fmt.Errorf("not found")
}

func (s *Store) ReloadAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Entries {
		cfgs, err := config.LoadFile(s.Entries[i].Path)
		if err != nil {
			continue
		}
		s.Entries[i].Configs = cfgs
	}
	return s.save()
}
