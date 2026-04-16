package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Settings struct {
	Theme          string `json:"theme"`
	TerminalTheme  string `json:"terminalTheme"`
	VimMode        bool   `json:"vimMode"`
	UIFontSize     int    `json:"uiFontSize"`
	BufferFontSize int    `json:"bufferFontSize"`
	TermFontSize   int    `json:"termFontSize"`
	LineHeight     string `json:"lineHeight"`

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
		s.data.LeftPanels = []string{"breakpoints", "callstack", "threads", "variables", "resources"}
	}
	if len(s.data.RightPanels) == 0 {
		s.data.RightPanels = []string{"terminal", "console"}
	}
	if s.data.DefaultLeftTab == "" {
		s.data.DefaultLeftTab = "breakpoints"
	}
	if s.data.DefaultRightTab == "" {
		s.data.DefaultRightTab = "terminal"
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

func (s *Service) Update(next Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = next
	s.applyDefaults()
	return s.save()
}
