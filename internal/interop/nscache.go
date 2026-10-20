package interop

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The cache of the probes keeps, per include set (compiler, folder, flags
// and includes), the kind of every path that was probed, or the include
// errors, and the macros; and per local header, the answer of the C header
// check (externc.go). It checks the files that the results depend on
// before it uses them.

// defaultCache is the cache of ProbeKinds, ProbeMacros and CheckCHeader,
// with the cache on disk (diskcache.go) as its second level.
var defaultCache = func() *cache {
	c := newCache()
	c.disk = &diskStore{}
	return c
}()

const (
	// maxSets is the number of include sets that the cache keeps, for
	// each probe. The language server runs for days, and every change to a
	// file's imports makes a new include set; the one used least recently
	// goes.
	maxSets = 64

	// maxKinds is the number of paths that an include set keeps. Misspelled
	// names pile up while someone types; when there are more, the set
	// keeps only the last run's paths.
	maxKinds = 4096

	// errorTTL is how long a result with include errors is used. The
	// cache can't see everything that fixes an include: a library that
	// is installed, or a missing header that appears next to a header
	// that includes it.
	errorTTL = 5 * time.Second

	// racyWindow: a file that changed this short a time before the
	// compiler started may have changed while it ran, after it was read,
	// so a result that depends on it is not cached. The go command's file
	// hash cache does the same, with the same two seconds, because some
	// file systems keep modification times in steps of a second or two.
	racyWindow = 2 * time.Second
)

// A cache is a probe cache. Its zero value is not ready; use newCache.
type cache struct {
	kinds    setMap[kindSet]
	macros   setMap[macroSet]
	cheaders setMap[cHeaderSet]

	errorTTL time.Duration // errorTTL, or less in tests

	// disk is the second level, the cache on disk (diskcache.go); nil for
	// none, as in the tests that count compiler runs.
	disk *diskStore

	// diskHits and diskStores count the answers read from the disk and
	// the entries written to it, for the tests.
	diskHits, diskStores atomic.Int64

	// runs, macroRuns and cRuns count the runs of the kind probe (one run
	// can run the compiler again without a path that stopped it, see
	// maxReruns), of the macro probe and of the C header check, for the
	// tests.
	runs, macroRuns, cRuns atomic.Int64
}

func newCache() *cache {
	return &cache{kinds: newSetMap[kindSet](), macros: newSetMap[macroSet](), cheaders: newSetMap[cHeaderSet](), errorTTL: errorTTL}
}

// A setMap keeps a T per include set, each with its own lock, and at most
// maxSets of them.
type setMap[T any] struct {
	mu    sync.Mutex
	sets  map[string]*lockedSet[T]
	clock uint64 // counts uses, for lastUse
}

// A lockedSet is a T and its lock.
type lockedSet[T any] struct {
	// lock is held while the set is checked, probed or changed: a
	// one-element semaphore, so that waiting for it can be canceled.
	lock chan struct{}

	lastUse uint64 // the map's clock at the last use; guarded by setMap.mu
	val     T
}

func newSetMap[T any]() setMap[T] {
	return setMap[T]{sets: make(map[string]*lockedSet[T])}
}

// lock returns the value of the include set key, locked, after it waited
// for the lock: the probe of another call for the same set comes first,
// and its answer may be in the cache then. unlock unlocks it.
func (m *setMap[T]) lock(ctx context.Context, key string) (val *T, unlock func(), err error) {
	m.mu.Lock()
	set := m.sets[key]
	if set == nil {
		if len(m.sets) >= maxSets {
			m.evictLocked()
		}
		set = &lockedSet[T]{lock: make(chan struct{}, 1)}
		m.sets[key] = set
	}
	m.clock++
	set.lastUse = m.clock
	m.mu.Unlock()
	select {
	case set.lock <- struct{}{}:
		return &set.val, func() { <-set.lock }, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// evictLocked removes the include set used least recently. A call that
// still uses it finishes with it.
func (m *setMap[T]) evictLocked() {
	var oldest string
	for key, set := range m.sets {
		if oldest == "" || set.lastUse < m.sets[oldest].lastUse {
			oldest = key
		}
	}
	delete(m.sets, oldest)
}

// A kindSet is what the cache knows of the kinds for one include set.
type kindSet struct {
	filled  bool            // the fields below hold a result
	files   []fileState     // what the result depends on
	kinds   map[string]Kind // per path, without a leading ::
	incErrs []IncludeError  // the include errors; then kinds is nil
	expires time.Time       // when incErrs is set: when to probe again
	born    time.Time       // when the oldest answer was probed (the disk entry's age)
}

// reset forgets the set's result.
func (s *kindSet) reset() { *s = kindSet{} }

// fileReads counts the files that stateOf read, for the tests.
var fileReads atomic.Int64

// A fileState is a file that a result depends on, as it was when the
// result was made: its content hash, or that it wasn't there. It also
// keeps the file's stat, so that a check reads and hashes the file only
// when the stat changed (as the go command does): a single-file library
// can be several MiB, and the language server checks on every keystroke.
type fileState struct {
	path   string
	exists bool
	hash   [sha256.Size]byte
	stat   fileStat
}

// A fileStat is what a stat says about a file's content: when the content
// changes, so does one of these. It is safe because a result is cached
// only when its files were last changed racyWindow before the probe, so
// a later change can't keep the modification time.
type fileStat struct {
	size, modTime int64 // modTime in nanoseconds
	sys           sysStat
}

func statOf(fi fs.FileInfo) fileStat {
	return fileStat{size: fi.Size(), modTime: fi.ModTime().UnixNano(), sys: sysStatOf(fi)}
}

// sameContent reports whether f and g are the same file with the same
// content (or both not there), whatever their stats say.
func (f *fileState) sameContent(g fileState) bool {
	return f.path == g.path && f.exists == g.exists && f.hash == g.hash
}

// unchanged reports whether the file still has the content that f says.
// When only its stat changed (it was touched, or written with the same
// content), f takes the new stat, once that is older than racyWindow.
func (f *fileState) unchanged(now time.Time) bool {
	fi, err := os.Stat(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return !f.exists
	}
	if err != nil || !f.exists {
		return false
	}
	if statOf(fi) == f.stat {
		return true
	}
	cur, mod, err := stateOf(f.path)
	if err != nil || !f.sameContent(cur) {
		return false
	}
	if mod.Before(now.Add(-racyWindow)) {
		f.stat = cur.stat
	}
	return true
}

// errChangedWhileRead is stateOf's error for a file that changed while it
// was read.
var errChangedWhileRead = errors.New("the file changed while it was read")

// testHookRead, when set, runs in stateOf between the first stat and the
// read, for the tests.
var testHookRead atomic.Pointer[func(path string)]

// stateOf returns the state of the file at path and its modification time.
// A file that isn't there has a state too; an error means that the file
// can't be read, or that it changed while it was read (then a result can't
// depend on it, and a check counts it as changed). The file is stat'ed
// before and after the read, and the stats must be the same: so the hash
// belongs to the stat that is kept, and the modification time for the
// racy-window check is the one after the read.
func stateOf(path string) (fileState, time.Time, error) {
	before, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fileState{path: path}, time.Time{}, nil
	}
	if err != nil {
		return fileState{}, time.Time{}, err
	}
	if hook := testHookRead.Load(); hook != nil {
		(*hook)(path)
	}
	fileReads.Add(1)
	data, err := os.ReadFile(path)
	if err != nil {
		return fileState{}, time.Time{}, err
	}
	after, err := os.Stat(path)
	if err != nil {
		return fileState{}, time.Time{}, err
	}
	if statOf(before) != statOf(after) {
		return fileState{}, time.Time{}, errChangedWhileRead
	}
	return fileState{path: path, exists: true, hash: sha256.Sum256(data), stat: statOf(after)}, after.ModTime(), nil
}

type missHookKey struct{}

// WithMissHook returns a context in which ProbeKinds (and ProbeNamespaces) call fn just before they
// start the compiler, because the cache had no answer. A caller that does more work when the cache is
// cold (typed interop starts the precompiled header of its probes) learns that this way, before the
// compiler is done. fn runs in the goroutine of the probe and must not block.
func WithMissHook(ctx context.Context, fn func()) context.Context {
	return context.WithValue(ctx, missHookKey{}, fn)
}

// probe is ProbeKinds with the cache c.
func (c *cache) probe(ctx context.Context, cxx build.CXX, dir string, flags, includes, paths []string) (Result, error) {
	return c.probeMode(ctx, cxx, dir, flags, includes, paths, true)
}

// probeNamespaces is ProbeNamespaces with the cache c.
func (c *cache) probeNamespaces(ctx context.Context, cxx build.CXX, dir string, flags, includes, names []string) (Result, error) {
	return c.probeMode(ctx, cxx, dir, flags, includes, names, false)
}

// probeMode is ProbeKinds (kinds) or ProbeNamespaces with the cache c.
// The kinds of an include set are kept per path; an answer of the
// namespace mode is Namespace, Macro or NotNamespace, and only
// NotNamespace says less than ProbeKinds would.
func (c *cache) probeMode(ctx context.Context, cxx build.CXX, dir string, flags, includes, paths []string, kinds bool) (Result, error) {
	if err := checkArgs(flags, includes); err != nil {
		return Result{}, fmt.Errorf("kind probe: %w", err)
	}
	// names are the paths without a leading ::, each once, and parts
	// their parts.
	var names []string
	var parts [][]string
	seen := make(map[string]bool)
	for _, path := range paths {
		p, err := splitPath(path)
		if err != nil {
			return Result{}, err
		}
		if name := strings.Join(p, "::"); !seen[name] && !reservedName(name) {
			seen[name] = true
			names = append(names, name)
			parts = append(parts, p)
		}
	}
	// Without includes nothing can declare anything (a .gp file with a
	// namespace rename but no header import): every path is Unknown, and
	// the caller reports the rename. With includes, the probe runs also
	// without paths (or with only reserved ones, which need no probe
	// line), to check the includes.
	if len(includes) == 0 {
		return result(paths, nil, nil, kinds), nil
	}
	if cxx.Family != build.Clang && cxx.Family != build.GCC {
		return Result{}, fmt.Errorf("kind probe: %w", unknownFamily(cxx))
	}
	if dir != "" {
		var err error
		if dir, err = filepath.Abs(dir); err != nil {
			return Result{}, err
		}
	}

	set, unlock, err := c.kinds.lock(ctx, setKey(cxx, dir, flags, includes))
	if err != nil {
		return Result{}, err
	}
	defer unlock()

	if set.filled && !c.valid(set.files, set.incErrs != nil, set.expires) {
		set.reset()
	}
	// With the cache on disk, a set that is older than maxDiskAge is
	// probed again (once an hour in an editor that runs for days): the file
	// list can't see a header that appears earlier on the search path, and
	// a long-lived set would also write its old age to the disk, over the
	// fresh entries of other processes. (Sets with include errors have no
	// age; they expire by errorTTL.)
	if set.filled && !set.born.IsZero() && time.Since(set.born) > maxDiskAge && c.disk.folder() != "" {
		set.reset()
	}
	// The disk entry's key is made when it is needed: a memory hit costs
	// no stat of the compiler.
	var key diskKey
	var onDisk, keyMade bool
	diskKeyOf := func() (diskKey, bool) {
		if !keyMade {
			key, onDisk = c.disk.keyFor("k", cxx, dir, flags, includes)
			keyMade = true
		}
		return key, onDisk
	}
	if !set.filled {
		if key, ok := diskKeyOf(); ok {
			c.loadKinds(key, set)
		}
	}
	if set.filled && set.incErrs != nil {
		return result(paths, nil, set.incErrs, kinds), nil
	}
	if set.filled && hasAll(set.kinds, names, kinds) {
		return result(paths, set.kinds, nil, kinds), nil
	}

	// Probe all the paths of this call, not only the new ones: the headers
	// take the time, not the paths, and so the answer comes from one run.
	if fn, ok := ctx.Value(missHookKey{}).(func()); ok {
		fn()
	}
	c.runs.Add(1)
	run, err := runProbe(ctx, cxx, dir, flags, includes, parts, kinds)
	if err != nil {
		return Result{}, err
	}
	files, ok := dependsOn(run.deps, run.depsOK, run.incErrs != nil, run.start, dir, includes)
	if !ok {
		// A file may have changed while the compiler ran: answer, but
		// don't cache.
		set.reset()
		return result(paths, kindMap(names, run.kinds), run.incErrs, kinds), nil
	}
	if run.incErrs != nil {
		set.reset()
		set.filled, set.files, set.incErrs = true, files, run.incErrs
		set.expires = time.Now().Add(c.errorTTL)
		return result(paths, nil, run.incErrs, kinds), nil
	}
	if !set.filled || !sameFiles(set.files, files) || len(set.kinds)+len(names) > maxKinds {
		set.reset()
		set.filled, set.files, set.kinds, set.born = true, files, make(map[string]Kind), run.start
	}
	for i, name := range names {
		// A NotNamespace doesn't replace a kind that says more.
		if _, ok := set.kinds[name]; !ok || run.kinds[i] != NotNamespace {
			set.kinds[name] = run.kinds[i]
		}
	}
	if key, ok := diskKeyOf(); ok {
		c.storeKinds(key, set)
	}
	return result(paths, set.kinds, nil, kinds), nil
}

// valid reports whether a result that depends on files can still be used.
// A result with include errors (failed) can be used until expires.
func (c *cache) valid(files []fileState, failed bool, expires time.Time) bool {
	now := time.Now()
	if failed && !now.Before(expires) {
		return false
	}
	for i := range files {
		if !files[i].unchanged(now) {
			return false
		}
	}
	return true
}

// setKey is the cache key of an include set. None of its parts can hold a
// NUL byte (checkArgs, and paths can't), so the key can't be confused with
// another one.
func setKey(cxx build.CXX, dir string, flags, includes []string) string {
	// The compiler's own arguments (CXX="g++ -m32") and the flags come
	// after their number, so that they can't be confused with the parts
	// after them.
	parts := append([]string{cxx.Path, fmt.Sprint(len(cxx.Args))}, cxx.Args...)
	// The target and its flags (a clang for Windows probes with --target and the MinGW-w64
	// headers): the same clang for two targets must not share an entry.
	target := build.ProbeTargetFlags(cxx)
	parts = append(parts, cxx.Family.String(), cxx.Version, cxx.GOOS, cxx.GOARCH, fmt.Sprint(len(target)))
	parts = append(parts, target...)
	parts = append(parts, dir, fmt.Sprint(len(flags)))
	parts = append(parts, flags...)
	return strings.Join(append(parts, includes...), "\x00")
}

// dependsOn returns the files that the result of a run depends on: the
// headers that the compiler read, the system headers too (deps, from
// its dependency file; depsOK says whether there was one), and for each
// quoted include, the file it names in dir, also when that isn't there
// (when it appears, it hides the header that was found elsewhere). ok is
// false when the result can't be cached: a file can't be read, or it
// changed shortly before the compiler started (racyWindow, from start).
// Without a dependency file (gcc and clang write none after a header that
// isn't found), only the quoted includes are known; that is enough for a
// result with include errors (failed), which expires.
//
// A limit: a header that appears earlier on the search path than the one
// that was found (dir/lib.h in front of inc/lib.h, found through -I) and
// would win now is not noticed, except for the top-level quoted includes in
// dir. The C header check has the same limit (runCHeaderCheck).
func dependsOn(deps []string, depsOK, failed bool, start time.Time, dir string, includes []string) (files []fileState, ok bool) {
	if !depsOK && !failed {
		return nil, false
	}
	paths := slices.Clip(deps)
	if dir != "" {
		for _, inc := range includes {
			if inc, _ = SplitExternC(inc); inc[0] == '"' {
				p := inc[1 : len(inc)-1]
				if !filepath.IsAbs(p) {
					p = filepath.Join(dir, p)
				}
				paths = append(paths, filepath.Clean(p))
			}
		}
	}
	seen := make(map[string]bool)
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		f, mod, err := stateOf(p)
		if err != nil || f.exists && mod.After(start.Add(-racyWindow)) {
			return nil, false
		}
		files = append(files, f)
	}
	return files, true
}

// sameFiles reports whether a and b are the same files with the same
// contents, in any order.
func sameFiles(a, b []fileState) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]fileState, len(a))
	for _, f := range a {
		m[f.path] = f
	}
	for _, f := range b {
		if g, ok := m[f.path]; !ok || !g.sameContent(f) {
			return false
		}
	}
	return true
}

// hasAll reports whether kinds has an answer for every name: any answer
// for the namespace mode, a kind of ProbeKinds (not NotNamespace) for
// allKinds.
func hasAll(kinds map[string]Kind, names []string, allKinds bool) bool {
	for _, n := range names {
		if k, ok := kinds[n]; !ok || allKinds && k == NotNamespace {
			return false
		}
	}
	return true
}

// kindMap maps names[i] to kinds[i]; kinds is nil after include errors.
func kindMap(names []string, kinds []Kind) map[string]Kind {
	m := make(map[string]Kind, len(names))
	for i, n := range names {
		if kinds != nil {
			m[n] = kinds[i]
		}
	}
	return m
}

// result returns the Result for the paths as the caller spelled them:
// their kinds from kinds (keyed without the leading ::), or Unknown for
// all of them when there are include errors; a reserved name is always
// NotFound. For the namespace mode (not allKinds), the kinds are the
// answers to the namespace question (nsAnswer). The Result has its own
// copy of incErrs, which the cache keeps.
func result(paths []string, kinds map[string]Kind, incErrs []IncludeError, allKinds bool) Result {
	r := Result{Kinds: make(map[string]Kind, len(paths)), IncludeErrors: slices.Clone(incErrs)}
	for _, path := range paths {
		name := strings.TrimPrefix(path, "::")
		k := Unknown
		switch {
		case reservedName(name):
			k = NotFound
		case incErrs == nil:
			k = kinds[name] // Unknown when kinds is nil
		}
		if !allKinds {
			k = nsAnswer(k)
		}
		r.Kinds[path] = k
	}
	return r
}

// reservedName reports whether the path name (without a leading ::)
// starts with gp_: gocpp's own names start with gp_ (ADR-010; the probe
// file's names are gp_kn_0, gp_inc_0, ...), and a user's gp_ names are
// escaped, so such a name is never something a .gp file can use. (Only
// the first part counts: gp_ names inside a C++ namespace are not
// gocpp's.)
func reservedName(name string) bool {
	return strings.HasPrefix(name, "gp_")
}
