package interop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/version"
)

// The probe results on disk (Phase 06 README, "Probe results on disk").
//
// The cache of cache.go is the first level and lives as long as the
// process. This is the second: a gocpp process that starts (every `gocpp
// emit`, `check` and `build`, and the editor after a restart) finds the
// answers of the C++ compiler that an earlier one wrote, as long as
// nothing that they depend on changed. The compiler takes about a second
// for the standard headers; reading an entry takes a few milliseconds.
//
// One file per (probe, key) in <user cache>/gocpp/probe/, named
// <probe>-<hash of the key>.json; the probes are k (the kinds of paths, the
// namespace probe's answers too), m (the macros) and c (the C header
// check). What an entry holds:
//
//   - the key in full (it is hashed for the name; the full text is compared
//     on load, so a hash collision is a miss);
//   - the files that the result depends on, with the content hash and the
//     stat of each, exactly as the memory cache keeps them (fileState);
//   - the result.
//
// Key: what can change an answer. A format version and gocpp itself (its
// version and the executable's size and modification time, because the
// probe's code decides the probe file and the compiler's flags); the
// compiler (path, arguments, family, version and the identity of its
// program file, as for the runtime's object files); the folder of the quote
// includes (the working folder when there is none), the flags and the
// includes (setKey); the environment that
// changes include paths and flags (build.KeyEnv); and, for a compiler that
// has arguments (a launcher such as ccache, which looks its compiler up in
// PATH), PATH.
//
// Validation: a loaded entry is used only when the memory cache would
// still use it: every file is checked with fileState.unchanged (stat, and
// the content hash when the stat differs), a missing file is still
// missing. An entry is written only when the memory cache would keep the
// result: no file changed within racyWindow before the probe started, and
// the probe wrote a dependency list. The list holds all the headers that
// the compiler read, the system headers too (the probes pass -MD): a
// system header can change without the compiler changing (a library that
// is updated in place), and that is what the list covers.
//
// Results with include errors are not written: they stay in memory for
// errorTTL, because something outside the list (a library that is
// installed) can fix them; an include error is a program that is being
// fixed, not one that is built a hundred times. For the same reason a C
// header check that came from a header that wasn't found is not written
// (the places that it watches are only some of the places where the header
// can appear), nor is one without a dependency list (it expires).
//
// Limits that no list can cover: a header that appears earlier on the
// search path than the one that was found (a new file in front of the
// found one, in a folder that isn't the package folder), and one that
// __has_include asks for. So an entry expires after maxDiskAge (the entry
// that merges new paths keeps the age of its oldest result), and a result
// that depends on the host's CPU (-march=native and the like, in the
// flags, the compiler's arguments or the environment) is not written at
// all: two machines can share one cache folder (a home folder over NFS).
//
// The key of a compiler with arguments (a launcher such as `ccache g++`)
// has the PATH text and the launcher's own identity, not that of the
// compiler that it starts.
//
// Safe for any number of processes: an entry is written to a temporary
// file in the same folder and renamed into place (a reader sees the old
// entry or the new one, never half of one); two writers of one key leave
// one of the two entries, and the loser's work is a miss later, not a
// wrong answer. An entry that can't be read, doesn't parse, has a wrong
// checksum or format, or another key is a miss, and is overwritten by the
// next probe. Nothing here is fatal: when the folder can't be made or
// written, the cache goes without it.
//
// The size is bounded: at most maxDiskEntries files and maxDiskBytes in
// all; when a write goes over, the entries that were used least recently
// (by modification time, which a hit refreshes once in diskTouch) go.

const (
	// diskFormat is the version of the entry format and of the meaning of
	// the keys. Raise it when either changes: older entries are ignored.
	diskFormat = 2

	// maxDiskAge is how old an entry can be and still be used. The list of
	// files that an entry depends on can't see a header that appears
	// earlier on the search path than the one that was found (or one that
	// a __has_include asks for), nor a header that appears in one of the
	// compiler's own folders after a "not found" is recorded; the memory
	// cache has that limit for the life of a process, the disk would have
	// it for good. So an entry expires.
	maxDiskAge = time.Hour

	maxDiskEntries = 512
	maxDiskBytes   = 128 << 20

	// maxDiskEntry is the size of the largest entry that is read: the
	// macro list of <windows.h> is about 25,000 names, 1 MiB of JSON.
	maxDiskEntry = 32 << 20

	// diskTouch is how old an entry is when a hit refreshes its
	// modification time (eviction's order of use).
	diskTouch = 10 * time.Minute

	// staleTemp is the age of a temporary file that eviction removes: a
	// process that was killed while it wrote one leaves it behind.
	staleTemp = time.Hour
)

// The environment variables of the probe cache on disk.
const (
	// EnvNoProbeCache turns the cache on disk off when it is set to
	// anything but "", "0" or "false": the probes always run the compiler
	// (the memory cache stays).
	EnvNoProbeCache = "GOCPP_NOPROBECACHE"

	// EnvProbeCache is the folder of the cache on disk (an absolute name)
	// instead of gocpp/probe in the user's cache folder. A test binary
	// uses the cache on disk only when it is set, so that a test never
	// writes the user's cache.
	EnvProbeCache = "GOCPP_PROBE_CACHE"
)

// A diskStore is the cache on disk of a cache. Its zero value looks the
// folder up in the environment (probeDiskDir); a store with dir set uses
// that folder, whatever the environment says (tests).
type diskStore struct {
	dir        string
	maxEntries int   // maxDiskEntries when 0
	maxBytes   int64 // maxDiskBytes when 0
}

// probeDiskDir returns the folder of the cache on disk that the
// environment asks for, or "" for no cache on disk. It looks at the
// environment each time, not once at the start: the variables can change
// while a test runs.
func probeDiskDir() string {
	if v := os.Getenv(EnvNoProbeCache); v != "" && v != "0" && v != "false" {
		return ""
	}
	if d := os.Getenv(EnvProbeCache); d != "" {
		if !filepath.IsAbs(d) {
			return ""
		}
		return d
	}
	if testing.Testing() {
		return ""
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "gocpp", "probe")
}

func (d *diskStore) folder() string {
	if d == nil {
		return ""
	}
	if d.dir != "" {
		return d.dir
	}
	return probeDiskDir()
}

// selfID identifies the running gocpp for the keys: its version and the
// real path (a Nix store path has a hash of the build in it), its size and
// modification time (a "dev" build changes its probe code without changing
// its version; a store file has the time 1), and the build's version
// control stamp when there is one. It is "" when the executable can't be
// looked at.
var selfID = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	id := fmt.Sprintf("%s\x00%s\x00%d\x00%d", version.Version, exe, fi.Size(), fi.ModTime().UnixNano())
	if bi, ok := debug.ReadBuildInfo(); ok {
		id += "\x00" + bi.GoVersion
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" || s.Key == "vcs.modified" {
				id += "\x00" + s.Key + "=" + s.Value
			}
		}
	}
	return id
})

// nativeFlag reports whether s asks the compiler for the host's CPU
// (-march=native, ...): then the macros (and so the answers) depend on the
// machine, and a cache folder that two machines share (a home folder over
// NFS) would serve one machine the other's answer.
func nativeFlag(s string) bool {
	return strings.Contains(s, "-march=native") || strings.Contains(s, "-mtune=native") || strings.Contains(s, "-mcpu=native")
}

// A diskKey names an entry.
type diskKey struct {
	path string // the entry's file
	full string // the key in full
	kind string // "k", "m" or "c"
}

// keyFor returns the key of the entry of the probe kind (k, m or c) for the
// include set (cxx, dir, flags, includes), or false when there is no cache
// on disk, or the key can't be made: the compiler's program file can't be
// looked at, or a part of the key isn't text (an entry is JSON).
func (d *diskStore) keyFor(kind string, cxx build.CXX, dir string, flags, includes []string) (diskKey, bool) {
	folder := d.folder()
	self := selfID()
	if folder == "" || self == "" {
		return diskKey{}, false
	}
	id, ok := cxx.Identity()
	if !ok {
		return diskKey{}, false
	}
	// Results that depend on the host's CPU stay in memory.
	env := build.KeyEnv()
	for _, list := range [][]string{flags, cxx.Args, env} {
		if slices.ContainsFunc(list, nativeFlag) {
			return diskKey{}, false
		}
	}
	var b strings.Builder
	part := func(parts ...string) {
		for _, p := range parts {
			fmt.Fprintf(&b, "%d:%s\x00", len(p), p)
		}
		b.WriteByte('\n')
	}
	part("gocpp-probe", fmt.Sprint(diskFormat), kind)
	part(self)
	part(id)
	part(setKey(cxx, dir, flags, includes))
	if dir == "" {
		// The compiler runs where the process is: relative names (a quoted
		// include, a relative -I) are found from there.
		wd, err := os.Getwd()
		if err != nil {
			return diskKey{}, false
		}
		part(wd)
	}
	part(env...)
	if len(cxx.Args) > 0 {
		// A launcher (ccache g++) looks its compiler up in PATH: the key has
		// the PATH text and the launcher's own identity, not the identity
		// of the compiler that it starts.
		part(os.Getenv("PATH"))
	}
	full := b.String()
	if !utf8.ValidString(full) {
		return diskKey{}, false
	}
	sum := sha256.Sum256([]byte(full))
	return diskKey{path: filepath.Join(folder, kind+"-"+hex.EncodeToString(sum[:16])+".json"), full: full, kind: kind}, true
}

// A diskEntry is the content of an entry file.
type diskEntry struct {
	Format int    `json:"format"`
	Kind   string `json:"kind"`
	Key    string `json:"key"`
	// Created is when the result was made (UnixNano), for maxDiskAge; for
	// an entry that merges results, when the oldest of them was.
	Created int64      `json:"created"`
	Files   []diskFile `json:"files"`

	Kinds map[string]int `json:"kinds"` // k: kind per path (also none: {})

	Objects []string `json:"objects,omitempty"` // m: object-like macros, sorted
	Funcs   []string `json:"funcs,omitempty"`   // m: function-like macros, sorted

	C     bool `json:"c,omitempty"`     // c: CHeader.C
	Guard bool `json:"guard,omitempty"` // c: CHeader.Guard
}

// A diskFile is a fileState.
type diskFile struct {
	Path    string   `json:"path"`
	Exists  bool     `json:"exists"`
	Hash    string   `json:"hash,omitempty"`
	Size    int64    `json:"size,omitempty"`
	ModTime int64    `json:"mtime,omitempty"`
	Sys     [2]int64 `json:"sys,omitempty"`
}

func diskFiles(files []fileState) ([]diskFile, bool) {
	out := make([]diskFile, len(files))
	for i, f := range files {
		if !utf8.ValidString(f.path) {
			return nil, false
		}
		df := diskFile{Path: f.path, Exists: f.exists}
		if f.exists {
			df.Hash = hex.EncodeToString(f.hash[:])
			df.Size, df.ModTime, df.Sys = f.stat.size, f.stat.modTime, f.stat.sys.words()
		}
		out[i] = df
	}
	return out, true
}

func (e *diskEntry) fileStates() ([]fileState, bool) {
	files := make([]fileState, len(e.Files))
	for i, df := range e.Files {
		f := fileState{path: df.Path, exists: df.Exists}
		if df.Path == "" {
			return nil, false
		}
		if df.Exists {
			h, err := hex.DecodeString(df.Hash)
			if err != nil || len(h) != len(f.hash) {
				return nil, false
			}
			copy(f.hash[:], h)
			f.stat = fileStat{size: df.Size, modTime: df.ModTime, sys: sysStatFrom(df.Sys)}
		}
		files[i] = f
	}
	return files, true
}

// checksum is the first line of an entry file: the hash of the rest, so
// that a file that was cut off or changed is a miss.
func checksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// load reads the entry of key. It reports false (a miss) for anything
// that is not an entry that was written for this key by this format.
func (d *diskStore) load(key diskKey) (*diskEntry, bool) {
	return d.loadAt(key, time.Now())
}

func (d *diskStore) loadAt(key diskKey, now time.Time) (*diskEntry, bool) {
	fi, err := os.Stat(key.path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxDiskEntry {
		return nil, false
	}
	data, err := os.ReadFile(key.path)
	if err != nil {
		return nil, false
	}
	sum, body, ok := bytes.Cut(data, []byte("\n"))
	if !ok {
		return nil, false
	}
	if string(sum) != checksum(body) {
		return nil, false
	}
	var e diskEntry
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, false
	}
	if e.Format != diskFormat || e.Kind != key.kind || e.Key != key.full {
		return nil, false
	}
	if age := now.Sub(time.Unix(0, e.Created)); age > maxDiskAge || age < -time.Minute {
		return nil, false
	}
	if time.Since(fi.ModTime()) > diskTouch {
		now := time.Now()
		os.Chtimes(key.path, now, now) // best effort: the order of eviction
	}
	return &e, true
}

// store writes the entry of key, and evicts the oldest entries when there
// are too many. It is silent about errors.
func (d *diskStore) store(key diskKey, e *diskEntry) {
	e.Format, e.Kind, e.Key = diskFormat, key.kind, key.full
	if e.Created == 0 {
		e.Created = time.Now().UnixNano()
	}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	folder := filepath.Dir(key.path)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(folder, ".tmp-*")
	if err != nil {
		return
	}
	_, err = tmp.Write(slices.Concat([]byte(checksum(body)), []byte("\n"), body))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), key.path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return
	}
	d.evict(folder, filepath.Base(key.path))
}

// evict removes the entries of folder that were used least recently while
// there are more than maxEntries of them or they have more than maxBytes,
// but never keep, the entry that was just written; and the temporary files
// that a killed process left.
func (d *diskStore) evict(folder, keep string) {
	maxEntries, maxBytes := d.maxEntries, d.maxBytes
	if maxEntries <= 0 {
		maxEntries = maxDiskEntries
	}
	if maxBytes <= 0 {
		maxBytes = maxDiskBytes
	}
	des, err := os.ReadDir(folder)
	if err != nil {
		return
	}
	type entry struct {
		name string
		size int64
		mod  time.Time
	}
	var entries []entry
	var total int64
	for _, de := range des {
		name := de.Name()
		fi, err := de.Info()
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		switch {
		case strings.HasPrefix(name, ".tmp-"):
			if time.Since(fi.ModTime()) > staleTemp {
				os.Remove(filepath.Join(folder, name))
			}
		case strings.HasSuffix(name, ".json"):
			entries = append(entries, entry{name, fi.Size(), fi.ModTime()})
			total += fi.Size()
		}
	}
	if len(entries) <= maxEntries && total <= maxBytes {
		return
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if c := a.mod.Compare(b.mod); c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	})
	n := len(entries)
	for _, e := range entries {
		if n <= maxEntries && total <= maxBytes {
			break
		}
		if e.name == keep {
			continue
		}
		os.Remove(filepath.Join(folder, e.name))
		n--
		total -= e.size
	}
}

// loadKinds fills set from the disk, when there is an entry for the
// include set that is still valid. It reports whether it did.
func (c *cache) loadKinds(key diskKey, set *kindSet) bool {
	e, ok := c.disk.load(key)
	if !ok {
		return false
	}
	files, ok := e.fileStates()
	if !ok || e.Kinds == nil {
		return false
	}
	kinds := make(map[string]Kind, len(e.Kinds))
	for name, k := range e.Kinds {
		if k <= int(Unknown) || Kind(k) > NotNamespace {
			return false
		}
		for part := range strings.SplitSeq(name, "::") {
			if !isCppName(part) {
				return false
			}
		}
		kinds[name] = Kind(k)
	}
	if !c.valid(files, false, time.Time{}) {
		return false
	}
	*set = kindSet{filled: true, files: files, kinds: kinds, born: time.Unix(0, e.Created)}
	c.diskHits.Add(1)
	return true
}

// storeKinds writes the result of set to the disk. A set with include
// errors is not written.
func (c *cache) storeKinds(key diskKey, set *kindSet) {
	files, ok := diskFiles(set.files)
	if !ok || !set.filled || set.incErrs != nil {
		return
	}
	e := &diskEntry{Files: files, Kinds: make(map[string]int, len(set.kinds))}
	if !set.born.IsZero() {
		e.Created = set.born.UnixNano()
	}
	for name, k := range set.kinds {
		if k == Unknown {
			return // not a kind that a later process can use
		}
		e.Kinds[name] = int(k)
	}
	c.disk.store(key, e)
	c.diskStores.Add(1)
}

// loadMacros returns the macros of the disk entry of key, when it is valid.
func (c *cache) loadMacros(key diskKey) (*macroSet, bool) {
	e, ok := c.disk.load(key)
	if !ok {
		return nil, false
	}
	files, ok := e.fileStates()
	if !ok {
		return nil, false
	}
	m := make(map[string]MacroKind, len(e.Objects)+len(e.Funcs))
	for _, n := range e.Objects {
		if !isCppName(n) {
			return nil, false
		}
		m[n] = ObjectLike
	}
	for _, n := range e.Funcs {
		if !isCppName(n) {
			return nil, false
		}
		m[n] = FunctionLike
	}
	if len(m) == 0 || len(m) != len(e.Objects)+len(e.Funcs) {
		return nil, false // a probe's result is never empty (parseMacros)
	}
	if !c.valid(files, false, time.Time{}) {
		return nil, false
	}
	c.diskHits.Add(1)
	return &macroSet{filled: true, files: files, macros: Macros{m: m}}, true
}

func (c *cache) storeMacros(key diskKey, set *macroSet) {
	files, ok := diskFiles(set.files)
	if !ok || !set.filled {
		return
	}
	e := &diskEntry{Files: files}
	for _, n := range set.macros.Names() {
		if set.macros.m[n] == FunctionLike {
			e.Funcs = append(e.Funcs, n)
		} else {
			e.Objects = append(e.Objects, n)
		}
	}
	c.disk.store(key, e)
	c.diskStores.Add(1)
}

// loadCHeader returns the answer of the disk entry of key, when it is valid.
func (c *cache) loadCHeader(key diskKey) (*cHeaderSet, bool) {
	e, ok := c.disk.load(key)
	if !ok {
		return nil, false
	}
	files, ok := e.fileStates()
	if !ok {
		return nil, false
	}
	if !c.valid(files, false, time.Time{}) {
		return nil, false
	}
	c.diskHits.Add(1)
	return &cHeaderSet{filled: true, files: files, res: CHeader{C: e.C, Guard: e.Guard}}, true
}

// storeCHeader writes the answer of set to the disk, unless it expires.
func (c *cache) storeCHeader(key diskKey, set *cHeaderSet) {
	files, ok := diskFiles(set.files)
	if !ok || !set.filled || !set.expires.IsZero() {
		return
	}
	c.disk.store(key, &diskEntry{Files: files, C: set.res.C, Guard: set.res.Guard})
	c.diskStores.Add(1)
}
