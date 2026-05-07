package adapter

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AdapterInfo is the DTO sent to the frontend for each registered adapter.
type AdapterInfo struct {
	Language    string `json:"language"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	InstallCmd  string `json:"installCmd"`
	InstallURL  string `json:"installUrl,omitempty"`
	Installing  bool   `json:"installing"`
	Error       string `json:"error,omitempty"`
}

// Service is the Wails-bound entry point for the adapter settings panel.
type Service struct {
	reg *Registry
	app *application.App

	mu         sync.Mutex
	installing map[string]bool
	lastError  map[string]string
}

// NewService creates an adapter management service bound to a registry.
func NewService(reg *Registry) *Service {
	return &Service{
		reg:        reg,
		installing: make(map[string]bool),
		lastError:  make(map[string]string),
	}
}

// SetApp injects the running application for event emission.
func (s *Service) SetApp(app *application.App) { s.app = app }

// List returns all registered adapters with their current status.
func (s *Service) List() []AdapterInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	specs := s.reg.All()
	out := make([]AdapterInfo, 0, len(specs))
	for _, spec := range specs {
		out = append(out, AdapterInfo{
			Language:    spec.Language,
			Label:       spec.Label,
			Description: spec.Description,
			Installed:   s.reg.Installed(spec.Language),
			InstallCmd:  spec.InstallCmd,
			InstallURL:  spec.InstallURL,
			Installing:  s.installing[spec.Language],
			Error:       s.lastError[spec.Language],
		})
	}
	return out
}

// Install starts an async installation for the given language. Emits events
// adapter:install:output for each line and adapter:install:done on completion.
func (s *Service) Install(language string) {
	s.mu.Lock()
	if s.installing[language] {
		s.mu.Unlock()
		return
	}
	s.installing[language] = true
	s.lastError[language] = ""
	s.mu.Unlock()

	s.emit("adapter:install:start", map[string]any{"language": language})

	spec, ok := s.reg.Get(language)
	if !ok {
		s.finishInstall(language, "adapter not registered")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := Install(ctx, s.reg, spec, func(line string) {
		s.emit("adapter:install:output", map[string]any{"language": language, "line": line})
	})

	s.mu.Lock()
	delete(s.installing, language)
	if err != nil {
		s.lastError[language] = err.Error()
	}
	s.mu.Unlock()

	if err != nil {
		s.emit("adapter:install:error", map[string]any{"language": language, "error": err.Error()})
	} else {
		s.emit("adapter:install:done", map[string]any{"language": language})
	}
}

func (s *Service) finishInstall(language, errMsg string) {
	s.mu.Lock()
	delete(s.installing, language)
	if errMsg != "" {
		s.lastError[language] = errMsg
	}
	s.mu.Unlock()
	s.emit("adapter:install:error", map[string]any{"language": language, "error": errMsg})
}

func (s *Service) emit(name string, data any) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit(name, data)
}
