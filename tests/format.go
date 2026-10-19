// Package tests is the gocpp test harness. The harness is the test file
// harness_test.go: one Go test binary that finds the test programs in
// tests/run, tests/errors, tests/golden, tests/diff, tests/interop,
// tests/debug (debug_test.go) and examples/, and runs each one through a
// gocpp binary that it builds.
//
// This file has the parts that need no gocpp binary: the readers of the
// test formats and the code that compares results. README.md describes the
// formats. Keep tests/ one Go package, so that "go test ./tests/... -update"
// only reaches the test binary that knows the -update flag.
package tests

import (
	"errors"
	"fmt"
	"go/build/constraint"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// lastPhase is the last phase of the plan (plans/2026-10-13-gocpp has
// phases 00 to 15).
const lastPhase = 15

// goMinor is the minor version of the Go that gocpp follows (Go 1.25): the
// release tags go1.1 to go1.25 hold in //go:build lines.
const goMinor = 25

// The comment lines of the test formats. Each one starts a line.
const (
	phaseLine         = "// phase: NN" // the form of the first line, for messages
	outputPrefix      = "// Output:"
	exitCodePrefix    = "// ExitCode:"
	stderrPrefix      = "// Stderr:"
	pkgConfigPrefix   = "// pkg-config:"
	displayPrefix     = "// display:"
	zoneinfoPrefix    = "// zoneinfo:"
	diffPrefix        = "// diff:"
	lsanPrefix        = "// lsan:"
	divergencePrefix  = "// divergence:"
	pendingPrefix     = "// pending:"
	stdInternalPrefix = "// std-internal:"
	goBuildPrefix     = "//go:build"
)

// A lineSet is a set of the optional comment lines.
type lineSet uint16

const (
	outputLine      lineSet = 1 << iota // the // Output: block; required where it is allowed
	exitCodeLine                        // // ExitCode: N
	stderrLine                          // // Stderr: regexp
	pkgConfigLine                       // // pkg-config: lib...
	displayLine                         // // display: required
	zoneinfoLine                        // // zoneinfo: required
	diffLine                            // // diff: stderr, // diff: combined
	lsanLine                            // // lsan: cycle
	divergenceLine                      // // divergence: D1
	pendingLine                         // // pending: step 7.1 (generics)
	stdInternalLine                     // // std-internal: tests internal/bytealg
	goBuildLine                         // //go:build expr
)

// optionalLines lists the optional comment lines with their prefixes.
var optionalLines = []struct {
	line   lineSet
	prefix string
}{
	{outputLine, outputPrefix},
	{exitCodeLine, exitCodePrefix},
	{stderrLine, stderrPrefix},
	{pkgConfigLine, pkgConfigPrefix},
	{displayLine, displayPrefix},
	{zoneinfoLine, zoneinfoPrefix},
	{diffLine, diffPrefix},
	{lsanLine, lsanPrefix},
	{divergenceLine, divergencePrefix},
	{pendingLine, pendingPrefix},
	{stdInternalLine, stdInternalPrefix},
	{goBuildLine, goBuildPrefix},
}

// The modes of a diff test (its // diff: line).
const (
	diffStdout   = ""         // compare stdout and the exit code (no // diff: line)
	diffStderr   = "stderr"   // compare stderr too
	diffCombined = "combined" // compare stdout and stderr merged into one pipe, and the exit code
)

// A format is what one kind of test may say in its comment lines.
type format struct {
	kind    string  // its folder: "run" for tests/run, "examples" for examples/
	allowed lineSet // the optional lines it may have
	strict  bool    // an optional line that is not allowed, or not in the header, is an error; otherwise it is ignored
}

// Every format honors //go:build lines. The tests that run a program take // lsan: cycle and
// // pending:; run and pkgs tests also take // std-internal:.
var (
	runFormat = format{"run", outputLine | exitCodeLine | stderrLine | lsanLine | divergenceLine | pendingLine | stdInternalLine | zoneinfoLine | goBuildLine, true}
	// A race test (tests/race) is a run test that gocpp builds with -race: a program with a data
	// race ends with exit code 66 and Go's report on stderr, a program without one is silent.
	raceFormat    = format{"race", outputLine | exitCodeLine | stderrLine | pendingLine | goBuildLine, true}
	errorsFormat  = format{"errors", goBuildLine, true}
	goldenFormat  = format{"golden", goBuildLine, true}
	diffFormat    = format{"diff", diffLine | lsanLine | pendingLine | goBuildLine, true}
	interopFormat = format{"interop", outputLine | exitCodeLine | stderrLine | pkgConfigLine | lsanLine | pendingLine | goBuildLine, true}
	// A multi-package test is a folder: a module with a main.gp (tests/pkgs).
	pkgsFormat  = format{"pkgs", outputLine | exitCodeLine | stderrLine | lsanLine | divergenceLine | pendingLine | stdInternalLine | goBuildLine, true}
	debugFormat = format{"debug", goBuildLine, true} // and the // gdb: comments (debug.go)

	// An interop error test (a test in tests/interop with // ERROR
	// comments) is built, not run: it has no // Output: block and no
	// // ExitCode: or // Stderr: line.
	interopErrorsFormat = format{"interop", pkgConfigLine | goBuildLine, true}

	// An example's main.gp is a normal program with doc comments, which may
	// start like a comment line of another format ("// Output: a table"),
	// so there the lines that examples don't use, and the optional lines
	// below the header, are ignored.
	examplesFormat = format{"examples", exitCodeLine | pkgConfigLine | displayLine | pendingLine | goBuildLine, false}
)

// A header is what a test file says about itself in its comment lines.
type header struct {
	phase       int              // the phase that makes the test pass (the first line, "// phase: NN")
	output      string           // the expected stdout, from the // Output: block
	exitCode    int              // the expected exit code (// ExitCode: N, default 0)
	stderr      []*regexp.Regexp // the // Stderr: regexps; none means stderr must be empty
	pkgConfig   []string         // the pkg-config packages the test needs (// pkg-config: ...)
	display     bool             // the program needs a display (// display: required)
	zoneinfo    bool             // the program loads time zones by name (// zoneinfo: required)
	diff        string           // a diff test's mode (// diff: ...): diffStdout, diffStderr or diffCombined
	cycle       bool             // the program leaks a reference cycle on purpose (// lsan: cycle)
	diverges    string           // the divergence of docs/spec/language.md that the test shows (// divergence: D1), or ""
	pending     string           // why the test fails until a step of its phase lands (// pending: ...), or ""
	stdInternal string           // why the test imports the standard library's internal packages (// std-internal: ...), or ""
	build       constraint.Expr  // the //go:build constraint, or nil
	buildLine   string           // the //go:build line, for messages
}

// parse reads the comment lines of a test file. path names the file in the
// errors, which have the form "path:line: message"; src is its contents.
//
// The first line must be the phase line. The optional lines belong to the
// header: the comment lines at the top of the file, before the package
// clause. In a strict format, an optional line further down is an error; in
// an example it is ignored, like any comment. The // Output: block, if the
// format has one, must end the file.
func (f format) parse(path string, src []byte) (*header, error) {
	lines := sourceLines(src)
	phase, err := parsePhase(path, lines)
	if err != nil {
		return nil, err
	}
	h := &header{phase: phase}
	end := len(lines)
	if f.allowed&outputLine != 0 {
		start := outputStart(lines)
		if start < 0 {
			return nil, fmt.Errorf("%s: no %q block at the end of the file (a program that prints nothing ends with an empty %q line)",
				path, outputPrefix, outputPrefix)
		}
		if h.output, err = parseOutput(path, lines, start); err != nil {
			return nil, err
		}
		end = start
	}
	var seen lineSet
	inHeader := true
	for i := 1; i < end; i++ {
		line := trimRight(lines[i])
		if line != "" && !strings.HasPrefix(line, "//") {
			inHeader = false // the package clause, or anything else that is not a comment
		}
		l, prefix, rest := optionalLine(line)
		switch {
		case l == 0:
		case f.allowed&l == 0:
			if f.strict {
				return nil, fmt.Errorf("%s:%d: tests in tests/%s have no %q line (see tests/%s/README.md)",
					path, i+1, f.kind, prefix, f.kind)
			}
		case !inHeader:
			if f.strict {
				return nil, fmt.Errorf("%s:%d: a %q line belongs at the top of the file, before the package clause",
					path, i+1, prefix)
			}
		default:
			if err := h.set(l, line, rest, seen); err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, i+1, err)
			}
			seen |= l
		}
	}
	return h, nil
}

// lsanEnv returns the environment change of a test that leaks a reference cycle on purpose
// (// lsan: cycle): ASAN_OPTIONS with detect_leaks=0 after the options in asanOptions (the
// ASAN_OPTIONS of the harness's environment), so that LeakSanitizer doesn't report the cycle in
// a sanitizer run (GOCPP_FLAGS=-asan). The later option wins, so it also wins over a
// detect_leaks=1 there.
func lsanEnv(asanOptions string) string {
	if asanOptions == "" {
		return "ASAN_OPTIONS=detect_leaks=0"
	}
	return "ASAN_OPTIONS=" + asanOptions + ":detect_leaks=0"
}

// optionalLine returns which optional line line is, with its prefix and the
// text after it, or 0 if it is none.
func optionalLine(line string) (l lineSet, prefix, rest string) {
	for _, o := range optionalLines {
		if rest, ok := strings.CutPrefix(line, o.prefix); ok {
			return o.line, o.prefix, rest
		}
	}
	return 0, "", ""
}

// exitCodeRx matches the number of an // ExitCode: line.
var exitCodeRx = regexp.MustCompile(`^\d{1,3}$`)

// divergenceRx matches the value of a // divergence: line, a divergence of
// docs/spec/language.md: D1, D23.
var divergenceRx = regexp.MustCompile(`^D[1-9]\d*$`)

// set records the optional line l: the whole line is line, the text after
// its prefix is rest. seen holds the lines read before it; only // Stderr:
// may come more than once.
func (h *header) set(l lineSet, line, rest string, seen lineSet) error {
	if seen&l != 0 && l != stderrLine {
		prefix := strings.TrimSuffix(line, rest)
		if l == pkgConfigLine {
			return fmt.Errorf("a second %q line (put every package on one line)", prefix)
		}
		return fmt.Errorf("a second %q line", prefix)
	}
	switch l {
	case exitCodeLine:
		s := strings.TrimSpace(rest)
		n, err := strconv.Atoi(s)
		if !exitCodeRx.MatchString(s) || err != nil || n > 255 {
			return fmt.Errorf("bad %q line: want an exit code from 0 to 255, not %q", exitCodePrefix, s)
		}
		h.exitCode = n
	case stderrLine:
		rx, err := regexp.Compile(strings.TrimPrefix(rest, " "))
		if err != nil {
			return fmt.Errorf("bad %q line: %v", stderrPrefix, err)
		}
		h.stderr = append(h.stderr, rx)
	case pkgConfigLine:
		libs := strings.Fields(rest)
		if len(libs) == 0 {
			return fmt.Errorf("bad %q line: want one or more pkg-config package names", pkgConfigPrefix)
		}
		for _, lib := range libs {
			if strings.HasPrefix(lib, "-") {
				return fmt.Errorf("bad %q line: %q is not a package name", pkgConfigPrefix, lib)
			}
		}
		h.pkgConfig = libs
	case displayLine:
		if s := strings.TrimSpace(rest); s != "required" {
			return fmt.Errorf("bad %q line: want \"// display: required\", not %q", displayPrefix, line)
		}
		h.display = true
	case zoneinfoLine:
		if s := strings.TrimSpace(rest); s != "required" {
			return fmt.Errorf("bad %q line: want \"// zoneinfo: required\", not %q", zoneinfoPrefix, line)
		}
		h.zoneinfo = true
	case diffLine:
		switch s := strings.TrimSpace(rest); s {
		case diffStderr, diffCombined:
			h.diff = s
		default:
			return fmt.Errorf("bad %q line: want %q or %q, not %q", diffPrefix, diffStderr, diffCombined, s)
		}
	case lsanLine:
		if s := strings.TrimSpace(rest); s != "cycle" {
			return fmt.Errorf("bad %q line: want \"// lsan: cycle\", not %q", lsanPrefix, line)
		}
		h.cycle = true
	case divergenceLine:
		s := strings.TrimSpace(rest)
		if !divergenceRx.MatchString(s) {
			return fmt.Errorf("bad %q line: want a divergence of docs/spec/language.md, such as D1, not %q", divergencePrefix, s)
		}
		h.diverges = s
	case pendingLine:
		s := strings.TrimSpace(rest)
		if s == "" {
			return fmt.Errorf("bad %q line: want the reason, such as \"// pending: step 7.1 (generic functions)\"", pendingPrefix)
		}
		h.pending = s
	case stdInternalLine:
		s := strings.TrimSpace(rest)
		if s == "" {
			return fmt.Errorf("bad %q line: want the reason, such as \"// std-internal: tests internal/bytealg\"", stdInternalPrefix)
		}
		h.stdInternal = s
	case goBuildLine:
		expr, err := constraint.Parse(line)
		if err != nil {
			return fmt.Errorf("bad %q line: %v", goBuildPrefix, err)
		}
		h.build, h.buildLine = expr, line
	}
	return nil
}

// unixOS lists the GOOS values for which the build tag "unix" holds (the
// list of the go command).
var unixOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "hurd": true,
	"illumos": true, "ios": true, "linux": true, "netbsd": true, "openbsd": true, "solaris": true,
}

// targetTag reports whether the build tag tag holds for the target of the
// tests, which is the machine that runs them: its GOOS and GOARCH, "unix"
// on a Unix system, and the release tags go1.1 to go1.25. Any other tag
// doesn't hold.
func targetTag(tag string) bool {
	switch {
	case tag == runtime.GOOS, tag == runtime.GOARCH:
		return true
	case tag == "unix":
		return unixOS[runtime.GOOS]
	}
	if minor, ok := strings.CutPrefix(tag, "go1."); ok {
		n, err := strconv.Atoi(minor)
		return err == nil && n >= 1 && n <= goMinor && strconv.Itoa(n) == minor
	}
	return false
}

// targetGate returns why the test with header h doesn't build for the
// target (its //go:build line excludes it), or "".
func targetGate(h *header) string {
	if h.build == nil || h.build.Eval(targetTag) {
		return ""
	}
	return fmt.Sprintf("its %q line excludes the target %s/%s", h.buildLine, runtime.GOOS, runtime.GOARCH)
}

// displayGate returns why a test that needs a display (display) can't run
// with the environment that getenv reads, or "".
func displayGate(display bool, getenv func(string) string) string {
	if !display || getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != "" {
		return ""
	}
	return "needs a display (// display: required), but neither DISPLAY nor WAYLAND_DISPLAY is set"
}

// zoneinfoDirs are the places where the time zone database of the system is, as package time looks
// for it on Unix (stdlib/time/zoneinfo_unix.gp; the ZONEINFO variable first).
var zoneinfoDirs = []string{"/usr/share/zoneinfo", "/usr/share/lib/zoneinfo", "/usr/lib/locale/TZ", "/etc/zoneinfo"}

// zoneinfoGate returns why a test that loads time zones by name (zoneinfo) can't run with the
// environment that getenv reads, or "": no time zone database is installed. exists says whether a
// file does; the test needs America/New_York, Europe/Berlin and Asia/Kolkata.
func zoneinfoGate(zoneinfo bool, getenv func(string) string, exists func(string) bool) string {
	if !zoneinfo {
		return ""
	}
	dirs := zoneinfoDirs
	if z := getenv("ZONEINFO"); z != "" {
		dirs = []string{z}
	}
	for _, d := range dirs {
		if exists(d+"/America/New_York") && exists(d+"/Europe/Berlin") && exists(d+"/Asia/Kolkata") {
			return ""
		}
	}
	return "needs the system's time zone database (// zoneinfo: required), but none with America/New_York is installed (see the ZONEINFO variable)"
}

// fileExists says whether the file p exists.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// phaseRx matches the phase line. NN has two digits, like the phases in the
// plan and in "gocpp help" ("Phase 02").
var phaseRx = regexp.MustCompile(`^// phase: (\d\d)$`)

// parsePhase reads the phase line, which must be the first line.
func parsePhase(path string, lines []string) (int, error) {
	if len(lines) == 0 {
		return 0, fmt.Errorf("%s: the file is empty; its first line must be the phase line %q", path, phaseLine)
	}
	first := trimRight(lines[0])
	m := phaseRx.FindStringSubmatch(first)
	if m == nil {
		return 0, fmt.Errorf("%s:1: the first line must be the phase line %q (NN from 00 to %02d: the phase that makes the test pass), not %q",
			path, phaseLine, lastPhase, first)
	}
	n, _ := strconv.Atoi(m[1])
	if n > lastPhase {
		return 0, fmt.Errorf("%s:1: there is no Phase %s: the plan has phases 00 to %02d", path, m[1], lastPhase)
	}
	return n, nil
}

// outputStart returns the index of the line that starts the // Output:
// block, or -1.
func outputStart(lines []string) int {
	for i, line := range lines {
		if strings.HasPrefix(line, outputPrefix) {
			return i
		}
	}
	return -1
}

// parseOutput reads the // Output: block that starts at lines[start] and
// returns the output it expects. As in a Go example, that is the comment
// lines after "// Output:" with the "//" and one space removed, and text
// after "// Output:" on the same line is the first line. Only blank lines
// may follow the block.
func parseOutput(path string, lines []string, start int) (string, error) {
	var out []string
	if first := strings.TrimLeft(trimRight(lines[start])[len(outputPrefix):], " "); first != "" {
		out = append(out, first)
	}
	for i := start + 1; i < len(lines); i++ {
		line := trimRight(lines[i])
		if line == "" {
			if blankFrom(lines, i) {
				break
			}
			return "", fmt.Errorf("%s:%d: a blank line in the %q block of line %d: write an empty output line as \"//\", and keep the block at the end of the file",
				path, i+1, outputPrefix, start+1)
		}
		text, ok := strings.CutPrefix(line, "//")
		if !ok {
			return "", fmt.Errorf("%s:%d: only // comment lines may follow %q (line %d): the block must end the file",
				path, i+1, outputPrefix, start+1)
		}
		out = append(out, strings.TrimPrefix(text, " "))
	}
	if len(out) == 0 {
		return "", nil
	}
	return strings.Join(out, "\n") + "\n", nil
}

// blankFrom reports whether lines[i:] are all blank.
func blankFrom(lines []string, i int) bool {
	for _, line := range lines[i:] {
		if trimRight(line) != "" {
			return false
		}
	}
	return true
}

// sourceLines splits a test file into lines: without a byte order mark,
// with "\r\n" read as "\n", and without the empty string after the final
// newline.
func sourceLines(src []byte) []string {
	s := strings.TrimPrefix(string(src), "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// trimRight removes the spaces and tabs at the end of a line (gofmt removes
// them from comments too).
func trimRight(s string) string {
	return strings.TrimRight(s, " \t")
}

// sameOutput reports whether a program's stdout matches the expected
// output of an // Output: block. As in a Go example test, space at the
// start and at the end of the whole output doesn't count.
func sameOutput(got, want string) bool {
	return strings.TrimSpace(got) == strings.TrimSpace(want)
}

// A wantError is an error or a warning that "gocpp check" must report: the
// regexp of a // ERROR or // WARNING comment and its line.
type wantError struct {
	file    string // the file of a folder test, relative to the folder (geom/geom.gp); "" for the one file of the test
	line    int
	expr    string // the regexp as written, after LINE is expanded
	rx      *regexp.Regexp
	warning bool // from a // WARNING comment
}

// The marks of an error test.
const (
	errorMark   = "ERROR"
	warningMark = "WARNING"
)

var (
	// markCommentRx finds the first // ERROR or // WARNING comment of a
	// line and the text after it.
	markCommentRx = regexp.MustCompile(`// (ERROR|WARNING)( .*)?$`)
	// nextMarkRx matches a // ERROR or // WARNING comment after the
	// regexps of the one before it.
	nextMarkRx = regexp.MustCompile(`^// (ERROR|WARNING)( |$)`)
	// lineRefRx matches LINE, LINE+N and LINE-N in a regexp.
	lineRefRx = regexp.MustCompile(`LINE(([+-])(\d+))?`)
)

// parseErrors reads the // ERROR and // WARNING comments of an error test,
// with the rules of Go's own test/ folder: `// ERROR "rx1" "rx2"` at the
// end of a line says that gocpp check reports one error per regexp on that
// line, and `// WARNING "rx"` one warning. Both can be on one line, one
// after the other (`// ERROR "rx1" // WARNING "rx2"`). The regexps are
// between double quotes and used as written (no escapes are processed, and
// a regexp can't contain a double quote). LINE in a regexp stands for
// "file.gp:N" of its own line, LINE+N and LINE-N for a line before or after
// it, and a line that contains "////" is skipped.
func parseErrors(path string, src []byte) ([]wantError, error) {
	base := filepath.Base(path)
	var wants []wantError
	for i, line := range sourceLines(src) {
		n := i + 1
		if strings.Contains(line, "////") {
			continue // a double comment turns ERROR off, as in Go's test/
		}
		m := markCommentRx.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mark, rest := m[1], m[2]
		for {
			exprs, next, err := quotedRegexps(mark, rest)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: bad // %s comment: %v", path, n, mark, err)
			}
			for _, expr := range exprs {
				expr = expandLINE(expr, base, n)
				rx, err := regexp.Compile(expr)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: bad regexp in a // %s comment: %v", path, n, mark, err)
				}
				wants = append(wants, wantError{line: n, expr: expr, rx: rx, warning: mark == warningMark})
			}
			if next == "" {
				break
			}
			nm := nextMarkRx.FindStringSubmatch(next)
			mark, rest = nm[1], next[len(nm[0])-len(nm[2]):]
		}
	}
	return wants, nil
}

// quotedRegexps splits the text after the mark (ERROR or WARNING) into its
// double-quoted regexps. It stops at a // ERROR or // WARNING comment after
// them and returns it in next.
func quotedRegexps(mark, s string) (exprs []string, next string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, "", fmt.Errorf(`want one or more regexps in double quotes after %s, like // %s "undefined: x"`, mark, mark)
	}
	for s != "" {
		if len(exprs) > 0 && nextMarkRx.MatchString(s) {
			return exprs, s, nil
		}
		if s[0] != '"' {
			return nil, "", fmt.Errorf("want a regexp in double quotes, not %q", s)
		}
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return nil, "", fmt.Errorf(`no closing quote in %q (a regexp can't contain a double quote; write \x22 for one)`, s)
		}
		exprs = append(exprs, s[1:1+end])
		s = strings.TrimLeft(s[2+end:], " \t")
	}
	return exprs, "", nil
}

// wantsErrors reports whether wants has an error (not only warnings): then
// gocpp check must exit with 1.
func wantsErrors(wants []wantError) bool {
	for _, w := range wants {
		if !w.warning {
			return true
		}
	}
	return false
}

// expandLINE replaces LINE, LINE+N and LINE-N in a regexp of a // ERROR
// comment on line n of the file base with "base:n", "base:n+N", "base:n-N".
func expandLINE(expr, base string, n int) string {
	return lineRefRx.ReplaceAllStringFunc(expr, func(m string) string {
		line := n
		if len(m) > len("LINE") {
			d, _ := strconv.Atoi(m[len("LINE+"):])
			if m[len("LINE")] == '-' {
				d = -d
			}
			line += d
		}
		return regexp.QuoteMeta(base) + ":" + strconv.Itoa(line)
	})
}

// A diagnostic is one error or warning that "gocpp check" printed, in the
// form "path/file.gp:LINE:COL: message" (the column may be missing); a
// warning's message starts with "warning: ".
type diagnostic struct {
	file    string // the path as printed
	line    int
	text    string // the message (of a warning: after "warning: ") and the indented lines after it: what a regexp must match
	raw     string // the error as printed, for reports
	warning bool
}

// warningPrefix starts the message of a warning (diag.Printer).
const warningPrefix = "warning: "

// diagnosticRx matches the first line of a diagnostic.
var diagnosticRx = regexp.MustCompile(`^(.+?):(\d+)(?::\d+)?: (.*)$`)

// parseDiagnostics splits the output of "gocpp check" into diagnostics,
// like Go's test/ runner does: a line that starts with a tab or a space
// continues the diagnostic before it (a second position, a source line, a
// caret), and a line that starts with "#" (a package header) is skipped.
// The lines that are none of these are returned in other.
func parseDiagnostics(out string) (diags []diagnostic, other []string) {
	var last *diagnostic
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " "):
			if last != nil {
				last.text += "\n" + line
				last.raw += "\n" + line
				continue
			}
		case strings.HasPrefix(line, "#"):
			continue
		default:
			if m := diagnosticRx.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[2])
				text, warning := strings.CutPrefix(m[3], warningPrefix)
				diags = append(diags, diagnostic{file: m[1], line: n, text: text, raw: line, warning: warning})
				last = &diags[len(diags)-1]
				continue
			}
		}
		other = append(other, line)
		last = nil
	}
	return diags, other
}

// sameDiagFile reports whether the file of a diagnostic, as gocpp prints it ("./main.gp",
// "geom/geom.gp"), is the file name: its base name for the file of a single-file test, or its
// path relative to the folder of a folder test (rel).
func sameDiagFile(diagFile, name string, rel bool) bool {
	if !rel {
		return filepath.Base(diagFile) == name
	}
	return strings.TrimPrefix(filepath.ToSlash(diagFile), "./") == name
}

// matchErrors compares the diagnostics of the test file base with the
// errors and warnings the test wants, like Go's test/ runner: each regexp
// must match a diagnostic of its kind (an ERROR regexp an error, a WARNING
// regexp a warning) on its line, and it uses up every such diagnostic
// there that it matches; a diagnostic that no regexp uses up is
// unexpected, and so is any other output. It returns one message per
// problem.
func matchErrors(base string, wants []wantError, diags []diagnostic, other []string) []string {
	used := make([]bool, len(diags))
	var problems []string
	for _, w := range wants {
		var onLine []string
		matched := false
		name := base
		if w.file != "" {
			name = w.file
		}
		for i, d := range diags {
			if used[i] || d.line != w.line || !sameDiagFile(d.file, name, w.file != "") || d.warning != w.warning {
				continue
			}
			if w.rx.MatchString(d.text) {
				used[i], matched = true, true
			} else {
				onLine = append(onLine, d.raw)
			}
		}
		kind := "error"
		if w.warning {
			kind = "warning"
		}
		switch {
		case matched:
		case len(onLine) == 0:
			problems = append(problems, fmt.Sprintf("%s:%d: missing %s %q", name, w.line, kind, w.expr))
		default:
			problems = append(problems, fmt.Sprintf("%s:%d: no %s matches %q; the line has:\n\t\t%s",
				name, w.line, kind, w.expr, strings.Join(onLine, "\n\t\t")))
		}
	}
	for i, d := range diags {
		switch {
		case used[i]:
		case d.warning:
			problems = append(problems, "unexpected warning: "+d.raw)
		default:
			problems = append(problems, "unexpected error: "+d.raw)
		}
	}
	for _, line := range other {
		problems = append(problems, "unexpected output: "+line)
	}
	return problems
}

// An example is one target program in examples/<name>/: main.gp and the
// files next to it (examples/README.md describes them).
type example struct {
	header *header  // from main.gp
	output []byte   // output.txt: the expected stdout
	args   []string // args.txt: the program's arguments
	stdin  []byte   // stdin.txt: its standard input; nil when there is no stdin.txt
	env    []string // env.txt: KEY=VALUE pairs to add to its environment
}

// loadExample reads the example in the folder dir; rel is the folder
// relative to the repository root ("examples/hello"), for errors.
func loadExample(dir, rel string) (*example, error) {
	src, err := os.ReadFile(filepath.Join(dir, "main.gp"))
	if err != nil {
		return nil, err
	}
	ex := &example{}
	if ex.header, err = examplesFormat.parse(rel+"/main.gp", src); err != nil {
		return nil, err
	}
	if ex.output, err = os.ReadFile(filepath.Join(dir, "output.txt")); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: no output.txt (the expected stdout of the program)", rel)
		}
		return nil, err
	}
	args, err := readOptional(filepath.Join(dir, "args.txt"))
	if err != nil {
		return nil, err
	}
	if args != nil {
		if ex.args, err = parseArgs(rel+"/args.txt", args); err != nil {
			return nil, err
		}
	}
	if ex.stdin, err = readOptional(filepath.Join(dir, "stdin.txt")); err != nil {
		return nil, err
	}
	env, err := readOptional(filepath.Join(dir, "env.txt"))
	if err != nil {
		return nil, err
	}
	if env != nil {
		if ex.env, err = parseEnv(rel+"/env.txt", env); err != nil {
			return nil, err
		}
	}
	return ex, nil
}

// parseEnv reads an example's env.txt: KEY=VALUE lines, added to the
// program's environment as they are. Empty lines don't count. (The same
// rules as tools/gen-example-output.)
func parseEnv(path string, data []byte) ([]string, error) {
	var env []string
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if key, _, ok := strings.Cut(line, "="); !ok || key == "" {
			return nil, fmt.Errorf("%s:%d: want a KEY=VALUE line, not %q", path, i+1, line)
		}
		env = append(env, line)
	}
	return env, nil
}

// readOptional reads a file that may be missing; then it returns nil and no
// error. A file that is there but empty gives an empty, non-nil slice.
func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err == nil && data == nil {
		data = []byte{}
	}
	return data, err
}

// parseArgs reads an example's args.txt: one line of arguments, split on
// spaces. The line may end with a newline.
func parseArgs(path string, data []byte) ([]string, error) {
	s := strings.TrimRight(string(data), "\r\n")
	if strings.ContainsAny(s, "\r\n") {
		lines := strings.Count(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"), "\n") + 1
		return nil, fmt.Errorf("%s: want one line of arguments, not %d lines", path, lines)
	}
	if args := strings.Fields(s); len(args) > 0 {
		return args, nil
	}
	return nil, nil
}

// listPrograms returns the .gp files directly in dir, sorted: each one is a
// test. Other files (READMEs, headers, snapshots) and folders are not.
func listPrograms(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gp") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// snapshotSuffix ends the name of a golden test's snapshot: x.gp has the
// snapshot x.cpp.golden.
const snapshotSuffix = ".cpp.golden"

// orphanSnapshots returns the snapshots in dir whose .gp file is missing.
func orphanSnapshots(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool)
	for _, e := range entries {
		have[e.Name()] = !e.IsDir()
	}
	var orphans []string
	for _, e := range entries {
		if base, ok := strings.CutSuffix(e.Name(), snapshotSuffix); ok && !e.IsDir() && !have[base+".gp"] {
			orphans = append(orphans, e.Name())
		}
	}
	return orphans, nil
}

// listExamples returns the folders directly in dir that have a main.gp
// file, sorted: each one is a target program. Other folders (docs, shared
// files) are not. It returns nothing when dir doesn't exist.
func listExamples(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, e.Name(), "main.gp"))
		switch {
		case err == nil:
			names = append(names, e.Name())
		case !errors.Is(err, fs.ErrNotExist):
			return nil, err
		}
	}
	return names, nil
}

// trimRoot replaces the paths of the repository root in s with "$ROOT".
// The generated C++ holds absolute paths (#line directives), and a snapshot
// must not depend on where the repository is checked out. The longer roots
// are replaced first, so that one root can contain another.
func trimRoot(s string, roots []string) string {
	roots = append([]string(nil), roots...)
	sort.Slice(roots, func(i, j int) bool { return len(roots[i]) > len(roots[j]) })
	for _, root := range roots {
		if root != "" {
			s = strings.ReplaceAll(s, root, "$ROOT")
		}
	}
	return s
}

// shellQuote quotes s for a POSIX shell when it needs quotes, so that a
// command in a test failure can be pasted into a shell.
func shellQuote(s string) string {
	const plain = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@%+=:,./-"
	if s != "" && strings.Trim(s, plain) == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// maxShown is how many lines of a text a test failure shows.
const maxShown = 60

// explainMismatch says how got differs from want, for a test failure: the
// first line that differs (quoted, so that space and newlines show), then
// both texts.
func explainMismatch(got, want string) string {
	g, w := strings.SplitAfter(got, "\n"), strings.SplitAfter(want, "\n")
	i := 0
	for i < len(g) && i < len(w) && g[i] == w[i] {
		i++
	}
	return fmt.Sprintf("first difference in line %d:\n\tgot:  %s\n\twant: %s\ngot:\n%swant:\n%s",
		i+1, lineAt(g, i), lineAt(w, i), indent(got), indent(want))
}

// lineAt returns lines[i] quoted, for explainMismatch.
func lineAt(lines []string, i int) string {
	if i >= len(lines) || i == len(lines)-1 && lines[i] == "" {
		return "(end of text)"
	}
	return strconv.Quote(lines[i])
}

// indent returns s with a tab before each line, and at most maxShown lines
// of it, for test failures.
func indent(s string) string {
	if s == "" {
		return "\t(nothing)\n"
	}
	lines := strings.SplitAfter(strings.TrimSuffix(s, "\n"), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == maxShown {
			fmt.Fprintf(&b, "\t... (%d more lines)\n", len(lines)-maxShown)
			break
		}
		b.WriteString("\t" + strings.TrimSuffix(line, "\n") + "\n")
	}
	return b.String()
}
