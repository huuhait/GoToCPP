package cache

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/lsp/memoize"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// The memoized pipeline (Phase 11 README §2.2, step 11.3). A Pipeline holds
// the caches that outlive snapshots, keyed by content hashes: parsed files,
// the imports of each package (metadata), checked packages and their
// summaries. Each snapshot has a state (the State of the session), which
// knows the files of the snapshot and a handle per package; Clone keeps
// what an edit doesn't touch.
//
// A package is checked alone: it is loaded with the summaries of the
// packages it imports (load.Config.Deps) and checked with their types from
// export data (check.Config.Deps), so an edit checks only the edited
// package. Its importers are checked again only when its API changed
// (the API hash, see apiHash): the key of a check is the hash of the
// package's own files, of what the resolution of its imports read (which
// folders exist, which gocpp.mod files: fs.go), and of the API hashes of
// its imports. Every load reads the files and folders of its snapshot
// (load.Config.FS), never the disk behind it.

// A Config says what a Pipeline loads and checks: the settings of load.Load
// and check.Check that gocpp check would use.
type Config struct {
	// Stdlib, StdlibDir and GoStd are those of load.Config. Stdlib must
	// not change while the pipeline lives (the embedded standard library
	// never does): its files are read once.
	Stdlib    fs.FS
	StdlibDir string
	GoStd     func(path string) bool

	// GOOS, GOARCH and Tags are the target (load.Config).
	GOOS, GOARCH string
	Tags         []string

	// Phase is check.Config.Phase.
	Phase int

	// CXX is check.Config.CXX: the C++ compiler of the namespace probe.
	// The pipeline asks for it once (nil: build.FindCXXFor, which runs the
	// compiler to see what it is).
	//
	// TODO(11.4, Phase 12): the answer is kept for the session, also an
	// error: a compiler that is installed later is not found until the
	// server starts again. Ask again after an error, now and then.
	CXX func(ctx context.Context) (build.CXX, error)

	// The memory caps of the caches; 0 means the default. MaxCheckBytes
	// caps the checked packages (with their ASTs, types.Info and the
	// types of their imports) by an estimate of their memory
	// (Package.cost), MaxSummaryBytes the summaries by the size of their
	// export data, and MaxParseBytes the parsed files by an estimate of
	// their memory (astFactor). The checked packages of the open documents stay while a
	// snapshot holds them (handle.last), whatever the cap.
	MaxCheckBytes, MaxSummaryBytes, MaxParseBytes int64
}

// The default caps: the memory budget is 400 MB for a 50k-line project
// (README §2.1).
const (
	defaultMaxCheckBytes   = 100 << 20
	defaultMaxSummaryBytes = 64 << 20
	defaultMaxParseBytes   = 32 << 20
	maxMetadata            = 8192 // a metadata is a few hundred bytes
)

// A Pipeline is the memoized pipeline of a session: the caches that its
// snapshots share. Its methods are safe for concurrent use.
type Pipeline struct {
	cfg    Config
	cfgKey Hash // the parts of cfg that results depend on

	parses    *memoize.Store[parseKey, *ParsedFile]
	checks    *memoize.Store[Hash, *Package]
	summaries *memoize.Store[Hash, *summary]

	// cxx finds the C++ compiler once.
	cxx *memoize.Promise[build.CXX]

	// sem holds a place for each type-check that runs, at most GOMAXPROCS.
	sem chan struct{}

	// metas keeps the metadata of each hash of headers (metadataFor).
	metaMu sync.Mutex
	metas  map[Hash]*metadata

	stdMu  sync.Mutex
	stdPkg map[string][]*fileHandle // the files of each standard package, by import path

	stats struct {
		parses, metadata, checks atomic.Int64
	}
}

// NewPipeline returns a pipeline with empty caches.
func NewPipeline(cfg Config) *Pipeline {
	if cfg.StdlibDir == "" {
		cfg.StdlibDir = "$GOCPP/stdlib"
	}
	if cfg.GOOS == "" {
		cfg.GOOS = runtime.GOOS
	}
	if cfg.GOARCH == "" {
		cfg.GOARCH = runtime.GOARCH
	}
	h := newHasher("config")
	h.str(cfg.StdlibDir)
	h.str(cfg.GOOS)
	h.str(cfg.GOARCH)
	h.int(int64(len(cfg.Tags)))
	for _, t := range cfg.Tags {
		h.str(t)
	}
	h.int(int64(cfg.Phase))
	p := &Pipeline{
		cfg:    cfg,
		cfgKey: h.sum(),
		parses: memoize.NewStore[parseKey](orDefault(cfg.MaxParseBytes, defaultMaxParseBytes), func(pf *ParsedFile, _ error) int64 {
			if pf == nil {
				return 1
			}
			if pf.Mode&parser.ImportsOnly != 0 {
				return int64(len(pf.Src)) + 64 // only the header and its imports
			}
			return astFactor*int64(len(pf.Src)) + 1
		}),
		metas: make(map[Hash]*metadata),
		checks: memoize.NewStore[Hash](orDefault(cfg.MaxCheckBytes, defaultMaxCheckBytes), func(pkg *Package, _ error) int64 {
			if pkg == nil {
				return 1
			}
			return pkg.cost + 1
		}),
		summaries: memoize.NewStore[Hash](orDefault(cfg.MaxSummaryBytes, defaultMaxSummaryBytes), func(s *summary, _ error) int64 {
			if s == nil {
				return 1
			}
			return int64(len(s.export)) + 256
		}),
		sem:    make(chan struct{}, runtime.GOMAXPROCS(0)),
		stdPkg: make(map[string][]*fileHandle),
	}
	find := cfg.CXX
	if find == nil {
		find = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, cfg.GOOS, cfg.GOARCH) }
	}
	p.cxx = memoize.NewPromise(find)
	return p
}

// astFactor is how much memory the AST of a file parsed with comments
// takes per byte of its source (about 10 to 20).
const astFactor = 15

func orDefault(n, def int64) int64 {
	if n == 0 {
		return def
	}
	return n
}

// findCXX is check.Config.CXX: the compiler, found once.
func (p *Pipeline) findCXX(ctx context.Context) (build.CXX, error) { return p.cxx.Get(ctx) }

// NewState returns the state of the first snapshot of a session
// (NewSession): it has no open documents.
func (p *Pipeline) NewState() State {
	return &state{p: p, files: map[string]*fileHandle{}, dirs: map[string]*dirHandle{},
		mods: map[string]*fileHandle{}, handles: map[pkgID]*handle{}}
}

// Stats counts the work the pipeline has done: files parsed, metadata
// loads, and package checks. The tests and benchmarks use it.
func (p *Pipeline) Stats() (parses, metadata, checks int64) {
	return p.stats.parses.Load(), p.stats.metadata.Load(), p.stats.checks.Load()
}

// A fileHandle is the content of a file in a snapshot: of its overlay (an
// open document), or else from disk (or from the standard library), read
// once. It never changes.
type fileHandle struct {
	path string // the absolute name; in Config.StdlibDir for the standard library
	src  []byte
	hash Hash
	err  error // reading failed (fs.ErrNotExist: there is no such file)
}

// A dirHandle is the .gp files in a folder: on disk and in the overlays.
type dirHandle struct {
	names []string // sorted
	isDir bool     // the folder exists: on disk (also when it can't be read) or in the overlays
	err   error    // the folder can't be read and has no overlays
}

// state is the State of a snapshot: the files it has read and the handles
// of the packages it has looked at. Clone copies what the changed
// documents don't touch. The maps are guarded by mu, which is never held
// while something is read, parsed or checked: two requests that need the
// same thing may both compute it, and the first one to store it wins
// (the values are the same).
type state struct {
	p        *Pipeline
	snap     *Snapshot           // nil for the first state
	overlays map[string]*Overlay // the open documents by file name

	mu       sync.Mutex
	released bool
	files    map[string]*fileHandle
	dirs     map[string]*dirHandle
	mods     map[string]*fileHandle // the gocpp.mod file of each folder, or nil
	handles  map[pkgID]*handle
}

// Clone implements State: the files, folders and packages that the changed
// documents don't touch stay (but the files and folders that were missing:
// they may be there now, see metadata.valid). A changed document changes its file, the
// listing of its folder, its package, every package whose imports were
// resolved with a read of the file or folder (an unsaved file in a new
// folder makes an import resolve), and every package that imports one of
// those, directly or not (they get new handles, whose keys see whether the
// API changed); a changed gocpp.mod changes every package.
func (s *state) Clone(next *Snapshot, changed map[protocol.DocumentURI]bool) State {
	n := &state{p: s.p, snap: next, overlays: make(map[string]*Overlay, len(next.overlays))}
	for uri, o := range next.overlays {
		if path := uri.Path(); path != "" {
			n.overlays[path] = o
		}
	}
	paths := make(map[string]bool, len(changed))
	dirs := make(map[string]bool, len(changed))
	modChanged := false
	for uri := range changed {
		if path := uri.Path(); path != "" {
			paths[path] = true
			dirs[filepath.Dir(path)] = true
			modChanged = modChanged || filepath.Base(path) == modFileName
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// What was missing is looked for again: an import may resolve now
	// (metadata.valid reads it).
	n.files = make(map[string]*fileHandle, len(s.files))
	for path, f := range s.files {
		if !paths[path] && !errors.Is(f.err, fs.ErrNotExist) {
			n.files[path] = f
		}
	}
	n.dirs = make(map[string]*dirHandle, len(s.dirs))
	for dir, d := range s.dirs {
		if !dirs[dir] && (len(d.names) > 0 || !errors.Is(d.err, fs.ErrNotExist)) {
			n.dirs[dir] = d
		}
	}
	n.mods = make(map[string]*fileHandle)
	n.handles = make(map[pkgID]*handle)
	if modChanged {
		return n // a module can change the import path of every package
	}
	for dir, m := range s.mods {
		n.mods[dir] = m
	}
	// The packages in the changed folders, and their importers.
	importers := make(map[pkgID][]pkgID)
	var stale []pkgID
	for id, h := range s.handles {
		for _, dep := range h.deps {
			importers[dep.id] = append(importers[dep.id], id)
		}
		for _, dep := range h.cut {
			importers[dep] = append(importers[dep], id)
		}
		if !id.std && dirs[id.dir] || h.meta.reads != nil && touches(h.meta.reads, paths, dirs) {
			stale = append(stale, id)
		}
	}
	gone := make(map[pkgID]bool)
	for len(stale) > 0 {
		id := stale[len(stale)-1]
		stale = stale[:len(stale)-1]
		if !gone[id] {
			gone[id] = true
			stale = append(stale, importers[id]...)
		}
	}
	for id, h := range s.handles {
		if !gone[id] {
			n.handles[id] = h
		}
	}
	return n
}

// touches reports whether one of the reads is of a changed file or folder.
func touches(reads []*read, paths, dirs map[string]bool) bool {
	for _, r := range reads {
		if paths[r.path] || dirs[r.path] {
			return true
		}
	}
	return false
}

// Release implements State: the state lets go of its handles, so the
// checked packages that only it holds can go. Its files stay readable for
// the cyclic checks in flight that read them (handle.fs); they go with the
// state.
func (s *state) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.released = true
	s.handles = nil
}

// errReleased is the error of a request on a snapshot that is destroyed,
// which is a bug of the caller.
var errReleased = errors.New("cache: the snapshot was destroyed")

// file returns the file with the absolute name path in the snapshot.
func (s *state) file(path string) (*fileHandle, error) {
	s.mu.Lock()
	f := s.files[path]
	s.mu.Unlock()
	if f != nil {
		return f, nil
	}
	f = &fileHandle{path: path}
	if o := s.overlays[path]; o != nil {
		f.src = []byte(o.Text())
	} else {
		f.src, f.err = os.ReadFile(path)
	}
	f.hash = hashOf(f.src)
	return store(s, filesOf, path, f)
}

// store puts v under key into the map of s that m returns, unless a value
// is there already, and returns the value that is there then.
func store[K comparable, V any](s *state, m func(*state) map[K]V, key K, v V) (V, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m(s) == nil {
		return v, nil // the handles of a released state
	}
	if old, ok := m(s)[key]; ok {
		return old, nil
	}
	m(s)[key] = v
	return v, nil
}

// The maps of a state, for store.
func filesOf(s *state) map[string]*fileHandle { return s.files }
func dirsOf(s *state) map[string]*dirHandle   { return s.dirs }
func modsOf(s *state) map[string]*fileHandle  { return s.mods }
func handlesOf(s *state) map[pkgID]*handle    { return s.handles }

// dir returns the .gp files of the folder dir in the snapshot: on disk and
// in the overlays.
func (s *state) dir(dir string) (*dirHandle, error) {
	s.mu.Lock()
	d := s.dirs[dir]
	s.mu.Unlock()
	if d != nil {
		return d, nil
	}
	d = &dirHandle{}
	entries, err := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gp") {
			d.names = append(d.names, e.Name())
		}
	}
	for path := range s.overlays {
		if filepath.Dir(path) == dir && strings.HasSuffix(path, ".gp") && !slices.Contains(d.names, filepath.Base(path)) {
			d.names = append(d.names, filepath.Base(path))
		}
	}
	slices.Sort(d.names)
	if err != nil && len(d.names) == 0 {
		d.err = err
	}
	d.isDir = err == nil || len(d.names) > 0
	if !d.isDir && !errors.Is(err, fs.ErrNotExist) {
		// A folder that can't be read (permissions) is still a folder.
		fi, serr := os.Stat(dir)
		d.isDir = serr == nil && fi.IsDir()
	}
	return store(s, dirsOf, dir, d)
}

// modFileName is the name of the file at the top of a module.
const modFileName = "gocpp.mod"

// modFile returns the gocpp.mod file of the folder dir, as the loader
// finds it (load.FindModule): in dir or the nearest folder above it, or nil.
func (s *state) modFile(dir string) (*fileHandle, error) {
	s.mu.Lock()
	m, ok := s.mods[dir]
	s.mu.Unlock()
	if ok {
		return m, nil
	}
	m = nil
	for d := dir; ; {
		f, err := s.file(filepath.Join(d, modFileName))
		if err != nil {
			return nil, err
		}
		if f.err == nil {
			m = f
			break
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return store(s, modsOf, dir, m)
}

// stdFiles returns the .gp files of the standard package path, read once
// for the pipeline, sorted by name.
func (p *Pipeline) stdFiles(importPath string) []*fileHandle {
	p.stdMu.Lock()
	defer p.stdMu.Unlock()
	if files, ok := p.stdPkg[importPath]; ok {
		return files
	}
	var files []*fileHandle
	if p.cfg.Stdlib != nil {
		entries, _ := fs.ReadDir(p.cfg.Stdlib, importPath)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".gp") {
				continue
			}
			f := &fileHandle{path: filepath.Join(p.cfg.StdlibDir, filepath.FromSlash(importPath), e.Name())}
			f.src, f.err = fs.ReadFile(p.cfg.Stdlib, path.Join(importPath, e.Name()))
			f.hash = hashOf(f.src)
			files = append(files, f)
		}
	}
	p.stdPkg[importPath] = files
	return files
}
