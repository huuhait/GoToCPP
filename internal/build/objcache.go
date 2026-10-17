package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The object cache (Phase 06 README §2.5, "Build graph and cache"): a content-addressed cache
// of the object files that the C++ compiler makes from the generated C++ (and from a package's
// own C and C++ files), like Go's build cache. A compile whose inputs are all unchanged is not
// run again: `gocpp build` of an unchanged program only links, and test suites that build the
// same program again and again skip the compiler.
//
// It is two levels, like ccache's direct mode:
//
//  1. A manifest, <cache>/obj/<key>.man, where the key is a hash of what is known before the
//     compiler runs: the text of the translation unit, the compile flags (without the paths of
//     this build: the work folder, -o), the compiler (path, arguments, version, the identity
//     of its program file), the environment that changes what it does (keyEnv, and the
//     compiler wrapper's variables), and the cache's format version. The manifest lists up to
//     objManifestEntries results. Each one names the object it made and the set of files the
//     compiler really read the last time (the dependency file of -MD: the headers, system ones
//     too), <set>.deps, with a hash and the stat of each. The set is a file of its own because
//     programs that read the same headers share it.
//  2. The object, <cache>/obj/<hash>.o, named by the SHA-256 of its bytes (so a file that
//     is there already is checked, never trusted): a lookup is a hit when every file of one
//     entry still has its content. A file whose size, times, inode and change time are as
//     recorded counts as unchanged without being read; otherwise it is hashed, and when it
//     has the recorded content the set is saved again with its new stat.
//
// A miss compiles with -MD -MF added and stores the result (while the link runs). A result is
// stored only when the compile succeeded and the compiler printed nothing (a warning must not
// vanish on a hit), and not when:
//   - the translation unit, or a header it read, uses __DATE__, __TIME__ or __TIMESTAMP__ (ccache's
//     rule);
//   - the translation unit, or a header of the user (not the system's, the compiler's or the
//     runtime's: libstdc++ uses it), uses __has_include or __has_include_next: the dependency
//     file doesn't list a header that was not found, so a later file of that name would not be
//     noticed;
//   - the flags make the compiler write more than the object, or read more than the depfile
//     says (uncacheableFlags: -M*, -save-temps, -Wp, -specs, -fdump-*, -fstack-usage,
//     -fopt-info, -gsplit-dwarf, -fprofile-*, --coverage, -fplugin, -E, -S, ...);
//   - a file it read, or the source file of the user, changed within racyWindow (two seconds)
//     before the compile started (the same rule as the namespace probe's cache, internal/interop):
//     a later edit could keep the modification time;
//   - the source file is not what the compiler read (it was edited between the key and the
//     compile);
//   - it read a file in the work folder other than the translation unit;
//   - the build keeps its work folder (-work, Builder.KeepWork): then the cache is not used at
//     all, so the files of the work folder have real paths in their debug info.
//
// The user's flags (GOCPP_CFLAGS, GOCPP_CXXFLAGS, the package's //gocpp: flags) are part of the key like
// the others: a compile with them is cached too. The flags of the precompiled header
// (-include-pch, -include) are not part of the key: the object is the same with and without
// it, so the key is that of the plain compile, and a stale header falls back to a compile
// without it (Program) as before. g++ lists none of the headers that a precompiled header
// replaced, so the header's own list (pch.go saveDeps, with the hashes of the files when it
// was built) is added to the entry; a file of it that has changed since is not stored.
//
// The work folder is random, and ends up in the object (the compiler's working folder and the
// name of the file in the debug info, __FILE__) and in the text of the translation unit (the
// driver's `#line N "$WORK/x.cpp"` back to the C++ after a block that maps to .gp lines). The
// key sees the text with the work folder written as $WORK, and a compile that can be stored
// gets -ffile-prefix-map=$WORK=/gocpp-work, so that the object is the same for any work
// folder. (The `#line` directives that point at .gp files are in the text and stay right.)
//
// Many gocpp commands use the folder at the same time (the test harness): every file is written
// to a temporary name and renamed, a manifest written by two at once keeps one of the two
// results, and anything that does not read back as written (a manifest that is not JSON, an
// object with another size or hash) is a miss. A folder that can't be created or written
// means no cache, without a message. GOCPP_NOOBJCACHE (any value except "", 0 and false)
// turns the cache off, and GOCPP_OBJCACHE_SIZE (bytes, or with a K, M or G suffix) sets its size
// limit, 2 GiB by default: after a store, at most once in objTrimEvery, the oldest files go
// until it is 90% full. The limit is soft: the folder can be bigger between two trims, and
// several gocpp commands that trim at once may remove a little more or less than asked. A hit
// touches the files it used, once in an hour, so "oldest" is "least recently used".
// `gocpp clean -cache` (Phase 06) empties <cache>/obj with the rest of the cache.
//
// Limits (ccache has them too): a header that appears earlier on the include path than the one
// the compiler found would win now. It is noticed when the new file is in a folder of -I,
// -iquote, -isystem or -idirafter that comes before the folder of the header that was found,
// or in the folder of another file of the user (a new x.h next to the including file wins
// over the one found elsewhere): the entry lists the paths that must stay absent. It is not
// noticed when a system header is shadowed by a new file of an -I folder, or when a new file
// appears earlier in CPATH, CPLUS_INCLUDE_PATH or the compiler's own include folders, which
// the dependency file can't tell. The runtime's include folder is a folder of the cache named
// by the hash of its files, so for programs this doesn't arise. A header changed with the same
// size, times, inode and change time (a file replaced by hand with the times set back) is not
// noticed. The compiler's own files other than its program (cc1plus behind a wrapper) are
// known by the version line only. The __has_include scan goes by where a file is (userFile):
// the user's are the files in a folder of -I, -iquote, -isystem or -idirafter (so third-party
// headers under -I are scanned, and one that probes makes the compile uncacheable), and the
// files outside the system prefixes (/usr, /nix/store, /opt, ...); a project under /opt or
// /usr/local that includes ../shared/x.h, which is in no such folder, is taken for the
// system's, and a probe in it is not noticed. On Windows no prefix matches, so everything
// outside the include folders is the user's, and the standard library's own probes make
// compiles uncacheable. An entry may ask for at most maxAbsentPaths files to be absent (the
// product of the folders and the names, see absentGroup, 10000): a compile with more user
// headers and folders than that is not stored, and -x prints "# not cached: too many include
// candidates". The cause is the product: every folder that holds a header of the user times
// every header of the user that was found through an include folder (a 200-header library in
// 50 folders included through one -iquote root is 10000). TODO: check the folders instead of
// every name (stat each folder, list the ones that changed), which needs no product. Objects made with -march=native and the like
// (NativeArch, also in NIX_CFLAGS_COMPILE) are not stored or looked up: they are for one
// machine.

const (
	// objCacheVersion changes when the format of the cache or the meaning of a key changes.
	objCacheVersion = "3"

	// objManifestEntries is how many results one manifest keeps (the last ones): going back
	// to an older version of a header finds its object again.
	objManifestEntries = 8

	// objRacyWindow: see the comment above (racyWindow of internal/interop).
	objRacyWindow = 2 * time.Second

	// objTouchAge is how old a file used by a hit must be to get a new modification time, and
	// objTrimEvery how often a store checks the size of the cache.
	objTouchAge  = time.Hour
	objTrimEvery = 10 * time.Minute

	defaultObjCacheMax = 2 << 30

	// objPrefixMap is the folder that the work folder is mapped to in the objects.
	objPrefixMap = "/gocpp-work"

	maxDepfile = 32 << 20
)

// An objCache is the folder of the object cache, <cache>/obj.
type objCache struct {
	dir string
	max int64 // the size limit in bytes
}

// objects returns the object cache of the builder, or nil when there is none: no cache
// folder, GOCPP_NOOBJCACHE set, a build that keeps its work folder (-work), or a folder that
// can't be created.
func (b *Builder) objects() *objCache {
	if b.Cache == "" || b.KeepWork || objCacheOff() {
		return nil
	}
	dir := filepath.Join(b.Cache, "obj")
	if os.MkdirAll(dir, 0o700) != nil {
		return nil
	}
	return &objCache{dir: dir, max: objCacheMax()}
}

// objCacheOff reports whether GOCPP_NOOBJCACHE turns the cache off (EnvOff).
func objCacheOff() bool { return EnvOff("GOCPP_NOOBJCACHE") }

// objCacheMax returns the size limit: GOCPP_OBJCACHE_SIZE, or the default.
func objCacheMax() int64 {
	s := strings.ToUpper(strings.TrimSpace(os.Getenv("GOCPP_OBJCACHE_SIZE")))
	s = strings.TrimSuffix(s, "B")
	mult := int64(1)
	if n := len(s); n > 0 {
		switch s[n-1] {
		case 'K':
			mult, s = 1<<10, s[:n-1]
		case 'M':
			mult, s = 1<<20, s[:n-1]
		case 'G':
			mult, s = 1<<30, s[:n-1]
		}
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 && v <= (1<<62)/mult {
		return v * mult
	}
	return defaultObjCacheMax
}

// compileCached makes the object obj of the translation unit src with the compile args (the
// complete command line after the compiler's own arguments, with -c src -o obj, but without
// the precompiled header) through the cache: from it when an entry matches, else by run, which
// is called with args plus the flags that make the dependency file, and returns what the
// compiler printed. On a hit it returns no output and no error, and writes a "# cache hit"
// line to Trace. The compile that run does can be any command with the same effect (Program
// adds the precompiled header); extraDeps, if not nil, returns the file that lists what the
// precompiled header it used was made from (pch.go), or "".
func (b *Builder) compileCached(ctx context.Context, c CXX, src, obj string, args []string, run func(args []string) ([]byte, error), extraDeps func() string) ([]byte, error) {
	// A compile that succeeded but whose output may be incomplete (errOutputOpen) is a success
	// for the caller, but it is not stored: the silence that makes it cacheable is not certain.
	outputOpen := false
	run0 := run
	run = func(args []string) ([]byte, error) {
		out, err := run0(args)
		if errors.Is(err, errOutputOpen) {
			outputOpen = true
			err = nil
		}
		return out, err
	}
	oc := b.objects()
	if oc == nil {
		if b.KeepWork && b.Cache != "" && !objCacheOff() {
			b.notCached("-work keeps the work folder")
		}
		return run(args)
	}
	if f := uncacheableFlag(slices.Concat(c.Args, args)); f != "" {
		b.notCached("the flag " + f + " writes or reads files that the cache cannot see")
		return run(args)
	}
	// An object made for the host's CPU is not for another machine that shares the folder.
	if NativeArch(args...) || NativeArch(c.Args...) || NativeArch(KeyEnv()...) {
		b.notCached("an option asks for the host's CPU (-march=native)")
		return run(args)
	}
	text, err := os.ReadFile(src)
	switch {
	case err != nil:
		return run(args)
	case hasTimeMacro(text):
		b.notCached(filepath.Base(src) + " uses __DATE__, __TIME__ or __TIMESTAMP__")
		return run(args)
	case hasInclusionProbe(text):
		b.notCached(filepath.Base(src) + " uses __has_include")
		return run(args)
	}
	tu := text
	if b.Work != "" {
		tu = bytes.ReplaceAll(text, []byte(b.Work), []byte("$WORK"))
	}
	keyArgs := b.keyArgs(args)
	key := b.objKey(c, keyArgs, tu)
	if hash, ok := oc.lookup(key, obj); ok {
		b.trace("# cache hit " + filepath.Base(src) + " " + hash[:16])
		return nil, nil
	}
	// A miss: compile, and store the object when the compile was silent.
	depfile := obj + ".d"
	flags := []string{"-MD", "-MF", depfile}
	for _, w := range b.workNames() {
		flags = append(flags, "-ffile-prefix-map="+w+"="+objPrefixMap)
	}
	start := time.Now()
	out, err := run(slices.Concat(args, flags))
	if err != nil || len(out) > 0 || outputOpen || ctx.Err() != nil {
		switch {
		case err != nil:
		case len(out) > 0:
			b.notCached("the compiler printed something (a warning must not vanish)")
		case outputOpen:
			b.notCached("the compiler's output may be incomplete (it was still open after the compiler exited)")
		}
		return out, err
	}
	depfiles := []string{depfile}
	pchDeps := ""
	if extraDeps != nil {
		pchDeps = extraDeps()
	}
	// The store (hashing the headers, copying the object) runs while the link does; Program
	// waits for it.
	dirs := searchDirs(args)
	b.stores.Go(func() {
		// The source must still be the text that was compiled: an edit between the read
		// above and the compiler's read would store the new object under the old text's key.
		if now, err := os.ReadFile(src); err != nil || !bytes.Equal(now, text) {
			b.notCached(filepath.Base(src) + " changed while it was compiled")
			return
		}
		entry, err := b.recordDeps(depfiles, pchDeps, src, start, dirs, c)
		if err != nil {
			b.notCached(err.Error())
			return
		}
		oc.store(key, entry, obj)
	})
	return out, nil
}

// notCached writes a line to Trace that says why a compile is not kept in the cache, so that
// the silence is not a mystery (gocpp build -x).
func (b *Builder) notCached(why string) {
	b.trace("# not cached: " + why)
}

// workNames returns the names of the work folder that the compiler can show: Work, and with a
// symbolic link in it (/tmp) the real path, which is the compiler's working folder.
func (b *Builder) workNames() []string {
	if b.Work == "" {
		return nil
	}
	names := []string{b.Work}
	if real, err := filepath.EvalSymlinks(b.Work); err == nil && real != b.Work {
		names = append(names, real)
	}
	return names
}

// uncacheableFlags are the flags (as prefixes) that make the compiler write a file besides the
// object, or read one that the dependency file doesn't list, or print what it does. A flag
// of GOCPP_CXXFLAGS is trusted and unchecked, so the list is long.
var uncacheableFlags = []string{
	"-M",                          // -MD, -MMD, -MF, -MJ ...: ours must be the only dependency output
	"-save-temps", "--save-temps", // files next to the object
	"-Wp,", "-Xpreprocessor", "-Xclang", "-Wa,", "-Xassembler", // pass anything on to a tool
	"-specs", "--specs", // replaces the driver's rules
	"-gsplit-dwarf", "-fdump-", "-fstack-usage", "-fcallgraph-info", "-fopt-info", "-fsave-optimization-record",
	"-foptimization-record", "-fprofile-", "-fauto-profile", "--coverage", "-ftest-coverage", "-fprofile",
	"-fplugin", "-fcrash-diagnostics", "-fmodule", "-fmodules", "-fprebuilt-module-path", "-fpch-", "-include-pch",
	"-fexperimental-", "-emit-", "-fembed-bitcode", "-ftime-report", "-ftime-trace", "-fsyntax-only",
	"-fsanitize-ignorelist", "-fsanitize-blacklist", "-fsanitize-coverage", "-fprofile-list", "-fxray-",
}

// uncacheableFlag returns the flag of args that makes a compile uncacheable, or "": a flag
// that writes files besides the object, or reads files that the dependency file doesn't list
// (see uncacheableFlags; a response file @file or a clang --config file is read by the driver,
// and its content is not in the key), or doesn't compile a source to an object (-E, -S, -###,
// -v, a header). The files of -include and -imacros are in the dependency file, and
// --sysroot, -isysroot, -iprefix, -iwithprefix, -idirafter and -imultilib are flags, so they
// are in the key.
func uncacheableFlag(args []string) string {
	for i, a := range args {
		switch {
		case a == "-E", a == "-S", a == "-###", a == "-v", a == "-V", a == "--version", a == "-time",
			strings.HasPrefix(a, "-print-"), strings.HasPrefix(a, "-dump"):
			return a
		case a == "-x" && i+1 < len(args) && strings.HasSuffix(args[i+1], "-header"),
			strings.HasPrefix(a, "-x") && strings.HasSuffix(a, "-header"): // a precompiled header, not an object
			return a
		case readsDriverFile(a):
			return a
		}
		for _, p := range uncacheableFlags {
			if strings.HasPrefix(a, p) {
				return a
			}
		}
	}
	return ""
}

// readsDriverFile reports whether the argument a makes the compiler driver read flags from a
// file: a response file @file, or clang's --config.
func readsDriverFile(a string) bool {
	return strings.HasPrefix(a, "@") || strings.HasPrefix(a, "--config")
}

// cacheableFlags reports whether a compile with the flags args may be cached (uncacheableFlag).
func cacheableFlags(args []string) bool { return uncacheableFlag(args) == "" }

// hasTimeMacro reports whether text uses a macro whose value is the time of the compile.
func hasTimeMacro(text []byte) bool {
	return bytes.Contains(text, []byte("__DATE__")) || bytes.Contains(text, []byte("__TIME__")) ||
		bytes.Contains(text, []byte("__TIMESTAMP__"))
}

// hasInclusionProbe reports whether text asks the preprocessor whether a header exists
// (__has_include, __has_include_next): the dependency file doesn't list a header that wasn't
// found, so a later file with that name would not be noticed.
func hasInclusionProbe(text []byte) bool {
	return bytes.Contains(text, []byte("__has_include"))
}

// keyArgs returns args as the key sees them: without -o and its value, and with the work
// folder written as $WORK.
func (b *Builder) keyArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			i++
			continue
		}
		a := args[i]
		if b.Work != "" {
			a = strings.ReplaceAll(a, b.Work, "$WORK")
		}
		out = append(out, a)
	}
	return out
}

// objKeyEnv reports whether the environment variable name changes what the compiler does and
// belongs to the key of a manifest: keyEnv (include paths, the Nix wrapper's flags and
// switches, deployment targets) and more of the Nix wrapper's and of gcc's and clang's own
// variables. Not all of NIX_*: NIX_BUILD_TOP is a random folder in every nix shell.
func objKeyEnv(name string) bool {
	if keyEnv(name) {
		return true
	}
	for _, p := range []string{"NIX_DONT_", "NIX_CC_", "NIX_DEBUG", "NIX_SET_BUILD_ID", "CCC_", "GCC_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// objKey returns the key of the manifest for a compile of the translation unit text tu (with
// the work folder written as $WORK) with the key flags keyArgs.
func (b *Builder) objKey(c CXX, keyArgs []string, tu []byte) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			fmt.Fprintf(h, "%d:%s\x00", len(p), p)
		}
		h.Write([]byte{'\n'})
	}
	write("gocpp object cache", objCacheVersion)
	writeCompiler(write, c)
	write(fmt.Sprint(c.C))
	write(keyArgs...)
	sum := sha256.Sum256(tu)
	write(hex.EncodeToString(sum[:]))
	write(environ(objKeyEnv)...)
	return hex.EncodeToString(h.Sum(nil))
}

// searchDirs returns the absolute folders that args add to the include path, in the order of
// the search: -iquote, -I, -isystem, -idirafter.
func searchDirs(args []string) []string {
	var dirs [4][]string
	for i := 0; i < len(args); i++ {
		for k, flag := range [...]string{"-iquote", "-I", "-isystem", "-idirafter"} {
			var d string
			switch {
			case args[i] == flag && i+1 < len(args):
				i++
				d = args[i]
			case strings.HasPrefix(args[i], flag) && len(args[i]) > len(flag) && flag == "-I":
				d = args[i][len(flag):]
			default:
				continue
			}
			if filepath.IsAbs(d) {
				dirs[k] = append(dirs[k], filepath.Clean(d))
			}
			break
		}
	}
	return slices.Concat(dirs[0], dirs[1], dirs[2], dirs[3])
}

// A depRec is a file that a compile read: its content hash and what stat said when it was
// recorded.
type depRec struct {
	Path string `json:"p"`
	Hash string `json:"h"`
	Size int64  `json:"s"`
	Mod  int64  `json:"m"`
	Ino  uint64 `json:"i,omitempty"`
	Chg  int64  `json:"c,omitempty"`
}

// An objEntry is one result in a manifest.
type objEntry struct {
	Obj     string        `json:"obj"`              // the SHA-256 of the object, which names its file
	Size    int64         `json:"size"`             // the object's size
	Set     string        `json:"set"`              // the name of the file that lists the files the compiler read (<set>.deps)
	Absent  []absentGroup `json:"absent,omitempty"` // files that must not exist: they would hide a dependency
	Except  []string      `json:"except,omitempty"` // files of the groups that exist and don't matter
	Version string        `json:"v"`

	deps []depRec // the contents of the set, when store has it
}

// An absentGroup is the files dir/rel for every dir in Dirs and rel in Rels, a product that is
// not written out (a package of 1000 headers in 40 folders would be 40000 paths): none of
// them may exist, unless it is a dependency itself or is in the entry's Except.
type absentGroup struct {
	Dirs []string `json:"d"`
	Rels []string `json:"r"`
}

// maxAbsentPaths is how many files an entry may ask to be absent: each costs a stat at every
// lookup (10000 take about 15 ms). A compile that would ask for more is not stored, and -x
// says so. A variable, for the tests.
var maxAbsentPaths = 10000

type objManifest struct {
	Version string     `json:"v"`
	Entries []objEntry `json:"e"`
}

// errTimeMacro is recordFile's error for a file that uses __DATE__, __TIME__ or __TIMESTAMP__.
var errTimeMacro = errors.New("the file uses a macro that holds the time")

// errRacy is recordFile's error for a file that changed within the racy window.
var errRacy = errors.New("the file changed less than two seconds before the compile started")

// recordFile reads the file p for an entry of a compile that started at start: its record and
// its content. It fails when the file can't be read, changed while it was read, uses a macro
// that holds the time (errTimeMacro), or changed within objRacyWindow before start (errRacy),
// unless racyOK says that it never changes: the files of the runtime's folder.
func recordFile(p string, start time.Time, racyOK bool) (depRec, []byte, error) {
	before, err := os.Stat(p)
	if err != nil {
		return depRec{}, nil, err
	}
	if !before.Mode().IsRegular() {
		return depRec{}, nil, fmt.Errorf("%s is not a regular file", p)
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return depRec{}, nil, err
	}
	after, err := os.Stat(p)
	if err != nil {
		return depRec{}, nil, err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || sysStatOf(before) != sysStatOf(after) {
		return depRec{}, nil, fmt.Errorf("%s changed while it was read", p)
	}
	if after.ModTime().After(start.Add(-objRacyWindow)) && !racyOK {
		return depRec{}, nil, errRacy
	}
	if hasTimeMacro(content) {
		return depRec{}, nil, errTimeMacro
	}
	sum := sha256.Sum256(content)
	sys := sysStatOf(after)
	return depRec{Path: p, Hash: hex.EncodeToString(sum[:]), Size: after.Size(), Mod: after.ModTime().UnixNano(),
		Ino: sys.ino, Chg: sys.ctime}, content, nil
}

// depPaths returns the files that the dependency files list, as clean absolute paths (a
// relative one is relative to the work folder, where the compiler ran), each once, without
// the translation unit src. ok is false when a file can't be read, or the list names a file of
// this build's work folder (other than src): that one can't be the same next time.
func (b *Builder) depPaths(depfiles []string, src string) (paths []string, ok bool) {
	seen := make(map[string]bool)
	for _, depfile := range depfiles {
		data, err := os.ReadFile(depfile)
		if err != nil || len(data) > maxDepfile {
			return nil, false
		}
		for _, p := range parseDepfile(data) {
			if !filepath.IsAbs(p) {
				p = filepath.Join(b.Work, p)
			}
			p = filepath.Clean(p)
			if p == src || seen[p] {
				continue
			}
			seen[p] = true
			if b.Work != "" && strings.HasPrefix(p, b.Work+string(filepath.Separator)) {
				return nil, false
			}
			paths = append(paths, p)
		}
	}
	return paths, true
}

// systemPrefixes are folders whose files are the system's or the compiler's, not the user's:
// they are not scanned for __has_include (the standard library's headers use it).
var systemPrefixes = []string{"/usr/", "/nix/store/", "/opt/", "/lib/", "/lib64/", "/Library/", "/System/", "/Applications/"}

// userFile reports whether the dependency p is the user's file, not a file of the system or
// the runtime: it is in a folder that the flags add to the include path (dirs), or it is not
// in a system folder (systemPrefixes). The compiler's own folder is not looked at: a compiler
// in ~/bin would make the home folder "the compiler's". The limits are in the comment at the
// top.
func userFile(p string, dirs []string, runtimes string) bool {
	if strings.HasPrefix(p, runtimes) {
		return false
	}
	if slices.ContainsFunc(dirs, func(dir string) bool {
		return strings.HasPrefix(p, strings.TrimRight(dir, string(filepath.Separator))+string(filepath.Separator))
	}) {
		return true
	}
	slash := filepath.ToSlash(p)
	return !slices.ContainsFunc(systemPrefixes, func(pre string) bool { return strings.HasPrefix(slash, pre) })
}

// A pchDeps is what a precompiled header was made from, as pch.go saves it next to the
// header: g++ lists none of those files for a compile that uses the header.
type pchDeps struct {
	Uncacheable bool     `json:"u,omitempty"` // a file uses __DATE__ or the like
	Deps        []depRec `json:"d"`
}

// recordDeps reads the dependency files of a compile that started at start (and the list of
// what the precompiled header was made from, pchFile, if the compile used one), and returns the
// entry's files with their hashes. The error says why the result must not be cached (see the
// comment at the top).
func (b *Builder) recordDeps(depfiles []string, pchFile, src string, start time.Time, dirs []string, c CXX) (entry objEntry, err error) {
	paths, ok := b.depPaths(depfiles, src)
	if !ok {
		return entry, errors.New("a dependency file cannot be read, or a file of the work folder is read")
	}
	runtimes := filepath.Join(b.Cache, "rt") + string(filepath.Separator)
	var user []depRec
	for _, p := range paths {
		rec, content, err := recordFile(p, start, strings.HasPrefix(p, runtimes))
		if err != nil {
			return entry, fmt.Errorf("%s: %w", p, err)
		}
		if userFile(p, dirs, runtimes) {
			if hasInclusionProbe(content) {
				return entry, fmt.Errorf("%s uses __has_include", p)
			}
			user = append(user, rec)
		}
		entry.deps = append(entry.deps, rec)
	}
	if pchFile != "" {
		// The files that the header was made from, as they were then: they must be the same
		// now, or the header is out of date and the object is not what these files make.
		var pd pchDeps
		data, err := os.ReadFile(pchFile)
		if err != nil || json.Unmarshal(data, &pd) != nil || pd.Uncacheable {
			return entry, errors.New("the precompiled header's list of files is missing or says uncacheable")
		}
		have := make(map[string]bool)
		for _, d := range entry.deps {
			have[d.Path] = true
		}
		for _, d := range pd.Deps {
			if !have[d.Path] {
				if !d.sameNow() {
					return entry, fmt.Errorf("%s changed since the precompiled header was built", d.Path)
				}
				entry.deps = append(entry.deps, d)
			}
		}
	}
	// Files that would hide one of the user's dependencies: the same relative name in a folder
	// of the include path that comes before the folder that has it, or in the folder of another
	// file of the user (the compiler looks for #include "x.h" next to the including file first).
	userDirs := make(map[string]bool)
	for _, d := range user {
		userDirs[filepath.Dir(d.Path)] = true
	}
	if b.Work == "" || !strings.HasPrefix(src, b.Work+string(filepath.Separator)) {
		userDirs[filepath.Dir(src)] = true
	}
	relsAt := make([]map[string]bool, len(dirs)) // the user's files by the include folder that has them
	allRels := make(map[string]bool)
	for _, d := range user {
		for i, dir := range dirs {
			if rel, found := strings.CutPrefix(d.Path, dir+string(filepath.Separator)); found {
				if relsAt[i] == nil {
					relsAt[i] = make(map[string]bool)
				}
				relsAt[i][rel] = true
				allRels[rel] = true
			}
		}
	}
	for i, rels := range relsAt {
		if len(rels) > 0 && i > 0 {
			entry.Absent = append(entry.Absent, absentGroup{Dirs: slices.Clone(dirs[:i]), Rels: sortedKeys(rels)})
		}
	}
	if len(allRels) > 0 {
		entry.Absent = append(entry.Absent, absentGroup{Dirs: sortedKeys(userDirs), Rels: sortedKeys(allRels)})
	}
	n := 0
	for _, g := range entry.Absent {
		n += len(g.Dirs) * len(g.Rels)
	}
	if n > maxAbsentPaths {
		return entry, fmt.Errorf("too many include candidates (%d > %d)", n, maxAbsentPaths)
	}
	// Candidates that exist and are not dependencies (a file the compiler had no reason to
	// read) must not make every lookup fail.
	isDep := make(map[string]bool, len(entry.deps))
	for _, d := range entry.deps {
		isDep[d.Path] = true
	}
	except := make(map[string]bool)
	for _, g := range entry.Absent {
		for _, dir := range g.Dirs {
			for _, rel := range g.Rels {
				cand := filepath.Join(dir, rel)
				if _, err := os.Lstat(cand); !errors.Is(err, fs.ErrNotExist) && !isDep[cand] {
					except[cand] = true
				}
			}
		}
	}
	entry.Except = sortedKeys(except)
	return entry, nil
}

// sortedKeys returns the keys of m in order.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// absentOK reports whether none of the files that the entry wants absent exists. A file that
// is a dependency (deps) or in Except doesn't count.
func (e *objEntry) absentOK(deps []depRec) bool {
	if len(e.Absent) == 0 {
		return true
	}
	skip := make(map[string]bool, len(deps)+len(e.Except))
	for _, d := range deps {
		skip[d.Path] = true
	}
	for _, p := range e.Except {
		skip[p] = true
	}
	n := 0
	for _, g := range e.Absent {
		for _, dir := range g.Dirs {
			for _, rel := range g.Rels {
				if n++; n > maxAbsentPaths {
					return false // a manifest that asks for more than a store would
				}
				cand := filepath.Join(dir, rel)
				if skip[cand] {
					continue
				}
				if _, err := os.Lstat(cand); !errors.Is(err, fs.ErrNotExist) {
					return false
				}
			}
		}
	}
	return true
}

// sameNow reports whether the file of the record still has its recorded content: its stat is
// as recorded, or its hash is.
func (d depRec) sameNow() bool {
	fi, err := os.Stat(d.Path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() != d.Size {
		return false
	}
	sys := sysStatOf(fi)
	if fi.ModTime().UnixNano() == d.Mod && sys.ino == d.Ino && sys.ctime == d.Chg {
		return true
	}
	data, err := os.ReadFile(d.Path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == d.Hash
}

// parseDepfile returns the files that a Makefile-style dependency file names after its
// target, in order: the main file first, then the headers. It understands the escapes of a
// space (\ ), of # and of $ ($$), and the line continuation.
func parseDepfile(data []byte) []string {
	s := strings.ReplaceAll(string(data), "\\\r\n", " ")
	s = strings.ReplaceAll(s, "\\\n", " ")
	var words []string
	var cur strings.Builder
	in := false
	flush := func() {
		if in {
			words = append(words, cur.String())
			cur.Reset()
			in = false
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case ' ', '\t', '\n', '\r':
			flush()
		case '$':
			if i+1 < len(s) && s[i+1] == '$' {
				i++
			}
			cur.WriteByte('$')
			in = true
		case '\\':
			n := 0
			for i+n < len(s) && s[i+n] == '\\' {
				n++
			}
			if i+n < len(s) && (s[i+n] == ' ' || s[i+n] == '#') {
				cur.WriteString(strings.Repeat(`\`, n/2))
				if n%2 == 1 {
					cur.WriteByte(s[i+n])
					i += n
				} else {
					i += n - 1
				}
			} else {
				cur.WriteString(strings.Repeat(`\`, n))
				i += n - 1
			}
			in = true
		default:
			cur.WriteByte(c)
			in = true
		}
	}
	flush()
	for i, w := range words {
		if strings.HasSuffix(w, ":") {
			return words[i+1:]
		}
	}
	return nil
}

func (oc *objCache) manifestPath(key string) string { return filepath.Join(oc.dir, key+".man") }
func (oc *objCache) objectPath(hash string) string  { return filepath.Join(oc.dir, hash+".o") }
func (oc *objCache) setPath(name string) string     { return filepath.Join(oc.dir, name+".deps") }

// A set is the list of the files that a compile read, with their hashes and stats, in a file
// of its own named by the SHA-256 of its text: the manifests of programs that read the same
// files (the headers of the runtime and the standard library, nearly always) share it, as it is
// most of an entry's size. A set that doesn't read back as its name says is no set.

// writeSet writes the set deps and returns its name, or "" when it can't be written.
func (oc *objCache) writeSet(deps []depRec) string {
	data, err := json.Marshal(append([]depRec{}, deps...))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:])
	if old, err := os.ReadFile(oc.setPath(name)); err == nil && sha256.Sum256(old) == sum {
		oc.touch(oc.setPath(name))
		return name
	}
	if !oc.writeFile(oc.setPath(name), data) {
		return ""
	}
	return name
}

// readSet returns the set of that name, or nil when it is missing or broken.
func (oc *objCache) readSet(name string) []depRec {
	data, err := os.ReadFile(oc.setPath(name))
	if err != nil || hex.EncodeToString(sha256Of(data)) != name {
		return nil
	}
	var deps []depRec
	if json.Unmarshal(data, &deps) != nil || deps == nil {
		return nil
	}
	return deps
}

func sha256Of(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func (oc *objCache) readManifest(key string) objManifest {
	var m objManifest
	data, err := os.ReadFile(oc.manifestPath(key))
	if err != nil || json.Unmarshal(data, &m) != nil || m.Version != objCacheVersion {
		return objManifest{Version: objCacheVersion}
	}
	return m
}

// lookup looks for an entry of the manifest key whose files are all as recorded and whose
// object is intact, and copies the object to obj. It returns the hash of the object.
func (oc *objCache) lookup(key, obj string) (hash string, ok bool) {
	m := oc.readManifest(key)
	hashes := make(map[string]hashedFile) // the files hashed in this lookup
	sets := make(map[string][]depRec)     // the sets read in this lookup
	for _, e := range m.Entries {
		if e.Version != objCacheVersion || !isHexName(e.Obj) || !isHexName(e.Set) {
			continue
		}
		deps, ok := sets[e.Set]
		if !ok {
			deps = oc.readSet(e.Set)
			sets[e.Set] = deps
		}
		if deps == nil {
			continue
		}
		valid, fresh := e.valid(deps, hashes)
		if !valid || !oc.copyOut(e, obj) {
			continue
		}
		if fresh != nil {
			oc.refresh(key, e, fresh)
		}
		oc.touch(oc.objectPath(e.Obj))
		oc.touch(oc.setPath(e.Set))
		oc.touch(oc.manifestPath(key))
		return e.Obj, true
	}
	return "", false
}

// A hashedFile is a file that a lookup had to read: its hash and its stat after the read.
type hashedFile struct {
	hash   string
	rec    depRec // the stat, in a record
	stable bool   // the stat didn't change during the read, and the modification and change times are older than the racy window
}

// hashFile reads the file path and returns its hash and stat. ok is false when it can't be
// read or changed while it was read.
func hashFile(path string) (h hashedFile, ok bool) {
	before, err := os.Stat(path)
	if err != nil {
		return h, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return h, false
	}
	after, err := os.Stat(path)
	if err != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || sysStatOf(before) != sysStatOf(after) {
		return h, false
	}
	sum := sha256.Sum256(data)
	sys := sysStatOf(after)
	return hashedFile{hash: hex.EncodeToString(sum[:]),
		rec:    depRec{Size: after.Size(), Mod: after.ModTime().UnixNano(), Ino: sys.ino, Chg: sys.ctime},
		stable: time.Since(after.ModTime()) > objRacyWindow && (sys.ctime == 0 || time.Since(time.Unix(0, sys.ctime)) > objRacyWindow)}, true
}

// valid reports whether the files deps (the set of the entry) have the recorded content and
// none of the files that must be absent exists. hashes remembers files already hashed. When a
// file has the recorded content but another stat (it was touched, or the store was copied),
// and its stat can be trusted from now on, fresh is the set with the new stat, to be saved so
// that the next lookup needn't read the file.
func (e *objEntry) valid(deps []depRec, hashes map[string]hashedFile) (valid bool, fresh []depRec) {
	for i, d := range deps {
		fi, err := os.Stat(d.Path)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() != d.Size {
			return false, nil
		}
		sys := sysStatOf(fi)
		if fi.ModTime().UnixNano() == d.Mod && sys.ino == d.Ino && sys.ctime == d.Chg {
			continue
		}
		h, ok := hashes[d.Path]
		if !ok {
			if h, ok = hashFile(d.Path); !ok {
				return false, nil
			}
			hashes[d.Path] = h
		}
		if h.hash != d.Hash {
			return false, nil
		}
		if h.stable {
			if fresh == nil {
				fresh = slices.Clone(deps)
			}
			fresh[i].Size, fresh[i].Mod, fresh[i].Ino, fresh[i].Chg = h.rec.Size, h.rec.Mod, h.rec.Ino, h.rec.Chg
		}
	}
	if !e.absentOK(deps) {
		return false, nil
	}
	return true, fresh
}

// refresh saves the set fresh (the files of the entry e with their new stat) and points the
// entry of the manifest key at it. Errors are ignored.
func (oc *objCache) refresh(key string, e objEntry, fresh []depRec) {
	set := oc.writeSet(fresh)
	if set == "" {
		return
	}
	m := oc.readManifest(key)
	for i := range m.Entries {
		if m.Entries[i].Obj == e.Obj && m.Entries[i].Set == e.Set {
			m.Entries[i].Set = set
		}
	}
	if js, err := json.Marshal(m); err == nil {
		oc.writeFile(oc.manifestPath(key), js)
	}
}

// isHexName reports whether s is a name that cannot leave the cache folder.
func isHexName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// copyOut copies the object of e to dst. It fails, and leaves nothing at dst, when the cached
// object is missing or is not what was stored (another size or hash): a corrupt object is
// removed, so that the next store replaces it.
func (oc *objCache) copyOut(e objEntry, dst string) bool {
	path := oc.objectPath(e.Obj)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	out, err := os.Create(dst)
	if err != nil {
		return false
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), f)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil || n != e.Size || hex.EncodeToString(h.Sum(nil)) != e.Obj {
		os.Remove(dst)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			os.Remove(path)
		}
		return false
	}
	return true
}

// touch sets the modification time of path to now if it is older than objTouchAge, so that the
// size limit removes the files that are used least recently.
func (oc *objCache) touch(path string) {
	if fi, err := os.Stat(path); err == nil && time.Since(fi.ModTime()) > objTouchAge {
		now := time.Now()
		os.Chtimes(path, now, now)
	}
}

// store puts the object obj into the cache as the result of the manifest key with the entry
// e, and trims the cache when it is time. Errors are ignored: the cache is an optimization.
// The object is named by the SHA-256 of its bytes, so that the name says what the file holds:
// one that is there already is checked against it and replaced when it differs (a lost write,
// a truncated file), never trusted.
func (oc *objCache) store(key string, e objEntry, obj string) {
	e.Version = objCacheVersion
	data, err := os.ReadFile(obj)
	if err != nil {
		return
	}
	sum := sha256.Sum256(data)
	e.Obj, e.Size = hex.EncodeToString(sum[:]), int64(len(data))
	if old, err := os.ReadFile(oc.objectPath(e.Obj)); err == nil && sha256.Sum256(old) == sum {
		oc.touch(oc.objectPath(e.Obj))
	} else if !oc.writeFile(oc.objectPath(e.Obj), data) {
		return
	}
	if e.Set = oc.writeSet(e.deps); e.Set == "" {
		return
	}
	m := oc.readManifest(key)
	entries := []objEntry{e}
	for _, o := range m.Entries {
		if (o.Obj != e.Obj || o.Set != e.Set) && len(entries) < objManifestEntries {
			entries = append(entries, o)
		}
	}
	m.Version, m.Entries = objCacheVersion, entries
	js, err := json.Marshal(m)
	if err != nil || !oc.writeFile(oc.manifestPath(key), js) {
		return
	}
	oc.maybeTrim()
}

// writeFile writes data to path atomically (a temporary file in the folder, renamed).
func (oc *objCache) writeFile(path string, data []byte) bool {
	tmp, err := os.CreateTemp(oc.dir, ".tmp-")
	if err != nil {
		return false
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
		return false
	}
	return true
}

// maybeTrim trims the cache if the last trim was objTrimEvery ago or never.
func (oc *objCache) maybeTrim() {
	marker := filepath.Join(oc.dir, ".trimmed")
	if fi, err := os.Stat(marker); err == nil && time.Since(fi.ModTime()) < objTrimEvery {
		return
	}
	// The marker first, so that gocpp commands that run at the same time don't all trim.
	if f, err := os.Create(marker); err == nil {
		f.Close()
	}
	oc.trim(oc.max)
}

// trim removes the oldest files of the folder until its files together are at most 90% of max
// bytes, and the temporary files that a killed gocpp left.
func (oc *objCache) trim(max int64) {
	entries, err := os.ReadDir(oc.dir)
	if err != nil {
		return
	}
	type file struct {
		name string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() || e.Name() == ".trimmed" {
			continue
		}
		if strings.HasPrefix(e.Name(), ".tmp-") {
			if time.Since(fi.ModTime()) > staleAge {
				os.Remove(filepath.Join(oc.dir, e.Name()))
			}
			continue
		}
		files = append(files, file{e.Name(), fi.Size(), fi.ModTime()})
		total += fi.Size()
	}
	if total <= max {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].mod.Equal(files[j].mod) {
			return files[i].mod.Before(files[j].mod)
		}
		return files[i].name < files[j].name
	})
	target := max / 10 * 9
	for _, f := range files {
		if total <= target {
			break
		}
		os.Remove(filepath.Join(oc.dir, f.name))
		total -= f.size
	}
}
