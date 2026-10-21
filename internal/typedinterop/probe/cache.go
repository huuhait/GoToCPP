package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The cache of the probe products on disk (step 12.7, ADR-088).
//
// What is cached: the results of expression, type and constant probes, one entry per
// reference (a "unit"), the declaration extractions of Runner.Decls, one entry per compiler
// run, and for both the list of headers the compiler read with a hash of their contents.
// Nothing here runs a compiler: units.go and decls.go decide what to ask and what to keep.
//
// Layout, under <user cache>/gocpp/probe/typed-v1/ (the kind probe of internal/interop keeps
// its files in the folder above, and `gocpp clean -cache` removes both):
//
//	c/ab/<hash>.json   the context of a probe file: its preamble's key, and the variants of
//	                   its header lists (a list of files, the hash of their contents)
//	u/ab/<hash>.json   the result of one unit, under (context key, hash of the headers, unit)
//	d/ab/<hash>.json   the part that one declaration run made, under its text's key, with the
//	                   header variants as the context has them
//	locks/<hash>.lock  a compiler run in progress (another gocpp waits for it)
//
// A file is a line with the SHA-256 of the rest, then the JSON. It is written to a temporary
// name in the same folder and renamed into place, so a reader sees the old file or the new one;
// one that is cut off, garbled or of another version is a miss and is written again. A read
// refreshes the modification time (once in touchAge), which is the order of eviction. The
// folder is bounded (Limits), checked by prune. When the folder cannot be made or written, the
// process goes on with its memory (each Runner keeps what it made).
//
// Environment: GOCPP_PROBE_CACHE is the folder (absolute), or off/0/false for none;
// GOCPP_NOPROBECACHE=1 turns it off too (the variable of the kind probe). A test binary uses
// no folder unless GOCPP_PROBE_CACHE says which.

// The environment variables of the cache.
const (
	// EnvProbeCache is the folder of the probe cache, the variable of the kind probe's cache
	// (internal/interop): the typed products are in its subfolder typed-v1. "off", "0" or
	// "false" turn the cache on disk off. Test binaries use it only when it is set.
	EnvProbeCache = "GOCPP_PROBE_CACHE"
	// EnvNoProbeCache turns the cache on disk off when it is set to anything but "", "0"
	// or "false" in any case.
	EnvNoProbeCache = "GOCPP_NOPROBECACHE"
)

const (
	// cacheFormat is the version of the file format. The folder is named after it.
	cacheFormat = 1
	cacheSub    = "typed-v1"

	// maxEntry is the largest file that is read.
	maxEntry = 64 << 20

	// touchAge is how old an entry is when a read refreshes its time.
	touchAge = 10 * time.Minute

	// staleTemp is the age of a temporary file that pruning removes.
	staleTemp = time.Hour

	// pruneEvery is the number of writes of a process between two prunings, and pruneMarkerAge how
	// soon after another process's pruning (the time of the marker file) none is made.
	pruneEvery     = 512
	pruneMarkerAge = time.Hour
)

// racyWindow: a header that was modified (its modification time, or its change time, which no
// program can set back) less than this before the compiler started, or at any time after, may have
// been modified again after the compiler read it, within one tick of the file system's clock and
// without its size or time changing: a result that depends on one is not kept at all (not in
// memory either), and the next run asks again. A variable for the tests.
var racyWindow = 2 * time.Second

// SetRacyWindow sets the racy window (racyWindow) and returns the function that puts it back. For tests of
// programs that write a header and then probe it: a window of 0 makes a header that was written a moment
// ago an ordinary one (the cache then trusts that nobody edits it while the compiler runs).
func SetRacyWindow(d time.Duration) (restore func()) {
	old := racyWindow
	racyWindow = d
	return func() { racyWindow = old }
}

// lockMargin is added to a run's time limit to get how long a lock is waited for (a variable for the tests).
var lockMargin = 15 * time.Second

// Limits are the bounds of the cache folder; prune removes the entries used least
// recently until they hold.
type Limits struct {
	MaxEntries int
	MaxBytes   int64
	MaxAge     time.Duration // an entry not used for this long goes
}

// DefaultLimits are the bounds of the cache folder.
var DefaultLimits = Limits{MaxEntries: 100000, MaxBytes: 256 << 20, MaxAge: 90 * 24 * time.Hour}

// CacheDir returns the folder of the typed probe cache that the environment asks for, or ""
// when there is none. It looks at the environment at each call.
func CacheDir() string {
	if build.EnvOff(EnvNoProbeCache) {
		return ""
	}
	v := os.Getenv(EnvProbeCache)
	switch strings.ToLower(v) {
	case "off", "0", "false", "no":
		return ""
	}
	if v != "" {
		if !filepath.IsAbs(v) {
			return ""
		}
		return filepath.Join(v, cacheSub)
	}
	if testing.Testing() {
		return ""
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "gocpp", "probe", cacheSub)
}

// The counters of the cache, for tests, `-stats` and the benchmarks.
var cacheCount struct {
	memHits, diskHits, misses atomic.Int64 // units, contexts and declaration parts looked up
	stores, storeErrors       atomic.Int64
	corrupt                   atomic.Int64 // files that were read but are not entries
	lockWaits                 atomic.Int64
	lockLeaks                 atomic.Int64 // locks that could not be removed (they are taken over later as stale)
	runs                      atomic.Int64 // compiler runs
	pruned                    atomic.Int64
	pchBuilds, pchUses        atomic.Int64 // precompiled headers made, and probe runs that loaded one
	pchRejects                atomic.Int64 // runs whose compiler refused the header
	pchRechecks               atomic.Int64 // runs that failed with the header and were made again without it
	typeRechecks              atomic.Int64 // type runs refused for a declaration of the header, made again without it
}

// A Counters is a snapshot of the cache's counters of this process.
type Counters struct {
	MemHits, DiskHits, Misses int64
	Stores, StoreErrors       int64
	Corrupt                   int64
	LockWaits                 int64
	LockLeaks                 int64
	Runs                      int64
	Pruned                    int64
	PCHBuilds, PCHUses        int64
	PCHRejects                int64
	PCHRechecks               int64
	TypeRechecks              int64
}

// CacheCounters returns the counters of this process.
func CacheCounters() Counters {
	c := &cacheCount
	return Counters{c.memHits.Load(), c.diskHits.Load(), c.misses.Load(), c.stores.Load(), c.storeErrors.Load(),
		c.corrupt.Load(), c.lockWaits.Load(), c.lockLeaks.Load(), c.runs.Load(), c.pruned.Load(),
		c.pchBuilds.Load(), c.pchUses.Load(), c.pchRejects.Load(), c.pchRechecks.Load(), c.typeRechecks.Load()}
}

// ResetCacheCounters sets the counters to zero.
func ResetCacheCounters() {
	c := &cacheCount
	all := []*atomic.Int64{
		&c.memHits, &c.diskHits, &c.misses, &c.stores, &c.storeErrors, &c.corrupt, &c.lockWaits, &c.lockLeaks,
		&c.runs, &c.pruned, &c.pchBuilds, &c.pchUses, &c.pchRejects, &c.pchRechecks, &c.typeRechecks,
	}
	for _, a := range all {
		a.Store(0)
	}
}

// ---------------------------------------------------------------- files

// checksum is the first line of an entry file.
func checksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// entryName returns the name of the file of the entry of kind (c, u, d) with the key
// parts, as a path under the cache folder.
func entryName(kind string, parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:%s\x00", len(p), p)
	}
	hex := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(kind, hex[:2], hex[2:]+".json")
}

// readEntry reads the body of the entry file path, or reports false for a file that is not there,
// is too big, is cut off or garbled.
func readEntry(path string) ([]byte, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if !fi.Mode().IsRegular() || fi.Size() > maxEntry {
		cacheCount.corrupt.Add(1)
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	sum, body, ok := bytes.Cut(data, []byte("\n"))
	if !ok || string(sum) != checksum(body) {
		cacheCount.corrupt.Add(1)
		return nil, false
	}
	if time.Since(fi.ModTime()) > touchAge {
		now := time.Now()
		os.Chtimes(path, now, now) // best effort: the order of eviction
	}
	return body, true
}

// writeDown remembers the folders that could not be written, so that a read-only cache does
// not cost an attempt per entry.
var writeDown sync.Map // folder -> time of the failure

const writeRetry = time.Minute

// writeEntry writes the entry file path atomically. It reports whether it did.
func writeEntry(path string, body []byte) bool {
	folder := filepath.Dir(path)
	root := folder
	for i := 0; i < 2; i++ {
		root = filepath.Dir(root)
	}
	if t, ok := writeDown.Load(root); ok && time.Since(t.(time.Time)) < writeRetry {
		return false
	}
	// A folder that cannot be made or written to (read-only, full) stops the writes for a
	// minute; a write that fails later (a rename onto a file that another process has open, on
	// Windows) is only lost.
	down := func() bool {
		writeDown.Store(root, time.Now())
		cacheCount.storeErrors.Add(1)
		return false
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return down()
	}
	tmp, err := os.CreateTemp(folder, ".tmp-*")
	if err != nil {
		return down()
	}
	_, err = tmp.Write(slices.Concat([]byte(checksum(body)), []byte("\n"), body))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		cacheCount.storeErrors.Add(1)
		return false
	}
	cacheCount.stores.Add(1)
	return true
}

// ---------------------------------------------------------------- the layers

// A store is what a Runner keeps in memory of the entries it read or made; the files on disk
// are the second layer.
type store struct {
	mu    sync.Mutex
	mem   map[string][]byte
	bytes int64
}

// maxMem bounds a Runner's memory of entries.
const maxMem = 256 << 20

func (rn *Runner) get(dir, name string) ([]byte, bool) {
	key := dir + "\x00" + name
	rn.st.mu.Lock()
	b, ok := rn.st.mem[key]
	rn.st.mu.Unlock()
	if ok {
		cacheCount.memHits.Add(1)
		return b, true
	}
	if dir == "" {
		return nil, false
	}
	b, ok = readEntry(filepath.Join(dir, name))
	if ok {
		cacheCount.diskHits.Add(1)
		rn.remember(key, b)
	}
	return b, ok
}

func (rn *Runner) remember(key string, body []byte) {
	rn.st.mu.Lock()
	defer rn.st.mu.Unlock()
	if rn.st.mem == nil {
		rn.st.mem = make(map[string][]byte)
	}
	if old, ok := rn.st.mem[key]; ok {
		rn.st.bytes -= int64(len(old))
	} else if rn.st.bytes+int64(len(body)) > maxMem {
		return
	}
	rn.st.mem[key] = body
	rn.st.bytes += int64(len(body))
}

// put keeps the entry in memory and, when toDisk and there is a folder, on disk.
func (rn *Runner) put(dir, name string, body []byte, toDisk bool) {
	rn.remember(dir+"\x00"+name, body)
	if dir == "" || !toDisk {
		return
	}
	if writeEntry(filepath.Join(dir, name), body) && cacheCount.stores.Load()%pruneEvery == 1 {
		maybePrune(dir, DefaultLimits, time.Now())
	}
}

// maybePrune prunes unless some process did within pruneMarkerAge (a walk of a big cache folder
// costs a second): the marker file's time says when.
func maybePrune(dir string, lim Limits, now time.Time) {
	marker := filepath.Join(dir, ".pruned")
	if fi, err := os.Stat(marker); err == nil && now.Sub(fi.ModTime()) >= 0 && now.Sub(fi.ModTime()) < pruneMarkerAge {
		return
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		return
	}
	prune(dir, lim, now)
}

// ---------------------------------------------------------------- locks

// A lock says that a process is running a compiler for a key: another one that needs the
// same answer waits for it, instead of running the same compiler again.

// lockWait is the longest that a process waits for another's run (the run's own limit, and a
// margin); a lock older than that belongs to a process that was killed.
func lockWait(timeout time.Duration) time.Duration {
	w := timeout + lockMargin
	if lockWaitCap > 0 && w > lockWaitCap {
		w = lockWaitCap
	}
	return w
}

// lockWaitCap, when it is not 0, is the longest wait for a lock (the tests make a lock that is never let
// go fail in seconds instead of after the time limit of a run).
var lockWaitCap time.Duration

// hostName is the name of this machine for the lock files (the process ids are only
// meaningful where they were made).
func hostName() string {
	h, err := os.Hostname()
	if err != nil || strings.ContainsAny(h, " \n") {
		return "unknown"
	}
	return h
}

// deadHolder reports whether the lock file was made by a process of this machine that is not
// running any more (killed): its lock is taken over at once instead of after the time limit.
func deadHolder(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var host string
	var pid int
	if n, _ := fmt.Sscanf(string(b), "%s %d", &host, &pid); n != 2 || host != hostName() || pid <= 0 {
		return false
	}
	return !pidAlive(pid)
}

// lockFile returns the lock file of the key.
func lockFile(dir, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "locks", hex.EncodeToString(sum[:16])+".lock")
}

// removeFile removes a file (os.Remove; a variable for the tests).
var removeFile = os.Remove

// lockRemoveTries and lockRemoveDelay bound the attempts to remove a lock file that fails for another reason
// than its absence.
const (
	lockRemoveTries = 200
	lockRemoveDelay = 5 * time.Millisecond
)

// removeLock removes the lock file at the release of a lock. On Windows a file that another process (a waiter
// that reads who holds it) has open cannot be deleted for a moment ("being used by another process"), so
// the removal is tried again for a second; a lock that stays is counted (cacheCount.lockLeaks) and is
// taken over by the next waiter as a stale one after the time limit.
func removeLock(path string) {
	for i := 0; i < lockRemoveTries; i++ {
		err := removeFile(path)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return
		}
		time.Sleep(lockRemoveDelay)
	}
	cacheCount.lockLeaks.Add(1)
}

// acquireLock makes the lock of key. It returns release, a function that removes it, and
// waited, which says that another process held the lock and let go (or the wait ran out, or
// the context ended): then the result may be there now, and the caller looks again before it
// runs anything. release is nil when there is no lock (no folder, or it cannot be written).
func acquireLock(ctx context.Context, dir, key string, timeout time.Duration) (release func(), waited bool) {
	if dir == "" {
		return nil, false
	}
	path := lockFile(dir, key)
	deadline := time.Now().Add(lockWait(timeout))
	delay := 5 * time.Millisecond
	for tries := 0; ; {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintf(f, "%s %d\n", hostName(), os.Getpid())
			f.Close()
			return func() { removeLock(path) }, waited
		}
		if errors.Is(err, fs.ErrNotExist) && tries < 2 {
			tries++
			if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
				return nil, waited // not writable: no lock, no wait
			}
			continue
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, waited // not writable: no lock, no wait
		}
		// Held by another process.
		if !waited {
			cacheCount.lockWaits.Add(1)
		}
		waited = true
		fi, serr := os.Stat(path)
		if serr != nil {
			continue // released meanwhile: take it
		}
		if time.Since(fi.ModTime()) > lockWait(timeout) || deadHolder(path) {
			removeLock(path) // a killed process's lock
			continue
		}
		if time.Now().After(deadline) {
			return nil, true
		}
		select {
		case <-ctx.Done():
			return nil, true
		case <-time.After(delay):
		}
		if delay < 100*time.Millisecond {
			delay *= 2
		}
	}
}

// ---------------------------------------------------------------- pruning, usage, cleaning

// A walked file is one entry of the cache folder.
type walked struct {
	path string
	size int64
	mod  time.Time
}

// walk lists the entry files of the cache folder dir; it removes the temporary files and the
// locks that a killed process left.
func walk(dir string, now time.Time) []walked {
	var out []walked
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p == filepath.Join(dir, "pch") {
			return fs.SkipDir // (the precompiled headers go by their own count: prunePCH)
		}
		if err != nil || d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		name := d.Name()
		switch {
		case strings.HasPrefix(name, ".tmp-"):
			if now.Sub(fi.ModTime()) > staleTemp {
				os.Remove(p)
			}
		case strings.HasSuffix(name, ".lock"):
			if now.Sub(fi.ModTime()) > lockWait(DefaultTimeout)+time.Minute {
				os.Remove(p)
			}
		case strings.HasSuffix(name, ".json"):
			out = append(out, walked{p, fi.Size(), fi.ModTime()})
		}
		return nil
	})
	return out
}

// prune removes the entries of the cache folder dir that were not used for lim.MaxAge, then
// those used least recently while there are more than lim.MaxEntries or lim.MaxBytes. It returns
// the number it removed.
func prune(dir string, lim Limits, now time.Time) int {
	files := walk(dir, now)
	removed := 0
	var total int64
	live := files[:0]
	for _, f := range files {
		if lim.MaxAge > 0 && now.Sub(f.mod) > lim.MaxAge {
			os.Remove(f.path)
			removed++
			continue
		}
		live = append(live, f)
		total += f.size
	}
	slices.SortFunc(live, func(a, b walked) int {
		if c := a.mod.Compare(b.mod); c != 0 {
			return c
		}
		return strings.Compare(a.path, b.path)
	})
	n := len(live)
	for _, f := range live {
		if (lim.MaxEntries <= 0 || n <= lim.MaxEntries) && (lim.MaxBytes <= 0 || total <= lim.MaxBytes) {
			break
		}
		os.Remove(f.path)
		removed++
		n--
		total -= f.size
	}
	cacheCount.pruned.Add(int64(removed))
	return removed
}

// Prune applies the bounds lim to the cache folder dir ("" for the environment's). It returns the number of
// entries removed.
func Prune(dir string, lim Limits) int {
	if dir == "" {
		if dir = CacheDir(); dir == "" {
			return 0
		}
	}
	return prune(dir, lim, time.Now())
}

// A Usage is what the cache folder holds.
type Usage struct {
	Dir     string           `json:"dir"`
	Entries int              `json:"entries"`
	Bytes   int64            `json:"bytes"`
	Kinds   map[string]int   `json:"kinds"` // contexts (c), units (u), declaration parts (d)
	Oldest  time.Time        `json:"oldest,omitzero"`
	Newest  time.Time        `json:"newest,omitzero"`
	Limits  Limits           `json:"limits"`
	Process Counters         `json:"process"`
	Extra   map[string]int64 `json:"extra,omitempty"`
}

// CacheUsage describes the cache folder dir ("" for the environment's).
func CacheUsage(dir string) Usage {
	if dir == "" {
		dir = CacheDir()
	}
	u := Usage{Dir: dir, Kinds: map[string]int{}, Limits: DefaultLimits, Process: CacheCounters()}
	if dir == "" {
		return u
	}
	for _, f := range walk(dir, time.Now()) {
		u.Entries++
		u.Bytes += f.size
		if rel, err := filepath.Rel(dir, f.path); err == nil {
			u.Kinds[strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]]++
		}
		if u.Oldest.IsZero() || f.mod.Before(u.Oldest) {
			u.Oldest = f.mod
		}
		if f.mod.After(u.Newest) {
			u.Newest = f.mod
		}
	}
	return u
}

// CleanCache removes the typed probe cache folder dir ("" for the environment's) and returns
// what it held. Only the folder of this format is touched: the kind probe's files next to it
// stay.
func CleanCache(dir string) (Usage, error) {
	if dir == "" {
		dir = CacheDir()
	}
	u := CacheUsage(dir)
	if dir == "" {
		return u, nil
	}
	if filepath.Base(dir) != cacheSub || !filepath.IsAbs(dir) {
		return u, fmt.Errorf("refusing to remove %s: not a typed probe cache folder", dir)
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return u, nil
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return u, os.Remove(dir) // a link is removed as a link
	}
	return u, os.RemoveAll(dir)
}
