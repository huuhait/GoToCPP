package interop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/huuhait/gocpp/internal/build"
)

// The namespace probe (Phase 01 §2.5, step 4) decides whether a name that
// a .gp file uses like a package (std in std.vector) is a C++ namespace.
// It asks the C++ compiler with one small file that holds the .gp file's
// includes and one namespace alias per candidate:
//
//	#include <vector>
//	; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;
//	#include "./physics.h"
//	; template <class> struct gp_inc_1; using gp_inc_1_c = ::gp_inc_1<int>;
//	#if defined(std)
//	#error gp_ns_macro
//	#else
//	namespace gp_ns_0 = ::std;
//	#endif
//	...
//	namespace gp_probe_end = ::gp_probe_no_such_namespace;
//	namespace gp_probe_eof {}
//
// The compiler checks it with -fsyntax-only and reports every error with
// its line. A candidate whose lines have an error is not a namespace.
//
// The other lines guard against what the headers and the candidates can do
// to the lines after them (all seen with both compilers):
//   - The scope check after each include fails when the header doesn't end
//     at the top level: in the middle of a declaration (int f( or int x =),
//     which would otherwise swallow the next line and with it a candidate,
//     or inside a block that it leaves open (extern "C" {, a namespace, a
//     class, a function, an enum). The ";" ends an unfinished declaration,
//     a template can't have C linkage, and ::gp_inc_N is found only at the
//     top level. So the error is on the culprit's own line, before the
//     next header can use up the open brace (clang's recovery inside
//     libstdc++ did that for an open extern "C" { before <vector>).
//   - The #if keeps macros out of the alias lines: a macro that expands to
//     an unbalanced brace would hide the lines after it, and a macro is
//     never a namespace.
//   - The gp_probe_end alias always fails, and it uses no name that a
//     header can define as a macro: its error shows that the compiler got
//     to the end of the file, so a compiler that stops early (or prints
//     nothing) can't make every candidate look like a namespace.
//   - The last line takes the "expected '}' at end of input" error of what
//     the scope checks don't catch (an open extern "C++" {); its "to match
//     this '{'" note says which header that is.
//
// An error in a header (or at an #include line, for a header that isn't
// found) belongs to the include that brought the header in: the compilers
// print the include stack ("In file included from gp_probe.cpp:3:") before
// the first message in a header, and the stack's entry in the probe file is
// that #include. clang prints the stack again whenever the header changes;
// gcc prints the stack of each #include only once, so the parser remembers
// which include brought in each header. When an include has an error, the
// probe can't tell anything about the candidates: they are Unknown, and the
// include errors are the result.
//
// Phase 04's kind probe grows from this one: it also tells types,
// templates and values apart, and it gets the package's flags.

// A Kind is what the namespace probe found out about a candidate name.
type Kind int

const (
	// Unknown means that the probe couldn't tell: an include failed
	// (Result.IncludeErrors), or the probe failed. It is the zero value,
	// so a candidate that is missing from Result.Kinds is Unknown too.
	Unknown Kind = iota

	// Namespace means that the name is a C++ namespace or a namespace
	// alias (namespace fs = std::filesystem), possibly found through an
	// inline namespace.
	Namespace

	// NotNamespace means that the name is not a namespace: nothing of that
	// name is declared, or it is a type, a template, a function, a
	// variable or a macro.
	NotNamespace
)

// String returns "Unknown", "Namespace" or "NotNamespace".
func (k Kind) String() string {
	switch k {
	case Unknown:
		return "Unknown"
	case Namespace:
		return "Namespace"
	case NotNamespace:
		return "NotNamespace"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// A Result is what ProbeNamespaces found out.
type Result struct {
	// Kinds maps each candidate, spelled as it was passed, to its kind.
	// When IncludeErrors is not empty, every candidate is Unknown, except
	// that a name that starts with gp_ (reserved for gocpp) is always
	// NotNamespace.
	Kinds map[string]Kind

	// IncludeErrors are the includes that didn't work, in include order: a
	// header that isn't found, or a header that doesn't compile. There is
	// at most one per include, with the compiler's first message about it.
	// After a header that isn't found the compiler stops, so the includes
	// after it are not checked.
	IncludeErrors []IncludeError
}

// An IncludeError is an include that didn't work.
type IncludeError struct {
	// Index is the index of the include in the includes passed to
	// ProbeNamespaces, and Include is that include: "<raylibb.h>".
	Index   int
	Include string

	// Kind says what kind of error it is, so that a caller can word its
	// message without knowing the compilers' messages.
	Kind IncludeErrorKind

	// File, Line and Column are where the compiler found the error when
	// it is in a header (also one that the include brings in). They are
	// "", 0 and 0 when the error is in the probe file: at the #include line
	// itself, because the header wasn't found, or at the check after it,
	// because the header ends in an unfinished declaration or an open
	// block (then Msg says so). Column is 0 when the compiler gave none.
	File         string
	Line, Column int

	// Msg is the compiler's first message about the include, such as
	// "'raylibb.h' file not found" (clang), "raylibb.h: No such file or
	// directory" (gcc) or "expected ';' after struct".
	Msg string
}

// An IncludeErrorKind says what is wrong with an include.
type IncludeErrorKind int

const (
	// HeaderErrors: the header, or a header that it includes, has an
	// error (IncludeError.File says where), or the compiler reported
	// another error than "not found" at the #include line (File is "").
	// It is the zero value.
	HeaderErrors IncludeErrorKind = iota

	// HeaderNotFound: the compiler didn't find the header of the include
	// ("'raylibb.h' file not found", "raylibb.h: No such file or
	// directory"). A header that isn't found by a header that the include
	// brings in is HeaderErrors, with the place of its #include line.
	HeaderNotFound

	// HeaderUnclosed: the header ends inside a declaration or a block
	// that it doesn't close (the scope check after the #include failed).
	// Msg says so, and then gives the compiler's message about the check
	// line of the probe file, which says little about the header.
	HeaderUnclosed
)

// String returns the name of the constant, such as "HeaderNotFound".
func (k IncludeErrorKind) String() string {
	switch k {
	case HeaderErrors:
		return "HeaderErrors"
	case HeaderNotFound:
		return "HeaderNotFound"
	case HeaderUnclosed:
		return "HeaderUnclosed"
	}
	return fmt.Sprintf("IncludeErrorKind(%d)", int(k))
}

// notFoundRx matches the compilers' messages for a header that isn't
// found: clang's "'x.h' file not found" and gcc's "x.h: No such file or
// directory" (with LC_ALL=C, so in English and with ASCII quotes). For
// <x.h> with only a quoted x.h next to the file, clang says "'x.h' file
// not found with <angled> include; use \"quotes\" instead" (and then
// includes it anyway), where gcc says it isn't found.
var notFoundRx = regexp.MustCompile(`^(?:'.*' file not found(?: with <angled> include; use "quotes" instead)?|.*: No such file or directory)$`)

// Error returns "include: msg", or "include: file:line:col: msg" for an
// error in a header.
func (e IncludeError) Error() string {
	if e.File == "" {
		return e.Include + ": " + e.Msg
	}
	pos := e.File + ":" + strconv.Itoa(e.Line)
	if e.Column > 0 {
		pos += ":" + strconv.Itoa(e.Column)
	}
	return e.Include + ": " + pos + ": " + e.Msg
}

// ProbeNamespaces asks the C++ compiler cxx which of the candidates are
// C++ namespaces, given the includes of a .gp file. It runs the compiler
// once, with -fsyntax-only, for all the candidates. With includes, it
// runs also for an empty candidate list (or one with only reserved names),
// to check the includes: a .gp file that imports only C headers has no
// namespace uses, but its headers must be there.
//
// includes are the texts of the #include lines, in source order, each
// with its delimiters: "<raylib.h>", "\"./physics.h\"". dir is the folder
// of the .gp file: the compiler runs there, and a quoted include is looked
// up there after the folder of the file that includes it, before the
// system folders (it is passed as -iquote, which doesn't affect <...>
// includes). dir can be "" when there is no folder.
//
// A candidate is a name, such as "std", or a namespace path from a
// namespace rename, such as "::std::filesystem" (the leading :: is
// optional). Each part must be a C++ name: a letter or _, then letters,
// digits and _. Result.Kinds has an entry for each candidate.
//
// Without includes, every candidate is Unknown and nothing runs: nothing
// can declare a namespace (a .gp file can have a namespace rename and no
// header import). A candidate that starts with gp_ is NotNamespace without
// a probe line: gocpp reserves those names (ADR-010).
//
// The answers are the compiler's own, and g++ and clang++ differ in one
// place: g++ declares namespace std even when no header does, so with only
// C headers (<stdio.h>) std is a Namespace for g++ and NotNamespace for
// clang++ (with <math.h>, whose libstdc++ wrapper declares std, both say
// Namespace).
//
// An include that doesn't work (a header that isn't found, or doesn't
// compile) is an IncludeError in the result, and then every candidate is
// Unknown. The error is for the probe itself: bad arguments, a compiler
// that doesn't run, a time limit (ctx, and at most one minute), or
// compiler output that the probe doesn't understand. Then too the
// candidates are Unknown, and the caller can go on without the probe.
//
// The results are cached in memory, per compiler, folder and includes.
// The cache notices changes to the headers outside the system folders
// (the compiler's dependency list, -MMD) and to the quoted includes' files
// in dir. It doesn't notice changes to system headers or to the
// environment; a result with include errors is used for 5 seconds at
// most, so that installing a missing library is noticed. (A disk cache
// comes with the build cache of Phase 06; its key must also hold the
// environment that changes the include paths, such as CPATH and
// NIX_CFLAGS_COMPILE.) ProbeNamespaces is safe for concurrent use; calls
// for the same compiler, folder and includes wait for each other, so that
// they run the compiler once.
func ProbeNamespaces(ctx context.Context, cxx build.CXX, dir string, includes, candidates []string) (Result, error) {
	return defaultCache.probe(ctx, cxx, dir, includes, candidates)
}

// probeTimeout limits one run of the probe, also when ctx has no deadline.
// The heaviest include sets take a few seconds.
const probeTimeout = time.Minute

// maxProbeOutput limits what the probe reads from the compiler. A probe
// with 200 bad candidates prints about 20 KiB; the cascade of errors of
// C++ headers that end up inside an open extern "C" block is 1 to 3 MiB.
// When the output is cut, the include errors found so far still count.
const maxProbeOutput = 8 << 20

// waitDelay is how long the probe waits for the compiler's output after
// the compiler exited or was stopped.
const waitDelay = 2 * time.Second

// checkInclude reports what is wrong with the include text inc, if
// anything. The text goes into the probe file as is, so it must be one
// header name on one line.
func checkInclude(inc string) error {
	if len(inc) >= 3 {
		inside := inc[1 : len(inc)-1]
		switch {
		case inc[0] == '<' && inc[len(inc)-1] == '>' && !strings.ContainsAny(inside, ">\n\r\x00"):
			return nil
		case inc[0] == '"' && inc[len(inc)-1] == '"' && !strings.ContainsAny(inside, "\"\n\r\x00"):
			return nil
		}
	}
	return fmt.Errorf("namespace probe: bad include %q: want <header> or \"header\"", inc)
}

// splitCandidate returns the parts of the candidate c ("::std::filesystem"
// gives std and filesystem), or an error if c is not a name or a
// namespace path.
func splitCandidate(c string) ([]string, error) {
	parts := strings.Split(strings.TrimPrefix(c, "::"), "::")
	for _, p := range parts {
		if !isCppName(p) {
			return nil, fmt.Errorf("namespace probe: bad candidate %q: want a C++ name or a namespace path like ::std::filesystem", c)
		}
	}
	return parts, nil
}

// isCppName reports whether s is a C++ identifier: a letter or _, then
// letters, digits and _. Like Go (and load's check of namespace paths), it
// allows non-ASCII letters and digits. These characters can't change the
// lines of the probe file around the name; whether the compiler accepts
// the name, and whether it is a keyword, the compiler decides.
func isCppName(s string) bool {
	for i, r := range s {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) || i == 0 && unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// includeLines and blockLines are the numbers of lines of the probe file
// per include (the #include and the scope check) and per candidate.
const (
	includeLines = 2
	blockLines   = 5
)

// A probeFile is the source of a probe, with the places of its parts. Its
// lines are, from line 1: includeLines lines per include, blockLines lines
// per candidate, the canary (the alias that must fail) and the last line.
type probeFile struct {
	src       []byte
	nIncludes int
	nCands    int
}

func (p *probeFile) firstBlock() int { return includeLines*p.nIncludes + 1 }
func (p *probeFile) canary() int     { return p.firstBlock() + blockLines*p.nCands }

// makeProbe returns the probe file for the includes and the candidates
// (each split into its parts).
func makeProbe(includes []string, cands [][]string) *probeFile {
	var b bytes.Buffer
	for i, inc := range includes {
		fmt.Fprintf(&b, "#include %s\n; template <class> struct gp_inc_%d; using gp_inc_%d_c = ::gp_inc_%d<int>;\n", inc, i, i, i)
	}
	for i, parts := range cands {
		b.WriteString("#if ")
		for j, p := range parts {
			if j > 0 {
				b.WriteString(" || ")
			}
			fmt.Fprintf(&b, "defined(%s)", p)
		}
		fmt.Fprintf(&b, "\n#error gp_ns_macro\n#else\nnamespace gp_ns_%d = ::%s;\n#endif\n", i, strings.Join(parts, "::"))
	}
	b.WriteString("namespace gp_probe_end = ::gp_probe_no_such_namespace;\n")
	b.WriteString("namespace gp_probe_eof {}\n")
	return &probeFile{src: b.Bytes(), nIncludes: len(includes), nCands: len(cands)}
}

// A probeRun is what one run of the probe found out.
type probeRun struct {
	kinds   []Kind // per candidate of the run; nil if incErrs isn't empty
	incErrs []IncludeError

	// deps are the headers outside the system folders that the compiler
	// read, from its dependency file (-MMD), and depsOK says whether there
	// was one: after a header that isn't found, there is none.
	deps   []string
	depsOK bool

	start time.Time // when the compiler started
}

// runProbe runs the probe for the candidates cands (split into parts) with
// the compiler cxx in the folder dir (absolute, or "").
func runProbe(ctx context.Context, cxx build.CXX, dir string, includes []string, cands [][]string) (*probeRun, error) {
	var familyArgs []string
	switch cxx.Family {
	case build.Clang:
		// -Qunused-arguments: no warnings about flags that a compiler
		// wrapper adds (NixOS). The include stacks of notes say which
		// include brought in a note's header (the "to match this '{'" of a
		// header that leaves a brace open). No spell checking: the probe
		// doesn't use "did you mean", and it costs time with many
		// misspelled candidates.
		familyArgs = []string{"-ferror-limit=0", "-Qunused-arguments", "-fno-caret-diagnostics",
			"-fdiagnostics-show-note-include-stack", "-fno-spell-checking"}
	case build.GCC:
		// Without macro tracking, gcc puts an error that comes from a
		// macro at the place where the macro is used, like clang. With it,
		// gcc prints the include stack of the header that defines the macro
		// (<features.h> for stdio.h's __BEGIN_DECLS), and the error would
		// go to the wrong include.
		familyArgs = []string{"-fmax-errors=0", "-fno-diagnostics-show-caret", "-ftrack-macro-expansion=0"}
	default:
		return nil, unknownFamily(cxx)
	}

	tmp, err := os.MkdirTemp("", "gocpp-nsprobe-")
	if err != nil {
		return nil, fmt.Errorf("namespace probe: %w", err)
	}
	defer os.RemoveAll(tmp)
	// The compiler runs in dir, so a relative TMPDIR must not stay relative.
	if tmp, err = filepath.Abs(tmp); err != nil {
		return nil, fmt.Errorf("namespace probe: %w", err)
	}
	pf := makeProbe(includes, cands)
	src := filepath.Join(tmp, "gp_probe.cpp")
	if err := os.WriteFile(src, pf.src, 0o644); err != nil {
		return nil, fmt.Errorf("namespace probe: %w", err)
	}
	depFile := filepath.Join(tmp, "gp_probe.d")

	args := []string{"-fsyntax-only", "-std=c++20", "-x", "c++", "-w", "-fdiagnostics-color=never"}
	args = append(args, familyArgs...)
	args = append(args, "-MMD", "-MF", depFile, "-MT", "gp_probe")
	if dir != "" {
		args = append(args, "-iquote", dir)
	}
	args = append(args, src)

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cxx.Path, args...)
	cmd.Dir = dir
	cmd.Env = probeEnv(tmp, dir)
	out := &capBuffer{max: maxProbeOutput, stop: cancel}
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = waitDelay
	inGroup(cmd)
	start := time.Now()
	err = cmd.Start()
	if err == nil {
		err = cmd.Wait()
		killGroup(cmd) // whatever the compiler left running
	}
	name := filepath.Base(cxx.Path)
	switch {
	case out.full:
		// The include errors so far are enough: then the candidates are
		// Unknown anyway.
		if run, err := readOutput(pf, out.buf.Bytes(), src, includes); err == nil && run.incErrs != nil {
			run.start = start
			return run, nil
		}
		return nil, fmt.Errorf("namespace probe: %s printed more than %d MiB", name, maxProbeOutput>>20)
	case ctx.Err() != nil:
		return nil, fmt.Errorf("namespace probe: %s was stopped: %w", name, ctx.Err())
	case cmd.ProcessState == nil:
		return nil, fmt.Errorf("namespace probe: %s didn't start: %w", name, err)
	case errors.Is(err, exec.ErrWaitDelay):
		return nil, fmt.Errorf("namespace probe: %s exited, but a process that it started kept its output open", name)
	case !cmd.ProcessState.Exited():
		return nil, fmt.Errorf("namespace probe: %s: %v%s", name, cmd.ProcessState, outputExcerpt(out.buf.Bytes()))
	}

	run, err := readOutput(pf, out.buf.Bytes(), src, includes)
	if err != nil {
		return nil, fmt.Errorf("namespace probe: %s: %w", name, err)
	}
	run.start = start
	if data, err := os.ReadFile(depFile); err == nil {
		run.deps, run.depsOK = parseDeps(data, src, dir), true
	}
	return run, nil
}

// unknownFamily is the error for a compiler that is neither clang nor gcc.
func unknownFamily(cxx build.CXX) error {
	return fmt.Errorf("namespace probe: C++ compiler %s: unknown family %v", cxx.Path, cxx.Family)
}

// probeEnv returns the environment of the compiler: build.CompilerEnv
// (the process's own, with LC_ALL=C), with the temporary folder tmp, so
// that nothing the compiler writes stays behind, and with PWD set to the
// folder dir that it runs in (os/exec sets PWD only when it makes the
// environment itself).
func probeEnv(tmp, dir string) []string {
	// A later KEY=value wins (os/exec keeps the last one).
	env := append(build.CompilerEnv(), "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
	if dir != "" && runtime.GOOS != "windows" {
		env = append(env, "PWD="+dir)
	}
	return env
}

// capBuffer keeps the first max bytes written to it. When more come, it
// calls stop and drops the rest.
type capBuffer struct {
	buf  bytes.Buffer
	max  int
	full bool
	stop func()
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.full {
		return len(p), nil
	}
	if room := b.max - b.buf.Len(); len(p) > room {
		b.buf.Write(p[:room])
		b.full = true
		b.stop()
		return len(p), nil
	}
	return b.buf.Write(p)
}

// The compiler's messages, as gcc and clang print them with
// -fno-diagnostics-show-caret and -fno-caret-diagnostics.
var (
	// A message at a place: "file:line:col: error: msg". Some of gcc's
	// messages have no column. The file can contain ":" (C:\x.h). gcc's
	// "sorry, unimplemented" is an error too.
	diagRx = regexp.MustCompile(`^(.+?):(\d+):(?:(\d+):)? (fatal error|error|sorry, unimplemented|warning|note|remark): (.*)$`)

	// A line of an include stack: clang prints one "In file included from
	// file:line:" line per level, outermost first; gcc prints "In file
	// included from file:line," and then "                 from file:line:"
	// lines, innermost first.
	inclRx = regexp.MustCompile(`^(?:In file included from|\s+from) (.+?):(\d+)(?::\d+)?[:,]$`)

	// Any other error: "g++: error: unrecognized command-line option",
	// "cc1plus: fatal error: ...", "clang++: error: clang frontend command
	// failed", or a crash ("internal compiler error").
	otherErrRx = regexp.MustCompile(`(?:^|: )(?:fatal error|error|internal compiler error): `)
)

// A diagnostic is one error or note that the compiler reported.
type diagnostic struct {
	severity  string // "note", or "error", "fatal error", "sorry, unimplemented"
	file      string // the file it is in, "" if the compiler gave no place
	line, col int
	msg       string // the message, without "error: "
	text      string // the whole line, for messages

	// probeLine is the line of the probe file it belongs to: its own line
	// when it is in the probe file, else the #include line that brought in
	// its header. It is 0 when the probe can't tell.
	probeLine int
}

func (d *diagnostic) isError() bool { return d.severity != "note" }

// parseDiagnostics returns the errors and notes in the compiler output out,
// for the probe file at the path probe (as the compiler got it). A message
// in a header belongs to the #include of the probe file in the include
// stack printed before it. When no stack is printed before it, the header
// had a message before: clang prints no stack for the next message in the
// same header, and gcc prints the stack of an #include only once. So the
// parser remembers, for each header, the probe line that brought it in.
func parseDiagnostics(out []byte, probe string) []diagnostic {
	var diags []diagnostic
	var stack []string          // the files of the include stack just printed
	stackLine := 0              // the stack's line in the probe file, 0 if none
	headers := map[string]int{} // header file -> probe line
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if m := inclRx.FindStringSubmatch(line); m != nil {
			if samePath(m[1], probe) {
				stackLine, _ = strconv.Atoi(m[2])
			} else {
				stack = append(stack, filepath.Clean(m[1]))
			}
			continue
		}
		m := diagRx.FindStringSubmatch(line)
		if m == nil {
			if otherErrRx.MatchString(line) {
				diags = append(diags, diagnostic{severity: "error", text: line})
			}
			continue
		}
		d := diagnostic{severity: m[4], file: m[1], msg: m[5], text: line}
		d.line, _ = strconv.Atoi(m[2])
		d.col, _ = strconv.Atoi(m[3])
		switch file := filepath.Clean(d.file); {
		case samePath(d.file, probe):
			d.probeLine = d.line
		case stackLine != 0:
			d.probeLine = stackLine
			headers[file] = stackLine
			for _, f := range stack {
				headers[f] = stackLine
			}
		default:
			d.probeLine = headers[file]
		}
		stack, stackLine = stack[:0], 0
		if d.severity != "warning" && d.severity != "remark" { // -w leaves none anyway
			diags = append(diags, d)
		}
	}
	return diags
}

// samePath reports whether the paths a and b, as a compiler prints them,
// name the same file. The compiler prints the probe file's path as it got
// it, so Clean is only for safety.
func samePath(a, b string) bool {
	return a == b || filepath.Clean(a) == filepath.Clean(b)
}

// readOutput reads the compiler output out of the probe pf at the path
// src: which candidates have errors, and which includes.
func readOutput(pf *probeFile, out []byte, src string, includes []string) (*probeRun, error) {
	run := &probeRun{kinds: make([]Kind, pf.nCands)}
	for i := range run.kinds {
		run.kinds[i] = Namespace
	}
	canary := false
	var unexpected []diagnostic
	diags := parseDiagnostics(out, src)
	for i, d := range diags {
		if !d.isError() {
			continue
		}
		switch n := d.probeLine; {
		case n >= 1 && n < pf.firstBlock():
			index := (n - 1) / includeLines
			ie := IncludeError{Index: index, Include: includes[index], Msg: d.msg}
			switch {
			case !samePath(d.file, src):
				ie.File, ie.Line, ie.Column = filepath.Clean(d.file), d.line, d.col
			case (n-1)%includeLines == 1:
				// The scope check after the #include.
				ie.Kind = HeaderUnclosed
				ie.Msg = "the header ends inside a declaration or a block that it doesn't close: " + d.msg
			case notFoundRx.MatchString(d.msg):
				ie.Kind = HeaderNotFound
			}
			run.incErrs = append(run.incErrs, ie)
		case n >= pf.firstBlock() && n < pf.canary():
			run.kinds[(n-pf.firstBlock())/blockLines] = NotNamespace
		case n == pf.canary():
			canary = true
		case n > pf.canary() && samePath(d.file, src):
			// The end of the file: a header leaves a brace open. Its
			// notes ("to match this '{'") say which one.
			if ie, ok := openBrace(d, diags[i+1:], pf, src, includes); ok {
				run.incErrs = append(run.incErrs, ie)
				break
			}
			unexpected = append(unexpected, d)
		default:
			unexpected = append(unexpected, d)
		}
	}
	switch {
	case len(run.incErrs) > 0:
		// Keep the first error of each include, in include order. The
		// errors mostly come in that order anyway, but gcc reads the whole
		// file before it parses it, so a header that isn't found comes
		// before the syntax errors of the headers above it.
		slices.SortStableFunc(run.incErrs, func(a, b IncludeError) int { return a.Index - b.Index })
		run.incErrs = slices.CompactFunc(run.incErrs, func(a, b IncludeError) bool { return a.Index == b.Index })
		run.kinds = nil
		return run, nil
	case len(unexpected) > 0:
		// A driver error (a bad flag), a crash: the probe can't say what
		// it means.
		return nil, fmt.Errorf("unexpected error: %s", strings.ReplaceAll(unexpected[0].text, src, filepath.Base(src)))
	case !canary:
		return nil, fmt.Errorf("it didn't check the whole probe file (no error for its line %d)%s", pf.canary(), outputExcerpt(out))
	}
	return run, nil
}

// openBrace returns the include error for the error d at the end of the
// probe file pf (at the path src), from the notes after it (notes): the
// first note in a header that an include brought in, such as "to match
// this '{'", gives the include and the place.
func openBrace(d diagnostic, notes []diagnostic, pf *probeFile, src string, includes []string) (IncludeError, bool) {
	for _, note := range notes {
		if note.isError() {
			break
		}
		if n := note.probeLine; n >= 1 && n < pf.firstBlock() && note.file != "" && !samePath(note.file, src) {
			index := (n - 1) / includeLines
			return IncludeError{Index: index, Include: includes[index], File: filepath.Clean(note.file),
				Line: note.line, Column: note.col, Msg: d.msg + " (" + note.msg + ")"}, true
		}
	}
	return IncludeError{}, false
}

// outputExcerpt returns the start of the compiler output out for an error
// message: "; its output: " and its first lines, or "; it printed nothing".
func outputExcerpt(out []byte) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return "; it printed nothing"
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 3 {
		lines = append(lines[:3], "...")
	}
	return "; its output: " + strings.Join(lines, " | ")
}

// parseDeps returns the files of the dependency file data (make syntax,
// from -MMD -MT gp_probe), without the probe file src itself. A relative
// path is relative to dir, where the compiler ran.
func parseDeps(data []byte, src, dir string) []string {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\\\n", " ")
	s, _ = strings.CutPrefix(s, "gp_probe:")
	var files []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		f := cur.String()
		cur.Reset()
		if !filepath.IsAbs(f) && dir != "" {
			f = filepath.Join(dir, f)
		}
		if f = filepath.Clean(f); !samePath(f, src) {
			files = append(files, f)
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\t' || s[i+1] == '#'):
			// gcc and clang write a space in a path as "\ " and # as "\#".
			cur.WriteByte(s[i+1])
			i++
		case c == '$' && i+1 < len(s) && s[i+1] == '$':
			cur.WriteByte('$')
			i++
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return files
}
