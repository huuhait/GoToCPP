package tests

import (
	"fmt"
	"go/build/constraint"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/version"
)

// The tests of this file check the parts of the harness that need no gocpp
// binary. Their inputs are in testdata/, which the harness never runs as
// tests (and which make lint doesn't gofmt-check).

// headerView is a header with its regexps as strings and its //go:build
// line instead of the constraint, for comparisons. (TestTargetGate checks
// the constraints.)
type headerView struct {
	phase       int
	output      string
	exitCode    int
	stderr      []string
	pkgConfig   []string
	display     bool
	zoneinfo    bool
	diff        string
	cycle       bool
	diverges    string
	pending     string
	stdInternal string
	build       string
}

func view(h *header) headerView {
	v := headerView{phase: h.phase, output: h.output, exitCode: h.exitCode, pkgConfig: h.pkgConfig,
		display: h.display, zoneinfo: h.zoneinfo, diff: h.diff, cycle: h.cycle, diverges: h.diverges, pending: h.pending, stdInternal: h.stdInternal, build: h.buildLine}
	for _, rx := range h.stderr {
		v.stderr = append(v.stderr, rx.String())
	}
	return v
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseHeader(t *testing.T) {
	tests := []struct {
		file string
		f    format
		want headerView
	}{
		{"run_ok.gp", runFormat, headerView{phase: 2, output: "hello\n"}},
		{"run_all_lines.gp", runFormat, headerView{
			phase:    3,
			output:   "a\n\n\tb  c\n", // "//" alone is an empty line; only one space after "//" goes
			exitCode: 2,
			stderr:   []string{"^panic: boom", "(?m)^goroutine"},
		}},
		{"run_same_line.gp", runFormat, headerView{phase: 2, output: "one\ntwo\n"}},
		{"run_empty_output.gp", runFormat, headerView{phase: 2}},
		{"interop_ok.gp", interopFormat, headerView{
			phase:     4,
			output:    "ok\n",
			stderr:    []string{""}, // an empty regexp: any stderr
			pkgConfig: []string{"sdl2", "raylib"},
		}},
		// "// Output:" in the middle of a line is not an // Output: line.
		{"diff_ok.gp", diffFormat, headerView{phase: 2}},
		{"diff_ok.gp", goldenFormat, headerView{phase: 2}},
		{"diff_ok.gp", errorsFormat, headerView{phase: 2}},
		{"diff_stderr.gp", diffFormat, headerView{phase: 2, diff: "stderr"}},
		{"diff_combined.gp", diffFormat, headerView{phase: 2, diff: "combined"}},
		{"run_lsan_divergence.gp", runFormat, headerView{phase: 3, cycle: true, diverges: "D1"}},
		{"diff_lsan.gp", diffFormat, headerView{phase: 3, cycle: true}},
		{"run_pending.gp", runFormat, headerView{phase: 7, output: "ok\n", pending: "step 7.1 (generic functions)"}},
		{"zoneinfo_ok.gp", runFormat, headerView{phase: 8, output: "ok\n", zoneinfo: true}},
		{"run_std_internal.gp", runFormat, headerView{phase: 9, output: "ok\n", stdInternal: "tests internal/bytealg"}},
		{"run_std_internal.gp", pkgsFormat, headerView{phase: 9, output: "ok\n", stdInternal: "tests internal/bytealg"}},
		// Every format honors a //go:build line in the header.
		{"gobuild_ok.gp", runFormat, headerView{phase: 14, build: "//go:build linux || windows"}},
		{"gobuild_ok.gp", interopFormat, headerView{phase: 14, build: "//go:build linux || windows"}},
	}
	for _, tt := range tests {
		path := "testdata/header/" + tt.file
		h, err := tt.f.parse(path, readFile(t, path))
		if err != nil {
			t.Errorf("%s as a %s test: %v", tt.file, tt.f.kind, err)
			continue
		}
		if got := view(h); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s as a %s test:\n got %+v\nwant %+v", tt.file, tt.f.kind, got, tt.want)
		}
	}
}

func TestParseHeaderMalformed(t *testing.T) {
	const phaseMsg = `the first line must be the phase line "// phase: NN" (NN from 00 to 15: the phase that makes the test pass), not `
	tests := []struct {
		file string
		f    format
		err  string // the error after "testdata/header/<file>:"
	}{
		{"empty.gp", runFormat, ` the file is empty; its first line must be the phase line "// phase: NN"`},
		{"phase_missing.gp", runFormat, `1: ` + phaseMsg + `"package main"`},
		{"phase_one_digit.gp", diffFormat, `1: ` + phaseMsg + `"// phase: 2"`},
		{"phase_second_line.gp", goldenFormat, `1: ` + phaseMsg + `""`},
		{"phase_capital.gp", errorsFormat, `1: ` + phaseMsg + `"// Phase: 02"`},
		{"phase_too_big.gp", runFormat, `1: there is no Phase 16: the plan has phases 00 to 15`},

		{"output_missing.gp", runFormat, ` no "// Output:" block at the end of the file`},
		{"output_missing.gp", interopFormat, ` no "// Output:" block at the end of the file`},
		{"output_code_after.gp", runFormat, `7: only // comment lines may follow "// Output:" (line 5): the block must end the file`},
		{"output_blank_inside.gp", runFormat, `9: a blank line in the "// Output:" block of line 7`},

		{"exitcode_two.gp", runFormat, `2: bad "// ExitCode:" line: want an exit code from 0 to 255, not "two"`},
		{"exitcode_negative.gp", runFormat, `2: bad "// ExitCode:" line: want an exit code from 0 to 255, not "-1"`},
		{"exitcode_too_big.gp", interopFormat, `2: bad "// ExitCode:" line: want an exit code from 0 to 255, not "256"`},
		{"exitcode_twice.gp", runFormat, `3: a second "// ExitCode:" line`},
		{"exitcode_after_package.gp", runFormat, `5: a "// ExitCode:" line belongs at the top of the file, before the package clause`},
		{"stderr_bad_regexp.gp", runFormat, "2: bad \"// Stderr:\" line: error parsing regexp: missing closing ): `panic: (`"},

		{"pkgconfig_in_run.gp", runFormat, `2: tests in tests/run have no "// pkg-config:" line (see tests/run/README.md)`},
		{"pkgconfig_empty.gp", interopFormat, `2: bad "// pkg-config:" line: want one or more pkg-config package names`},
		{"pkgconfig_flag.gp", interopFormat, `2: bad "// pkg-config:" line: "--libs" is not a package name`},
		{"pkgconfig_twice.gp", interopFormat, `3: a second "// pkg-config:" line (put every package on one line)`},

		// The formats without an // Output: block don't take one, and the
		// other optional lines only where they are allowed.
		{"diff_with_output.gp", diffFormat, `7: tests in tests/diff have no "// Output:" line (see tests/diff/README.md)`},
		{"diff_with_exitcode.gp", diffFormat, `2: tests in tests/diff have no "// ExitCode:" line (see tests/diff/README.md)`},
		{"run_ok.gp", goldenFormat, `11: tests in tests/golden have no "// Output:" line`},
		{"run_all_lines.gp", errorsFormat, `2: tests in tests/errors have no "// ExitCode:" line`},
		// An interop error test (// ERROR comments) is built, not run.
		{"run_ok.gp", interopErrorsFormat, `11: tests in tests/interop have no "// Output:" line`},
		{"run_all_lines.gp", interopErrorsFormat, `2: tests in tests/interop have no "// ExitCode:" line`},

		{"diff_bad_mode.gp", diffFormat, `2: bad "// diff:" line: want "stderr" or "combined", not "both"`},
		{"diff_twice.gp", diffFormat, `3: a second "// diff:" line`},
		{"diff_in_run.gp", runFormat, `2: tests in tests/run have no "// diff:" line (see tests/run/README.md)`},
		{"zoneinfo_bad.gp", runFormat, `2: bad "// zoneinfo:" line: want "// zoneinfo: required", not "// zoneinfo: maybe"`},
		{"zoneinfo_in_diff.gp", diffFormat, `2: tests in tests/diff have no "// zoneinfo:" line (see tests/diff/README.md)`},
		{"display_in_run.gp", runFormat, `2: tests in tests/run have no "// display:" line (see tests/run/README.md)`},
		{"gobuild_bad.gp", runFormat, `3: bad "//go:build" line: unexpected end of expression`},
		{"gobuild_twice.gp", goldenFormat, `4: a second "//go:build" line`},
		{"gobuild_after_package.gp", runFormat, `5: a "//go:build" line belongs at the top of the file, before the package clause`},

		{"lsan_bad.gp", runFormat, `2: bad "// lsan:" line: want "// lsan: cycle", not "// lsan: leaks"`},
		{"lsan_twice.gp", runFormat, `3: a second "// lsan:" line`},
		{"diff_lsan.gp", goldenFormat, `2: tests in tests/golden have no "// lsan:" line`},
		{"diff_lsan.gp", errorsFormat, `2: tests in tests/errors have no "// lsan:" line`},
		{"divergence_bad.gp", runFormat, `2: bad "// divergence:" line: want a divergence of docs/spec/language.md, such as D1, not "1"`},
		{"pending_bad.gp", runFormat, `2: bad "// pending:" line: want the reason, such as "// pending: step 7.1 (generic functions)"`},
		{"run_pending.gp", errorsFormat, `2: tests in tests/errors have no "// pending:" line`},
		{"std_internal_bad.gp", runFormat, `2: bad "// std-internal:" line: want the reason, such as "// std-internal: tests internal/bytealg"`},
		{"run_std_internal.gp", diffFormat, `2: tests in tests/diff have no "// std-internal:" line`},
		{"run_std_internal.gp", errorsFormat, `2: tests in tests/errors have no "// std-internal:" line`},
		{"divergence_in_diff.gp", diffFormat, `2: tests in tests/diff have no "// divergence:" line`},
	}
	for _, tt := range tests {
		path := "testdata/header/" + tt.file
		_, err := tt.f.parse(path, readFile(t, path))
		if want := path + ":" + tt.err; err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%s as a %s test:\n got error %v\nwant error %s...", tt.file, tt.f.kind, err, want)
		}
	}
}

func TestLsanEnv(t *testing.T) {
	for _, tt := range []struct{ env, want string }{
		{"", "ASAN_OPTIONS=detect_leaks=0"},
		{"abort_on_error=1", "ASAN_OPTIONS=abort_on_error=1:detect_leaks=0"},
		{"detect_leaks=1", "ASAN_OPTIONS=detect_leaks=1:detect_leaks=0"}, // the later one wins
	} {
		if got := lsanEnv(tt.env); got != tt.want {
			t.Errorf("lsanEnv(%q) = %q, want %q", tt.env, got, tt.want)
		}
	}
}

func TestParseHeaderLineEndings(t *testing.T) {
	// A byte order mark, Windows line ends and space at the ends of lines.
	src := "\ufeff// phase: 02 \r\n// ExitCode: 3\t\r\n\r\npackage main\r\n\r\n// Output:\r\n// a \r\n//\r\n// b\r\n"
	h, err := runFormat.parse("crlf.gp", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := view(h), (headerView{phase: 2, output: "a\n\nb\n", exitCode: 3}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseErrorComments(t *testing.T) {
	type want struct {
		line int
		expr string
	}
	path := "testdata/errors/ok.gp"
	wants, err := parseErrors(path, readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	var got []want
	for _, w := range wants {
		got = append(got, want{w.line, w.expr})
	}
	expected := []want{
		{8, "declared and not used: x"},
		{9, "undefined: y"}, // two regexps on one line: two errors
		{9, "second regexp"},
		{10, `cannot use 1 \(untyped int constant\) as string value`},
		{11, `x\x22y|other`},
		// line 12 has //// ERROR: turned off; line 15 has // ERRORS, which is not ERROR
		{14, "assignment mismatch"},
		{14, `ok\.gp:13, ok\.gp:14 and ok\.gp:16`}, // LINE-1, LINE and LINE+2
		{16, ""},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("got  %v\nwant %v", got, expected)
	}

	// The regexps are used as written: \( is a regexp escape, \x22 is a
	// double quote, and the dot of the file name in LINE is only a dot.
	for _, m := range []struct {
		i    int
		text string
		ok   bool
	}{
		{3, "cannot use 1 (untyped int constant) as string value in variable declaration", true},
		{4, `x"y`, true},
		{4, "other", true},
		{6, "see ok.gp:13, ok.gp:14 and ok.gp:16", true},
		{6, "see okXgp:13, ok.gp:14 and ok.gp:16", false},
		{7, "any message", true},
	} {
		if got := wants[m.i].rx.MatchString(m.text); got != m.ok {
			t.Errorf("regexp %q on %q: match %v, want %v", wants[m.i].expr, m.text, got, m.ok)
		}
	}

	// Regexps may follow each other without space; tabs and spaces around
	// them don't count.
	wants, err = parseErrors("x.gp", []byte("// phase: 01\nf() // ERROR \"a\"\"b\"\t \"c\"  \n"))
	if err != nil {
		t.Fatal(err)
	}
	var exprs []string
	for _, w := range wants {
		exprs = append(exprs, fmt.Sprint(w.line, ":", w.expr))
	}
	if got, want := strings.Join(exprs, " "), "2:a 2:b 2:c"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	path = "testdata/errors/none.gp"
	if wants, err := parseErrors(path, readFile(t, path)); err != nil || len(wants) != 0 {
		t.Errorf("%s: got %v, %v; want no errors", path, wants, err)
	}
}

func TestParseWarningComments(t *testing.T) {
	type want struct {
		line    int
		expr    string
		warning bool
	}
	path := "testdata/errors/warnings.gp"
	wants, err := parseErrors(path, readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	var got []want
	for _, w := range wants {
		got = append(got, want{w.line, w.expr, w.warning})
	}
	expected := []want{
		{3, "an ordinary comment", true}, // a WARNING comment after a comment
		{7, "declared and not used: x", false},
		{7, "w1", true}, // ERROR, then WARNING, on one line
		{7, "w2", true},
		{8, "only a warning", true}, // WARNING, then ERROR
		{8, "then an error", false},
		{9, `warnings\.gp:9`, true}, // LINE works in WARNING too
		// line 10 has // WARNINGS, which is not WARNING; line 11 has //// WARNING: turned off
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("got  %v\nwant %v", got, expected)
	}
	if !wantsErrors(wants) {
		t.Error("wantsErrors = false, want true")
	}
	if wantsErrors(wants[:1]) || wantsErrors(nil) {
		t.Error("wantsErrors = true for warnings only")
	}
}

func TestParseErrorCommentsMalformed(t *testing.T) {
	tests := []struct {
		file string
		err  string // the error after "testdata/errors/<file>:"
	}{
		{"bare.gp", `8: bad // ERROR comment: want one or more regexps in double quotes after ERROR`},
		{"unquoted.gp", `8: bad // ERROR comment: want a regexp in double quotes, not "undefined: x"`},
		{"unterminated.gp", `8: bad // ERROR comment: no closing quote in "\"undefined: x" (a regexp can't contain a double quote; write \x22 for one)`},
		{"junk.gp", `8: bad // ERROR comment: want a regexp in double quotes, not "or \"x\""`},
		{"bad_regexp.gp", "8: bad regexp in a // ERROR comment: error parsing regexp: missing closing ): `missing (`"},
		{"bare_warning.gp", `8: bad // WARNING comment: want one or more regexps in double quotes after WARNING`},
		{"mark_after_mark.gp", `8: bad // WARNING comment: want one or more regexps in double quotes after WARNING`},
	}
	for _, tt := range tests {
		path := "testdata/errors/" + tt.file
		_, err := parseErrors(path, readFile(t, path))
		if want := path + ":" + tt.err; err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%s:\n got error %v\nwant error %s...", tt.file, err, want)
		}
	}
}

func TestParseDiagnostics(t *testing.T) {
	out := "# main\r\n" + // a package header: skipped
		"x.gp:4:2: undefined: a\r\n" +
		"\tx.gp:3:1: other declaration of a\n" + // continues the error above
		"./sub/x.gp:12: no column\n" +
		`C:\work\x.gp:5:6: a Windows path: with colons` + "\n" +
		"x.gp:2:1: warning: looks like a directive\n" + // a warning
		"\n" +
		"too many errors\n" +
		"\tafter a line that is not an error\n"
	diags, other := parseDiagnostics(out)
	wantDiags := []diagnostic{
		{"x.gp", 4, "undefined: a\n\tx.gp:3:1: other declaration of a", "x.gp:4:2: undefined: a\n\tx.gp:3:1: other declaration of a", false},
		{"./sub/x.gp", 12, "no column", "./sub/x.gp:12: no column", false},
		{`C:\work\x.gp`, 5, "a Windows path: with colons", `C:\work\x.gp:5:6: a Windows path: with colons`, false},
		{"x.gp", 2, "looks like a directive", "x.gp:2:1: warning: looks like a directive", true},
	}
	if !reflect.DeepEqual(diags, wantDiags) {
		t.Errorf("diagnostics:\n got %+v\nwant %+v", diags, wantDiags)
	}
	if want := []string{"too many errors", "\tafter a line that is not an error"}; !reflect.DeepEqual(other, want) {
		t.Errorf("other lines: got %q, want %q", other, want)
	}
}

func TestMatchDiagnostics(t *testing.T) {
	src := "// phase: 01\n" +
		"package main\n" +
		"func main() {\n" +
		"\ta // ERROR \"undefined: a\"\n" + // line 4
		"\tb // ERROR \"undefined: b\" \"second\"\n" + // line 5
		"\tc\n" + // line 6
		"}\n"
	wants, err := parseErrors("x.gp", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		out  string
		want []string // the problems
	}{
		{"exact", "# main\nx.gp:4:2: undefined: a\n./x.gp:5:2: undefined: b\nx.gp:5:3: second error\n", nil},
		{"absolute path, no column", "/src/x.gp:4: undefined: a\nx.gp:5:2: undefined: b\nx.gp:5:3: second error\n", nil},
		// As in Go's test/ runner, a regexp uses up every error on its line that it matches.
		{"one regexp, two errors", "x.gp:4:2: undefined: a\nx.gp:4:7: undefined: a\nx.gp:5:2: undefined: b\nx.gp:5:3: second error\n", nil},
		{"the regexp matches a continued line", "x.gp:4:2: undefined: a\nx.gp:5:2: no such thing\n\tundefined: b\nx.gp:5:3: second error\n", nil},
		{"missing", "x.gp:4:2: undefined: a\n", []string{
			`x.gp:5: missing error "undefined: b"`,
			`x.gp:5: missing error "second"`,
		}},
		{"wrong message", "x.gp:4:2: undefined: q\nx.gp:5:2: undefined: b\nx.gp:5:3: second error\n", []string{
			"x.gp:4: no error matches \"undefined: a\"; the line has:\n\t\tx.gp:4:2: undefined: q",
			"unexpected error: x.gp:4:2: undefined: q",
		}},
		{"unexpected", "x.gp:4:2: undefined: a\nx.gp:5:2: undefined: b\nx.gp:5:3: second error\nx.gp:6:2: undefined: c\n", []string{
			"unexpected error: x.gp:6:2: undefined: c",
		}},
		{"another file", "y.gp:4:2: undefined: a\nx.gp:5:2: undefined: b\nx.gp:5:3: second error\n", []string{
			`x.gp:4: missing error "undefined: a"`,
			"unexpected error: y.gp:4:2: undefined: a",
		}},
		{"other output", "x.gp:4:2: undefined: a\nx.gp:5:2: undefined: b\nx.gp:5:3: second error\npanic: oops\n", []string{
			"unexpected output: panic: oops",
		}},
	}
	for _, tt := range tests {
		diags, other := parseDiagnostics(tt.out)
		if got := matchErrors("x.gp", wants, diags, other); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}

	// Warnings: a WARNING regexp matches only a warning (after
	// "warning: "), an ERROR regexp only an error.
	src = "// phase: 01\n" +
		"package main\n" +
		"// a // WARNING \"looks like\"\n" + // line 3
		"func main() {\n" +
		"\tx // ERROR \"undefined: x\" // WARNING \"odd\"\n" + // line 5
		"}\n"
	if wants, err = parseErrors("x.gp", []byte(src)); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		out  string
		want []string
	}{
		{"exact", "x.gp:3:1: warning: looks like a directive\nx.gp:5:2: undefined: x\nx.gp:5:2: warning: odd\n", nil},
		{"warning for error", "x.gp:3:1: warning: looks like a directive\nx.gp:5:2: warning: undefined: x\nx.gp:5:2: warning: odd\n", []string{
			`x.gp:5: missing error "undefined: x"`,
			"unexpected warning: x.gp:5:2: warning: undefined: x",
		}},
		{"error for warning", "x.gp:3:1: looks like a directive\nx.gp:5:2: undefined: x\nx.gp:5:2: warning: odd\n", []string{
			`x.gp:3: missing warning "looks like"`,
			"unexpected error: x.gp:3:1: looks like a directive",
		}},
		{"wrong warning", "x.gp:3:1: warning: looks like a directive\nx.gp:5:2: undefined: x\nx.gp:5:2: warning: even\n", []string{
			"x.gp:5: no warning matches \"odd\"; the line has:\n\t\tx.gp:5:2: warning: even",
			"unexpected warning: x.gp:5:2: warning: even",
		}},
		{"unexpected warning", "x.gp:3:1: warning: looks like a directive\nx.gp:5:2: undefined: x\nx.gp:5:2: warning: odd\nx.gp:6:1: warning: more\n", []string{
			"unexpected warning: x.gp:6:1: warning: more",
		}},
	} {
		diags, other := parseDiagnostics(tt.out)
		if got := matchErrors("x.gp", wants, diags, other); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}

	// A test without // ERROR comments wants no errors at all.
	diags, other := parseDiagnostics("x.gp:1:1: oops\n")
	if got, want := matchErrors("x.gp", nil, diags, other), []string{"unexpected error: x.gp:1:1: oops"}; !reflect.DeepEqual(got, want) {
		t.Errorf("no wants: got %q, want %q", got, want)
	}
}

func TestSameOutput(t *testing.T) {
	tests := []struct {
		got, want string
		same      bool
	}{
		{"a\nb\n", "a\nb\n", true},
		{"a\nb", "a\nb\n", true},     // like a Go example: space at the end doesn't count...
		{"\n\na\n", "a\n", true},     // ...nor at the start
		{"", "", true},               // a program that prints nothing
		{"a \nb\n", "a\nb\n", false}, // space at the end of a line inside the output counts
		{"a\n\nb\n", "a\nb\n", false},
		{"a\nb\n", "a\nc\n", false},
	}
	for _, tt := range tests {
		if got := sameOutput(tt.got, tt.want); got != tt.same {
			t.Errorf("sameOutput(%q, %q) = %v, want %v", tt.got, tt.want, got, tt.same)
		}
	}
}

func TestRunMismatches(t *testing.T) {
	panicRx := regexp.MustCompile(`^panic: boom\n`)
	goroutineRx := regexp.MustCompile(`(?m)^goroutine 1 \[running\]:$`)
	run := func(stdout, stderr string, exit int) result {
		return result{stdout: []byte(stdout), stderr: []byte(stderr), exit: exit, status: fmt.Sprintf("exit status %d", exit)}
	}
	tests := []struct {
		name  string
		r     result
		want  expect
		probs []string // the start of each message
		shown bool
	}{
		{"same", run("a\nb\n", "", 0), expect{stdout: "a\nb"}, nil, false},
		{"space at the ends", run("\na\nb  \n\n", "", 0), expect{stdout: "a\nb"}, nil, false},
		{"space inside", run("a \nb\n", "", 0), expect{stdout: "a\nb"}, []string{"stdout differs"}, false},
		{"exact", run("a\n", "", 0), expect{stdout: "a", exact: true}, []string{"stdout differs"}, false},
		{"exit code", run("a\n", "", 2), expect{stdout: "a"}, []string{"exit status 2, want exit code 0"}, false},
		{"stderr not empty", run("", "oops\n", 0), expect{}, []string{"stderr must be empty"}, true},
		{"any stderr", run("", "oops\n", 0), expect{anyStderr: true}, nil, false},
		{"panic", run("x\n", "panic: boom\n\ngoroutine 1 [running]:\n", 2),
			expect{stdout: "x", exitCode: 2, stderr: []*regexp.Regexp{panicRx, goroutineRx}}, nil, false},
		{"stderr regexp", run("x\n", "panic: bang\n\ngoroutine 1 [running]:\n", 2),
			expect{stdout: "x", exitCode: 2, stderr: []*regexp.Regexp{panicRx, goroutineRx}},
			[]string{fmt.Sprintf("stderr doesn't match the // Stderr: regexp %q", panicRx.String())}, false},
		{"tsan warning", run("", "==================\nWARNING: ThreadSanitizer: data race (pid=1)\n", 0), expect{anyStderr: true},
			[]string{"the output has a ThreadSanitizer report"}, false},
		{"tsan warning, exit code 66", run("", "WARNING: ThreadSanitizer: data race (pid=1)\n", 66),
			expect{exitCode: 66, anyStderr: true}, []string{"the output has a ThreadSanitizer report"}, false},
		{"tsan fatal", run("", "FATAL: ThreadSanitizer: unexpected memory mapping\n", 66),
			expect{exitCode: 66, anyStderr: true}, []string{"the output has a ThreadSanitizer report"}, false},
		{"tsan warning on stdout (combined)", run("WARNING: ThreadSanitizer: data race\n", "", 0), expect{stdout: "x", anyStderr: true},
			[]string{"the output has a ThreadSanitizer report", "stdout differs"}, false},
		{"tsan word in a program's text", run("ThreadSanitizer is fine\n", "", 0), expect{stdout: "ThreadSanitizer is fine"}, nil, false},
		{"everything", run("y\n", "", 1), expect{stdout: "x", exitCode: 2, stderr: []*regexp.Regexp{panicRx}},
			[]string{"exit status 1, want exit code 2", "stdout differs", "stderr doesn't match"}, false},
	}
	for _, tt := range tests {
		probs, shown := runMismatches(tt.r, tt.want)
		ok := len(probs) == len(tt.probs) && shown == tt.shown
		for i := 0; ok && i < len(probs); i++ {
			ok = strings.HasPrefix(probs[i], tt.probs[i])
		}
		if !ok {
			t.Errorf("%s: got %q (shown %v), want messages starting with %q (shown %v)", tt.name, probs, shown, tt.probs, tt.shown)
		}
	}
}

func TestGoRejects(t *testing.T) {
	tests := []struct {
		out    string
		reject bool
	}{
		{"# difftest\n./slices_append.go:8:2: undefined: foo\n", true},
		{"# difftest\nslices_append.go:12: syntax error\n", true},
		{"# difftest\n./slices_append.go:3:8: package raylib.h is not in std\n", true},
		{"/tmp/x/001/slices_append.go:13:2: invalid import path: <stdlib.h>\n", true}, // the whole path
		{"/tmp/x/001/other.go:13:2: invalid import path: <stdlib.h>\n", false},
		{"go: go.mod requires go >= 1.99 (running go 1.25.14; GOTOOLCHAIN=local)\n", false},
		{"go: errors parsing go.mod:\ngo.mod:3: invalid go version '1.x'\n", false},
		{"# difftest\n./other.go:8:2: undefined: foo\n", false},          // another file
		{"# difftest\n./xslices_append.go:8:2: undefined: foo\n", false}, // not the same name
		{"build constraints exclude all Go files in /tmp/x\n", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := goRejects(tt.out, "slices_append.go"); got != tt.reject {
			t.Errorf("goRejects(%q) = %v, want %v", tt.out, got, tt.reject)
		}
	}
	if got := goFileName("slices_append.gp"); got != "slices_append.go" {
		t.Errorf("goFileName = %q", got)
	}
}

func TestParseArgs(t *testing.T) {
	path := "testdata/examples/args/args.txt"
	args, err := parseArgs(path, readFile(t, path))
	if want := []string{"-n", "2", "file.txt"}; err != nil || !reflect.DeepEqual(args, want) {
		t.Errorf("%s: got %q, %v; want %q", path, args, err, want)
	}
	path = "testdata/examples/bad_args/args.txt"
	if _, err := parseArgs(path, readFile(t, path)); err == nil || err.Error() != path+": want one line of arguments, not 2 lines" {
		t.Errorf("%s: got error %v", path, err)
	}
	for _, tt := range []struct {
		data string
		want []string
		err  bool
	}{
		{"", nil, false},
		{"\n", nil, false},
		{"a\tb  c \r\n\r\n", []string{"a", "b", "c"}, false},                 // newlines at the end are fine
		{`(error|warn)\b x.txt`, []string{`(error|warn)\b`, "x.txt"}, false}, // no shell quoting
		{"a\n\nb\n", nil, true},
		{"a\rb\n", nil, true}, // a lone \r ends a line too, as in tools/gen-example-output
	} {
		args, err := parseArgs("args.txt", []byte(tt.data))
		if (err != nil) != tt.err || !reflect.DeepEqual(args, tt.want) {
			t.Errorf("parseArgs(%q) = %q, %v; want %q (error: %v)", tt.data, args, err, tt.want, tt.err)
		}
	}
}

func TestLoadExample(t *testing.T) {
	load := func(name string) (*example, error) {
		return loadExample("testdata/examples/"+name, "testdata/examples/"+name)
	}
	ex, err := load("hello")
	if err != nil {
		t.Fatal(err)
	}
	// main.gp's doc comment has lines that look like // Output: and
	// // Stderr: lines, and a comment below the package clause looks like an
	// // ExitCode: line; an example ignores them.
	if got, want := view(ex.header), (headerView{phase: 2}); !reflect.DeepEqual(got, want) {
		t.Errorf("hello: header %+v, want %+v", got, want)
	}
	if string(ex.output) != "hello\n" || ex.args != nil || ex.stdin != nil {
		t.Errorf("hello: output %q, args %q, stdin %q; want \"hello\\n\" and no args or stdin", ex.output, ex.args, ex.stdin)
	}

	if ex, err = load("args"); err != nil {
		t.Fatal(err)
	}
	if got, want := view(ex.header), (headerView{phase: 9, exitCode: 1, pkgConfig: []string{"zlib"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("args: header %+v, want %+v", got, want)
	}
	if string(ex.output) != "two lines\nof output\n" || !reflect.DeepEqual(ex.args, []string{"-n", "2", "file.txt"}) ||
		string(ex.stdin) != "line one\nline two\n" {
		t.Errorf("args: output %q, args %q, stdin %q", ex.output, ex.args, ex.stdin)
	}

	// An empty stdin.txt is an empty standard input, not a missing one.
	if ex, err = load("empty_stdin"); err != nil {
		t.Fatal(err)
	}
	if ex.stdin == nil || len(ex.stdin) != 0 || len(ex.output) != 0 {
		t.Errorf("empty_stdin: output %q, stdin %#v; want both empty and stdin not nil", ex.output, ex.stdin)
	}

	// A GUI example: a library, a display and env.txt (KEY=VALUE lines,
	// empty lines skipped, the rest of a line after the first = is the
	// value, \r\n line ends).
	if ex, err = load("gui"); err != nil {
		t.Fatal(err)
	}
	if got, want := view(ex.header), (headerView{phase: 4, pkgConfig: []string{"raylib"}, display: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("gui: header %+v, want %+v", got, want)
	}
	if want := []string{"GOCPP_SMOKE=1", "WITH_EQUALS=a=b", "EMPTY="}; !reflect.DeepEqual(ex.env, want) {
		t.Errorf("gui: env %q, want %q", ex.env, want)
	}

	if ex, err = load("windows_only"); err != nil {
		t.Fatal(err)
	}
	if got, want := view(ex.header), (headerView{phase: 14, build: "//go:build windows"}); !reflect.DeepEqual(got, want) {
		t.Errorf("windows_only: header %+v, want %+v", got, want)
	}

	for _, tt := range []struct{ name, err string }{
		{"bad_no_output", "testdata/examples/bad_no_output: no output.txt (the expected stdout of the program)"},
		{"bad_args", "testdata/examples/bad_args/args.txt: want one line of arguments, not 2 lines"},
		{"bad_phase", `testdata/examples/bad_phase/main.gp:1: the first line must be the phase line "// phase: NN"`},
		{"bad_exitcode", `testdata/examples/bad_exitcode/main.gp:2: bad "// ExitCode:" line: want an exit code from 0 to 255, not "x"`},
		{"bad_env", `testdata/examples/bad_env/env.txt:2: want a KEY=VALUE line, not "NOVALUE"`},
		{"bad_display", `testdata/examples/bad_display/main.gp:2: bad "// display:" line: want "// display: required", not "// display: maybe"`},
	} {
		if _, err := load(tt.name); err == nil || !strings.HasPrefix(err.Error(), tt.err) {
			t.Errorf("%s:\n got error %v\nwant error %s...", tt.name, err, tt.err)
		}
	}
}

func TestListExampleDirs(t *testing.T) {
	// docs/ has no main.gp, and notes.txt is not a folder.
	names, err := listExamples("testdata/examples")
	want := []string{"args", "bad_args", "bad_display", "bad_env", "bad_exitcode", "bad_no_output", "bad_phase",
		"empty_stdin", "gui", "hello", "windows_only"}
	if err != nil || !reflect.DeepEqual(names, want) {
		t.Errorf("got %q, %v; want %q", names, err, want)
	}
	if names, err := listExamples("testdata/no-such-folder"); names != nil || err != nil {
		t.Errorf("a missing examples folder: got %q, %v; want nothing", names, err)
	}
}

func TestListPrograms(t *testing.T) {
	// Not tests: README.md, helper.h, notes.gp.txt and sub/c.gp.
	names, err := listPrograms("testdata/discover/run")
	if want := []string{"a.gp", "b.gp"}; err != nil || !reflect.DeepEqual(names, want) {
		t.Errorf("got %q, %v; want %q", names, err, want)
	}
	if _, err := listPrograms("testdata/no-such-folder"); err == nil {
		t.Error("a missing folder: no error")
	}
}

func TestOrphanSnapshots(t *testing.T) {
	orphans, err := orphanSnapshots("testdata/discover/golden")
	if want := []string{"lost.cpp.golden"}; err != nil || !reflect.DeepEqual(orphans, want) {
		t.Errorf("got %q, %v; want %q", orphans, err, want)
	}
}

func TestTrimRoot(t *testing.T) {
	src := "#line 3 \"/home/me/gocpp/tests/golden/x.gp\"\n#line 4 \"/data/gocpp/tests/golden/x.gp\"\n"
	want := "#line 3 \"$ROOT/tests/golden/x.gp\"\n#line 4 \"$ROOT/tests/golden/x.gp\"\n"
	// The repository is reached through a link (/home/me/gocpp) and has a
	// real path (/data/gocpp); both are replaced.
	if got := trimRoot(src, []string{"/home/me/gocpp", "/data/gocpp"}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// The longer root is replaced first, whatever the order of the list:
	// otherwise /data/gocpp/sub/a would become $ROOT/sub/a.
	if got := trimRoot("/data/gocpp/sub/a", []string{"/data/gocpp", "/data/gocpp/sub"}); got != "$ROOT/a" {
		t.Errorf("nested roots: got %q, want %q", got, "$ROOT/a")
	}
}

func TestExplainMismatch(t *testing.T) {
	tests := []struct {
		got, want, first string
	}{
		{"a\nb\nc\n", "a\nx\nc\n", "first difference in line 2:\n\tgot:  \"b\\n\"\n\twant: \"x\\n\"\n"},
		{"a", "a\n", "first difference in line 1:\n\tgot:  \"a\"\n\twant: \"a\\n\"\n"},
		{"a\n", "a\nb\n", "first difference in line 2:\n\tgot:  (end of text)\n\twant: \"b\\n\"\n"},
	}
	for _, tt := range tests {
		if got := explainMismatch(tt.got, tt.want); !strings.HasPrefix(got, tt.first) {
			t.Errorf("explainMismatch(%q, %q) =\n%s\nwant it to start with\n%s", tt.got, tt.want, got, tt.first)
		}
	}
	long := strings.Repeat("x\n", maxShown+5)
	if got := indent(long); !strings.HasSuffix(got, "\t... (5 more lines)\n") || strings.Count(got, "\n") != maxShown+1 {
		t.Errorf("indent of %d lines:\n%s", maxShown+5, got)
	}
}

func TestPhaseGate(t *testing.T) {
	for _, tt := range []struct {
		phase, limit int
		from, want   string
	}{
		{0, 0, "now: Phase 00", ""},
		{3, 3, "now: Phase 03", ""},
		{2, 3, "now: Phase 03", ""},
		{3, 0, "now: Phase 00", "needs Phase 03 (now: Phase 00)"},
		{1, 1, "GOCPP_TEST_PHASE=01", ""},
		{2, 1, "GOCPP_TEST_PHASE=01", "needs Phase 02 (GOCPP_TEST_PHASE=01)"},
	} {
		if got := phaseGate(tt.phase, tt.limit, tt.from); got != tt.want {
			t.Errorf("phaseGate(%d, %d, %q) = %q, want %q", tt.phase, tt.limit, tt.from, got, tt.want)
		}
	}
}

// TestGateEnv checks that a gocpp command of the harness gets
// GOCPP_GATE_PHASE=NN exactly for a test of a phase NN after version.Phase,
// so gocpp's gates open up to that test's own phase and no further, whatever
// GOCPP_TEST_PHASE is.
func TestGateEnv(t *testing.T) {
	for _, tt := range []struct {
		phase int
		want  string
	}{
		{0, ""},
		{version.Phase, ""},
		{version.Phase + 1, fmt.Sprintf("GOCPP_GATE_PHASE=%02d", version.Phase+1)},
		{15, "GOCPP_GATE_PHASE=15"},
	} {
		if got := gateEnv(tt.phase); got != tt.want {
			t.Errorf("gateEnv(%d) = %q, want %q", tt.phase, got, tt.want)
		}
	}
	t.Setenv("GOCPP_TEST_PHASE", "15") // must not matter
	j := gocppJob(t.TempDir(), []string{"A=1"}, "clang++", 15, "run", ".")
	if want := []string{"A=1", "GOCPP_CXX=clang++", "GOCPP_GATE_PHASE=15"}; !slices.Equal(j.env, want) {
		t.Errorf("gocppJob env = %q, want %q", j.env, want)
	}
	if j := gocppJob(t.TempDir(), nil, "", version.Phase, "run", "."); len(j.env) != 0 {
		t.Errorf("gocppJob env = %q for a test of version.Phase, want none", j.env)
	}
	if j := checkJob(t.TempDir(), "x.gp", 1); len(j.env) != 0 {
		t.Errorf("checkJob env = %q for a Phase 01 test, want none", j.env)
	}
}

func TestPhaseLimit(t *testing.T) {
	for _, tt := range []struct {
		env   string
		phase int
		from  string
		err   bool
	}{
		{"", version.Phase, fmt.Sprintf("now: Phase %02d", version.Phase), false}, // not set: version.Phase
		{"01", 1, "GOCPP_TEST_PHASE=01", false},
		{"1", 1, "GOCPP_TEST_PHASE=1", false},
		{"00", 0, "GOCPP_TEST_PHASE=00", false},
		{"15", 15, "GOCPP_TEST_PHASE=15", false},
		{"16", 0, "", true}, // there is no Phase 16
		{"001", 0, "", true},
		{"-1", 0, "", true},
		{"+1", 0, "", true},
		{" 1", 0, "", true},
		{"one", 0, "", true},
	} {
		phase, from, err := phaseLimit(tt.env)
		if phase != tt.phase || from != tt.from || (err != nil) != tt.err {
			t.Errorf("phaseLimit(%q) = %d, %q, %v; want %d, %q, error %v", tt.env, phase, from, err, tt.phase, tt.from, tt.err)
		}
	}
	_, _, err := phaseLimit("16")
	if want := "GOCPP_TEST_PHASE=16: want a phase from 00 to 15, for example GOCPP_TEST_PHASE=01"; fmt.Sprint(err) != want {
		t.Errorf("phaseLimit(\"16\"): error %v, want %s", err, want)
	}
}

func TestMissingLibraries(t *testing.T) {
	if got := missingLibraries(nil); got != "" {
		t.Errorf("no libraries: got %q, want \"\"", got)
	}
	// Either pkg-config is not installed or it can't find the package; both
	// messages name it.
	if got := missingLibraries([]string{"gocpp-no-such-package"}); !strings.HasPrefix(got, "needs gocpp-no-such-package") {
		t.Errorf("a missing library: got %q", got)
	}
}

func TestCapBuffer(t *testing.T) {
	stops := 0
	b := &capBuffer{max: 5, stop: func() { stops++ }}
	for _, s := range []string{"abc", "defg", "h"} {
		// Every write reports all of it as written, so that os/exec's copy
		// from the pipe goes on (and the command can be stopped).
		if n, err := b.Write([]byte(s)); n != len(s) || err != nil {
			t.Errorf("Write(%q) = %d, %v", s, n, err)
		}
	}
	if b.buf.String() != "abcde" || !b.full || stops != 1 {
		t.Errorf("kept %q, full %v, stopped %d times; want \"abcde\", true, 1", b.buf.String(), b.full, stops)
	}
}

func TestParseEnv(t *testing.T) {
	for _, tt := range []struct {
		data string
		want []string
		err  string
	}{
		{"", nil, ""},
		{"A=1\n\n\nB=\n", []string{"A=1", "B="}, ""},
		{"A=1\r\nB=x=y", []string{"A=1", "B=x=y"}, ""},
		{"A=1\n=2\n", nil, `env.txt:2: want a KEY=VALUE line, not "=2"`},
		{"A=1\n  \n", nil, `env.txt:2: want a KEY=VALUE line, not "  "`}, // like the generator: only "" is skipped
	} {
		env, err := parseEnv("env.txt", []byte(tt.data))
		if got := fmt.Sprint(err); tt.err != "" && got != tt.err || tt.err == "" && err != nil || !reflect.DeepEqual(env, tt.want) {
			t.Errorf("parseEnv(%q) = %q, %v; want %q, %s", tt.data, env, err, tt.want, tt.err)
		}
	}
}

func TestTargetTag(t *testing.T) {
	for tag, want := range map[string]bool{
		runtime.GOOS:          true,
		runtime.GOARCH:        true,
		"unix":                unixOS[runtime.GOOS],
		"go1.1":               true,
		"go1.25":              true, // gocpp follows Go 1.25
		"go1.26":              false,
		"go1.0":               false,
		"go1.025":             false,
		"go2":                 false,
		"cgo":                 false, // any other tag doesn't hold
		"gc":                  false,
		"gocpp":               false,
		"no_such_os_or_arch1": false,
	} {
		if got := targetTag(tag); got != want {
			t.Errorf("targetTag(%q) = %v, want %v", tag, got, want)
		}
	}
}

func TestTargetGate(t *testing.T) {
	notWindows := runtime.GOOS != "windows"
	for _, tt := range []struct {
		line     string
		excluded bool
	}{
		{"", false}, // no //go:build line
		{"//go:build " + runtime.GOOS, false},
		{"//go:build !" + runtime.GOOS, true},
		{"//go:build windows", notWindows},
		{"//go:build !windows", !notWindows},
		{"//go:build linux || windows", runtime.GOOS != "linux" && runtime.GOOS != "windows"},
		{"//go:build unix && " + runtime.GOARCH, !unixOS[runtime.GOOS]},
		{"//go:build go1.21", false},
		{"//go:build go1.99", true},
	} {
		src := "// phase: 02\n" + tt.line + "\n\npackage main\n"
		h, err := runFormat.parse("x.gp", []byte(src+"\n// Output:\n"))
		if err != nil {
			t.Fatalf("%q: %v", tt.line, err)
		}
		got := targetGate(h)
		want := ""
		if tt.excluded {
			want = fmt.Sprintf("its %q line excludes the target %s/%s", tt.line, runtime.GOOS, runtime.GOARCH)
		}
		if got != want {
			t.Errorf("%q: targetGate = %q, want %q", tt.line, got, want)
		}
	}
}

func TestDisplayGate(t *testing.T) {
	env := func(vars ...string) func(string) string {
		return func(key string) string {
			for i := 0; i+1 < len(vars); i += 2 {
				if vars[i] == key {
					return vars[i+1]
				}
			}
			return ""
		}
	}
	const noDisplay = "needs a display (// display: required), but neither DISPLAY nor WAYLAND_DISPLAY is set"
	for _, tt := range []struct {
		display bool
		getenv  func(string) string
		want    string
	}{
		{false, env(), ""}, // no // display: line, no display: fine
		{true, env(), noDisplay},
		{true, env("DISPLAY", ""), noDisplay},
		{true, env("DISPLAY", ":0"), ""},
		{true, env("WAYLAND_DISPLAY", "wayland-0"), ""},
	} {
		if got := displayGate(tt.display, tt.getenv); got != tt.want {
			t.Errorf("displayGate(%v, ...) = %q, want %q", tt.display, got, tt.want)
		}
	}
}

func TestZoneinfoGate(t *testing.T) {
	env := func(vars ...string) func(string) string {
		return func(key string) string {
			for i := 0; i+1 < len(vars); i += 2 {
				if vars[i] == key {
					return vars[i+1]
				}
			}
			return ""
		}
	}
	has := func(dir string) func(string) bool {
		return func(p string) bool {
			return strings.HasPrefix(p, dir+"/") && (strings.HasSuffix(p, "/America/New_York") || strings.HasSuffix(p, "/Europe/Berlin") || strings.HasSuffix(p, "/Asia/Kolkata"))
		}
	}
	for _, tt := range []struct {
		zoneinfo bool
		getenv   func(string) string
		exists   func(string) bool
		skip     bool
	}{
		{false, env(), has("/nowhere"), false}, // no // zoneinfo: line: fine
		{true, env(), has("/nowhere"), true},
		{true, env(), has("/usr/share/zoneinfo"), false},
		{true, env(), has("/etc/zoneinfo"), false},
		{true, env("ZONEINFO", "/opt/tz"), has("/usr/share/zoneinfo"), true}, // ZONEINFO replaces the system's places
		{true, env("ZONEINFO", "/opt/tz"), has("/opt/tz"), false},
	} {
		got := zoneinfoGate(tt.zoneinfo, tt.getenv, tt.exists)
		if (got != "") != tt.skip {
			t.Errorf("zoneinfoGate(%v, ...) = %q, want a skip: %v", tt.zoneinfo, got, tt.skip)
		}
	}
}

func TestSkipReason(t *testing.T) {
	// The first reason counts, in this order: phase, target, display,
	// libraries.
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	excluded, err := constraint.Parse("//go:build !" + runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	libs := []string{"gocpp-no-such-package"}
	c := &config{phase: 2, phaseFrom: "GOCPP_TEST_PHASE=02"}
	for i, tt := range []struct {
		h    header
		kind string
	}{
		{header{phase: 0}, ""},
		{header{phase: 2}, ""}, // up to the limit
		{header{phase: 3, build: excluded, display: true, pkgConfig: libs}, "needs a later phase"},
		{header{phase: 0, build: excluded, buildLine: "//go:build !" + runtime.GOOS, display: true, pkgConfig: libs}, "builds for another target"},
		{header{phase: 0, display: true, pkgConfig: libs}, "needs a display"},
		{header{phase: 0, pkgConfig: libs}, "needs a missing library"},
		{header{phase: 2, pending: "step 7.1"}, "pending"},
	} {
		why, kind := c.skipReason(&tt.h)
		if kind != tt.kind || (why == "") != (kind == "") {
			t.Errorf("test %d: skipReason = %q, %q; want the kind %q", i, why, kind, tt.kind)
		}
	}
	if why, _ := c.skipReason(&header{phase: 3}); why != "needs Phase 03 (GOCPP_TEST_PHASE=02)" {
		t.Errorf("the phase reason is %q", why)
	}
	// GOCPP_TEST_PENDING=1 runs the pending tests.
	p := &config{phase: 2, phaseFrom: "GOCPP_TEST_PHASE=02", pending: true}
	if why, _ := p.skipReason(&header{phase: 2, pending: "step 7.1"}); why != "" {
		t.Errorf("a pending test skipped with GOCPP_TEST_PENDING=1: %q", why)
	}
	if why, _ := c.skipReason(&header{phase: 2, pending: "step 7.1"}); why != "pending: step 7.1 (GOCPP_TEST_PENDING=1 runs it)" {
		t.Errorf("the pending reason is %q", why)
	}
}

func TestCompareWithRealGo(t *testing.T) {
	ok := result{stdout: []byte("a\n"), stderr: []byte("e\n"), exit: 0, status: "exit status 0"}
	other := func(f func(*result)) result { r := ok; f(&r); return r }
	tests := []struct {
		mode string
		gp   result
		want []string // the start of each problem
	}{
		{diffStdout, ok, nil},
		{diffStderr, ok, nil},
		{diffCombined, ok, nil},
		{diffStdout, other(func(r *result) { r.exit, r.status = 2, "exit status 2" }), []string{"exit status 2, but real Go gives exit status 0"}},
		{diffStdout, other(func(r *result) { r.stdout = []byte("b\n") }), []string{"stdout differs from real Go's:\nfirst difference in line 1:"}},
		// Without // diff: stderr, stderr doesn't count...
		{diffStdout, other(func(r *result) { r.stderr = []byte("other\n") }), nil},
		// ...with it, it does, byte for byte.
		{diffStderr, other(func(r *result) { r.stderr = []byte("e") }), []string{"stderr differs from real Go's:\nfirst difference in line 1:\n\tgot:  \"e\"\n\twant: \"e\\n\""}},
		{diffStderr, other(func(r *result) { r.stdout, r.stderr = []byte("b\n"), nil }), []string{"stdout differs", "stderr differs"}},
		// In the combined mode stdout holds both streams; the order counts.
		{diffCombined, other(func(r *result) { r.stdout = []byte("e\na\n") }), []string{"the output (stdout and stderr in one pipe) differs from real Go's"}},
	}
	for i, tt := range tests {
		got := compareDiff(tt.mode, tt.gp, ok)
		match := len(got) == len(tt.want)
		for j := 0; match && j < len(got); j++ {
			match = strings.HasPrefix(got[j], tt.want[j])
		}
		if !match {
			t.Errorf("test %d (mode %q): got %q, want problems starting with %q", i, tt.mode, got, tt.want)
		}
	}
}

func TestCheckRequired(t *testing.T) {
	both := []compiler{{name: "g++", cxx: "g++"}, {name: "clang++", cxx: "clang++"}}
	noClang := []compiler{{name: "g++", cxx: "g++"}, {name: "clang++", cxx: "clang++", missing: `clang++ is not installed: exec: "clang++": executable file not found in $PATH`}}
	onlyGCC := []compiler{{name: "g++", cxx: "g++"}}
	byPath := []compiler{{name: "clang++", cxx: "/opt/llvm/bin/clang++"}}
	for _, tt := range []struct {
		compilers     []compiler
		required, cxx string
		err           string
	}{
		{both, "g++,clang++", "", ""},
		{both, " g++ , clang++ ,", "", ""}, // spaces and empty items don't count
		{noClang, "", "", ""},              // nothing required: a missing compiler is a skip
		{noClang, "g++", "", ""},
		{noClang, "g++,clang++", "", `GOCPP_REQUIRE_CXX=g++,clang++ needs clang++, but clang++ is not installed: exec: "clang++": executable file not found in $PATH`},
		{onlyGCC, "g++,clang++", "g++", "GOCPP_REQUIRE_CXX=g++,clang++ needs clang++, but GOCPP_CXX=g++ tests only that compiler"},
		{both, "nope++", "", "GOCPP_REQUIRE_CXX=nope++ needs nope++, which the tests don't use: they use g++ and clang++, or the compiler in GOCPP_CXX"},
		{byPath, "clang++", "/opt/llvm/bin/clang++", ""},               // the subtest name...
		{byPath, "/opt/llvm/bin/clang++", "/opt/llvm/bin/clang++", ""}, // ...or the GOCPP_CXX value
	} {
		err := checkRequired(tt.compilers, tt.required, tt.cxx)
		if got := fmt.Sprint(err); tt.err == "" && err != nil || tt.err != "" && got != tt.err {
			t.Errorf("checkRequired(%v, %q, %q) = %v, want %q", tt.compilers, tt.required, tt.cxx, err, tt.err)
		}
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"g++":                  "g++",
		"testdata/poem.txt":    "testdata/poem.txt",
		"-n":                   "-n",
		"GOCPP_SMOKE=1":        "GOCPP_SMOKE=1",
		`(error|warn)\b`:       `'(error|warn)\b'`,
		"a b":                  "'a b'",
		"it's":                 `'it'\''s'`,
		"":                     "''",
		"$HOME":                "'$HOME'",
		"examples/grep-lite/x": "examples/grep-lite/x",
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestCheckJob(t *testing.T) {
	// gocpp check gets -e, so that a test can want more than 10 errors.
	j := checkJob(filepath.Join(repoRoot, "tests", "errors"), "many.gp", 1)
	if want := []string{"check", "-e", "many.gp"}; !reflect.DeepEqual(j.args, want) || !j.combined {
		t.Errorf("checkJob: args %q, combined %v; want %q and true", j.args, j.combined, want)
	}
	if want := "cd tests/errors && gocpp check -e many.gp"; j.show != want {
		t.Errorf("checkJob shows %q, want %q", j.show, want)
	}
}

func TestProgramFile(t *testing.T) {
	// A folder test is run as ".", and the compiler prints its file as main.gp.
	for arg, want := range map[string]string{"x.gp": "x.gp", ".": "main.gp"} {
		if got := (&program{arg: arg}).file(); got != want {
			t.Errorf("arg %q: file() = %q, want %q", arg, got, want)
		}
	}
}

// TestMakefileTestPhase checks CI_TEST_PHASE of the Makefile, the phase that make ci, make quick
// and make ci-sanitize run the tests up to: a number from 00 to 15, never below version.Phase
// (the tests of the finished phases must run in make ci).
func TestMakefileTestPhase(t *testing.T) {
	src := readFile(t, "../Makefile")
	m := regexp.MustCompile(`(?m)^CI_TEST_PHASE\s*=\s*(\d\d)\s*$`).FindSubmatch(src)
	if m == nil {
		t.Fatal("the Makefile has no line CI_TEST_PHASE = NN (two digits)")
	}
	n, _ := strconv.Atoi(string(m[1]))
	if n < version.Phase || n > lastPhase {
		t.Errorf("CI_TEST_PHASE = %02d, want %02d (version.Phase) to %02d", n, version.Phase, lastPhase)
	}
	if !regexp.MustCompile(`(?m)^test .*ci-sanitize.*: export GOCPP_TEST_PHASE \?= \$\(CI_TEST_PHASE\)$`).Match(src) {
		t.Error("the Makefile does not set GOCPP_TEST_PHASE to $(CI_TEST_PHASE) for test, quick and ci-sanitize")
	}
}

// TestPendingFiles lists the tests with a // pending: line: exactly the known ones, so that hiding
// another test needs a deliberate edit here. A pending test of a finished phase is an error
// (checkPending).
func TestPendingFiles(t *testing.T) {
	want := []string{"../examples/grep-lite/main.gp", "../examples/macros/main.gp", "../examples/must-errors/main.gp", "../examples/wc/main.gp", "../examples/workers/main.gp"}
	var got []string
	for _, root := range []string{"../tests", "../examples"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".gp") {
				return nil
			}
			for line := range strings.Lines(string(readFile(t, path))) {
				if strings.HasPrefix(line, "package ") {
					break // the header ends at the package clause
				}
				if strings.HasPrefix(line, pendingPrefix) {
					got = append(got, filepath.ToSlash(path))
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("tests with a // pending: line:\n got %q\nwant %q", got, want)
	}
}

func TestCheckPending(t *testing.T) {
	if err := checkPending("x.gp", &header{phase: version.Phase + 1, pending: "step"}); err != nil {
		t.Errorf("a pending test of a later phase: %v", err)
	}
	if err := checkPending("x.gp", &header{phase: version.Phase}); err != nil {
		t.Errorf("a test without a pending line: %v", err)
	}
	err := checkPending("x.gp", &header{phase: version.Phase, pending: "step"})
	if err == nil || !strings.Contains(err.Error(), "pending: test of a finished phase") {
		t.Errorf("a pending test of a finished phase: %v", err)
	}
}

// TestAllowInternalOnlyByHeader checks that the harness does not pass a developer's
// GOCPP_ALLOW_INTERNAL on to the gocpp runs (TestMain unsets it) and that runEnv sets it for a test
// with "// std-internal:" only.
func TestAllowInternalOnlyByHeader(t *testing.T) {
	if v, ok := os.LookupEnv(allowInternalName); ok {
		t.Errorf("%s is %q in the test binary's environment; TestMain must remove it", allowInternalName, v)
	}
	if env := runEnv(&header{}); slices.Contains(env, allowInternalEnv) {
		t.Errorf("runEnv(no std-internal line) = %q", env)
	}
	if env := runEnv(&header{stdInternal: "tests internal/bytealg"}); !slices.Contains(env, allowInternalEnv) {
		t.Errorf("runEnv(std-internal line) = %q, want %s", env, allowInternalEnv)
	}
}
