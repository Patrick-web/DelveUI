// Package search powers the project-wide find-in-files feature.
//
// The service walks the workspace concurrently, applies .gitignore plus
// user includes/excludes (doublestar globs), and streams matches back to
// the frontend via Wails events. Each request gets a fresh ID so stale
// results from a cancelled run never leak into the UI.
package search

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/uuid"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Workspace is the small slice of WorkspaceService the search package needs.
// Defined here as an interface to keep the dependency one-way.
type Workspace interface {
	Root() string
}

type Request struct {
	Query         string   `json:"query"`
	Root          string   `json:"root"` // optional override; defaults to workspace root
	Regex         bool     `json:"regex"`
	CaseSensitive bool     `json:"caseSensitive"`
	WholeWord     bool     `json:"wholeWord"`
	Includes      []string `json:"includes"`
	Excludes      []string `json:"excludes"`
	MaxResults    int      `json:"maxResults"` // 0 means default cap
}

type Range [2]int // [start, end) byte offsets within Text

type Match struct {
	Path   string  `json:"path"`
	Rel    string  `json:"rel"`
	Line   int     `json:"line"`
	Col    int     `json:"col"`
	Text   string  `json:"text"`
	Ranges []Range `json:"ranges"`
}

type DoneEvent struct {
	ID         string `json:"id"`
	Files      int    `json:"files"`
	Matches    int    `json:"matches"`
	DurationMs int64  `json:"durationMs"`
	Truncated  bool   `json:"truncated"`
	Cancelled  bool   `json:"cancelled"`
}

type batchEvent struct {
	ID      string  `json:"id"`
	Matches []Match `json:"matches"`
}

type errorEvent struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

const (
	defaultMaxResults    = 2000
	maxFileSizeBytes     = 4 * 1024 * 1024 // 4MB; bigger files almost always assets
	maxLineDisplayLen    = 400             // truncate Match.Text past this
	matchContextChars    = 200             // chars around a long-line match
	binarySniffBytes     = 8 * 1024
	batchFlushSize       = 50
	batchFlushIntervalMs = 50
)

// Default directory ignores layered on top of .gitignore. These mirror the
// frontend file-tree's hiddenDirs list so search behaviour matches what a
// user would expect from QuickOpen.
var alwaysSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
	".cache": true, ".idea": true, ".vscode": true, ".zed": true, ".delveui": true,
	"dist": true, "build": true,
}

// Service holds in-flight request state. Only one search runs at a time —
// starting a new one cancels the previous; this matches the UX where the
// user types and we re-run on debounce.
type Service struct {
	ws  Workspace
	app *application.App

	mu        sync.Mutex
	currentID string
	cancel    context.CancelFunc
}

func New(ws Workspace) *Service { return &Service{ws: ws} }

func (s *Service) SetApp(app *application.App) { s.app = app }

// Search starts an asynchronous search. The returned id is used for cancel
// and to tag streaming events so stale results can be ignored on the client.
func (s *Service) Search(req Request) (string, error) {
	if strings.TrimSpace(req.Query) == "" {
		return "", errors.New("empty query")
	}
	root := req.Root
	if root == "" && s.ws != nil {
		root = s.ws.Root()
	}
	if root == "" {
		return "", errors.New("no workspace root")
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("invalid root: %s", root)
	}
	if req.MaxResults <= 0 {
		req.MaxResults = defaultMaxResults
	}

	matcher, err := buildMatcher(req)
	if err != nil {
		return "", err
	}

	id := uuid.NewString()

	// Cancel any prior run so events from it are ignored on the client side
	// (the client filters by id, but we also stop the goroutine to free CPU).
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.currentID = id
	s.cancel = cancel
	s.mu.Unlock()

	go s.run(ctx, id, root, req, matcher)
	return id, nil
}

// Cancel stops the current search if its id matches. Stale cancels are no-ops
// so frontend race conditions don't accidentally kill a fresh search.
func (s *Service) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentID == id && s.cancel != nil {
		s.cancel()
	}
	return nil
}

// matcher abstracts literal vs. regex matching so the worker loop is simple.
type matcher interface {
	findAll(line []byte) []Range
}

type literalMatcher struct {
	needle    []byte
	fold      bool // case-insensitive
	wholeWord bool
}

func (m *literalMatcher) findAll(line []byte) []Range {
	var hay []byte
	var needle []byte
	if m.fold {
		// Lower-case both to compare. We allocate one buffer per line which
		// is fine — file I/O dominates this loop.
		hay = bytes.ToLower(line)
		needle = bytes.ToLower(m.needle)
	} else {
		hay = line
		needle = m.needle
	}
	all := findAllBytes(hay, needle)
	if !m.wholeWord || len(all) == 0 {
		return all
	}
	out := all[:0]
	for _, r := range all {
		if isWordBoundary(line, r[0]) && isWordBoundary(line, r[1]) {
			out = append(out, r)
		}
	}
	return out
}

// isWordBoundary reports whether position pos in line sits at the edge of a
// word — i.e. one side is a word char and the other isn't (or is out of
// bounds). Mirrors regexp's \b semantics for ASCII identifier chars.
func isWordBoundary(line []byte, pos int) bool {
	left := pos > 0 && isWordByte(line[pos-1])
	right := pos < len(line) && isWordByte(line[pos])
	return left != right
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func findAllBytes(hay, needle []byte) []Range {
	if len(needle) == 0 {
		return nil
	}
	var out []Range
	off := 0
	for {
		i := bytes.Index(hay[off:], needle)
		if i < 0 {
			return out
		}
		start := off + i
		end := start + len(needle)
		out = append(out, Range{start, end})
		off = end
	}
}

type regexMatcher struct {
	re *regexp.Regexp
}

func (m *regexMatcher) findAll(line []byte) []Range {
	idx := m.re.FindAllIndex(line, -1)
	if len(idx) == 0 {
		return nil
	}
	out := make([]Range, len(idx))
	for i, p := range idx {
		out[i] = Range{p[0], p[1]}
	}
	return out
}

func buildMatcher(req Request) (matcher, error) {
	if req.Regex {
		flags := ""
		if !req.CaseSensitive {
			flags = "(?i)"
		}
		pattern := req.Query
		if req.WholeWord {
			pattern = `\b(?:` + pattern + `)\b`
		}
		re, err := regexp.Compile(flags + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		return &regexMatcher{re: re}, nil
	}
	return &literalMatcher{
		needle:    []byte(req.Query),
		fold:      !req.CaseSensitive,
		wholeWord: req.WholeWord,
	}, nil
}

// run performs the walk + grep. All event emission happens here so the
// caller-side API stays synchronous-looking.
func (s *Service) run(ctx context.Context, id, root string, req Request, m matcher) {
	start := time.Now()

	ig := loadGitignore(root)
	includes, excludes := normalizeGlobs(req.Includes), normalizeGlobs(req.Excludes)

	paths := make(chan string, 256)
	results := make(chan Match, 256)
	var walkWG, workersWG sync.WaitGroup

	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	for i := 0; i < workers; i++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			for p := range paths {
				if ctx.Err() != nil {
					return
				}
				rel, _ := filepath.Rel(root, p)
				rel = filepath.ToSlash(rel)
				scanFile(ctx, p, rel, m, results)
			}
		}()
	}

	walkWG.Add(1)
	go func() {
		defer walkWG.Done()
		defer close(paths)
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if ctx.Err() != nil {
				return filepath.SkipAll
			}
			if err != nil {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			if d.IsDir() {
				name := d.Name()
				if name != "." && (alwaysSkipDirs[name] || strings.HasPrefix(name, ".") && relSlash != ".") {
					return filepath.SkipDir
				}
				if ig != nil && relSlash != "." && ig.MatchesPath(relSlash+"/") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			if ig != nil && ig.MatchesPath(relSlash) {
				return nil
			}
			if !globAllow(relSlash, includes, excludes) {
				return nil
			}
			select {
			case <-ctx.Done():
				return filepath.SkipAll
			case paths <- path:
			}
			return nil
		})
	}()

	go func() {
		walkWG.Wait()
		workersWG.Wait()
		close(results)
	}()

	// Batched emission: flush every batchFlushSize matches OR every
	// batchFlushIntervalMs, whichever comes first. Keeps the UI responsive
	// without firing one event per match on huge searches.
	batch := make([]Match, 0, batchFlushSize)
	files := map[string]struct{}{}
	totalMatches := 0
	truncated := false
	cancelled := false
	tick := time.NewTicker(batchFlushIntervalMs * time.Millisecond)
	defer tick.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		// Copy the slice so subsequent appends to the reusable backing array
		// don't race with Wails' async JSON encoding of the previous batch.
		out := make([]Match, len(batch))
		copy(out, batch)
		s.emit("search:match", batchEvent{ID: id, Matches: out})
		batch = batch[:0]
	}

drain:
	for {
		select {
		case <-ctx.Done():
			cancelled = true
			break drain
		case mm, ok := <-results:
			if !ok {
				break drain
			}
			files[mm.Path] = struct{}{}
			totalMatches++
			batch = append(batch, mm)
			if len(batch) >= batchFlushSize {
				flush()
			}
			if totalMatches >= req.MaxResults {
				truncated = true
				// Stop early — cancel context so the walker/workers wind down.
				s.mu.Lock()
				if s.currentID == id && s.cancel != nil {
					s.cancel()
				}
				s.mu.Unlock()
				// keep draining a moment so workers don't block on send
				go func() { for range results { } }()
				break drain
			}
		case <-tick.C:
			flush()
		}
	}
	flush()

	s.emit("search:done", DoneEvent{
		ID:         id,
		Files:      len(files),
		Matches:    totalMatches,
		DurationMs: time.Since(start).Milliseconds(),
		Truncated:  truncated,
		Cancelled:  cancelled && !truncated,
	})

	// Clear current id if we're still the active search.
	s.mu.Lock()
	if s.currentID == id {
		s.currentID = ""
		s.cancel = nil
	}
	s.mu.Unlock()
}

func (s *Service) emit(name string, data any) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit(name, data)
}

// scanFile reads a file line-by-line and pushes matches onto results.
// Skips files that look binary or are over the size cap.
func scanFile(ctx context.Context, path, rel string, m matcher, results chan<- Match) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > maxFileSizeBytes {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// Binary sniff: a NUL in the first 8KB is a strong signal.
	head := make([]byte, binarySniffBytes)
	n, _ := f.Read(head)
	if bytes.IndexByte(head[:n], 0) >= 0 {
		return
	}
	// Reset to start so we scan from line 1.
	if _, err := f.Seek(0, 0); err != nil {
		return
	}

	sc := bufio.NewScanner(f)
	// Allow long lines; some generated/JSON files have multi-MB lines but we
	// already bailed on >4MB files above, so 1MB per line is plenty.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		if ctx.Err() != nil {
			return
		}
		lineNo++
		line := sc.Bytes()
		ranges := m.findAll(line)
		if len(ranges) == 0 {
			continue
		}
		text, adjusted := truncateLine(line, ranges)
		mm := Match{
			Path:   path,
			Rel:    rel,
			Line:   lineNo,
			Col:    ranges[0][0] + 1,
			Text:   text,
			Ranges: adjusted,
		}
		select {
		case <-ctx.Done():
			return
		case results <- mm:
		}
	}
}

// truncateLine returns a display string and adjusted ranges. For very long
// lines we centre on the first match so the user can still see context.
func truncateLine(line []byte, ranges []Range) (string, []Range) {
	if len(line) <= maxLineDisplayLen {
		return string(line), ranges
	}
	first := ranges[0]
	startCtx := first[0] - matchContextChars
	if startCtx < 0 {
		startCtx = 0
	}
	endCtx := first[1] + matchContextChars
	if endCtx > len(line) {
		endCtx = len(line)
	}
	prefix := ""
	suffix := ""
	if startCtx > 0 {
		prefix = "…"
	}
	if endCtx < len(line) {
		suffix = "…"
	}
	display := prefix + string(line[startCtx:endCtx]) + suffix
	shift := len(prefix) - startCtx
	out := make([]Range, 0, len(ranges))
	for _, r := range ranges {
		if r[0] < startCtx || r[1] > endCtx {
			continue
		}
		out = append(out, Range{r[0] + shift, r[1] + shift})
	}
	return display, out
}

func loadGitignore(root string) *gitignore.GitIgnore {
	p := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	ig, err := gitignore.CompileIgnoreFile(p)
	if err != nil {
		return nil
	}
	return ig
}

func normalizeGlobs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, filepath.ToSlash(p))
	}
	return out
}

// globAllow returns true if the file (path relative to root, slash-separated)
// passes the include + exclude filters. Empty includes means "everything";
// empty excludes means "exclude nothing".
func globAllow(rel string, includes, excludes []string) bool {
	if len(includes) > 0 {
		ok := false
		for _, g := range includes {
			if matched, _ := doublestar.Match(g, rel); matched {
				ok = true
				break
			}
			// Allow "*.go" to match files at any depth, mirroring VS Code UX.
			if !strings.Contains(g, "/") {
				if matched, _ := doublestar.Match("**/"+g, rel); matched {
					ok = true
					break
				}
			}
		}
		if !ok {
			return false
		}
	}
	for _, g := range excludes {
		if matched, _ := doublestar.Match(g, rel); matched {
			return false
		}
		if !strings.Contains(g, "/") {
			if matched, _ := doublestar.Match("**/"+g, rel); matched {
				return false
			}
		}
	}
	return true
}
