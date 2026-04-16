package themes

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed all:bundled
var bundledFS embed.FS

type TerminalColors struct {
	Black        string `json:"black"`
	Red          string `json:"red"`
	Green        string `json:"green"`
	Yellow       string `json:"yellow"`
	Blue         string `json:"blue"`
	Magenta      string `json:"magenta"`
	Cyan         string `json:"cyan"`
	White        string `json:"white"`
	BrightBlack  string `json:"brightBlack"`
	BrightRed    string `json:"brightRed"`
	BrightGreen  string `json:"brightGreen"`
	BrightYellow string `json:"brightYellow"`
	BrightBlue   string `json:"brightBlue"`
	BrightMagenta string `json:"brightMagenta"`
	BrightCyan   string `json:"brightCyan"`
	BrightWhite  string `json:"brightWhite"`
	Background   string `json:"background"`
	Foreground   string `json:"foreground"`
	Cursor       string `json:"cursor"`
}

type ThemeStyle struct {
	Bg          string `json:"bg"`
	BgElevated  string `json:"bgElevated"`
	BgSubtle    string `json:"bgSubtle"`
	Surface     string `json:"surface"`
	Text        string `json:"text"`
	TextMuted   string `json:"textMuted"`
	TextFaint   string `json:"textFaint"`
	Border      string `json:"border"`
	BorderSubtle string `json:"borderSubtle"`
	Accent      string `json:"accent"`
	AccentSubtle string `json:"accentSubtle"`
	Danger      string `json:"danger"`
	Warning     string `json:"warning"`
	Success     string `json:"success"`
	Info        string `json:"info"`
	SynKeyword  string `json:"synKeyword"`
	SynString   string `json:"synString"`
	SynNumber   string `json:"synNumber"`
	SynFn       string `json:"synFn"`
	SynComment  string `json:"synComment"`
	Terminal    TerminalColors `json:"terminal"`
}

type ThemeDefinition struct {
	Name       string     `json:"name"`
	Author     string     `json:"author"`
	Appearance string     `json:"appearance"`
	Style      ThemeStyle `json:"style"`
}

type ThemeMeta struct {
	Name       string `json:"name"`
	Author     string `json:"author"`
	Appearance string `json:"appearance"`
	Bundled    bool   `json:"bundled"`
}

type Service struct {
	mu       sync.Mutex
	userDir  string
	bundled  map[string]ThemeDefinition
}

func NewService() (*Service, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	userDir := filepath.Join(cfgDir, "DelveUI", "themes")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, err
	}
	s := &Service{userDir: userDir, bundled: make(map[string]ThemeDefinition)}
	s.loadBundled()
	return s, nil
}

func (s *Service) loadBundled() {
	entries, _ := bundledFS.ReadDir("bundled")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := bundledFS.ReadFile("bundled/" + e.Name())
		if err != nil {
			continue
		}
		var t ThemeDefinition
		if json.Unmarshal(data, &t) == nil && t.Name != "" {
			s.bundled[t.Name] = t
		}
	}
}

func (s *Service) List() []ThemeMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]bool)
	var out []ThemeMeta
	// user themes first (override bundled)
	for _, t := range s.readUserThemes() {
		out = append(out, ThemeMeta{Name: t.Name, Author: t.Author, Appearance: t.Appearance, Bundled: false})
		seen[t.Name] = true
	}
	for _, t := range s.bundled {
		if !seen[t.Name] {
			out = append(out, ThemeMeta{Name: t.Name, Author: t.Author, Appearance: t.Appearance, Bundled: true})
		}
	}
	return out
}

func (s *Service) Get(name string) (ThemeDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// user override
	for _, t := range s.readUserThemes() {
		if t.Name == name {
			return t, nil
		}
	}
	if t, ok := s.bundled[name]; ok {
		return t, nil
	}
	return ThemeDefinition{}, fmt.Errorf("theme %q not found", name)
}

func (s *Service) Install(data string) (ThemeMeta, error) {
	var t ThemeDefinition
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return ThemeMeta{}, fmt.Errorf("invalid theme JSON: %w", err)
	}
	if t.Name == "" {
		return ThemeMeta{}, fmt.Errorf("theme must have a name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slug := strings.ReplaceAll(strings.ToLower(t.Name), " ", "-")
	path := filepath.Join(s.userDir, slug+".json")
	formatted, _ := json.MarshalIndent(t, "", "  ")
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return ThemeMeta{}, err
	}
	return ThemeMeta{Name: t.Name, Author: t.Author, Appearance: t.Appearance, Bundled: false}, nil
}

func (s *Service) ImportFile(path string) (ThemeMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemeMeta{}, err
	}
	return s.Install(string(data))
}

func (s *Service) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bundled[name]; ok {
		return fmt.Errorf("cannot remove bundled theme %q", name)
	}
	entries, _ := os.ReadDir(s.userDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.userDir, e.Name()))
		if err != nil {
			continue
		}
		var t ThemeDefinition
		if json.Unmarshal(data, &t) == nil && t.Name == name {
			return os.Remove(filepath.Join(s.userDir, e.Name()))
		}
	}
	return fmt.Errorf("user theme %q not found", name)
}

func (s *Service) readUserThemes() []ThemeDefinition {
	entries, _ := os.ReadDir(s.userDir)
	var out []ThemeDefinition
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.userDir, e.Name()))
		if err != nil {
			continue
		}
		var t ThemeDefinition
		if json.Unmarshal(data, &t) == nil && t.Name != "" {
			out = append(out, t)
		}
	}
	return out
}
