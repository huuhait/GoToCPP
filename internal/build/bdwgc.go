package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/fsretry"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// bdwgc (phase 16 README §2.4): the Boehm-Demers-Weiser collector, vendored in
// runtime/third_party/bdwgc and embedded in the gocpp binary. It is built as C, once per C
// compiler (and its version), sanitizer, target and flag set, into a static library in the cache,
// and only a program that asks for it links it (Builder.Bdwgc). Nothing else in the build refers
// to it.
//
// The source is extracted from the binary like the C++ runtime (runtime.go), into
// <cache>/bdwgc/<version>-<hash>/, and the library is kept next to it in lib/<key>/libbdwgc.a.
// The key is a hash of the C compiler (path, arguments, family, version and the identity of its
// program file), the target, the flags (the fixed defines of §2.4, the sanitizer, and the caller's
// flags), the sources, the environment that changes what the compiler does (KeyEnv) and the
// archiver (rtlib.go's rules). A new vendored tree has a new <hash>, so a stale library is never
// found. Building and sharing work like the runtime's library: one process builds a key at a
// time, and the finished library is renamed into place.
//
// Configuration: bdwgc is configured with -D flags and no config.h (HAVE_CONFIG_H is not
// defined), which is what its CMake build does on Linux. The fixed defines are the ones of
// phase 16 §2.4; the rest is what bdwgc's own CMakeLists.txt adds for a Linux build with
// threads. See bdwgcDefines. The build does not run configure or CMake.
//
// Sanitizers: bdwgc adapts to a sanitizer only when it is compiled with it. The ASan variant is
// -fsanitize=address,undefined (like the runtime's asan mode); bdwgc finds it with
// __SANITIZE_ADDRESS__ (GCC) or __has_feature (clang) and provides __asan_default_options.
// The TSan variant is -fsanitize=thread with -DTHREAD_SANITIZER: GCC 15 has
// __has_feature(thread_sanitizer), which bdwgc's gcconfig.h uses, but older GCC has only
// __SANITIZE_THREAD__, which it does not look at, so the define comes from the build.
// Both variants build and the smoke test passes with g++ and clang++ (runtime/test/bdwgc_test.cpp).

// A BdwgcSanitizer is the sanitizer that bdwgc is built with. It must match the one of the
// program that links it.
type BdwgcSanitizer int

const (
	BdwgcNoSanitizer BdwgcSanitizer = iota
	BdwgcASan
	BdwgcTSan
)

// String returns "none", "asan" or "tsan".
func (s BdwgcSanitizer) String() string {
	switch s {
	case BdwgcNoSanitizer:
		return "none"
	case BdwgcASan:
		return "asan"
	case BdwgcTSan:
		return "tsan"
	}
	return fmt.Sprintf("BdwgcSanitizer(%d)", int(s))
}

// A BdwgcConfig is what the caller chooses of a bdwgc build; the compiler, the target, the cache
// and the work folder come from the Builder.
type BdwgcConfig struct {
	// Sanitizer is the sanitizer of the program that links the library. Builder.ASan (a -asan
	// build) and a -fsanitize=thread flag in Builder.Env.CFlags (a -race build) override it; a
	// configuration that asks for the other sanitizer is an error, so a library is never linked
	// into a program that was built with a sanitizer the library does not have.
	Sanitizer BdwgcSanitizer

	// CFlags are C flags after gocpp's own: the optimization level (-O2 for a release build),
	// -g, and what the user's GOCPP_CFLAGS adds. A later -O wins over gocpp's default -O0.
	CFlags []string
}

// A BdwgcLib is the result of Builder.Bdwgc: the static library, the flags to compile code that
// includes gc.h, and the flags to link a program with it.
type BdwgcLib struct {
	// Archive is the absolute path of libbdwgc.a.
	Archive string

	// CompileFlags are the flags of a compile of code that uses bdwgc: -I<dir>/include,
	// -DGC_THREADS (gc.h declares the thread API only with it) and -DGC_NO_THREAD_REDIRECTS
	// (registration is explicit: gc.h must not turn pthread_create into GC_pthread_create).
	CompileFlags []string

	// LinkFlags are the flags after the archive on a link: -lpthread and -ldl, and the
	// sanitizer's flag.
	LinkFlags []string
}

// Bdwgc builds the vendored bdwgc for the builder's C++ compiler (its C compiler is Builder.CC,
// or the one that FindCC finds for it), target and flags, and returns the library and its flags.
// A program that does not call this links no bdwgc symbol.
func (b *Builder) Bdwgc(ctx context.Context, cfg BdwgcConfig) (*BdwgcLib, error) {
	if b.CXX.Path == "" {
		return nil, errors.New("build: Bdwgc needs the program's C++ compiler (Builder.CXX)")
	}
	goos := b.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "linux" {
		// The Windows source is vendored (src/windows/win32_threads.c) and BdwgcSources("windows")
		// lists the Win32 build, but its defines, OS layer and link flags are M10 work.
		return nil, fmt.Errorf("build: the bdwgc build is wired for linux only (the Windows source is vendored, but the MinGW build is M10 work); this build is for %s", goos)
	}
	cc := b.CC
	if cc.Path == "" {
		var err error
		if cc, err = FindCC(ctx, b.CXX); err != nil {
			return nil, err
		}
	}
	dir, err := extractBdwgc(b.Cache, b.Work)
	if err != nil {
		return nil, err
	}
	san, err := b.bdwgcSanitizer(cfg.Sanitizer)
	if err != nil {
		return nil, err
	}
	cfg.Sanitizer = san
	flags := b.bdwgcFlags(cc, goos, cfg)
	srcs := rtfiles.BdwgcSources(goos)
	a, err := FindArchiver(ctx, cc, bdwgcLTO(flags))
	if err != nil {
		return nil, err
	}
	lib, err := b.cachedBdwgc(ctx, dir, cc, goos, srcs, flags, a)
	if err != nil {
		return nil, err
	}
	return &BdwgcLib{
		Archive:      lib,
		CompileFlags: []string{"-I", filepath.Join(dir, "include"), "-DGC_THREADS", "-DGC_NO_THREAD_REDIRECTS"},
		LinkFlags:    bdwgcLinkFlags(cfg.Sanitizer),
	}, nil
}

// bdwgcSanitizer returns the sanitizer that bdwgc is built with: the builder's own (-asan, or a
// -race flag in its environment) when it has one, else want. A want that contradicts the
// builder's sanitizer is an error, because a library built without a sanitizer cannot be linked
// into a program that has one (and the other way around).
func (b *Builder) bdwgcSanitizer(want BdwgcSanitizer) (BdwgcSanitizer, error) {
	switch {
	case b.ASan:
		if want != BdwgcNoSanitizer && want != BdwgcASan {
			return 0, fmt.Errorf("build: bdwgc: the builder builds with -asan, but the configuration asks for the %s sanitizer", want)
		}
		return BdwgcASan, nil
	case slices.Contains(b.Env.CFlags, "-fsanitize=thread"):
		if want != BdwgcNoSanitizer && want != BdwgcTSan {
			return 0, fmt.Errorf("build: bdwgc: the builder builds with -race, but the configuration asks for the %s sanitizer", want)
		}
		return BdwgcTSan, nil
	}
	return want, nil
}

// bdwgcFlags returns the C flags of the compile of bdwgc's sources: gocpp's base flags, the fixed
// defines, the sanitizer, the user's C flags (GOCPP_CFLAGS) and the caller's flags.
func (b *Builder) bdwgcFlags(cc CXX, goos string, cfg BdwgcConfig) []string {
	flags := []string{"-std=gnu17", "-O0", "-g", "-fno-strict-aliasing"}
	flags = append(flags, bdwgcDefines(goos)...)
	switch cfg.Sanitizer {
	case BdwgcASan:
		flags = append(flags, "-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all")
	case BdwgcTSan:
		flags = append(flags, "-fsanitize=thread", "-fno-omit-frame-pointer", "-DTHREAD_SANITIZER")
	}
	if cc.Family == Clang {
		flags = append(flags, "-Qunused-arguments")
	}
	// The user's flags and the caller's come after gocpp's, so that a later -O or -std wins; the
	// warnings of bdwgc (with a user's -Wall) must not fail the build.
	flags = append(flags, b.Env.CFlags...)
	flags = append(flags, cfg.CFlags...)
	return append(flags, "-Wno-error")
}

// bdwgcDefines returns the -D flags of bdwgc's build: the fixed ones of phase 16 §2.4 (including
// GC_DISABLE_INCREMENTAL: gocpp owns SIGSEGV), and what bdwgc's CMakeLists.txt adds for a Linux
// build with threads, parallel marking, thread-local allocation, gcj support and the disclaim
// interface. DONT_ADD_BYTE_AT_END is not among them (the extra byte at the end of an object
// stays; §2.4).
func bdwgcDefines(goos string) []string {
	defs := []string{
		// §2.4, fixed.
		"-DGC_THREADS",
		"-DPARALLEL_MARK",            // MAX_MARKERS is bdwgc's default of 16
		"-DTHREAD_LOCAL_ALLOC",       // without it allocating threads did not scale (vlang/v#27486)
		"-DGC_BUILTIN_ATOMIC",        // the compiler's atomics; no libatomic_ops
		"-DLARGE_CONFIG",             // heaps above about 4 GB
		"-DUSE_MMAP", "-DUSE_MUNMAP", // the heap returns memory to the system
		"-DALL_INTERIOR_POINTERS",
		// §2.4: incremental mode off. gocpp's runtime owns SIGSEGV; bdwgc's virtual dirty bit
		// implementations (MPROTECT_VDB, or SOFT_VDB on a kernel with soft-dirty) install a
		// write-fault handler that would take it. gcconfig.h defines those macros itself on
		// x86_64 Linux, so only GC_DISABLE_INCREMENTAL turns the mode (and its handler) off;
		// GC_enable_incremental becomes a no-op. TinyGo does the same.
		"-DGC_DISABLE_INCREMENTAL",
		// bdwgc's own Linux defaults (CMakeLists.txt).
		"-DNO_EXECUTE_PERMISSION",
		"-D_REENTRANT",
		"-DGC_GCJ_SUPPORT",
		"-DGC_ENABLE_SUSPEND_THREAD",
		"-DENABLE_DISCLAIM",
		"-DJAVA_FINALIZATION",
		"-DGC_ATOMIC_UNCOLLECTABLE",
		"-DHANDLE_FORK",
		// §2.5: registration is explicit, not bdwgc's macros that redirect pthread_create. The
		// define also reaches the code that includes gc.h (BdwgcLib.CompileFlags).
		"-DGC_NO_THREAD_REDIRECTS",
	}
	if goos == "linux" {
		// What configure or CMake finds on glibc: dl_iterate_phdr for the data roots of shared
		// libraries, dladdr for names in reports, and pthread_setname_np(pthread_t, const char*).
		defs = append(defs, "-DHAVE_DL_ITERATE_PHDR", "-DHAVE_DLADDR", "-DHAVE_PTHREAD_SETNAME_NP_WITH_TID")
	}
	return defs
}

// bdwgcLinkFlags returns the flags after the archive on a link: the C library's threads and
// dynamic loading, and the sanitizer's runtime.
func bdwgcLinkFlags(san BdwgcSanitizer) []string {
	flags := []string{"-lpthread", "-ldl"}
	switch san {
	case BdwgcASan:
		flags = append([]string{"-fsanitize=address,undefined"}, flags...)
	case BdwgcTSan:
		flags = append([]string{"-fsanitize=thread"}, flags...)
	}
	return flags
}

// bdwgcLTO reports whether the flags make LTO objects (then the archiver must read them).
func bdwgcLTO(flags []string) bool {
	return slices.ContainsFunc(flags, func(f string) bool {
		return strings.HasPrefix(f, "-flto") || f == "-fuse-linker-plugin"
	})
}

// bdwgcName is the name of the bdwgc folder in <cache>/bdwgc: the version and a hash of the
// vendored files, so that a new tree never shares a folder with an old one.
func bdwgcName() string {
	return rtfiles.BdwgcVersion() + "-" + bdwgcHash()
}

// bdwgcHash is a hash of the vendored bdwgc's files: their paths and contents.
var bdwgcHash = sync.OnceValue(func() string {
	h := sha256.New()
	err := fs.WalkDir(rtfiles.BdwgcFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(rtfiles.BdwgcFS(), p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		panic("build: can't read the embedded bdwgc: " + err.Error()) // it is in the binary
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
})

// bdwgcFile is one file of the vendored bdwgc, to check an extracted folder cheaply.
type bdwgcFile struct {
	path string
	size int64
}

// bdwgcFiles lists the files of the vendored bdwgc.
var bdwgcFiles = sync.OnceValue(func() []bdwgcFile {
	var files []bdwgcFile
	fs.WalkDir(rtfiles.BdwgcFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, bdwgcFile{p, fi.Size()})
		return nil
	})
	return files
})

// extractBdwgc returns the folder of the extracted bdwgc: <cache>/bdwgc/<name> when there is a
// cache folder, else <work>/bdwgc. A folder that is there already is checked cheaply (every file
// exists with the right size); a broken one is replaced. The writes are atomic, as in
// ExtractRuntime.
func extractBdwgc(cache, work string) (string, error) {
	if cache == "" {
		if work == "" {
			return "", errors.New("build: bdwgc needs a cache folder or a work folder")
		}
		dir := filepath.Join(work, "bdwgc")
		if checkBdwgc(dir) == nil {
			return dir, nil
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("extracting bdwgc: %w", err)
		}
		if err := writeBdwgc(dir); err != nil {
			return "", fmt.Errorf("extracting bdwgc: %w", err)
		}
		return dir, nil
	}
	root := filepath.Join(cache, "bdwgc")
	dir := filepath.Join(root, bdwgcName())
	if checkBdwgc(dir) == nil {
		return dir, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("extracting bdwgc: %w", err)
	}
	removeStale(root)
	tmp, err := os.MkdirTemp(root, ".tmp-")
	if err != nil {
		return "", fmt.Errorf("extracting bdwgc: %w", err)
	}
	defer os.RemoveAll(tmp) // after a successful rename, there is nothing left
	if err := writeBdwgc(tmp); err != nil {
		return "", fmt.Errorf("extracting bdwgc: %w", err)
	}
	if err := fsretry.Rename(tmp, dir); err == nil {
		return dir, nil
	}
	// Another gocpp extracted it at the same time (the rename fails when dir is a folder that is
	// not empty), or dir is broken.
	if checkBdwgc(dir) == nil {
		return dir, nil
	}
	old, err := os.MkdirTemp(root, ".old-")
	if err != nil {
		return "", fmt.Errorf("extracting bdwgc: %w", err)
	}
	defer os.RemoveAll(old)
	if err := fsretry.Rename(dir, filepath.Join(old, "bdwgc")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("extracting bdwgc: replacing the broken %s: %w", dir, err)
	}
	if err := fsretry.Rename(tmp, dir); err != nil && checkBdwgc(dir) != nil {
		return "", fmt.Errorf("extracting bdwgc: %w", err)
	}
	return dir, nil
}

// writeBdwgc writes the vendored bdwgc into the folder dir, which must exist.
func writeBdwgc(dir string) error {
	return fs.WalkDir(rtfiles.BdwgcFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(rtfiles.BdwgcFS(), p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

// checkBdwgc checks cheaply that dir holds the vendored bdwgc: every file is there, with the right
// size.
func checkBdwgc(dir string) error {
	for _, f := range bdwgcFiles() {
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.path)))
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() || fi.Size() != f.size {
			return fmt.Errorf("%s: the file differs from the vendored bdwgc", f.path)
		}
	}
	return nil
}

// bdwgcLibName is the name of the bdwgc library in its folder.
const bdwgcLibName = "libbdwgc.a"

// cachedBdwgc returns the bdwgc library for the flags in dir/lib/<key>/libbdwgc.a, making it
// first if it is not there. The rules are cachedLib's (rtlib.go): one process builds a key, the
// others wait, and the finished library is renamed into place.
func (b *Builder) cachedBdwgc(ctx context.Context, dir string, cc CXX, goos string, srcs, flags []string, a Archiver) (string, error) {
	root := filepath.Join(dir, "lib")
	key := b.bdwgcKey(cc, goos, srcs, flags, a)
	final := filepath.Join(root, key, bdwgcLibName)
	if libOK(final) {
		return final, nil
	}
	if err := os.MkdirAll(filepath.Join(root, key), 0o755); err != nil {
		return "", err
	}
	removeStale(root)

	// One process builds the library; the others wait for it.
	lockName := filepath.Join(root, key+".lock")
	deadline := time.Now().Add(libLockWait)
	for {
		lock, busy, err := tryLockFile(lockName)
		if err != nil {
			break // no lock on this file system: build without it
		}
		if !busy {
			defer lock.unlock()
			break
		}
		if libOK(final) {
			return final, nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(libLockPoll):
		}
	}
	if libOK(final) { // made by the process that had the lock
		return final, nil
	}

	tmp, err := os.MkdirTemp(root, ".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	objDir := filepath.Join(tmp, "obj")
	b.trace("mkdir -p " + quoteWord(objDir))
	if err := os.Mkdir(objDir, 0o755); err != nil {
		return "", err
	}
	objs, err := b.compileBdwgc(ctx, dir, objDir, cc, srcs, flags)
	if err != nil {
		return "", err
	}
	if b.DryRun {
		return final, nil // the compiles were shown, not run; there is no library to make
	}
	lib := filepath.Join(tmp, bdwgcLibName)
	if err := b.makeLib(ctx, a, lib, objs); err != nil {
		return "", err
	}
	if !libOK(lib) {
		return "", fmt.Errorf("the archiver %s made no bdwgc library", a.Path)
	}
	b.trace("mv " + quoteWord(lib) + " " + quoteWord(final))
	if err := fsretry.Rename(lib, final); err != nil && !libOK(final) {
		return "", err
	}
	return final, nil
}

// compileBdwgc compiles bdwgc's sources srcs (paths in the extracted folder dir, such as
// src/alloc.c) into objDir, at the same time, and returns the object files. Warnings go to Stderr;
// they do not fail the build.
func (b *Builder) compileBdwgc(ctx context.Context, dir, objDir string, cc CXX, srcs, flags []string) ([]string, error) {
	objs := make([]string, len(srcs))
	acts := make([]action, len(srcs))
	for i, rel := range srcs {
		src := filepath.Join(dir, filepath.FromSlash(rel))
		objs[i] = filepath.Join(objDir, strings.ReplaceAll(strings.TrimSuffix(rel, ".c"), "/", "_")+".o")
		args := slices.Concat(flags, []string{"-I", filepath.Join(dir, "include"), "-c", src, "-o", objs[i]})
		acts[i] = action{name: rel, run: func() (func(), error) {
			return nil, b.run(ctx, cc, "the bdwgc source "+rel, false, args)
		}}
	}
	errs, _ := b.runActions(ctx, acts)
	for _, err := range errs {
		if err != nil {
			return nil, err // the first one: one compile error is enough
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return objs, nil
}

// bdwgcKey returns the name of the folder of the bdwgc library for the compiler cc, the target
// goos and the flags: a hash of what the library depends on.
func (b *Builder) bdwgcKey(cc CXX, goos string, srcs, flags []string, a Archiver) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			fmt.Fprintf(h, "%d:%s\x00", len(p), p)
		}
		h.Write([]byte{'\n'})
	}
	writeCompiler(write, cc)
	arch := cc.GOARCH
	if arch == "" {
		arch = runtime.GOARCH
	}
	write("bdwgc", rtfiles.BdwgcVersion(), bdwgcHash(), goos, arch)
	write(flags...)
	write(srcs...)
	write(KeyEnv()...)
	write(a.identity()...)
	if salt := PluginSalt(slices.Concat(cc.Args, flags)); len(salt) > 0 {
		write(salt...) // the compiler plugins that the flags load, by the hash of their files
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
