package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// A Family is a family of C++ compilers. Some flags work with only one
// family (g++ rejects clang's -Qunused-arguments), so flags are chosen by
// family.
type Family int

const (
	// Clang is clang++ and the compilers built on it: a compiler that
	// defines __clang__.
	Clang Family = iota + 1

	// GCC is g++: a compiler that defines __GNUC__ but not __clang__
	// (clang defines __GNUC__ too).
	GCC
)

// String returns "clang" or "gcc".
func (f Family) String() string {
	switch f {
	case Clang:
		return "clang"
	case GCC:
		return "gcc"
	}
	return fmt.Sprintf("Family(%d)", int(f))
}

// A CXX is a C++ compiler, as FindCXX or LookCXX found it.
type CXX struct {
	// Path is the absolute path of the compiler program, as it was found
	// in PATH (symbolic links are not resolved).
	Path string

	// Family is Clang or GCC.
	Family Family

	// Version is the first line that the compiler prints for --version,
	// such as "clang version 21.1.8" or "g++ (GCC) 15.3.0". Caches use it,
	// with Path and Family, to tell compilers apart.
	Version string
}

// The programs that FindCXX tries when neither GOCPP_CXX nor CXX is set,
// in this order.
var defaultCXX = []string{"clang++", "g++"}

// FindCXX finds the C++ compiler that gocpp uses: the program in
// GOCPP_CXX, else the program in CXX, else clang++, else g++. GOCPP_CXX
// and CXX hold one program: a name, which is looked up in PATH, or a
// path (for now without arguments: Phase 02 decides about "ccache g++").
// When one of them is set, FindCXX uses its program or fails: it doesn't
// go on to the next one, so a compiler that was chosen is never swapped
// for another one without a word. Without them, a clang++ that doesn't
// work (LookCXX fails) is skipped for g++.
//
// FindCXX only finds the compiler and its family; Phase 02 adds the
// version checks.
func FindCXX(ctx context.Context) (CXX, error) {
	for _, env := range []string{"GOCPP_CXX", "CXX"} {
		if prog := os.Getenv(env); prog != "" {
			cxx, err := LookCXX(ctx, prog)
			if err != nil {
				return CXX{}, fmt.Errorf("%s: %w", env, err)
			}
			return cxx, nil
		}
	}
	var problems []string
	for _, prog := range defaultCXX {
		if _, err := exec.LookPath(prog); err != nil {
			continue
		}
		cxx, err := LookCXX(ctx, prog)
		if err == nil {
			return cxx, nil
		}
		if ctx.Err() != nil {
			return CXX{}, err
		}
		problems = append(problems, err.Error())
	}
	if problems == nil {
		return CXX{}, errors.New("no C++ compiler found: clang++ and g++ are not in PATH (GOCPP_CXX can name the compiler to use)")
	}
	return CXX{}, fmt.Errorf("no working C++ compiler found (GOCPP_CXX can name the compiler to use): %s", strings.Join(problems, "; "))
}

// LookCXX returns the C++ compiler prog: a program name, which is looked
// up in PATH, or a path. It runs the compiler twice: with --version, for
// CXX.Version, and with -dM -E, to see whether it defines __clang__ or
// __GNUC__. The answer is cached for the program file, so another call
// runs nothing while the file stays the same.
func LookCXX(ctx context.Context, prog string) (CXX, error) {
	path, err := exec.LookPath(prog)
	if err != nil {
		if strings.ContainsAny(prog, " \t") {
			return CXX{}, fmt.Errorf("the C++ compiler %q was not found (it must be one program, without arguments, for now): %w", prog, err)
		}
		return CXX{}, fmt.Errorf("the C++ compiler %q was not found: %w", prog, err)
	}
	// A relative path ("./bin/c++") must not change meaning when the
	// compiler later runs in another folder.
	if path, err = filepath.Abs(path); err != nil {
		return CXX{}, fmt.Errorf("the C++ compiler %q: %w", prog, err)
	}
	return lookPath(ctx, path)
}

// detectTimeout limits each run of the compiler in LookCXX, also when ctx
// has no deadline: a compiler that hangs must not hang gocpp.
const detectTimeout = 30 * time.Second

// maxDetectOutput limits what LookCXX reads from one run of the compiler.
// -dM -E prints about 20 KiB.
const maxDetectOutput = 1 << 20

// A fileStamp tells whether a program file is still the same: its real
// path (with symbolic links resolved, so that a profile link that now
// points at another compiler is noticed), size and modification time.
type fileStamp struct {
	real    string
	size    int64
	modTime int64 // UnixNano
}

func stampOf(path string) (fileStamp, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fileStamp{}, err
	}
	fi, err := os.Stat(real)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{real: real, size: fi.Size(), modTime: fi.ModTime().UnixNano()}, nil
}

// detected caches what lookPath found, by program path.
var detected = struct {
	sync.Mutex
	m map[string]detection
}{m: make(map[string]detection)}

type detection struct {
	stamp fileStamp
	cxx   CXX
}

// lookPath returns the compiler at the absolute path path, from the cache
// when the file hasn't changed.
func lookPath(ctx context.Context, path string) (CXX, error) {
	stamp, err := stampOf(path)
	if err != nil {
		return CXX{}, fmt.Errorf("the C++ compiler %s: %w", path, err)
	}
	detected.Lock()
	d, ok := detected.m[path]
	detected.Unlock()
	if ok && d.stamp == stamp {
		return d.cxx, nil
	}
	cxx, err := detect(ctx, path)
	if err != nil {
		return CXX{}, err
	}
	detected.Lock()
	detected.m[path] = detection{stamp: stamp, cxx: cxx}
	detected.Unlock()
	return cxx, nil
}

// detect runs the compiler at path to find out its version line and its
// family.
func detect(ctx context.Context, path string) (CXX, error) {
	out, err := runDetect(ctx, path, "--version")
	if err != nil {
		return CXX{}, err
	}
	version := firstLine(out)
	if version == "" {
		return CXX{}, fmt.Errorf("the C++ compiler %s printed nothing for --version", path)
	}
	// "-x c++ -" reads an empty C++ file from stdin (exec gives the
	// program the null device), so this works without a temporary file.
	out, err = runDetect(ctx, path, "-dM", "-E", "-x", "c++", "-")
	if err != nil {
		return CXX{}, err
	}
	family := familyOf(out)
	if family == 0 {
		return CXX{}, fmt.Errorf("the C++ compiler %s (%s) is neither clang nor gcc: it defines neither __clang__ nor __GNUC__", path, version)
	}
	return CXX{Path: path, Family: family, Version: version}, nil
}

// runDetect runs the compiler at path with args and returns what it
// printed on stdout. It runs with CompilerEnv, so that the version line is
// the same in every locale and nothing writes files.
func runDetect(ctx context.Context, path string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = CompilerEnv()
	stdout := &limitBuffer{max: maxDetectOutput}
	stderr := &limitBuffer{max: maxDetectOutput}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// If a process that the compiler started keeps stdout open, stop
	// waiting for it soon after the compiler exits.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	show := strings.Join(args, " ")
	switch {
	case ctx.Err() != nil:
		return nil, fmt.Errorf("the C++ compiler %s: %s: %w", path, show, ctx.Err())
	case err != nil:
		if msg := firstLine(stderr.buf.Bytes()); msg != "" {
			return nil, fmt.Errorf("the C++ compiler %s: %s failed: %v: %s", path, show, err, msg)
		}
		return nil, fmt.Errorf("the C++ compiler %s: %s failed: %v", path, show, err)
	case stdout.full:
		return nil, fmt.Errorf("the C++ compiler %s: %s printed more than %d bytes", path, show, maxDetectOutput)
	}
	return stdout.buf.Bytes(), nil
}

// dropEnv are the variables that CompilerEnv leaves out: they make gcc or
// clang write files of their own (dependency lists, logs) or change the
// messages (LANGUAGE picks a translation; LC_ALL=C should win anyway).
var dropEnv = map[string]bool{
	"DEPENDENCIES_OUTPUT": true, "SUNPRO_DEPENDENCIES": true, // gcc
	"CC_PRINT_HEADERS": true, "CC_PRINT_OPTIONS": true, "CC_LOG_DIAGNOSTICS": true, // clang
	"GCC_EXTRA_DIAGNOSTIC_OUTPUT": true, "LANGUAGE": true,
}

// CompilerEnv returns the environment for a run of the C++ compiler whose
// output gocpp reads: the process's own, which the compiler needs (PATH,
// and the include paths in CPATH or a Nix wrapper's NIX_CFLAGS_COMPILE; a
// clean environment would lose them), without the variables that make the
// compiler write files of its own or translate its messages, and with
// LC_ALL=C, so that the messages are the same in every locale. A caller
// can append more KEY=value entries; os/exec keeps the last one of a key.
func CompilerEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if runtime.GOOS == "windows" {
			name = strings.ToUpper(name)
		}
		if !dropEnv[name] {
			env = append(env, kv)
		}
	}
	return append(env, "LC_ALL=C")
}

// familyOf returns the compiler family that the macro list out (from -dM
// -E) shows, or 0. clang defines __GNUC__ too, so __clang__ decides.
func familyOf(out []byte) Family {
	var clang, gnuc bool
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] != "#define" {
			continue
		}
		switch f[1] {
		case "__clang__":
			clang = true
		case "__GNUC__":
			gnuc = true
		}
	}
	switch {
	case clang:
		return Clang
	case gnuc:
		return GCC
	}
	return 0
}

// firstLine returns the first line of out that isn't blank, without
// space around it.
func firstLine(out []byte) string {
	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// limitBuffer keeps the first max bytes written to it and drops the rest.
type limitBuffer struct {
	buf  bytes.Buffer
	max  int
	full bool
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); len(p) > room {
		b.buf.Write(p[:room])
		b.full = true
		return len(p), nil
	}
	return b.buf.Write(p)
}
