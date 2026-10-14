package interop

import (
	"context"
	"crypto/sha256"
	"errors"
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

// The cache of the namespace probe keeps, per include set (compiler,
// folder and includes), the kind of every candidate that was probed, or
// the include errors. It checks the files that the results depend on
// before it uses them.

// defaultCache is the cache of ProbeNamespaces.
var defaultCache = newCache()

const (
	// maxSets is the number of include sets that the cache keeps. The
	// language server runs for days, and every change to a file's imports
	// makes a new include set; the one used least recently goes.
	maxSets = 64

	// maxKinds is the number of candidates that an include set keeps.
	// Misspelled names pile up while someone types; when there are more,
	// the set keeps only the last run's candidates.
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

// A cache is a namespace probe cache. Its zero value is not ready; use
// newCache.
type cache struct {
	mu    sync.Mutex
	sets  map[string]*includeSet
	clock uint64 // counts uses, for lastUse

	errorTTL time.Duration // errorTTL, or less in tests

	// runs counts the compiler runs, for the tests.
	runs atomic.Int64
}

func newCache() *cache {
	return &cache{sets: make(map[string]*includeSet), errorTTL: errorTTL}
}

// An includeSet is what the cache knows for one include set.
type includeSet struct {
	// lock is held while the set is checked, probed or changed: a
	// one-element semaphore, so that waiting for it can be canceled.
	lock chan struct{}

	lastUse uint64 // the cache's clock at the last use; guarded by cache.mu

	filled  bool            // the fields below hold a result
	files   []fileState     // what the result depends on
	kinds   map[string]Kind // per candidate, without a leading ::
	incErrs []IncludeError  // the include errors; then kinds is nil
	expires time.Time       // when incErrs is set: when to probe again
}

// reset forgets the set's result.
func (s *includeSet) reset() {
	s.filled, s.files, s.kinds, s.incErrs, s.expires = false, nil, nil, nil, time.Time{}
}

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

// probe is ProbeNamespaces with the cache c.
func (c *cache) probe(ctx context.Context, cxx build.CXX, dir string, includes, candidates []string) (Result, error) {
	for _, inc := range includes {
		if err := checkInclude(inc); err != nil {
			return Result{}, err
		}
	}
	// names are the candidates without a leading ::, each once, and parts
	// their parts.
	var names []string
	var parts [][]string
	seen := make(map[string]bool)
	for _, cand := range candidates {
		p, err := splitCandidate(cand)
		if err != nil {
			return Result{}, err
		}
		if name := strings.Join(p, "::"); !seen[name] && !reservedName(name) {
			seen[name] = true
			names = append(names, name)
			parts = append(parts, p)
		}
	}
	if len(candidates) == 0 {
		return Result{Kinds: map[string]Kind{}}, nil
	}
	// Without includes nothing can declare a namespace (a .gp file with a
	// namespace rename but no header import): every candidate is Unknown,
	// and the caller reports the rename. Reserved names need no probe.
	if len(includes) == 0 || len(names) == 0 {
		return result(candidates, nil, nil), nil
	}
	if cxx.Family != build.Clang && cxx.Family != build.GCC {
		return Result{}, unknownFamily(cxx)
	}
	if dir != "" {
		var err error
		if dir, err = filepath.Abs(dir); err != nil {
			return Result{}, err
		}
	}

	set, err := c.lock(ctx, setKey(cxx, dir, includes))
	if err != nil {
		return Result{}, err
	}
	defer func() { <-set.lock }()

	if set.filled && !c.valid(set) {
		set.reset()
	}
	if set.filled && set.incErrs != nil {
		return result(candidates, nil, set.incErrs), nil
	}
	if set.filled && hasAll(set.kinds, names) {
		return result(candidates, set.kinds, nil), nil
	}

	// Probe all the candidates of this call, not only the new ones: the
	// headers take the time, not the candidates, and so the answer comes
	// from one run.
	c.runs.Add(1)
	run, err := runProbe(ctx, cxx, dir, includes, parts)
	if err != nil {
		return Result{}, err
	}
	files, ok := dependsOn(run, dir, includes)
	if !ok {
		// A file may have changed while the compiler ran: answer, but
		// don't cache.
		set.reset()
		return result(candidates, kindMap(names, run.kinds), run.incErrs), nil
	}
	if run.incErrs != nil {
		set.reset()
		set.filled, set.files, set.incErrs = true, files, run.incErrs
		set.expires = time.Now().Add(c.errorTTL)
		return result(candidates, nil, run.incErrs), nil
	}
	if !set.filled || !sameFiles(set.files, files) || len(set.kinds)+len(names) > maxKinds {
		set.reset()
		set.filled, set.files, set.kinds = true, files, make(map[string]Kind)
	}
	for i, name := range names {
		set.kinds[name] = run.kinds[i]
	}
	return result(candidates, set.kinds, nil), nil
}

// lock returns the include set for key, locked, after it waited for the
// lock: the probe of another call for the same set comes first, and its
// answer may be in the cache then.
func (c *cache) lock(ctx context.Context, key string) (*includeSet, error) {
	c.mu.Lock()
	set := c.sets[key]
	if set == nil {
		if len(c.sets) >= maxSets {
			c.evictLocked()
		}
		set = &includeSet{lock: make(chan struct{}, 1)}
		c.sets[key] = set
	}
	c.clock++
	set.lastUse = c.clock
	c.mu.Unlock()
	select {
	case set.lock <- struct{}{}:
		return set, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// evictLocked removes the include set used least recently. A call that
// still uses it finishes with it.
func (c *cache) evictLocked() {
	var oldest string
	for key, set := range c.sets {
		if oldest == "" || set.lastUse < c.sets[oldest].lastUse {
			oldest = key
		}
	}
	delete(c.sets, oldest)
}

// valid reports whether the result of the filled set can still be used.
func (c *cache) valid(set *includeSet) bool {
	if set.incErrs != nil && !time.Now().Before(set.expires) {
		return false
	}
	now := time.Now()
	for i := range set.files {
		if !set.files[i].unchanged(now) {
			return false
		}
	}
	return true
}

// setKey is the cache key of an include set. None of its parts can hold a
// NUL byte (checkInclude, and paths can't), so the key can't be confused
// with another one.
func setKey(cxx build.CXX, dir string, includes []string) string {
	return strings.Join(append([]string{cxx.Path, cxx.Family.String(), cxx.Version, dir}, includes...), "\x00")
}

// dependsOn returns the files that the result of run depends on: the
// headers outside the system folders that the compiler read (its
// dependency file), and for each quoted include, the file it names in dir,
// also when that isn't there (when it appears, it hides the header that
// was found elsewhere). ok is false when the result can't be cached: a
// file can't be read, or it changed shortly before the compiler started
// (racyWindow). Without a dependency file (gcc and clang write none after
// a header that isn't found), only the quoted includes are known; that is
// enough for a result with include errors, which expires.
func dependsOn(run *probeRun, dir string, includes []string) (files []fileState, ok bool) {
	if !run.depsOK && run.incErrs == nil {
		return nil, false
	}
	paths := slices.Clip(run.deps)
	if dir != "" {
		for _, inc := range includes {
			if inc[0] == '"' {
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
		if err != nil || f.exists && mod.After(run.start.Add(-racyWindow)) {
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

// hasAll reports whether kinds has every name.
func hasAll(kinds map[string]Kind, names []string) bool {
	for _, n := range names {
		if _, ok := kinds[n]; !ok {
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

// result returns the Result for the candidates as the caller spelled them:
// their kinds from kinds (keyed without the leading ::), or Unknown for
// all of them when there are include errors; a reserved name is always
// NotNamespace. The Result has its own copy of incErrs, which the cache
// keeps.
func result(candidates []string, kinds map[string]Kind, incErrs []IncludeError) Result {
	r := Result{Kinds: make(map[string]Kind, len(candidates)), IncludeErrors: slices.Clone(incErrs)}
	for _, cand := range candidates {
		name := strings.TrimPrefix(cand, "::")
		switch {
		case reservedName(name):
			r.Kinds[cand] = NotNamespace
		case incErrs == nil:
			r.Kinds[cand] = kinds[name] // Unknown when kinds is nil
		default:
			r.Kinds[cand] = Unknown
		}
	}
	return r
}

// reservedName reports whether the candidate name (without a leading ::)
// starts with gp_: gocpp's own names start with gp_ (ADR-010; the probe
// file's aliases are gp_ns_0, gp_inc_0, ...), and a user's gp_ names are
// escaped, so such a name is never a namespace a .gp file can use.
func reservedName(name string) bool {
	return strings.HasPrefix(name, "gp_")
}
