// Package adapter maps language names to DAP server process specs so the
// session layer can spawn and initialize debug adapters without knowing
// anything about specific tools (dlv, debugpy, node-debug, …).
//
// Adding a new language is one Register() call in main.go.
package adapter

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ProcessSpec describes how to spawn a DAP server for a given language.
// It is a pure value type — no behavior — so the session layer reads it
// and acts accordingly.
type ProcessSpec struct {
	// Language is the ecosystem tag matched against config.LaunchConfig.Language.
	// Examples: "go", "python", "node".
	Language string

	// AdapterID is the value sent as DAP InitializeArguments.AdapterID.
	AdapterID string

	// DAPType is the value sent as the "type" field in the DAP launch/attach
	// request. Corresponds to the debug type in VS Code / Zed config files.
	DAPType string

	// Binary is the absolute path to the DAP server executable.
	Binary string

	// BinaryArgs are CLI arguments placed before the listen address when
	// spawning the DAP server. For example, Delve expects ["dap"].
	BinaryArgs []string

	// PortFlag controls how the listen address is passed to the adapter.
	// "$PORT" and "$HOST" are replaced with the actual values. Spaces split
	// the result into separate argv entries. Default (empty): "--listen $HOST:$PORT".
	// Examples:
	//   ""                        → --listen 127.0.0.1:5678
	//   "--port $PORT"            → --port 5678
	//   "$PORT $HOST"             → 5678 127.0.0.1  (positional, js-debug style)
	PortFlag string

	// TargetViaCLI indicates that the debug target (program + args) should
	// be passed as positional CLI arguments after the listen address,
	// rather than through the DAP Launch request. Required by debugpy.
	// When set, the session layer appends Program and Args to the adapter's
	// command line and omits them from the DAP launch arguments.
	TargetViaCLI bool

	// ExtraPath entries are appended to the subprocess PATH environment
	// variable so common toolchain binaries (compilers, linters, runtimes)
	// are reachable even in minimal environments like macOS .app bundles.
	ExtraPath []string

	// Label is the human-readable display name shown in settings and icons.
	// Examples: "Go (Delve)", "Python (debugpy)".
	Label string

	// Description is a one-line summary shown in the settings UI.
	Description string

	// BinaryName is the bare executable name used to auto-discover the
	// adapter via exec.LookPath + ExtraPath. Example: "dlv".
	BinaryName string

	// InstallCmd is the shell command that installs the adapter.
	// Example: "go install github.com/go-delve/delve/cmd/dlv@latest".
	InstallCmd string

	// InstallURL links to documentation or the adapter homepage.
	InstallURL string
}

// Registry holds the fixed set of registered ProcessSpec values and a
// mutex for safe concurrent access.
type Registry struct {
	mu     sync.RWMutex
	byLang map[string]ProcessSpec
}

// NewRegistry returns an empty registry ready for Register calls.
func NewRegistry() *Registry {
	return &Registry{byLang: make(map[string]ProcessSpec)}
}

// Register adds a ProcessSpec to the registry. Panics if Language is empty
// so startup wiring errors are caught immediately.
func (r *Registry) Register(spec ProcessSpec) {
	if spec.Language == "" {
		panic("adapter: Register called with empty Language")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byLang[spec.Language] = spec
}

// Resolve returns the ProcessSpec for the given language. If language is
// empty, it defaults to "go" for backwards compatibility with launch
// configs that predate the multi-language adapter layer.
func (r *Registry) Resolve(language string) (ProcessSpec, error) {
	if language == "" {
		language = "go"
	}
	r.mu.RLock()
	spec, ok := r.byLang[language]
	r.mu.RUnlock()
	if !ok {
		return ProcessSpec{}, fmt.Errorf("no debug adapter registered for language %q", language)
	}
	if !r.Installed(language) {
		return ProcessSpec{}, fmt.Errorf("debug adapter for %q is not installed", language)
	}
	return spec, nil
}

// Get returns the ProcessSpec for the given language regardless of whether
// its Binary is set. Used by the UI to list all adapters.
func (r *Registry) Get(language string) (ProcessSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byLang[language]
	return s, ok
}

// SetBinary overrides the binary path for an existing language's ProcessSpec.
// Used at runtime when the user configures a custom adapter path via settings.
// Returns an error if the language is not registered.
func (r *Registry) SetBinary(language, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	spec, ok := r.byLang[language]
	if !ok {
		return fmt.Errorf("no adapter registered for language %q", language)
	}
	spec.Binary = path
	r.byLang[language] = spec
	return nil
}

// Rediscover runs FindBinary for the given language and updates its Binary
// field. If the binary is not found, Binary is left as-is (possibly empty).
func (r *Registry) Rediscover(language string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	spec, ok := r.byLang[language]
	if !ok {
		return
	}
	spec.Binary = FindBinary(spec.BinaryName, spec.ExtraPath)
	r.byLang[language] = spec
}

// Installed returns true if the adapter's Binary is set, points to a
// file that exists on disk, AND any file-path arguments in BinaryArgs
// also exist (e.g. the DAP server JS file for js-debug).
func (r *Registry) Installed(language string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.byLang[language]
	if !ok || spec.Binary == "" {
		return false
	}
	if _, err := os.Stat(spec.Binary); err != nil {
		return false
	}
	// Check that file-path arguments exist (e.g. js-debug's dapDebugServer.js).
	for _, arg := range spec.BinaryArgs {
		if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "~") {
			path := arg
			if strings.HasPrefix(path, "~/") {
				if home, _ := os.UserHomeDir(); home != "" {
					path = home + path[1:]
				}
			}
			if _, err := os.Stat(path); err != nil {
				return false
			}
		}
	}
	return true
}

// All returns a copy of every registered ProcessSpec (for UI listing).
func (r *Registry) All() []ProcessSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProcessSpec, 0, len(r.byLang))
	for _, s := range r.byLang {
		out = append(out, s)
	}
	return out
}

// FindBinary searches for binaryName in PATH first (via exec.LookPath),
// then in each extraPath/binaryName combination. Returns absolute path
// or empty string if not found.
func FindBinary(binaryName string, extraPaths []string) string {
	if p, err := exec.LookPath(binaryName); err == nil {
		return p
	}
	for _, dir := range extraPaths {
		p := dir + "/" + binaryName
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
