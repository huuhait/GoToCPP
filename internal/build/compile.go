package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	rtfiles "github.com/huuhait/gocpp/runtime"

	"github.com/huuhait/gocpp/internal/fsretry"
)

// A Builder compiles the C++ of a program and links it with the runtime
// (Phase 02: one translation unit per program; Phase 06 brings the action
// graph).
type Builder struct {
	CXX      CXX    // the compiler
	ASan     bool   // build with the sanitizers (-asan)
	NoChecks bool   // without the bounds and nil checks (-nochecks)
	GOOS     string // the operating system, for the runtime's OS layer

	// Work is the work folder (absolute): the object files go there, and
	// the compiles run in it.
	Work string

	// Cache is the cache folder (such as ~/.cache/gocpp), where the
	// runtime is extracted once and its object files are kept. "" means
	// no cache: then the runtime is extracted into Work and compiled for
	// this build only.
	Cache string

	// KeepWork says that the work folder is kept for the user (gocpp build -work): the object
	// cache is not used then, so that its files have real paths in them (the debug info of
	// the objects the cache makes names a folder that doesn't exist).
	KeepWork bool

	// Trace, if not nil, gets each command before it runs, as a shell
	// line, with Work written as $WORK (gocpp build -x).
	Trace io.Writer

	// Stderr gets what a compiler prints when it succeeds: its warnings.
	Stderr io.Writer

	// Env are the trusted flags of the environment (ReadEnvFlags): the C++
	// ones go to every compile, also of the runtime (so they are part of
	// the key of the runtime's object files), and GOCPP_LDFLAGS to the
	// link, after the package's link flags.
	Env EnvFlags

	// Package are the flags of the program's package, after the allow-list
	// (check.Package.Flags): Compile goes to the compile of the generated
	// C++ and of the package's C and C++ files, after the environment's
	// flags, and Link to the link, after the object files. A program may have several packages
	// (Phase 06 step 6.2); the flags are those of one package.
	Package PackageFlags

	// Dir is the folder of the program's package, and CFiles and CXXFiles
	// are the names of its C and C++ files there (load.Package): Program
	// compiles them (see sources.go) and links them in. CC is the C
	// compiler (FindCC); it is needed only with CFiles.
	Dir              string
	CFiles, CXXFiles []string
	CC               CXX

	// SourceWarnings, if not nil, gets what the compiler printed for a C
	// or C++ file of the package that compiled (its warnings), instead of
	// Stderr: the driver shows them under the package's name, as errors in
	// these files. It is called for one file at a time.
	SourceWarnings func(out []byte)

	// PackageDir is the folder of the package's .gp files, or "". The
	// compile of the generated C++ gets -iquote PackageDir, so that its
	// local header imports (#include "./physics.h") are found there, as by
	// the compiler probes (internal/interop), and never a system header:
	// -iquote, unlike -I, doesn't apply to #include <vector>.
	PackageDir string

	// GenRoot is the folder that holds the generated files of a program of several packages
	// (Link): the units include each other's headers by their path in it (gpkg/<import path>/pkg.hpp),
	// so it is an -iquote folder of their compiles.
	GenRoot string

	// Mode is what the build makes (modes.go): a program (the zero value is the same as ModeExe), or a
	// library, which compiles everything with -fPIC (and, for c-shared, hidden visibility) and, for the
	// static ones, ends in an archive instead of a link. Soname names a shared library.
	Mode   Mode
	Soname string

	// VersionScript, for c-shared, is the file that names the functions the library exports. ImportLib,
	// for c-shared on Windows, is the import library that the link writes (libx.dll.a); neither is
	// used on the other system.
	VersionScript string
	ImportLib     string

	// NoManifest leaves the application manifest out of a Windows program (GOCPP_NO_MANIFEST): for a
	// program that brings its own resource file through the link flags.
	NoManifest bool

	// Jobs is how many compiles run at the same time (gocpp build -p N): 0 means DefaultJobs
	// (GOMAXPROCS).
	Jobs int

	// Rec, if not nil, collects what the build did for the up-to-date check (stamp.go): every compile
	// that the object cache answered or stored, the link, and the reasons that make the build
	// unstampable.
	Rec *Record

	// Stats, if not nil, counts the compiles, cache hits and links of the builds (executor.go).
	Stats *Stats

	modeApplied bool // applyMode ran
	// Plan, if not nil, collects every compile and the link of the build (plan.go).
	Plan *Plan

	// DryRun says that the build runs no compiler and no linker: the commands are shown (Trace)
	// and recorded (Plan), and nothing is cached (gocpp build -n, gocpp export).
	DryRun bool

	// Rebuild says that the object cache and the runtime's cached objects are not read: every
	// compile runs again (gocpp build -a). The new objects are stored.
	Rebuild bool

	// EmitLLVM is "ll" or "bc" when the build also writes the LLVM IR of every compile of a
	// translation unit (clang only), into LLVMDir (gocpp build -emit-llvm).
	EmitLLVM string
	LLVMDir  string

	sc *statCache // the stats of this build's object cache lookups (Link)

	// LTO builds the runtime and the generated C++ with link-time optimization (-flto: gocpp
	// build -lto); the runtime's library is then made with the archiver that belongs to the
	// compiler (FindArchiver), and a build fails when there is none.
	LTO bool

	ar       *archiverResult // the archiver of this build (archiver)
	pchFresh sync.Map        // precompiled headers whose files were checked in this build (pch.go)

	pchDone chan struct{}  // closed when the precompiled header that startPCH started is built
	stores  sync.WaitGroup // the stores of the object cache that are running (objcache.go)
	traceMu sync.Mutex
}

// A CompileError is a compile or a link that failed. For the generated
// C++ and the runtime, without flags of the user, it is a gocpp bug; for a
// C or C++ file of the package (Source), an error of the user.
type CompileError struct {
	What    string  // what was compiled: "the generated C++", "the runtime", "the C file a.c"
	Source  string  // the package's C or C++ file that didn't compile (absolute), or ""
	Include string  // for the generated C++: the runtime's include folder (its headers are gocpp's), or ""
	Link    bool    // the link failed, not a compile
	Lib     *CxxLib // for a source of a library of a module: the library (Phase 06 step 6.1d); Source is the file
	PkgPath string  // the import path of the package whose unit or source failed (Link), or ""
	Unit    string  // the generated translation unit that did not compile (Link), or ""
	LibLine int     // and LibText the line of the recipe that listed it (LibLine is its line in gocpp.mod)
	LibText string
	Command string // the command, as Trace shows it
	Output  []byte // what the compiler printed (stdout and stderr in one stream)
	Err     error  // how it failed: an *exec.ExitError, a signal, ...
}

func (e *CompileError) Error() string {
	verb := "did not compile"
	if e.Link {
		verb = "did not link"
	}
	return fmt.Sprintf("%s %s: %v", e.What, verb, e.Err)
}

func (e *CompileError) Unwrap() error { return e.Err }

// maxCompileOutput limits what is kept of a compiler's output.
const maxCompileOutput = 4 << 20

// Program compiles the C++ file src (the output of gocpp's emitter) and
// the package's C and C++ files (CFiles, CXXFiles), and links them with
// the runtime into the program out. Both paths are absolute. A compile or
// link that fails is a *CompileError. It is Link for a program of one
// package with one translation unit.
func (b *Builder) Program(ctx context.Context, src, out string) error {
	pb := PackageBuild{Dir: b.Dir, Flags: b.Package, CFiles: b.CFiles, CXXFiles: b.CXXFiles, QuoteDir: b.PackageDir,
		Units: []string{src}, SourceWarnings: b.SourceWarnings}
	return b.Link(ctx, []PackageBuild{pb}, out)
}

// runGenerated compiles the generated C++ with args, with the precompiled header pc when there
// is a ready one (it may be nil), and reports whether it did. When the compiler refuses the
// header, it compiles again without it and removes the header if that works. The error can be
// errOutputOpen (see runOutput).
func (b *Builder) runGenerated(ctx context.Context, pc *pch, args []string) (out []byte, usedPCH bool, err error) {
	if pc == nil || !pc.ready() {
		out, err = b.runOutput(ctx, b.CXX, "the generated C++", false, args)
		return out, false, err
	}
	withPCH := slices.Insert(slices.Clone(args), slices.Index(args, "-c"), pc.args()...)
	out, err = b.runOutput(ctx, b.CXX, "the generated C++", false, withPCH)
	var ce *CompileError
	if errors.As(err, &ce) && (pchRefused(ce.Output, pc.file) || compilerDied(ce)) {
		// The compiler may have refused the header (it is out of date) or died on it (a
		// header that is damaged): compile again without it, and remove it if that works
		// (also when its output was cut short).
		died := compilerDied(ce)
		out, err := b.runOutput(ctx, b.CXX, "the generated C++", false, args)
		if err == nil || errors.Is(err, errOutputOpen) {
			if died {
				b.trace("# the compiler died with the precompiled header: removed, compiled without it")
			}
			pc.remove()
		}
		return out, false, err
	}
	return out, true, err
}

// runtime returns the folder of the runtime and its object files for the
// compile flags flags, compiling them first if they are not in the cache. With
// testing, the object files of package testing's natives (rtfiles.TestingSources)
// come too, in a set of their own in the cache: a program that does not import
// testing has none of them.
func (b *Builder) runtime(ctx context.Context, flags []string, testing bool) (dir string, objs []string, err error) {
	srcs := rtfiles.Sources(b.GOOS)
	var tsrcs []string
	if testing {
		tsrcs = rtfiles.TestingSources(b.GOOS)
	}
	if b.Cache != "" {
		if dir, err = ExtractRuntime(b.Cache); err == nil && (b.DryRun || b.Rebuild) {
			// The runtime is compiled again, or only listed: into the work folder, not the cache.
			b.Plan.setRuntime(dir)
			b.recordRuntime(dir, srcs, flags)
			objDir := filepath.Join(b.Work, "rtobj")
			if err := os.Mkdir(objDir, 0o755); err != nil {
				return "", nil, err
			}
			objs, err = b.compileRuntime(ctx, dir, objDir, srcs, flags)
			return dir, objs, err
		}
		if err == nil {
			b.Plan.setRuntime(dir)
			b.recordRuntime(dir, srcs, flags)
			b.startPCH(ctx, dir) // while the runtime's object files compile
			// The runtime as one static library (rtlib.go); its object files when there is no
			// archiver (without LTO, which needs the matching one). Package testing's natives
			// (tsrcs) are never in the library: their global operator new and delete must be in a
			// program that imports testing and in no other, and an archive member is taken only
			// for a symbol that something wants. They are object files of a set of their own, in
			// front of the library (they use it).
			a, aerr := b.rtArchiver(ctx)
			switch {
			case aerr == nil && !noRuntimeLib() && !b.Mode.Archive():
				var lib string
				if lib, err = b.cachedLib(ctx, dir, srcs, flags, a); err == nil {
					var more []string
					if len(tsrcs) > 0 {
						more, err = b.cachedObjects(ctx, dir, tsrcs, flags)
					}
					if err == nil {
						return dir, append(more, lib), nil
					}
				}
			case aerr != nil && (b.ltoOn() || os.Getenv("GOCPP_AR") != ""):
				// LTO objects can't go without the matching archiver, and a GOCPP_AR that
				// names nothing usable is the user's mistake: say so.
				return "", nil, aerr
			default:
				if objs, err = b.cachedObjects(ctx, dir, srcs, flags); err == nil {
					if len(tsrcs) == 0 {
						return dir, objs, nil
					}
					var more []string
					if more, err = b.cachedObjects(ctx, dir, tsrcs, flags); err == nil {
						return dir, append(objs, more...), nil
					}
				}
			}
			var ce *CompileError
			if errors.As(err, &ce) || ctx.Err() != nil {
				return "", nil, err
			}
		}
		// The cache folder can't be written (a read-only home, a sandbox):
		// go on without it.
	}
	dir = filepath.Join(b.Work, "rt")
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", nil, err
	}
	if err := WriteRuntime(dir); err != nil {
		return "", nil, fmt.Errorf("extracting the runtime: %w", err)
	}
	b.Plan.setRuntime(dir)
	b.recordRuntime(dir, srcs, flags)
	objDir := filepath.Join(b.Work, "rtobj")
	if err := os.Mkdir(objDir, 0o755); err != nil {
		return "", nil, err
	}
	if objs, err = b.compileRuntime(ctx, dir, objDir, slices.Concat(srcs, tsrcs), flags); err != nil {
		return "", nil, err
	}
	return dir, objs, nil
}

// cachedObjects returns the runtime's object files for flags from
// dir/obj/<key>, compiling them into it first if they are not there.
func (b *Builder) cachedObjects(ctx context.Context, dir string, srcs, flags []string) ([]string, error) {
	objRoot := filepath.Join(dir, "obj")
	final := filepath.Join(objRoot, b.runtimeKey(srcs, flags))
	objs := make([]string, len(srcs))
	for i, s := range srcs {
		objs[i] = filepath.Join(final, runtimeObjectName(s))
	}
	if allExist(objs) {
		return objs, nil
	}
	if err := os.MkdirAll(objRoot, 0o755); err != nil {
		return nil, err
	}
	removeStale(objRoot)
	tmp, err := os.MkdirTemp(objRoot, ".tmp-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// -x shows the steps as shell lines that work when pasted.
	b.trace("mkdir -p " + quoteWord(tmp))
	if _, err := b.compileRuntime(ctx, dir, tmp, srcs, flags); err != nil {
		return nil, err
	}
	b.trace("mv " + quoteWord(tmp) + " " + quoteWord(final))
	if err := fsretry.Rename(tmp, final); err != nil {
		// Another gocpp compiled them at the same time and won, or final
		// is broken (a file is missing): move it aside, as another gocpp
		// may be doing the same, and put ours in its place.
		if allExist(objs) {
			return objs, nil
		}
		old, err := os.MkdirTemp(objRoot, ".old-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(old)
		fsretry.Rename(final, filepath.Join(old, "obj"))
		if err := fsretry.Rename(tmp, final); err != nil && !allExist(objs) {
			return nil, err
		}
	}
	return objs, nil
}

// allExist reports whether all the files exist.
func allExist(files []string) bool {
	for _, f := range files {
		if fi, err := os.Stat(f); err != nil || !fi.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// compileRuntime compiles the runtime sources srcs (paths of
// rtfiles.Sources) of the runtime folder dir into objDir, at the same
// time, and returns the object files.
func (b *Builder) compileRuntime(ctx context.Context, dir, objDir string, srcs, flags []string) ([]string, error) {
	objs := make([]string, len(srcs))
	acts := make([]action, len(srcs))
	for i, s := range srcs {
		objs[i] = filepath.Join(objDir, runtimeObjectName(s))
		args := runtimeArgs(dir, s, flags, objs[i])
		acts[i] = action{name: s, run: func() (func(), error) {
			b.Stats.runtimeCompiled()
			return nil, b.run(ctx, b.CXX, "the runtime", false, args)
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

// keyEnv are the environment variables, besides the compiler and the
// flags, that change the runtime's object files: the include paths, and
// the extra flags and hardening of a Nix compiler wrapper (whose
// variables have a suffix for the target: NIX_CFLAGS_COMPILE_FOR_TARGET),
// and the deployment targets of Apple's compilers.
func keyEnv(name string) bool {
	switch name {
	case "CPATH", "CPLUS_INCLUDE_PATH", "C_INCLUDE_PATH", "GCC_EXEC_PREFIX", "COMPILER_PATH",
		"SDKROOT", "DEVELOPER_DIR", "CCC_ADD_ARGS", "CCC_OVERRIDE_OPTIONS",
		"MACOSX_DEPLOYMENT_TARGET", "IPHONEOS_DEPLOYMENT_TARGET", "TVOS_DEPLOYMENT_TARGET", "WATCHOS_DEPLOYMENT_TARGET":
		return true
	}
	// The Nix wrapper script also reads NIX_ENFORCE_NO_NATIVE (it drops
	// -march=native) and NIX_ENFORCE_PURITY, and its NIX_CC_WRAPPER_* and
	// NIX_BINTOOLS_* variables say which wrappers already added their flags.
	return strings.HasPrefix(name, "NIX_CFLAGS_COMPILE") || strings.HasPrefix(name, "NIX_HARDENING_ENABLE") ||
		strings.HasPrefix(name, "NIX_CXXSTDLIB_COMPILE") || strings.HasPrefix(name, "NIX_ENFORCE_") ||
		strings.HasPrefix(name, "NIX_CC_WRAPPER_") || strings.HasPrefix(name, "NIX_BINTOOLS_")
}

// KeyEnv returns the environment variables that change what the compiler
// does with include paths and extra flags (CPATH, CPLUS_INCLUDE_PATH,
// NIX_CFLAGS_COMPILE*, ...) as sorted KEY=value entries, for the keys of
// caches of the compiler's results (the runtime's object files, the probe
// results of internal/interop).
func KeyEnv() []string {
	return environ(keyEnv)
}

// runtimeKey returns the name of the folder of the runtime's object files
// for flags: a hash of the compiler (path, arguments, family, version and
// the identity of its program file),
// the flags, the sources and the environment that keyEnv picks.
func (b *Builder) runtimeKey(srcs, flags []string) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			fmt.Fprintf(h, "%d:%s\x00", len(p), p)
		}
		h.Write([]byte{'\n'})
	}
	writeCompiler(write, b.CXX)
	write(flags...)
	write(srcs...)
	write(KeyEnv()...)
	if salt := PluginSalt(slices.Concat(b.CXX.Args, flags)); len(salt) > 0 {
		write(salt...) // the compiler plugins that the flags load, by the hash of their files
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// writeCompiler writes what tells compilers apart to a key: the path, the arguments, the
// family, the version and the identity of the program file.
func writeCompiler(write func(parts ...string), c CXX) {
	write(c.Path)
	write(c.Args...)
	write(c.Family.String(), c.Version)
	if c.GOOS != "" {
		write("target", c.GOOS, c.GOARCH) // a cross compiler; the host's keys stay as they were
	}
	st := c.stamp
	write(st.real, fmt.Sprint(st.size), fmt.Sprint(st.modTime))
	// The size and the time can be set back after the file was replaced: its content tells too
	// (hashed once for the stat of the file: fileContentID).
	if st.real != "" {
		write(fileContentID(st.real))
	}
}

// environ returns the entries NAME=value of the environment whose name keep accepts, sorted.
func environ(keep func(name string) bool) []string {
	var env []string
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); keep(name) {
			env = append(env, kv)
		}
	}
	slices.Sort(env)
	return env
}

// run runs the compiler c with args in Work. what and link describe the
// step for a CompileError. On success, what the compiler printed (its
// warnings) goes to Stderr.
func (b *Builder) run(ctx context.Context, c CXX, what string, link bool, args []string) error {
	out, err := b.runOutput(ctx, c, what, link, args)
	b.warn(out)
	if errors.Is(err, errOutputOpen) {
		return nil
	}
	return err
}

// warn writes what a compiler printed when it succeeded to Stderr.
func (b *Builder) warn(out []byte) {
	if len(out) > 0 && b.Stderr != nil {
		b.traceMu.Lock()
		b.Stderr.Write(out)
		b.traceMu.Unlock()
	}
}

// waitDelay is how long a run of the compiler waits for its output after it exited: a process
// that the compiler started and that keeps the output open must not make gocpp wait. On a
// machine under load the goroutines that copy the output can need seconds, so it is not short.
var waitDelay = 5 * time.Second

// retryWaitDelay is waitDelay for the second run of a command whose output is data (the version
// of the compiler, the flags of pkg-config), after the first run exited but its output was not
// complete in time: that output can't be used, so the run is repeated and waited for longer.
var retryWaitDelay = 30 * time.Second

// errOutputOpen is runOutput's error for a compiler that exited with a success status, but
// whose output was still open (a process that it started, or a slow machine) when waitDelay
// ran out: the compile succeeded, and the output that comes with the error may be incomplete.
// It is not a failure: callers that don't care about the difference use b.run or compileCached,
// which don't return it.
var errOutputOpen = errors.New("the compiler exited, but its output was not complete in time")

// outputCut reports whether err, the result of cmd.Wait, says that the command exited with a
// success status but its output was not complete when WaitDelay ran out (os/exec returns
// ErrWaitDelay only for a command that otherwise succeeded).
func outputCut(cmd *exec.Cmd, err error) bool {
	return errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success()
}

// runOutput is run, but it returns what the compiler printed when it
// succeeds instead of writing it to Stderr. When the compiler succeeded but
// its output was cut short (see errOutputOpen), it returns what it got and errOutputOpen.
func (b *Builder) runOutput(ctx context.Context, c CXX, what string, link bool, args []string) ([]byte, error) {
	return b.runCommand(ctx, c, what, link, args, args)
}

// runCommand is runOutput that shows the command (-x, and CompileError.Command) as shown, not as
// args: the link of a recorded build has -Wl,-t, which the user did not ask for.
func (b *Builder) runCommand(ctx context.Context, c CXX, what string, link bool, args, shown []string) ([]byte, error) {
	show := b.show(c, shown)
	b.trace(show)
	if b.DryRun {
		return nil, nil
	}
	if assemblerNeedsFix(c, args) {
		return b.runViaFixedAssembly(ctx, c, what, link, args, show)
	}
	return b.execCommand(ctx, c, what, link, args, show)
}

// execCommand runs the compiler c with args and returns what it printed (see runCommand); a
// failure is a CompileError that names show as the command.
func (b *Builder) execCommand(ctx context.Context, c CXX, what string, link bool, args []string, show string) ([]byte, error) {
	out := &limitBuffer{max: maxCompileOutput}
	args, doneRsp, err := b.fitCommand(c, args) // a command too long for Windows goes into a response file
	if err != nil {
		return nil, err
	}
	defer doneRsp()
	cmd, err := Start(ctx, func() *exec.Cmd {
		cmd := c.Command(ctx, args...)
		cmd.Dir = b.Work
		cmd.Stdout, cmd.Stderr = out, out
		// A process that the compiler started and that keeps the output
		// open must not make gocpp wait.
		cmd.WaitDelay = waitDelay
		InGroup(cmd)
		return cmd
	})
	if err == nil {
		err = cmd.Wait()
		KillGroup(cmd) // whatever the compiler left running
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if outputCut(cmd, err) {
		return out.buf.Bytes(), errOutputOpen
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) && cmd.ProcessState == nil {
			return nil, fmt.Errorf("running the %s %s: %w", c.kind(), c.Name(), err)
		}
		return nil, &CompileError{What: what, Link: link, Command: show, Output: out.buf.Bytes(), Err: err}
	}
	return out.buf.Bytes(), nil
}

// show returns the command of the compiler c with args as a shell line,
// with Work written as $WORK.
func (b *Builder) show(c CXX, args []string) string {
	words := append([]string{c.Path}, c.Args...)
	words = append(words, args...)
	var s strings.Builder
	for i, w := range words {
		if i > 0 {
			s.WriteByte(' ')
		}
		// The prefix map of the object cache names the work folder: written as $WORK too.
		if rest, ok := strings.CutPrefix(w, "-ffile-prefix-map="+b.Work+"="); ok && b.Work != "" {
			s.WriteString("-ffile-prefix-map=$WORK=" + quoteWord(rest))
			continue
		}
		s.WriteString(ShellQuoteWork(w, b.Work))
	}
	return s.String()
}

// trace writes the line to Trace, if it is set.
func (b *Builder) trace(line string) {
	if b.Trace == nil {
		return
	}
	b.traceMu.Lock()
	defer b.traceMu.Unlock()
	fmt.Fprintln(b.Trace, line)
}

// ShellQuoteWork returns the word w for a shell line of -x: quoted when it
// needs it, and with the work folder work at its start written as $WORK,
// like go build -x does.
func ShellQuoteWork(w, work string) string {
	if work != "" && w == work {
		return "$WORK"
	}
	if work != "" && strings.HasPrefix(w, work+string(filepath.Separator)) {
		return "$WORK" + quoteWord(w[len(work):])
	}
	return quoteWord(w)
}

// quoteWord quotes s for a POSIX shell when it has characters that the
// shell would treat specially.
func quoteWord(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=+,:@%^", r))
	}) < 0 {
		return s
	}
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// MoveFile moves the file from to the path to, replacing it: with a
// rename, or a copy when they are on different file systems. The file
// keeps its mode. When to exists and isn't a regular file (a device such
// as /dev/null, a named pipe), the program is written into it, as `go
// build -o /dev/null` does, and from is removed.
func MoveFile(from, to string) error {
	if fi, err := os.Stat(to); err == nil && !fi.Mode().IsRegular() && !fi.IsDir() {
		return writeInto(from, to)
	}
	// A read-only output (on Windows the attribute stops a rename onto the file; on Linux the folder decides) is replaced, as cmd/go does.
	makeWritable(to)
	if err := fsretry.Rename(from, to); err == nil {
		return nil
	}
	fi, err := os.Stat(from)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	// Write next to to and rename, so that a failure doesn't leave half a
	// program there.
	tmp, err := os.CreateTemp(filepath.Dir(to), "."+filepath.Base(to)+".tmp-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmp.Name(), fi.Mode().Perm()); err != nil {
			return err
		}
	}
	return replaceError(fsretry.Rename(tmp.Name(), to), to)
}

// writeInto writes the file from into to, which exists and isn't a
// regular file (MoveFile), and removes from.
func writeInto(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(to, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(from)
}
