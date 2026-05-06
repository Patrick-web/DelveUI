// Package discovery exposes a pluggable interface for finding runnable /
// debuggable targets within a workspace, without requiring a hand-written
// debug config file. Each language registers a Provider; targets are
// surfaced to the UI as a live, refreshable list.
package discovery

import (
	"context"
	"sync"

	"github.com/jp/DelveUI/internal/config"
)

// Kind classifies a discovered target. New kinds may be added as more
// providers come online — UI groups by Kind so unknown values render as
// their string label.
type Kind string

const (
	KindRun       Kind = "run"
	KindTest      Kind = "test"
	KindBenchmark Kind = "benchmark"
	KindExample   Kind = "example"
	KindAttach    Kind = "attach"
)

// Target is a single thing the user can launch or attach. IDs are stable
// across refreshes for the same logical target so UI selection / running
// state survives a re-scan.
type Target struct {
	ID          string            `json:"id"`
	Provider    string            `json:"provider"`
	Kind        Kind              `json:"kind"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	Dir         string            `json:"dir"`
	Program     string            `json:"program"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	// EnvFiles are the dotenv files discovered for this target, in order
	// of precedence (later entries override earlier). The launcher loads
	// each file and merges the values into the launch env at run time.
	EnvFiles []string `json:"envFiles,omitempty"`
	// PID is set for KindAttach targets.
	PID int `json:"pid,omitempty"`
	// Source location for "go to definition" affordance in the UI.
	SourceFile string `json:"sourceFile,omitempty"`
	SourceLine int    `json:"sourceLine,omitempty"`
}

// Provider is the contract a language plugin implements. Implementations
// must be safe for concurrent use; Discover/DiscoverProcesses may be called
// from arbitrary goroutines.
type Provider interface {
	Name() string

	// Discover scans a workspace root for launchable targets. Implementations
	// should bail out cleanly on context cancellation and avoid descending
	// into common ignored directories (vendor, node_modules, .git, …).
	Discover(ctx context.Context, root string) ([]Target, error)

	// DiscoverProcesses returns currently-running processes the provider
	// considers attachable. Returning (nil, nil) is fine for providers that
	// don't support attach — the UI's manual process picker still works.
	DiscoverProcesses(ctx context.Context, root string) ([]Target, error)

	// ToLaunchConfig bridges a discovered Target back into the existing
	// LaunchConfig type so the session manager can launch it without
	// knowing about discovery. The caller has already merged env files.
	ToLaunchConfig(t Target) config.LaunchConfig
}

// Registry holds the active set of providers. It's intentionally tiny —
// providers register themselves at process startup via main.go (no init()
// magic, so the import graph stays explicit).
type Registry struct {
	mu        sync.RWMutex
	providers []Provider
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, len(r.providers))
	copy(out, r.providers)
	return out
}

// Find returns the provider with the given name, or nil if absent.
func (r *Registry) Find(name string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}
