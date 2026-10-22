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
	"slices"
	"strconv"
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

// A CXX is a C++ compiler, as FindCXX or LookCXX found it, or a C
// compiler (C set), as FindCC found it.
type CXX struct {
	// Path is the absolute path of the compiler program, as it was found
	// in PATH (symbolic links are not resolved).
	Path string

	// Args are the arguments that come with the program in GOCPP_CXX or
	// CXX, before gocpp's own: "g++" for CXX="ccache g++". They are
	// usually empty. Every run of the compiler passes them first
	// (Command).
	Args []string

	// Family is Clang or GCC.
	Family Family

	// Version is the first line that the compiler prints for --version,
	// such as "clang version 21.1.8" or "g++ (GCC) 15.3.0". Caches use it,
	// with Path, Args and Family, to tell compilers apart.
	Version string

	// Major is the major version of the compiler, from its predefined
	// macros: __clang_major__ for clang, __GNUC__ for gcc. LookCXX
	// rejects a compiler older than MinClang or MinGCC.
	Major int

	// C says that this is a C compiler (FindCC), not a C++ compiler: it is
	// detected with -x c, and messages call it "the C compiler".
	C bool

	// GOOS and GOARCH are the target of a compiler that FindCXXFor found for another system than
	// the one that runs gocpp ("windows", "amd64"); they are empty for the host's compiler. They
	// choose the target's flags (Flags, LinkFlags) and are part of every cache key.
	GOOS, GOARCH string

	// Source says where FindCXXFor found a Windows target's compiler: "PATH", "MSYS2 ucrt64 (C:\msys64\ucrt64\bin)",
	// "pinned llvm-mingw 20261006 (downloaded)". It is empty for the other compilers; gocpp env shows it.
	Source string

	// stamp is the program file as LookCXX saw it (its real path, size
	// and modification time), for the cache keys of the runtime's object
	// files: a compiler that was replaced by another with the same version
	// line gets new ones. It is zero for a CXX made by hand.
	stamp fileStamp
}

// The oldest compiler versions that gocpp supports (ADR-003): the first
// ones with enough of C++20.
const (
	MinClang = 17
	MinGCC   = 13
)

// Command returns the command that runs the compiler with args, after
// the compiler's own Args. It runs with CompilerEnv.
func (c CXX) Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.Path, c.compilerArgs(append(slices.Clip(c.Args), args...))...) // paths the compiler can read (safeargs.go)
	cmd.Env = CompilerEnv()
	return cmd
}

// Identity returns a string that tells the compiler's program file: its
// real path (symbolic links resolved), size and modification time, as they
// are now. A compiler that was replaced by another with the same path and
// version line has another identity. ok is false when the file can't be
// looked at. A cache key uses it with Path, Args, Family and Version.
func (c CXX) Identity() (id string, ok bool) {
	st, err := stampOf(c.Path)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s\x00%d\x00%d", st.real, st.size, st.modTime), true
}

// kind returns "C compiler" or "C++ compiler", for messages.
func (c CXX) kind() string {
	if c.C {
		return "C compiler"
	}
	return "C++ compiler"
}

// Name returns the compiler as messages show it: the base name of Path,
// with the Args ("ccache g++").
func (c CXX) Name() string {
	return strings.Join(append([]string{filepath.Base(c.Path)}, c.Args...), " ")
}

// The programs that FindCXX tries when neither GOCPP_CXX nor CXX is set,
// in this order.
var defaultCXX = []string{"clang++", "g++"}

// FindCXX finds the C++ compiler that gocpp uses: the compiler in
// GOCPP_CXX, else the compiler in CXX, else clang++, else g++. GOCPP_CXX
// and CXX hold a program, a name that is looked up in PATH or a path,
// and maybe arguments after it: CXX="ccache g++". They are split into
// words like the go command splits CC: at spaces, and a word can be put
// in single or double quotes (with no escapes inside), so that a path
// with spaces can be named ('C:\Program Files\LLVM\bin\clang++.exe').
// When one of them is set, FindCXX uses its compiler or fails: it doesn't
// go on to the next one, so a compiler that was chosen is never swapped
// for another one without a word. Without them, a clang++ that doesn't
// work or is too old (LookCXX fails) is skipped for g++.
func FindCXX(ctx context.Context) (CXX, error) {
	for _, env := range []string{"GOCPP_CXX", "CXX"} {
		if cxx, ok, err := cxxFromEnv(ctx, env); ok {
			return cxx, err
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

// SplitCommand splits the value of GOCPP_CXX or CXX into words, like the
// go command splits CC (cmd/internal/quoted.Split): at spaces, tabs and
// newlines, and a word in single or double quotes keeps its spaces. There
// are no escapes, and a quote inside a word is a normal character.
func SplitCommand(s string) ([]string, error) {
	var words []string
	for {
		s = strings.TrimLeft(s, " \t\n\r")
		if s == "" {
			return words, nil
		}
		if q := s[0]; q == '"' || q == '\'' {
			end := strings.IndexByte(s[1:], q)
			if end < 0 {
				return nil, fmt.Errorf("unterminated %c string", q)
			}
			words = append(words, s[1:1+end])
			s = s[2+end:]
			continue
		}
		end := strings.IndexAny(s, " \t\n\r")
		if end < 0 {
			end = len(s)
		}
		words = append(words, s[:end])
		s = s[end:]
	}
}

// LookCXX returns the C++ compiler prog: a program name, which is looked
// up in PATH, or a path, without arguments. It runs the compiler twice:
// with --version, for CXX.Version, and with -dM -E, to see whether it
// defines __clang__ or __GNUC__, and its major version. A compiler older
// than MinClang or MinGCC is an error that shows its version. The answer
// is cached for the program file, so another call runs nothing while the
// file stays the same.
func LookCXX(ctx context.Context, prog string) (CXX, error) {
	return lookCommand(ctx, prog, nil, false)
}

// lookCommand is LookCXX for the program prog with the arguments args, for
// a C compiler with c.
func lookCommand(ctx context.Context, prog string, args []string, c bool) (CXX, error) {
	kind := CXX{C: c}.kind()
	show := strings.Join(append([]string{prog}, args...), " ")
	path, err := exec.LookPath(prog)
	if err != nil {
		return CXX{}, fmt.Errorf("the %s %q was not found: %w", kind, show, err)
	}
	// A relative path ("./bin/c++") must not change meaning when the
	// compiler later runs in another folder.
	if path, err = filepath.Abs(path); err != nil {
		return CXX{}, fmt.Errorf("the %s %q: %w", kind, show, err)
	}
	if len(args) == 0 {
		args = nil // the same CXX as LookCXX's
	}
	return lookPath(ctx, path, slices.Clone(args), c)
}

// detectTimeout limits each run of the compiler in LookCXX (plus the wait for its output: a run
// that is repeated, see runDetect, has its own), also when ctx
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

// detected caches what lookPath found, by the program path and the
// arguments (commandKey).
var detected = struct {
	sync.Mutex
	m map[string]detection
}{m: make(map[string]detection)}

type detection struct {
	stamp fileStamp
	cxx   CXX
}

// commandKey is the key of a compiler command in caches: the path and the
// arguments, separated by NUL bytes, which none of them can hold.
func commandKey(path string, args []string) string {
	return strings.Join(append([]string{path}, args...), "\x00")
}

// lookPath returns the compiler at the absolute path path, run with the
// arguments args (a C compiler with c), from the cache when the program
// file hasn't changed.
// (With arguments, the program is often a launcher such as ccache, and
// only its file is watched: a cached answer stays while the process
// runs, even if the compiler that it starts changes.)
func lookPath(ctx context.Context, path string, args []string, c bool) (CXX, error) {
	stamp, err := stampOf(path)
	if err != nil {
		return CXX{}, fmt.Errorf("the %s %s: %w", CXX{C: c}.kind(), path, err)
	}
	key := commandKey(path, args)
	if c {
		key = "c\x00" + key
	}
	detected.Lock()
	d, ok := detected.m[key]
	detected.Unlock()
	if ok && d.stamp == stamp {
		return d.cxx, nil
	}
	cxx, found := loadDetected(key, stamp, CXX{Path: path, Args: args, C: c, stamp: stamp})
	if !found {
		cxx, err = detect(ctx, CXX{Path: path, Args: args, C: c, stamp: stamp})
		if err != nil {
			return CXX{}, err
		}
		saveDetected(key, stamp, cxx)
	}
	detected.Lock()
	detected.m[key] = detection{stamp: stamp, cxx: cxx}
	detected.Unlock()
	return cxx, nil
}

// detect runs the compiler c (Path, Args and C set) to find out its version
// line, its family and its major version.
func detect(ctx context.Context, c CXX) (CXX, error) {
	show := c.Path
	if len(c.Args) > 0 {
		show = strings.Join(append([]string{c.Path}, c.Args...), " ")
	}
	out, err := runDetect(ctx, c, show, "--version")
	if err != nil {
		return CXX{}, err
	}
	c.Version = firstLine(out)
	if c.Version == "" {
		return CXX{}, fmt.Errorf("the %s %s printed nothing for --version", c.kind(), show)
	}
	// "-x c++ -" reads an empty C++ file from stdin (exec gives the
	// program the null device), so this works without a temporary file.
	lang := "c++"
	if c.C {
		lang = "c"
	}
	out, err = runDetect(ctx, c, show, "-dM", "-E", "-x", lang, "-")
	if err != nil {
		return CXX{}, err
	}
	c.Family, c.Major = familyOf(out)
	switch {
	case c.Family == 0:
		return CXX{}, fmt.Errorf("the %s %s (%s) is neither clang nor gcc: it defines neither __clang__ nor __GNUC__", c.kind(), show, c.Version)
	case c.Family == Clang && c.Major < MinClang:
		return CXX{}, fmt.Errorf("the %s %s (%s) is too old: gocpp needs clang %d or newer", c.kind(), show, c.Version, MinClang)
	case c.Family == GCC && c.Major < MinGCC:
		return CXX{}, fmt.Errorf("the %s %s (%s) is too old: gocpp needs gcc %d or newer", c.kind(), show, c.Version, MinGCC)
	}
	return c, nil
}

// runDetect runs the compiler c with args and returns what it printed on
// stdout. It runs with CompilerEnv, so that the version line is the same
// in every locale and nothing writes files. show is the compiler for
// messages.
func runDetect(ctx context.Context, c CXX, show string, args ...string) ([]byte, error) {
	var stdout, stderr *limitBuffer
	var cmd *exec.Cmd
	var err error
	var actx context.Context // the context of the last run: its timeout is per run
	// The output is data. When the compiler exited with a success status, but its output was
	// not complete in time (a loaded machine), it can't be used: run it again, and wait longer.
	for _, delay := range []time.Duration{waitDelay, retryWaitDelay} {
		stdout = &limitBuffer{max: maxDetectOutput}
		stderr = &limitBuffer{max: maxDetectOutput}
		var cancel context.CancelFunc
		actx, cancel = context.WithTimeout(ctx, detectTimeout+delay)
		defer cancel()
		cmd, err = Start(actx, func() *exec.Cmd {
			cmd := c.Command(actx, args...)
			cmd.Stdout, cmd.Stderr = stdout, stderr
			// If a process that the compiler started keeps stdout open, stop
			// waiting for it soon after the compiler exits.
			cmd.WaitDelay = delay
			InGroup(cmd) // a process that it left running is killed below, and with the time limit
			return cmd
		})
		if err == nil {
			err = cmd.Wait()
			KillGroup(cmd) // whatever it left running (a process that held the output)
		}
		if !outputCut(cmd, err) {
			break
		}
	}
	what := strings.Join(args, " ")
	switch {
	case actx.Err() != nil:
		return nil, fmt.Errorf("the %s %s: %s: %w", c.kind(), show, what, actx.Err())
	case err != nil:
		if msg := firstLine(stderr.buf.Bytes()); msg != "" {
			return nil, fmt.Errorf("the %s %s: %s failed: %v: %s", c.kind(), show, what, err, msg)
		}
		return nil, fmt.Errorf("the %s %s: %s failed: %v", c.kind(), show, what, err)
	case stdout.full:
		return nil, fmt.Errorf("the %s %s: %s printed more than %d bytes", c.kind(), show, what, maxDetectOutput)
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
//
// NIX_CC_USE_RESPONSE_FILE=0 is set unless the user chose a (non-empty) value: Nix's clang
// wrapper hands its arguments to the real clang in a response file written
// with bash's printf %q, which in the C locale writes a non-ASCII path as
// $'\320\272...'. clang reads that as the file name "$/tmp/320272...", so every
// compile in a folder with a non-ASCII letter fails. Without the response file
// the wrapper runs clang with the arguments as they are. Other compilers
// ignore the variable.
func CompilerEnv() []string {
	var env []string
	hasNixResponseFile := false
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if runtime.GOOS == "windows" {
			name = strings.ToUpper(name)
		}
		if name == nixResponseFile && !strings.HasSuffix(kv, "=") {
			hasNixResponseFile = true // an empty value counts as not set
		}
		if !dropEnv[name] {
			env = append(env, kv)
		}
	}
	if !hasNixResponseFile {
		env = append(env, nixResponseFile+"=0")
	}
	return append(env, "LC_ALL=C")
}

// nixResponseFile is the variable that tells Nix's compiler wrappers whether to pass the
// arguments to the compiler in a response file.
const nixResponseFile = "NIX_CC_USE_RESPONSE_FILE"

// familyOf returns the compiler family that the macro list out (from -dM
// -E) shows, or 0, and its major version (__clang_major__ or __GNUC__;
// 0 if it is missing). clang defines __GNUC__ too, so __clang__ decides.
func familyOf(out []byte) (Family, int) {
	var clang, gnuc bool
	var clangMajor, gnucMajor int
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] != "#define" {
			continue
		}
		n := 0
		if len(f) > 2 {
			n, _ = strconv.Atoi(f[2])
		}
		switch f[1] {
		case "__clang__":
			clang = true
		case "__clang_major__":
			clangMajor = n
		case "__GNUC__":
			gnuc, gnucMajor = true, n
		}
	}
	switch {
	case clang:
		return Clang, clangMajor
	case gnuc:
		return GCC, gnucMajor
	}
	return 0, 0
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
