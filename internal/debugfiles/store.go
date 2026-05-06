package debugfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jp/DelveUI/internal/config"
)

// Entry represents one project. The unit is always a folder; if the project's
// launch configs live somewhere outside the standard `.zed`/`.vscode`/
// `.delveui` locations (e.g. an externally-shared launch.json), `LaunchFile`
// records that explicit override.
//
// `Stale` is computed at load — it is true when `Path` no longer exists on
// disk. The frontend renders stale entries dimmed with a remove affordance.
type Entry struct {
	ID         string                `json:"id"`
	Path       string                `json:"path"`
	Label      string                `json:"label"`
	LaunchFile string                `json:"launchFile,omitempty"`
	AddedAt    time.Time             `json:"addedAt"`
	LastUsed   time.Time             `json:"lastUsed,omitempty"`
	Configs    []config.LaunchConfig `json:"configs"`
	Stale      bool                  `json:"-"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	Entries []Entry `json:"entries"`
	// Active records the path of the most-recently-loaded project so the app
	// can restore it on next launch (when the `restoreLastProject` setting is
	// on). It mirrors what `workspace.Store.Active` used to track — kept here
	// so there's a single source of truth for project state.
	Active string `json:"active,omitempty"`
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
	s.refreshStaleness()
	return s, nil
}

// loadedEntry mirrors the on-disk shape, including legacy fields we want to
// migrate (Kind, IsDefault). Entries are normalized into the current Entry
// schema in `migrateLoaded` so the in-memory store never carries the legacy
// shapes.
type loadedEntry struct {
	ID         string                `json:"id"`
	Path       string                `json:"path"`
	Label      string                `json:"label"`
	LaunchFile string                `json:"launchFile,omitempty"`
	Kind       string                `json:"kind,omitempty"`      // legacy: "file"|"folder"
	IsDefault  bool                  `json:"isDefault,omitempty"` // legacy: replaced by restoreLastProject setting
	AddedAt    time.Time             `json:"addedAt"`
	LastUsed   time.Time             `json:"lastUsed,omitempty"`
	Configs    []config.LaunchConfig `json:"configs"`
}

type loadedFile struct {
	Entries []loadedEntry `json:"entries"`
	Active  string        `json:"active,omitempty"`
}

func (s *Store) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var raw loadedFile
	if err := json.Unmarshal(b, &raw); err != nil {
		return
	}
	s.Active = raw.Active
	s.Entries = nil
	for _, le := range raw.Entries {
		s.Entries = append(s.Entries, migrateLoaded(le))
	}
	s.dedupeByPath()
}

// migrateLoaded normalizes a legacy file-kind entry into a folder-kind entry
// with optional LaunchFile. Idempotent — already-folder entries pass through
// unchanged. Run on every load so older debug-files.json files transparently
// move to the new shape.
func migrateLoaded(le loadedEntry) Entry {
	e := Entry{
		ID:         le.ID,
		Path:       le.Path,
		Label:      le.Label,
		LaunchFile: le.LaunchFile,
		AddedAt:    le.AddedAt,
		LastUsed:   le.LastUsed,
		Configs:    le.Configs,
	}
	if le.Kind == "folder" {
		return e
	}
	// Legacy file-kind (or empty Kind from very old entries). Path points at
	// a launch.json file; convert to its containing project folder.
	parent := filepath.Dir(le.Path)
	parentBase := filepath.Base(parent)
	switch parentBase {
	case ".zed", ".vscode", ".delveui":
		// Standard layout — folder root is one level up from .zed/.vscode/.
		e.Path = filepath.Dir(parent)
	default:
		// Non-standard location: keep the file as an explicit override so we
		// still find the configs the user originally registered.
		e.Path = parent
		e.LaunchFile = le.Path
	}
	if e.Label == "" || e.Label == filepath.Base(le.Path) {
		e.Label = filepath.Base(e.Path)
		if e.Label == "" || e.Label == "." || e.Label == "/" {
			e.Label = filepath.Base(le.Path)
		}
	}
	return e
}

// dedupeByPath collapses duplicate folder entries that the migration may have
// produced (e.g. user had both `~/proj/.zed/debug.json` and `~/proj` registered
// separately). Keep the entry with the most recent LastUsed/AddedAt.
func (s *Store) dedupeByPath() {
	if len(s.Entries) < 2 {
		return
	}
	byPath := map[string]int{}
	out := make([]Entry, 0, len(s.Entries))
	for _, e := range s.Entries {
		if idx, ok := byPath[e.Path]; ok {
			// Pick the newer of the two.
			existing := out[idx]
			if betterStamp(e).After(betterStamp(existing)) {
				// Merge the more-recent entry, but keep an existing LaunchFile
				// override if the newer one has none.
				if e.LaunchFile == "" {
					e.LaunchFile = existing.LaunchFile
				}
				out[idx] = e
			}
			continue
		}
		byPath[e.Path] = len(out)
		out = append(out, e)
	}
	s.Entries = out
}

func betterStamp(e Entry) time.Time {
	if !e.LastUsed.IsZero() {
		return e.LastUsed
	}
	return e.AddedAt
}

// refreshStaleness stats every entry path so the UI can render missing
// projects. Cheap on app startup; entries are typically few.
func (s *Store) refreshStaleness() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Entries {
		_, err := os.Stat(s.Entries[i].Path)
		s.Entries[i].Stale = err != nil
	}
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

// Clear removes every registered project entry.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = nil
	s.Active = ""
	return s.save()
}

func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.Entries))
	copy(out, s.Entries)
	return out
}

// ActivePath returns the most-recently-loaded project path, used by the app
// to restore state on launch.
func (s *Store) ActivePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Active
}

// MarkActive sets Active and updates LastUsed for the matching entry. Path is
// matched by exact equality with Entry.Path; callers should pass the folder
// path, not a launch.json file path.
func (s *Store) MarkActive(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = path
	now := time.Now()
	for i := range s.Entries {
		if s.Entries[i].Path == path {
			s.Entries[i].LastUsed = now
			break
		}
	}
	return s.save()
}

// MostRecent returns the entry to load when restoring the last session. Picks
// the entry with the largest LastUsed (falling back to AddedAt). Returns nil
// when there are no entries.
func (s *Store) MostRecent() *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Entries) == 0 {
		return nil
	}
	// Match Active explicitly first if it points at a known entry — Active is
	// updated synchronously on every project switch, while LastUsed is best-
	// effort.
	if s.Active != "" {
		for i := range s.Entries {
			if s.Entries[i].Path == s.Active {
				e := s.Entries[i]
				return &e
			}
		}
	}
	idx := 0
	for i := 1; i < len(s.Entries); i++ {
		if betterStamp(s.Entries[i]).After(betterStamp(s.Entries[idx])) {
			idx = i
		}
	}
	e := s.Entries[idx]
	return &e
}

// Add registers a path as a project. Accepts either a folder or a launch.json
// file path — both normalize to a folder-kind entry. Idempotent: repeated
// calls for the same resolved folder bump LastUsed instead of inserting.
func (s *Store) Add(path string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	folderPath, launchFile, label, err := normalizePath(path)
	if err != nil {
		return Entry{}, err
	}

	// Idempotent on folder path. If the caller passed a launch.json file we
	// promote LaunchFile onto an existing entry so the explicit reference is
	// preserved.
	for i := range s.Entries {
		if s.Entries[i].Path == folderPath {
			if launchFile != "" && s.Entries[i].LaunchFile == "" {
				s.Entries[i].LaunchFile = launchFile
			}
			s.Entries[i].LastUsed = time.Now()
			if err := s.save(); err != nil {
				return Entry{}, err
			}
			return s.Entries[i], nil
		}
	}

	cfgs, _ := loadConfigsFor(folderPath, launchFile)

	now := time.Now()
	entry := Entry{
		ID:         uuid.NewString(),
		Path:       folderPath,
		Label:      label,
		LaunchFile: launchFile,
		AddedAt:    now,
		LastUsed:   now,
		Configs:    cfgs,
	}
	s.Entries = append(s.Entries, entry)
	return entry, s.save()
}

// normalizePath turns a user-supplied path (file or folder) into the
// canonical folder + optional LaunchFile tuple. Returns an error if the path
// doesn't exist; callers can decide whether to surface that.
func normalizePath(path string) (folder, launchFile, label string, err error) {
	info, statErr := os.Stat(path)
	if statErr != nil {
		return "", "", "", statErr
	}
	if info.IsDir() {
		return path, "", filepath.Base(path), nil
	}
	parent := filepath.Dir(path)
	parentBase := filepath.Base(parent)
	switch parentBase {
	case ".zed", ".vscode", ".delveui":
		folder = filepath.Dir(parent)
		label = filepath.Base(folder)
		return folder, "", label, nil
	default:
		folder = parent
		label = filepath.Base(folder)
		if label == "" || label == "." || label == "/" {
			label = filepath.Base(path)
		}
		return folder, path, label, nil
	}
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.Entries {
		if e.ID == id {
			if e.Path == s.Active {
				s.Active = ""
			}
			s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("not found")
}

// RemoveStale drops every entry whose Path no longer exists on disk. Returns
// the number of entries removed; 0 when everything is healthy.
func (s *Store) RemoveStale() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	out := s.Entries[:0]
	for _, e := range s.Entries {
		if _, err := os.Stat(e.Path); err != nil {
			if e.Path == s.Active {
				s.Active = ""
			}
			removed++
			continue
		}
		out = append(out, e)
	}
	s.Entries = out
	if err := s.save(); err != nil {
		return removed, err
	}
	return removed, nil
}

// Recent returns up to `limit` entries ordered by recency. Used by the
// frontend to render an MRU list on the welcome page.
func (s *Store) Recent(limit int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.Entries))
	copy(out, s.Entries)
	sort.SliceStable(out, func(i, j int) bool {
		return betterStamp(out[i]).After(betterStamp(out[j]))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) Reload(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Entries {
		if s.Entries[i].ID == id {
			cfgs, err := loadConfigsFor(s.Entries[i].Path, s.Entries[i].LaunchFile)
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
		cfgs, err := loadConfigsFor(s.Entries[i].Path, s.Entries[i].LaunchFile)
		if err != nil {
			continue
		}
		s.Entries[i].Configs = cfgs
	}
	return s.save()
}

// loadConfigsFor resolves launch configs for a folder-kind entry. When
// LaunchFile is set, it takes priority — that's how legacy "imported a
// non-standard launch.json" entries continue to work post-migration.
func loadConfigsFor(folder, launchFile string) ([]config.LaunchConfig, error) {
	if launchFile != "" {
		return config.LoadFile(launchFile)
	}
	_, cfgs, err := config.LoadFromWorkspace(folder)
	if err != nil {
		// A folder without any debug config is still a valid project — the
		// discovery layer populates Run targets from Go source on demand.
		return nil, nil
	}
	return cfgs, nil
}
