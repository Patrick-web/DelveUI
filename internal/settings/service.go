package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type VimMapping struct {
	Lhs  string `json:"lhs"`
	Rhs  string `json:"rhs"`
	Mode string `json:"mode"` // "normal" | "visual" | "insert"
}

type Settings struct {
	Theme          string       `json:"theme"`
	TerminalTheme  string       `json:"terminalTheme"`
	VimMode        bool         `json:"vimMode"`
	VimMappings    []VimMapping `json:"vimMappings"`
	UIFontSize     int          `json:"uiFontSize"`
	BufferFontSize int          `json:"bufferFontSize"`
	TermFontSize   int          `json:"termFontSize"`
	LineHeight     string       `json:"lineHeight"`
	DlvPath        string       `json:"dlvPath"`

	// RestoreLastProject controls the auto-open behavior on launch. When true,
	// the most-recently-active project is reopened. When false, the user lands
	// on the welcome page.
	RestoreLastProject *bool `json:"restoreLastProject,omitempty"`

	LeftPanels     []string `json:"leftPanels"`
	RightPanels    []string `json:"rightPanels"`
	DefaultLeftTab  string  `json:"defaultLeftTab"`
	DefaultRightTab string  `json:"defaultRightTab"`
}

type Service struct {
	mu   sync.Mutex
	path string
	data Settings
}

func NewService() (*Service, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cfgDir, "DelveUI")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Service{path: filepath.Join(dir, "settings.json")}
	s.load()
	s.applyDefaults()
	return s, nil
}

func (s *Service) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &s.data)
}

func (s *Service) applyDefaults() {
	if s.data.Theme == "" {
		s.data.Theme = "One Dark"
	}
	if s.data.TerminalTheme == "" {
		s.data.TerminalTheme = "follow"
	}
	if s.data.UIFontSize == 0 {
		s.data.UIFontSize = 13
	}
	if s.data.BufferFontSize == 0 {
		s.data.BufferFontSize = 13
	}
	if s.data.TermFontSize == 0 {
		s.data.TermFontSize = 12
	}
	if s.data.LineHeight == "" {
		s.data.LineHeight = "standard"
	}
	if len(s.data.LeftPanels) == 0 {
		s.data.LeftPanels = []string{"filetree", "breakpoints", "callstack", "threads", "variables", "watch", "resources"}
	}
	if len(s.data.RightPanels) == 0 {
		s.data.RightPanels = []string{"source", "terminal", "console"}
	}
	// Migrate: ensure new panels are present in existing configs
	s.ensurePanel(&s.data.LeftPanels, "filetree")
	s.ensurePanel(&s.data.LeftPanels, "watch")
	s.ensurePanel(&s.data.RightPanels, "source")
	if s.data.DefaultLeftTab == "" {
		s.data.DefaultLeftTab = "breakpoints"
	}
	if s.data.DefaultRightTab == "" {
		s.data.DefaultRightTab = "terminal"
	}
	if s.data.RestoreLastProject == nil {
		t := true
		s.data.RestoreLastProject = &t
	}
	if s.data.VimMappings == nil {
		s.data.VimMappings = []VimMapping{}
	}
}

func (s *Service) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

func (s *Service) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

func (s *Service) ensurePanel(panels *[]string, id string) {
	for _, p := range *panels {
		if p == id {
			return
		}
	}
	*panels = append([]string{id}, *panels...)
}

func (s *Service) Update(next Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = next
	s.applyDefaults()
	return s.save()
}

// Reset clears all settings back to defaults and persists them.
func (s *Service) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = Settings{}
	s.applyDefaults()
	return s.save()
}
