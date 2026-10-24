package cache

import (
	"bytes"
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
	"syscall"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache/filecache"
	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/memoize"
	"github.com/huuhait/gocpp/internal/lsp/trace"
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

	// CXX finds the C++ compiler of the namespace probe (nil:
	// build.FindCXXFor, which runs the compiler to see what it is), and
	// Probe is the probe (nil: interop.ProbeNamespaces). The pipeline calls
	// them in the background only, never while a check waits (probe.go):
	// the compiler once, and again cxxRetry after an error, so that a
	// compiler installed later is found.
	CXX   func(ctx context.Context) (build.CXX, error)
	Probe func(ctx context.Context, cxx build.CXX, dir string, flags, includes, candidates []string) (interop.Result, error)

	// CHeader is the C header check of a local .h header (nil:
	// interop.CheckCHeaderFor, which also looks for the C compiler). Like
	// Probe, the pipeline calls it in the background only (probe.go).
	CHeader func(ctx context.Context, cxx build.CXX, dir string, flags []string, include string) (interop.CHeader, error)

	// PkgConfig runs pkg-config for a //gocpp:pkg-config line (nil:
	// build.PkgConfig). Like Probe, the pipeline calls it in the background
	// only (probe.go).
	PkgConfig build.PkgConfigFunc

	// IncludeDirs, if not nil, replaces the compiler run that lists the
	// system include folders of the C++ compiler cxx, for go to definition
	// on a header import (Pipeline.HeaderFile; the tests use it).
	IncludeDirs func(ctx context.Context, cxx build.CXX) ([]string, error)

	// OnProbe, if not nil, is called (on a goroutine of the pipeline) when
	// the background work of the namespace probe has changed an answer
	// that checks may have used: then the packages that are Stale should
	// be checked again in a new snapshot (Session.Recheck).
	OnProbe func()

	// OnAdapt, if not nil, is called (on a goroutine of the pipeline) with the folder of a package
	// whose argument adaptation diagnostics are ready or have changed (Snapshot.AdaptDiagnostics),
	// so that the server publishes its diagnostics again. It must not block.
	OnAdapt func(dir string)

	// ReadFile, if not nil, reads the files of the disk (load.ReadRegular otherwise: a named pipe is an error, not a wait;
	// the tests use it to make a file busy).
	ReadFile func(path string) ([]byte, error)

	// OnReadFailure, if not nil, is called when a file of the disk could not be read although the read was repeated
	// (another program has it open without sharing for longer than the repeats last: IsBusy says so, or the machine is out of
	// descriptors). The snapshot has no file then, and no event will come when the file is free again, so the server reads
	// it again in the background and makes a new snapshot when it can (Server.readFailed). It must not block.
	OnReadFailure func(path string, err error)

	// The memory caps of the caches; 0 means the default. MaxCheckBytes
	// caps the checked packages (with their ASTs, types.Info and the
	// types of their imports) by an estimate of their memory
	// (Package.cost), MaxSummaryBytes the summaries by the size of their
	// export data, and MaxParseBytes the parsed files by an estimate of
	// their memory (astFactor). The checked packages of the open documents stay while a
	// snapshot holds them (handle.last), whatever the cap.
	MaxCheckBytes, MaxSummaryBytes, MaxParseBytes int64

	// Disk, if not nil, is the cache on disk of the summaries (disk.go):
	// a summary that is not in memory is looked for there before its
	// package is checked, and the summaries that the pipeline computes are
	// written there in the background. The key of an entry has StdlibDir,
	// GOOS, GOARCH, Tags and Phase, but not Stdlib and GoStd (the standard
	// library itself): the version of the cache (filecache.Options.Version)
	// must change when they do. gocpp embeds them in its binary, so the
	// default version, the hash of the binary, does; a test that uses
	// another standard library must use another version. The pipeline
	// doesn't close it.
	Disk *filecache.Cache

	// Trace, if not nil, records the spans of the background work (the
	// namespace probes) for gocpp lsp -trace; the spans of a request or a
	// diagnostics run go to the recorder of their own context.
	Trace *trace.Recorder
}

// The default caps: the memory budget is 400 MB for a 50k-line project
// (README §2.1).
const (
	defaultMaxSummaryBytes = 64 << 20
	defaultMaxParseBytes   = 32 << 20
	maxMetadata            = 8192 // a metadata is a few hundred bytes
)

// A Pipeline is the memoized pipeline of a session: the caches that its
// snapshots share. Its methods are safe for concurrent use.
type Pipeline struct {
	cfg    Config
	cfgKey Hash // the parts of cfg that results depend on

	// testLoaded, if not nil, sees the program of a focused check right after its load (for the
	// tests).
	testLoaded func(*load.Program)

	parses    *memoize.Store[parseKey, *ParsedFile]
	checks    *memoize.Store[Hash, *Package]
	summaries *memoize.Store[Hash, *summary]

	// prober runs the compiler lookup and the namespace probes in the
	// background (probe.go).
	prober  *prober
	headers *headerIndexer // optional background C/C++ completion names
	members *memberIndexer // optional background receiver/member facts
	adapt   *adaptIndexer  // optional background argument adaptation diagnostics (adaptindex.go)

	// importSets keeps the decoded imports of the last check of each package
	// (importset.go).
	importSets importSets

	// Ordinary and patched-file checks each keep a separate bounded cohort per package, in the same
	// cost-capped check store. A completion/repair must not evict ordinary edit/undo versions, or vice versa.
	versions        versionKeeper
	variantVersions versionKeeper

	// sem holds a place for each type-check that runs, at most GOMAXPROCS.
	sem chan struct{}

	// metas keeps the metadata of each hash of headers (metadataFor).
	metaMu sync.Mutex
	metas  map[Hash]*metadata

	stdMu   sync.Mutex
	stdPkg  map[string][]*fileHandle // the files of each standard package, by import path
	stdList                          // StdPackages

	// The writes to the disk cache run in the background, at most
	// maxDiskWrites at a time; Close waits for them.
	diskMu  sync.Mutex
	closed  bool // guarded by diskMu
	diskWG  sync.WaitGroup
	diskSem chan struct{}

	includeDirsCache // HeaderFile

	stats struct {
		parses, metadata, checks         atomic.Int64
		testChecks, focused              atomic.Int64
		diskHits, diskMisses, diskWrites atomic.Int64
	}
}

// maxDiskWrites is the number of writes to the disk cache at the same time.
const maxDiskWrites = 4

// NewPipeline returns a pipeline with empty caches.
func NewPipeline(cfg Config) *Pipeline {
	if cfg.StdlibDir == "" {
		cfg.StdlibDir = "$GOCPP/stdlib"
	}
	// As the names of the files have it (filepath.Join): $GOCPP\stdlib on
	// Windows, for inStdlib.
	cfg.StdlibDir = filepath.Clean(cfg.StdlibDir)
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
		checks: memoize.NewStore[Hash](orDefault(cfg.MaxCheckBytes, DefaultMaxCheckBytes()), func(pkg *Package, _ error) int64 {
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
		sem:     make(chan struct{}, runtime.GOMAXPROCS(0)),
		diskSem: make(chan struct{}, maxDiskWrites),
		stdPkg:  make(map[string][]*fileHandle),
	}
	find := cfg.CXX
	if find == nil {
		find = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, cfg.GOOS, cfg.GOARCH) }
	}
	probe := cfg.Probe
	if probe == nil {
		probe = interop.ProbeNamespaces
	}
	p.prober = newProber(find, probe, cfg.OnProbe)
	p.headers = newHeaderIndexer(cfg)
	p.members = newMemberIndexer(cfg)
	p.adapt = newAdaptIndexer(cfg)
	p.prober.rec = cfg.Trace
	if cfg.PkgConfig != nil {
		p.prober.pc = cfg.PkgConfig
	}
	if cfg.CHeader != nil {
		p.prober.cheader = cfg.CHeader
	}
	return p
}

// Close stops the background work of the pipeline (the compiler lookup and
// the namespace probes) and waits for it to end, and for the writes to the
// disk cache that have started. Checks after Close don't start background
// work: their probe answers stay what they are, and their summaries are
// not written to disk.
func (p *Pipeline) Close() {
	p.adapt.close()
	p.members.close()
	p.headers.close()
	p.prober.close()
	p.diskMu.Lock()
	p.closed = true
	p.diskMu.Unlock()
	p.diskWG.Wait()
}

// HeadersChanged tells the pipeline that C/C++ headers may have changed on
// disk: the answers of the namespace probe are checked again in the
// background (Config.OnProbe says when one changed).
func (p *Pipeline) HeadersChanged() {
	p.adapt.refresh()
	p.members.refresh()
	p.headers.refresh()
	p.prober.headersChanged()
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

// NewState returns the state of the first snapshot of a session
// (NewSession): it has no open documents.
func (p *Pipeline) NewState() State {
	return &state{p: p, files: &layered[string, *fileHandle]{}, dirs: &layered[string, *dirHandle]{},
		mods: &layered[string, *fileHandle]{}, handles: &layered[pkgID, *handle]{}}
}

// Stats counts the work the pipeline has done: files parsed, metadata
// loads, and package checks. The tests and benchmarks use it.
func (p *Pipeline) Stats() (parses, metadata, checks int64) {
	return p.stats.parses.Load(), p.stats.metadata.Load(), p.stats.checks.Load()
}

// FocusedChecks returns the number of focused checks (focus.go) that started, the ones that
// declined after the load included. They are not in the checks of Stats.
func (p *Pipeline) FocusedChecks() int64 { return p.stats.focused.Load() }

// A CacheMetrics describes one of the pipeline's caches.
type CacheMetrics struct {
	Entries  int   `json:"entries"`  // the values kept
	Bytes    int64 `json:"bytes"`    // their estimated memory
	MaxBytes int64 `json:"maxBytes"` // the cap (0: none)
	Hits     int64 `json:"hits"`
	Misses   int64 `json:"misses"`
}

// HitRate returns hits / (hits + misses), or 0 when there were none.
func (m CacheMetrics) HitRate() float64 {
	if m.Hits+m.Misses == 0 {
		return 0
	}
	return float64(m.Hits) / float64(m.Hits+m.Misses)
}

// SetMaxCheckBytes changes the memory cap of the check cache (0: the default,
// DefaultMaxCheckBytes); what is over it goes at once.
func (p *Pipeline) SetMaxCheckBytes(n int64) {
	p.checks.SetMaxCost(orDefault(n, DefaultMaxCheckBytes()))
}

// Metrics describes the pipeline for gocpp/stats: the work done, the disk
// cache, and the caches in memory.
type Metrics struct {
	Parses, MetadataLoads, Checks    int64
	DiskHits, DiskMisses, DiskWrites int64
	Caches                           map[string]CacheMetrics // "parse", "check", "summary"
}

// Metrics returns the metrics of the pipeline now.
func (p *Pipeline) Metrics() Metrics {
	m := Metrics{Parses: p.stats.parses.Load(), MetadataLoads: p.stats.metadata.Load(), Checks: p.stats.checks.Load(),
		DiskHits: p.stats.diskHits.Load(), DiskMisses: p.stats.diskMisses.Load(), DiskWrites: p.stats.diskWrites.Load(),
		Caches: make(map[string]CacheMetrics, 3)}
	add := func(name string, n int, cost, max, hits, misses int64) {
		m.Caches[name] = CacheMetrics{Entries: n, Bytes: cost, MaxBytes: max, Hits: hits, Misses: misses}
	}
	n, c := p.parses.Stats()
	h, ms := p.parses.Counters()
	add("parse", n, c, p.parses.MaxCost(), h, ms)
	n, c = p.checks.Stats()
	h, ms = p.checks.Counters()
	add("check", n, c, p.checks.MaxCost(), h, ms)
	n, c = p.summaries.Stats()
	h, ms = p.summaries.Counters()
	add("summary", n, c, p.summaries.MaxCost(), h, ms)
	return m
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
	// tests are the full names of the _test.gp files of the listing (made
	// with the listing: TestFiles); all are the full names of every file,
	// made when asked for (SourceFiles).
	tests   []string
	allOnce sync.Once
	all     []string
	dirName string
	isDir   bool  // the folder exists: on disk (also when it can't be read) or in the overlays
	err     error // the folder can't be read and has no overlays
	// failed: reading the folder gave an error that is not "does not
	// exist" (permissions, too many open files), whatever the overlays
	// add to the listing: it is read again by the next snapshot.
	failed bool
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
	files    *layered[string, *fileHandle]
	dirs     *layered[string, *dirHandle]
	mods     *layered[string, *fileHandle] // the gocpp.mod file of each folder, or nil
	handles  *layered[pkgID, *handle]
	// order holds the handles in the order they were stored in (cloneHandles).
	order []*handle
	// tests holds the checks with test files by folder (CheckTests), made
	// when asked for.
	tests map[string]*memoize.Promise[*TestCheck]
}

// Clone implements State: the files, folders and packages that the changed
// documents don't touch stay (but the files and folders that were missing:
// they may be there now, see metadata.valid). A changed document changes its file, the
// listing of its folder, its package, every package whose imports were
// resolved with a read of the file or folder (an unsaved file in a new
// folder makes an import resolve), and every package that imports one of
// those, directly or not (they get new handles, whose keys see whether the
// API changed); a changed gocpp.mod changes every package. A changed path
// may be a folder (Session.Invalidate, or the deletion of a folder that the
// editor reports): then it changes everything under it too.
//
// A package of ch.Packages, and every package that imports it, directly or
// not, gets a new handle too (its check key is made again, from the
// summaries of its imports), but nothing is read again.
func (s *state) Clone(next *Snapshot, ch Change) State {
	changed := ch.Files
	n := &state{p: s.p, snap: next, overlays: make(map[string]*Overlay, len(next.overlays))}
	for uri, o := range next.overlays {
		if path := uri.Path(); path != "" {
			n.overlays[path] = o
		}
	}
	paths := make(nameSet, len(changed))
	dirs := make(nameSet, len(changed))
	recheck := make(nameSet, len(ch.Packages))
	for dir := range ch.Packages {
		recheck.add(dir)
	}
	var trees []string // the changed paths, as folders
	modChanged := false
	for uri := range changed {
		if path := uri.Path(); path != "" {
			paths.add(path)
			dirs.add(path) // it may be a folder
			dirs.add(filepath.Dir(path))
			trees = append(trees, path+string(filepath.Separator))
			modChanged = modChanged || fspath.Equal(filepath.Base(path), modFileName)
		}
	}
	// under reports whether path is in one of the changed folders.
	under := func(path string) bool {
		for _, t := range trees {
			if fspath.HasPrefixDir(path, t) {
				return true
			}
		}
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// What was missing is looked for again: an import may resolve now
	// (metadata.valid reads it).
	// A read that failed for any reason (a file that could not be read for
	// a moment) is made again too: a failure is no fact about the files
	// until a notification says so, and a snapshot must never keep one.
	n.files = s.files.clone(func(path string, f *fileHandle) bool {
		return paths.has(path) || under(path) || f.err != nil
	})
	n.dirs = s.dirs.clone(func(dir string, d *dirHandle) bool {
		return dirs.has(dir) || under(dir) || d.failed || len(d.names) == 0 && d.err != nil
	})
	if modChanged {
		// A module can change the import path of every package.
		n.mods = &layered[string, *fileHandle]{}
		n.handles = &layered[pkgID, *handle]{}
		return n
	}
	n.mods = s.mods.clone(func(dir string, m *fileHandle) bool {
		return dirs.has(dir) || under(dir) || m != nil && m.err != nil
	})
	n.handles, n.order = s.cloneHandles(func(h *handle) bool {
		return !h.id.std && (dirs.has(h.id.dir) || under(h.id.dir) || recheck.has(h.id.dir) || h.failedRead()) ||
			h.meta.reads != nil && touches(h.meta.reads, paths, dirs, under)
	})
	return n
}

// cloneHandles returns the handles of s that stay when the handles that stale says so for change:
// those, and every package that imports one of them, directly or not, are dropped. s.mu is held.
//
// The order of the handles is the order they were stored in, and a handle is stored after the
// handles of its imports (handle makes those first), so one pass over it finds the importers of
// a stale handle without an index of importers; the imports that make a cycle (cut) are the
// exception, and a pass is repeated while one of them changes the result.
func (s *state) cloneHandles(stale func(*handle) bool) (*layered[pkgID, *handle], []*handle) {
	if s.handles == nil {
		return &layered[pkgID, *handle]{}, nil
	}
	gone := make(map[pkgID]bool)
	hasCut := false
	for changed := true; changed; changed = changed && hasCut {
		changed = false
		for _, h := range s.order {
			if gone[h.id] {
				continue
			}
			if stale(h) || importsGone(h, gone) {
				gone[h.id] = true
				changed = true
			}
			hasCut = hasCut || len(h.cut) > 0
		}
	}
	kept := make([]*handle, 0, len(s.order)-len(gone))
	for _, h := range s.order {
		if !gone[h.id] {
			kept = append(kept, h)
		}
	}
	return s.handles.clone(func(id pkgID, _ *handle) bool { return gone[id] }), kept
}

// importsGone reports whether h imports a package of gone.
func importsGone(h *handle, gone map[pkgID]bool) bool {
	for _, dep := range h.deps {
		if gone[dep.id] {
			return true
		}
	}
	for _, id := range h.cut {
		if gone[id] {
			return true
		}
	}
	return false
}

// touches reports whether one of the reads is of a changed file or folder.
func touches(reads []*read, paths, dirs nameSet, under func(string) bool) bool {
	for _, r := range reads {
		if paths.has(r.path) || dirs.has(r.path) || under(r.path) {
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
	s.order = nil
}

// errReleased is the error of a request on a snapshot that is destroyed,
// which is a bug of the caller.
var errReleased = errors.New("cache: the snapshot was destroyed")

// file returns the file with the absolute name path in the snapshot.
func (s *state) file(path string) (*fileHandle, error) {
	path = fspath.Canon(path)
	s.mu.Lock()
	f, _ := s.files.get(path)
	s.mu.Unlock()
	if f != nil {
		return f, nil
	}
	f = &fileHandle{path: path}
	if o := s.overlays[path]; o != nil {
		f.src = []byte(o.Text())
	} else {
		read := s.p.cfg.ReadFile
		if read == nil {
			read = load.ReadRegular // a named pipe with the name of a source is an error, not a wait
		}
		f.src, f.err = retryRead(func() ([]byte, error) { return read(path) })
		if f.err != nil && transientRead(f.err) != 0 && s.p.cfg.OnReadFailure != nil {
			s.p.cfg.OnReadFailure(path, f.err)
		}
		// An editor shows a file without its byte order mark (VS Code takes it off and writes it back), so the
		// columns of line 0 that the server answers with must not count it. Windows editors write one often.
		f.src = bytes.TrimPrefix(f.src, utf8BOM)
	}
	f.hash = hashOf(f.src)
	return store(s, filesOf, path, f)
}

var utf8BOM = []byte("\xef\xbb\xbf")

// retryRead calls read, and again (a few times, after a short wait) when it
// fails because the process or the machine has no file descriptors or
// memory for a moment (a loaded machine does that), or, on Windows, because
// another program has the file open without sharing for a moment (a formatter
// that writes it, a virus scanner that looks at it: ERROR_SHARING_VIOLATION).
// The error must not become a fact about the files.
func retryRead[T any](read func() (T, error)) (T, error) {
	var v T
	var err error
	for try, wait := 0, 5*time.Millisecond; ; try++ {
		v, err = read()
		kind := transientRead(err)
		if err == nil || kind == 0 || try >= kind {
			return v, err
		}
		retrySleep(wait)
		if kind == transientResources {
			wait *= 4
		} else {
			wait *= 2
		}
	}
}

// retrySleep waits between the repeats of a read (the tests count the waits).
var retrySleep = time.Sleep

// ErrBusy is the error of a read that fails because another program has the file (the tests make it; on Windows the
// system's sharing and lock violations are busy too).
var ErrBusy = errors.New("the file is in use by another program")

// IsBusy reports whether err says that the file is in use by another program, which passes.
func IsBusy(err error) bool { return transientRead(err) == transientSharing }

// What transientRead says: how many times a failed read is repeated (0: not at all).
const (
	transientResources = 3 // out of descriptors or memory: waits of 5, 20 and 80 ms
	transientSharing   = 7 // a sharing violation on Windows: 5, 10, 20, ... 320 ms, about 0.6 s
)

// transientRead tells whether err is one that passes: how many times the read is repeated, or 0.
func transientRead(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) || errors.Is(err, syscall.ENOMEM) {
		return transientResources
	}
	if errors.Is(err, ErrBusy) {
		return transientSharing
	}
	if runtime.GOOS == "windows" {
		var en syscall.Errno
		if errors.As(err, &en) && (en == 32 || en == 33) { // ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION
			return transientSharing
		}
	}
	return 0
}

// store puts v under key into the table of s that table returns, unless a value is there already,
// and returns the value that is there then.
func store[K comparable, V any](s *state, table func(*state) *layered[K, V], key K, v V) (V, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := table(s)
	if t == nil {
		return v, nil // the handles of a released state
	}
	if old, ok := t.get(key); ok {
		return old, nil
	}
	t.put(key, v)
	return v, nil
}

// The tables of a state, for store.
func filesOf(s *state) *layered[string, *fileHandle] { return s.files }
func dirsOf(s *state) *layered[string, *dirHandle]   { return s.dirs }
func modsOf(s *state) *layered[string, *fileHandle]  { return s.mods }

// storeHandle puts h under id into the handles of s, unless there is one, and returns the one that
// is there then. It keeps the order of the handles (cloneHandles).
func (s *state) storeHandle(id pkgID, h *handle) *handle {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handles == nil {
		return h // a released state
	}
	if old, ok := s.handles.get(id); ok {
		return old
	}
	s.handles.put(id, h)
	s.order = append(s.order, h)
	return h
}

// dir returns the .gp files of the folder dir in the snapshot: on disk and
// in the overlays.
func (s *state) dir(dir string) (*dirHandle, error) {
	dir = fspath.Canon(dir)
	s.mu.Lock()
	d, _ := s.dirs.get(dir)
	s.mu.Unlock()
	if d != nil {
		return d, nil
	}
	d = &dirHandle{dirName: dir}
	entries, err := retryRead(func() ([]os.DirEntry, error) { return os.ReadDir(dir) })
	d.failed = err != nil && !errors.Is(err, fs.ErrNotExist)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gp") {
			d.names = append(d.names, e.Name())
		}
	}
	for path := range s.overlays {
		if fspath.Equal(filepath.Dir(path), dir) && strings.HasSuffix(path, ".gp") && !containsName(d.names, filepath.Base(path)) {
			d.names = append(d.names, filepath.Base(path))
		}
	}
	slices.Sort(d.names)
	for _, name := range d.names {
		if strings.HasSuffix(name, "_test.gp") {
			d.tests = append(d.tests, filepath.Join(dir, name))
		}
	}
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

// A nameSet is a set of file names, compared by the rule of the file system for letter case (fspath.Key).
type nameSet map[string]bool

func (s nameSet) add(name string)      { s[fspath.Key(name)] = true }
func (s nameSet) has(name string) bool { return s[fspath.Key(name)] }

// containsName reports whether names has name, by the rule of the file system (letter case counts or not).
func containsName(names []string, name string) bool {
	if !fspath.FoldCase() {
		return slices.Contains(names, name)
	}
	return slices.ContainsFunc(names, func(n string) bool { return fspath.Equal(n, name) })
}

// modFileName is the name of the file at the top of a module.
const modFileName = "gocpp.mod"

// modFile returns the gocpp.mod file of the folder dir, as the loader
// finds it (load.FindModule): in dir or the nearest folder above it, or nil.
func (s *state) modFile(dir string) (*fileHandle, error) {
	dir = fspath.Canon(dir)
	s.mu.Lock()
	m, ok := s.mods.get(dir)
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
