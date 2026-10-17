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
	// flags, and Link to the link, after the object files. Until Phase 06,
	// a program is one package.
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

	traceMu sync.Mutex
}

// A CompileError is a compile or a link that failed. For the generated
// C++ and the runtime, without flags of the user, it is a gocpp bug; for a
// C or C++ file of the package (Source), an error of the user.
type CompileError struct {
	What    string // what was compiled: "the generated C++", "the runtime", "the C file a.c"
	Source  string // the package's C or C++ file that didn't compile (absolute), or ""
	Include string // for the generated C++: the runtime's include folder (its headers are gocpp's), or ""
	Link    bool   // the link failed, not a compile
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
// link that fails is a *CompileError.
func (b *Builder) Program(ctx context.Context, src, out string) error {
	// With flags of the environment, the runtime's compiles get gocpp's
	// language flags after them too (as the generated C++ does), then
	// -Wno-error (valid code must not fail for a warning that a user's
	// -Werror -Wshadow turns into an error) and -x c++, which comes right
	// before each runtime source. All of them are part of the key of the
	// runtime's object files.
	flags := CompileFlags(b.CXX, b.ASan, b.NoChecks)
	if env := b.Env.CXXCompile(); len(env) > 0 {
		flags = slices.Concat(flags, env, LanguageFlags(), []string{"-Wno-error", "-x", "c++"})
	}
	rtDir, rtObjs, rtErr := b.runtime(ctx, flags)
	if rtErr != nil {
		return rtErr
	}
	obj := filepath.Join(b.Work, strings.TrimSuffix(filepath.Base(src), ".cpp")+".o")
	// The runtime's include folder comes before the package's -I folders,
	// so that no package folder can hide a gocpp/ header. After the flags
	// of the environment and of the package come gocpp's language flags
	// and those of generated C++ again, and -x c++ right before the source,
	// so that no flag of a package (-std=c++17, -ffast-math, -x c) turns
	// them off: for each of them the last one wins. -Wno-error comes before
	// the flags of generated C++: a package's -Werror -Wshadow must not make
	// valid Go fail (for i := ...; { i := i }), and the -Werror= of
	// GeneratedFlags (g++'s conditionally-supported and narrowing) stays an error.
	// Without such flags the command stays short (-x shows it).
	var quote []string // -iquote: PackageDir
	if b.PackageDir != "" {
		quote = []string{"-iquote", b.PackageDir}
	}
	compile := slices.Concat(CompileFlags(b.CXX, b.ASan, b.NoChecks), GeneratedFlags(b.CXX), quote, []string{"-I", filepath.Join(rtDir, "include"), "-c", src, "-o", obj})
	if user := slices.Concat(b.Env.CXXCompile(), b.Package.Compile); len(user) > 0 {
		compile = slices.Concat(CompileFlags(b.CXX, b.ASan, b.NoChecks), quote, []string{"-I", filepath.Join(rtDir, "include")},
			user, LanguageFlags(), []string{"-Wno-error"}, GeneratedFlags(b.CXX), []string{"-c", "-x", "c++", src, "-o", obj})
	}
	// The package's C and C++ files compile at the same time. An error in
	// them is reported first: it is the user's.
	var srcObjs []string
	srcErr := make(chan error, 1)
	go func() {
		var err error
		srcObjs, err = b.compileSources(ctx, rtDir)
		srcErr <- err
	}()
	genErr := b.run(ctx, b.CXX, "the generated C++", false, compile)
	if err := <-srcErr; err != nil {
		return err
	}
	if genErr != nil {
		var ce *CompileError
		if errors.As(genErr, &ce) {
			ce.Include = filepath.Join(rtDir, "include")
		}
		return genErr
	}
	// The libraries after the object files that use them, as a linker
	// that reads its inputs once (GNU ld with static libraries) needs.
	link := slices.Concat(Flags(b.CXX, b.ASan), []string{obj}, srcObjs, rtObjs, b.Package.Link, b.Env.LDFlags, []string{"-o", out})
	return b.run(ctx, b.CXX, "the generated C++", true, link)
}

// runtime returns the folder of the runtime and its object files for the
// compile flags flags, compiling them first if they are not in the cache.
func (b *Builder) runtime(ctx context.Context, flags []string) (dir string, objs []string, err error) {
	srcs := rtfiles.Sources(b.GOOS)
	if b.Cache != "" {
		if dir, err = ExtractRuntime(b.Cache); err == nil {
			if objs, err = b.cachedObjects(ctx, dir, srcs, flags); err == nil {
				return dir, objs, nil
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
	objDir := filepath.Join(b.Work, "rtobj")
	if err := os.Mkdir(objDir, 0o755); err != nil {
		return "", nil, err
	}
	if objs, err = b.compileRuntime(ctx, dir, objDir, srcs, flags); err != nil {
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
	if err := os.Rename(tmp, final); err != nil {
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
		os.Rename(final, filepath.Join(old, "obj"))
		if err := os.Rename(tmp, final); err != nil && !allExist(objs) {
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
	errs := make([]error, len(srcs))
	var wg sync.WaitGroup
	for i, s := range srcs {
		objs[i] = filepath.Join(objDir, runtimeObjectName(s))
		args := slices.Concat(flags, []string{"-I", filepath.Join(dir, "include"), "-c", filepath.Join(dir, filepath.FromSlash(s)), "-o", objs[i]})
		wg.Go(func() { errs[i] = b.run(ctx, b.CXX, "the runtime", false, args) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err // the first one: one compile error is enough
		}
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
	var env []string
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); keyEnv(name) {
			env = append(env, kv)
		}
	}
	slices.Sort(env)
	return env
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
	write(b.CXX.Path)
	write(b.CXX.Args...)
	write(b.CXX.Family.String(), b.CXX.Version)
	st := b.CXX.stamp
	write(st.real, fmt.Sprint(st.size), fmt.Sprint(st.modTime))
	write(flags...)
	write(srcs...)
	write(KeyEnv()...)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// run runs the compiler c with args in Work. what and link describe the
// step for a CompileError. On success, what the compiler printed (its
// warnings) goes to Stderr.
func (b *Builder) run(ctx context.Context, c CXX, what string, link bool, args []string) error {
	out, err := b.runOutput(ctx, c, what, link, args)
	if len(out) > 0 && b.Stderr != nil {
		b.traceMu.Lock()
		b.Stderr.Write(out)
		b.traceMu.Unlock()
	}
	return err
}

// runOutput is run, but it returns what the compiler printed when it
// succeeds instead of writing it to Stderr.
func (b *Builder) runOutput(ctx context.Context, c CXX, what string, link bool, args []string) ([]byte, error) {
	show := b.show(c, args)
	b.trace(show)
	out := &limitBuffer{max: maxCompileOutput}
	cmd, err := Start(ctx, func() *exec.Cmd {
		cmd := c.Command(ctx, args...)
		cmd.Dir = b.Work
		cmd.Stdout, cmd.Stderr = out, out
		// A process that the compiler started and that keeps the output
		// open must not make gocpp wait.
		cmd.WaitDelay = time.Second
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
	if err := os.Rename(from, to); err == nil {
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
	return os.Rename(tmp.Name(), to)
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
