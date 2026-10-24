package interop

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The macro probe (Phase 04 §2.2, probe 1) lists the macros that are
// defined after a set of includes: $CXX -E -dM over a file with only the
// #include lines, with the same -iquote folder and flags as the kind probe.
// The compiler prints one "#define NAME value" or "#define NAME(args)
// value" line per macro, as the macros are at the end of the file (a macro
// that a header #undefs is not there).
//
// The list keeps the compiler's predefined macros (__GNUC__, __linux__,
// and linux and unix with -std=gnu++20): for "is stdio.EOF a macro" they
// don't matter, but for the names that collide with the names gocpp emits
// (step 4.3) they matter as much as the headers' own macros, because the
// generated code sees them too. Telling them apart would need a second run
// (the macros of an empty file); nothing needs it yet.

// A MacroKind says whether a name is a macro, and which kind.
type MacroKind int

const (
	// NotMacro: the name is not a macro. It is the zero value.
	NotMacro MacroKind = iota

	// ObjectLike: #define EOF (-1). The name is replaced wherever it is
	// used.
	ObjectLike

	// FunctionLike: #define max(a, b) ... The name is replaced only when
	// a ( follows it.
	FunctionLike
)

// String returns the name of the constant, such as "ObjectLike".
func (k MacroKind) String() string {
	switch k {
	case NotMacro:
		return "NotMacro"
	case ObjectLike:
		return "ObjectLike"
	case FunctionLike:
		return "FunctionLike"
	}
	return fmt.Sprintf("MacroKind(%d)", int(k))
}

// Macros is the set of macros after a set of includes. Its zero value has
// no macros. It never changes, so it can be shared.
type Macros struct {
	m map[string]MacroKind
}

// Kind returns the kind of macro that name is, or NotMacro.
func (ms Macros) Kind(name string) MacroKind { return ms.m[name] }

// Len returns the number of macros.
func (ms Macros) Len() int { return len(ms.m) }

// Names returns the names of the macros, sorted.
func (ms Macros) Names() []string {
	names := make([]string, 0, len(ms.m))
	for name := range ms.m {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ProbeMacros returns the macros that are defined after the includes,
// with the compiler cxx, the folder dir and the flags: the arguments are
// those of ProbeKinds, and ProbeKinds says Macro for a path when one of its
// parts is in the result, or is one of the compiler's built-in macros that
// -dM doesn't list (__LINE__, __FILE__, __COUNTER__, __has_include,
// __has_builtin, and with g++ _Pragma): #if defined(...) is true for them.
// Without includes the result has the compiler's predefined macros (the
// compiler runs then too).
//
// The error is for an include that doesn't preprocess (a header that isn't
// found, an #error in a header) and for a probe that fails; the kind probe
// says more about include errors, so callers run it first.
//
// The results are cached in memory and on disk, per compiler, folder,
// flags and includes, and checked against the headers as ProbeKinds' are;
// a failure is not cached. ProbeMacros is safe for concurrent use.
func ProbeMacros(ctx context.Context, cxx build.CXX, dir string, flags, includes []string) (Macros, error) {
	return defaultCache.probeMacros(ctx, cxx, dir, flags, includes)
}

// maxMacroOutput limits the macro list that the probe reads: <windows.h>
// defines about 25,000 macros, about 2 MiB of #define lines.
const maxMacroOutput = 64 << 20

// A macroSet is what the cache knows of the macros of one include set.
type macroSet struct {
	filled bool        // the fields below hold a result
	files  []fileState // what the result depends on
	macros Macros
}

// probeMacros is ProbeMacros with the cache c.
func (c *cache) probeMacros(ctx context.Context, cxx build.CXX, dir string, flags, includes []string) (Macros, error) {
	if err := checkArgs(flags, includes); err != nil {
		return Macros{}, fmt.Errorf("macro probe: %w", err)
	}
	if err := checkLocalHeaders(dir, includes); err != nil {
		return Macros{}, fmt.Errorf("macro probe: %w", err)
	}
	if cxx.Family != build.Clang && cxx.Family != build.GCC {
		return Macros{}, fmt.Errorf("macro probe: %w", unknownFamily(cxx))
	}
	if dir != "" {
		var err error
		if dir, err = filepath.Abs(dir); err != nil {
			return Macros{}, err
		}
	}
	set, unlock, err := c.macros.lock(ctx, setKey(cxx, dir, flags, includes))
	if err != nil {
		return Macros{}, err
	}
	defer unlock()
	if set.filled && c.valid(set.files, false, time.Time{}) {
		return set.macros, nil
	}
	*set = macroSet{}
	key, onDisk := c.disk.keyFor("m", cxx, dir, flags, includes)
	if onDisk {
		if loaded, ok := c.loadMacros(key); ok {
			*set = *loaded
			return set.macros, nil
		}
	}
	c.macroRuns.Add(1)
	ms, deps, start, err := runMacroProbe(ctx, cxx, dir, flags, includes)
	if err != nil {
		return Macros{}, err
	}
	if files, ok := dependsOn(deps, true, false, start, dir, includes); ok {
		*set = macroSet{filled: true, files: files, macros: ms}
		if onDisk {
			c.storeMacros(key, set)
		}
	}
	return ms, nil
}

// runMacroProbe runs the macro probe, and returns the macros, the headers
// that the compiler read (the system headers too), and when it started.
func runMacroProbe(ctx context.Context, cxx build.CXX, dir string, flags, includes []string) (Macros, []string, time.Time, error) {
	var start time.Time
	tmp, err := probeTemp()
	if err != nil {
		return Macros{}, nil, start, fmt.Errorf("macro probe: %w", err)
	}
	defer os.RemoveAll(tmp)
	var b bytes.Buffer
	for i, inc := range includes {
		// The kind probe's lines, so that the headers see the same: also
		// the extern "C" block of an include marked with ExternC.
		writeInclude(&b, i, inc)
	}
	src := filepath.Join(tmp, "gp_macros.cpp")
	if err := os.WriteFile(src, b.Bytes(), 0o644); err != nil {
		return Macros{}, nil, start, fmt.Errorf("macro probe: %w", err)
	}
	depFile := filepath.Join(tmp, "gp_macros.d")
	args, err := compilerArgs(cxx, []string{"-E", "-dM"}, dir, flags, depFile, src)
	if err != nil {
		return Macros{}, nil, start, fmt.Errorf("macro probe: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, probeLimit())
	defer cancel()
	stdout := &capBuffer{max: maxMacroOutput, stop: cancel}
	stderr := &capBuffer{max: maxProbeOutput, stop: cancel}
	start = time.Now()
	cmd, err := runCompiler(ctx, cxx, dir, tmp, args, stdout, stderr)
	name := cxx.Name()
	switch {
	case stdout.full:
		return Macros{}, nil, start, fmt.Errorf("macro probe: %s printed more than %d MiB of macros", name, maxMacroOutput>>20)
	case stderr.full:
		return Macros{}, nil, start, fmt.Errorf("macro probe: %s printed more than %d MiB of messages", name, maxProbeOutput>>20)
	}
	if err := runFailure(ctx, name, cmd, err, stderr.buf.Bytes()); err != nil {
		return Macros{}, nil, start, fmt.Errorf("macro probe: %w", err)
	}
	if !cmd.ProcessState.Success() {
		msg := strings.ReplaceAll(strings.TrimSpace(stderr.buf.String()), src, filepath.Base(src))
		return Macros{}, nil, start, fmt.Errorf("macro probe: %s failed (%v)%s", name, cmd.ProcessState, outputExcerpt([]byte(msg)))
	}
	ms, err := parseMacros(stdout.buf.Bytes())
	if err != nil {
		return Macros{}, nil, start, fmt.Errorf("macro probe: %s: %w", name, err)
	}
	data, err := os.ReadFile(depFile)
	if err != nil {
		return Macros{}, nil, start, fmt.Errorf("macro probe: %s wrote no dependency file: %w", name, err)
	}
	return ms, parseDeps(data, src, dir), start, nil
}

// parseMacros reads the output of -E -dM: one "#define NAME value" or
// "#define NAME(args) value" line per macro. A line can't continue on the
// next (the compiler prints each definition on one line), and an empty
// output is not a macro list (every compiler predefines some).
func parseMacros(out []byte) (Macros, error) {
	m := make(map[string]MacroKind)
	for line := range bytes.SplitSeq(out, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 {
			continue
		}
		rest, ok := bytes.CutPrefix(line, []byte("#define "))
		if !ok {
			return Macros{}, fmt.Errorf("unexpected line in the macro list: %q", line)
		}
		end := bytes.IndexAny(rest, " (")
		if end < 0 {
			end = len(rest)
		}
		name := string(rest[:end])
		if !isCppName(name) {
			return Macros{}, fmt.Errorf("unexpected line in the macro list: %q", line)
		}
		m[name] = ObjectLike
		if end < len(rest) && rest[end] == '(' {
			m[name] = FunctionLike
		}
	}
	if len(m) == 0 {
		return Macros{}, fmt.Errorf("it printed no macros")
	}
	return Macros{m: m}, nil
}
