package tests

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestDebug checks that the programs in tests/debug can be debugged at the
// .gp level. It builds each one with gocpp build (the default build, -g
// -O0; GOCPP_FLAGS doesn't apply) and each compiler, then runs two
// subtests on the binary:
//   - lines reads the DWARF line table (checkLineTable in debug.go);
//   - gdb runs a gdb session made from the program's // gdb: comments
//     (gdbSteps), and is skipped when gdb is missing or can't trace
//     programs here.
//
// See tests/debug/README.md.
func TestDebug(t *testing.T) {
	c := setup(t)
	c.runPrograms(t, "tests/debug", loadTests(t, debugFormat), c.debugTest)
}

// debugTest builds a debug test with each compiler and checks the binary.
func (c *config) debugTest(t *testing.T, p *program) {
	c.forEachCompiler(t, func(t *testing.T, cc compiler) {
		bin := filepath.Join(t.TempDir(), p.name+exe)
		r := c.run(t, gocppJob(p.dir, nil, cc.cxx, p.header.phase, "build", "-o", bin, p.arg))
		if r.problem != "" || r.exit != 0 {
			t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
		}
		t.Run("lines", func(t *testing.T) { checkLines(t, p.debug, bin, r.cmd) })
		t.Run("gdb", func(t *testing.T) { c.gdbTest(t, p.debug, bin, r.cmd) })
	})
}

// checkLines checks the line table of bin, built from p by the command
// build.
func checkLines(t *testing.T, p *debugProgram, bin, build string) {
	if runtime.GOOS == "windows" {
		t.Skip("the line table check reads ELF binaries; Windows binaries (PE) come with Phase 14")
	}
	problems, summary, err := p.checkLineTable(bin)
	if err != nil {
		t.Fatalf("%s: %v", build, err)
	}
	if len(problems) > 0 {
		t.Errorf("%s: the line table of the binary is wrong:\n\t%s", build, strings.Join(problems, "\n\t"))
	}
	t.Log(summary)
}

// gdbTest runs the gdb session of p (gdbSteps) on bin, built from p by the
// command build, and checks what gdb printed for each step.
func (c *config) gdbTest(t *testing.T, p *debugProgram, bin, build string) {
	gdb, err := exec.LookPath("gdb")
	if err != nil {
		c.skipGDB(t, fmt.Sprintf("gdb is not installed (the dev shell, nix develop, has it): %v", err))
	}
	steps := p.gdbSteps()
	// -nx: no gdbinit files. No shell to start the program (fish would read
	// its config), no debuginfod downloads, no auto-loaded scripts (such as
	// libstdc++'s printers from a system path), no colors, no line wrapping;
	// LC_ALL=C keeps the messages in English.
	args := []string{"-nx", "-batch",
		"-iex", "set startup-with-shell off", "-iex", "set debuginfod enabled off",
		"-iex", "set auto-load python-scripts off", "-iex", "set style enabled off",
		"-ex", "set width 0", "-ex", "set height 0"}
	for i, s := range steps {
		args = append(args, "-ex", `echo \n`+gdbStepMark(i)+`\n`, "-ex", s.cmd)
	}
	args = append(args, "--args", bin)
	show := "gdb"
	for _, a := range args {
		show += " " + shellQuote(a)
	}
	j := job{dir: filepath.Dir(bin), env: []string{"DEBUGINFOD_URLS=", "LC_ALL=C"}, name: gdb, args: args, show: show, combined: true}
	r := c.run(t, j)
	if r.problem != "" {
		t.Fatalf("%s: %s (%s)\noutput:\n%s", r.cmd, r.problem, r.status, indent(string(r.stdout)))
	}
	out := string(r.stdout)
	parts := splitGDBOutput(out, len(steps))
	if i := slices.IndexFunc(steps, func(s gdbStep) bool { return s.cmd == "run" }); i >= 0 {
		if m := ptraceProblem(parts[i]); m != "" {
			c.skipGDB(t, fmt.Sprintf("gdb can't trace programs here (%q): ptrace is not allowed, as in some containers and sandboxes", m))
		}
	}
	for i, s := range steps {
		if s.check == nil {
			continue
		}
		if err := s.check(parts[i]); err != nil {
			t.Fatalf("%s: gdb command %q: %v\ngdb printed:\n%s\nThe binary comes from:\n\t%s\nThe session (the binary is gone after the test; gdb: %s):\n\t%s\nIts output:\n%s",
				s.where, s.cmd, err, indent(parts[i]), build, r.status, r.cmd, indent(out))
		}
	}
	t.Logf("%d gdb commands, %d stops", len(steps), len(p.stops))
}

// skipGDB skips the gdb part of a debug test with the message why, or
// fails it with GOCPP_REQUIRE_GDB=1 (make ci).
func (c *config) skipGDB(t *testing.T, why string) {
	t.Helper()
	if c.requireGDB {
		t.Fatalf("%s; GOCPP_REQUIRE_GDB=1 (make ci) needs gdb (GOCPP_REQUIRE_GDB= make ci turns this off)", why)
	}
	t.Skip(why)
}

// debugSrc is a debug test for the unit tests of debug.go. Its lines:
// 7 func add, 8 the stop in add, 9 its brace, 11 func main, 12 the call,
// 13 the stop in main.
const debugSrc = `// phase: 02

package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10 // gdb: print a = 7; print b ~ ^5$; bt gpkg_main::add gpkg_main::main; next 9 13
}

func main() {
	s, big := add(7, 5)
	fmt.Println(s, big) // gdb: print s = 12; next 14
}
`

func mustParseDebug(t *testing.T, src string) *debugProgram {
	t.Helper()
	p, err := parseDebug("tests/debug/x.gp", "/work/x.gp", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDebug(t *testing.T) {
	p := mustParseDebug(t, debugSrc)
	wantFuncs := []debugFunc{{name: "add", nameLine: 7, start: 7, end: 9}, {name: "main", nameLine: 11, start: 11, end: 14}}
	if !slices.Equal(p.funcs, wantFuncs) {
		t.Errorf("funcs = %+v, want %+v", p.funcs, wantFuncs)
	}
	// add ends with a return, so its brace (9) needn't have code; main's (14) must.
	if want := []int{7, 8, 11, 12, 13, 14}; !slices.Equal(p.code, want) {
		t.Errorf("code = %v, want %v", p.code, want)
	}
	if len(p.stops) != 2 || p.stops[0].line != 8 || p.stops[1].line != 13 {
		t.Fatalf("stops = %+v, want lines 8 and 13", p.stops)
	}
	c := p.stops[0].cmds
	if len(c) != 4 || c[0].op != "print" || c[0].expr != "a" || c[0].want != "7" || c[0].rx != nil ||
		c[1].expr != "b" || c[1].rx == nil || c[1].rx.String() != "^5$" ||
		!slices.Equal(c[2].frames, []string{"gpkg_main::add", "gpkg_main::main"}) ||
		c[3].op != "next" || !slices.Equal(c[3].lines, []int{9, 13}) {
		t.Errorf("the commands of line 8 = %+v", c)
	}
	// The first " = " or " ~ " ends the expression.
	for _, tt := range []struct{ cmd, expr, want, rx string }{
		{"print a == b = true", "a == b", "true", ""},
		{"print s ~ gp_data_ = 0x[0-9a-f]+ \"go\"", "s", "", `gp_data_ = 0x[0-9a-f]+ "go"`},
		{"print c = '~' ~ x", "c", "'~' ~ x", ""},
	} {
		cmds, err := parseGDBCommands(tt.cmd)
		if err != nil || len(cmds) != 1 {
			t.Errorf("%q: %+v, %v", tt.cmd, cmds, err)
			continue
		}
		c := cmds[0]
		if c.expr != tt.expr || c.want != tt.want || (c.rx == nil) != (tt.rx == "") || c.rx != nil && c.rx.String() != tt.rx {
			t.Errorf("%q: expr %q, value %q, regexp %v; want %q, %q, %q", tt.cmd, c.expr, c.want, c.rx, tt.expr, tt.want, tt.rx)
		}
	}
	if !p.paths["/work/x.gp"] || !p.isCPP("/tmp/w/x.gp.cpp") || p.isCPP("/work/x.gp") || p.isCPP("x.gp.cpp") {
		t.Errorf("paths = %v; isCPP is wrong", p.paths)
	}
}

func TestCodeLines(t *testing.T) {
	src := `// phase: 02

package main

func f(n int) {
	const c = 1
	var x, y = 1, 2
	var (
		z int
	)
	if n > c {
		x++
	}
	for {
		break
	}
	for i := 0; i < n; i++ {
		y += i
	}
	switch {
	case n == 0:
		fallthrough
	default:
	}
	switch n {
	case 1:
	}
loop:
	for range n {
		continue loop
	}
	{
		_ = z
	}
	return // gdb: print n = 0
}
`
	p := mustParseDebug(t, src)
	want := []int{5, 7, 9, 11, 12, 15, 17, 18, 29, 30, 33, 35}
	if !slices.Equal(p.code, want) {
		t.Errorf("code = %v, want %v", p.code, want)
	}
}

func TestParseDebugMalformed(t *testing.T) {
	tests := []struct {
		line string // replaces line 13 of debugSrc
		want string
	}{
		{"\tfmt.Println(s, big) // gdb: step", `13: unknown command "step" in a "// gdb:" comment: want print, bt or next`},
		{"\tfmt.Println(s, big) // gdb: print s", `13: bad "print" command: want "print EXPR = VALUE" or "print EXPR ~ REGEXP", not "print s"`},
		{"\tfmt.Println(s, big) // gdb: print = 1", `13: bad "print" command`},
		{"\tfmt.Println(s, big) // gdb: print s ~ (", `13: bad "print" command "print s ~ (": error parsing regexp`},
		{"\tfmt.Println(s, big) // gdb: bt", `13: bad "bt" command`},
		{"\tfmt.Println(s, big) // gdb: bt main", `13: bt: main is not a func of the program (write the name as gdb shows it: gpkg_main::f)`},
		{"\tfmt.Println(s, big) // gdb: next", `13: bad "next" command`},
		{"\tfmt.Println(s, big) // gdb: next 14 x", `13: bad "next" command: want the line where each next stops ("next 12 13"), not "next 14 x"`},
		{"\tfmt.Println(s, big) // gdb: next 0", `13: bad "next" command`},
		{"\tfmt.Println(s, big) // gdb: next 10", `13: next: line 10 is not in a func`},
		{"\tfmt.Println(s, big) // gdb: print s = 12;", `13: unknown command ""`},
		{"\t// gdb: print s = 12", `13: a "// gdb:" comment belongs at the end of a line with a statement`},
		{"\tfmt.Println(s, big) //gdb: print s = 12", `13: write a gdb comment as "// gdb: ...", not "//gdb: print s = 12"`},
		{"\tfmt.Println(s, big) // GDB : print s = 12", `13: write a gdb comment as "// gdb: ...", not "// GDB : print s = 12"`},
	}
	lines := strings.Split(debugSrc, "\n")
	for _, tt := range tests {
		src := slices.Clone(lines)
		src[12] = tt.line
		_, err := parseDebug("tests/debug/x.gp", "/work/x.gp", []byte(strings.Join(src, "\n")))
		if err == nil || !strings.HasPrefix(err.Error(), "tests/debug/x.gp:"+tt.want) {
			t.Errorf("line 13 %q: error %v, want tests/debug/x.gp:%s", tt.line, err, tt.want)
		}
	}
	// A case clause may have no code (a C++ switch case), so it can't be a stop.
	src := "// phase: 02\n\npackage main\n\nfunc main() {\n\tswitch 1 {\n\tcase 1: // gdb: next 8\n\t}\n}\n"
	if _, err := parseDebug("x.gp", "/work/x.gp", []byte(src)); err == nil || !strings.Contains(err.Error(), "x.gp:7: a \"// gdb:\" comment belongs") {
		t.Errorf("a stop on a case line: error %v", err)
	}
	src = "// phase: 02\n\npackage main\n\nfunc main() {\n}\n"
	if _, err := parseDebug("x.gp", "/work/x.gp", []byte(src)); err == nil || err.Error() != `x.gp: no "// gdb:" comment: a debug test needs at least one` {
		t.Errorf("no stop: error %v", err)
	}
}

func TestGDBSteps(t *testing.T) {
	p := mustParseDebug(t, debugSrc)
	var got []string
	for _, s := range p.gdbSteps() {
		got = append(got, s.cmd)
	}
	// The next in add stops at line 13, the second stop: no breakpoint
	// and no continue for it.
	want := []string{
		"tbreak x.gp:8", "run", "frame", "print a", "print b", "bt", "next", "frame", "next", "frame",
		"frame", "print s", "next", "frame",
		"continue",
	}
	if !slices.Equal(got, want) {
		t.Errorf("steps:\n got %q\nwant %q", got, want)
	}
}

func TestGDBChecks(t *testing.T) {
	p := mustParseDebug(t, debugSrc)
	steps := p.gdbSteps()
	out := func(i int, s string) error {
		t.Helper()
		if steps[i].check == nil {
			t.Fatalf("step %d (%s) has no check", i, steps[i].cmd)
		}
		return steps[i].check(s)
	}
	tests := []struct {
		step int
		out  string
		want string // "" for no error
	}{
		{0, "Temporary breakpoint 1 at 0x338d: file /work/x.gp, line 8.\n", ""},
		{0, "Temporary breakpoint 1 at 0x338d: file /work/x.gp, line 9.\n", "gdb put the breakpoint at /work/x.gp:9, not at line 8 of tests/debug/x.gp"},
		{0, "Temporary breakpoint 1 at 0x338d: file /else/x.gp, line 8.\n", "gdb put the breakpoint at /else/x.gp:8"},
		{0, "No source file named x.gp.\n", "gdb set no breakpoint"},
		// A line with code in two functions: the func and a lambda.
		{0, "Temporary breakpoint 1 at 0x34ce: x.gp:8. (2 locations)\n", ""},
		{0, "Temporary breakpoint 1 at 0x34ce: x.gp:9. (2 locations)\n", "gdb put the breakpoint at x.gp:9, not at line 8"},
		{0, "Temporary breakpoint 1 at 0x34ce: y.gp:8. (2 locations)\n", "gdb put the breakpoint at y.gp:8"},
		{1, "\nTemporary breakpoint 1, gpkg_main::add (a=7, b=5) at /work/x.gp:8\n8\t...\n", ""},
		{1, "[Inferior 1 (process 42) exited normally]\n", "the program ended before it reached line 8"},
		{2, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n8\t\treturn\n", ""},
		{2, "#0  gpkg_main::main () at /work/x.gp:8\n", "stopped in gpkg_main::main at /work/x.gp:8, want gpkg_main::add at line 8"},
		{2, "#0  0x0000555555557412 in gpkg_main::add (a=7, b=5) at /work/x.gp:9\n", "stopped in gpkg_main::add at /work/x.gp:9, want gpkg_main::add at line 8"},
		{2, "#0  gp::run_main (argc=1) at /rt/src/rt.cpp:529\n", "stopped in gp::run_main at /rt/src/rt.cpp:529"},
		{2, "No stack.\n", "gdb shows no frame"},
		{3, "$1 = 7\n", ""},
		{3, "$1 = 70\n", "print a = 70, want 7"},
		{3, "No symbol \"a\" in current context.\n", "gdb printed no value"},
		{4, "$2 = 5\n", ""},
		{4, "$2 = 5 '\\005'\n", `print b = 5 '\005', which doesn't match "^5$"`},
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#2  0x000055555555b8d0 in gp::run_main (argc=1, argv=0x7ff, init=0x0, main=0x5555555573ce <gpkg_main::main()>) at /rt/src/rt.cpp:519\n" +
			"#3  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/gocpp-build1/x.gp.cpp:27\n", ""},
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#2  0x000055555555749e in main (argc=1, argv=0x7ff) at /work/x.gp:16\n", "the outermost frame is main at /work/x.gp:16, want main at a line of the generated x.gp.cpp"},
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x00005555555573ed in gpkg_main::main () at /tmp/w/x.gp.cpp:30\n" +
			"#2  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", "frame #1 is gpkg_main::main at /tmp/w/x.gp.cpp:30, not at a line of main in tests/debug/x.gp (lines 11 to 14)"},
		{5, "#0  gpkg_main::main () at /work/x.gp:12\n" +
			"#1  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", "the program's frames are [gpkg_main::main], want [gpkg_main::add gpkg_main::main]"},
		{5, "#0  0x00007ffff7a3b2c4 in write () from /lib/libc.so.6\n" +
			"#1  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#2  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#3  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", "frame #0 is write (no line information); the program's frames must come first"},
		// A deferred call: a run of the runtime's frames (gp::, std::) ending with
		// gp::defer_frame::gp_run may come between the program's frames, nothing else.
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x000055555555b2f1 in std::__invoke_impl<void, void (*&)(long, long), long, long> (__f=...) at /usr/include/c++/15/bits/invoke.h:63\n" +
			"#2  0x000055555555c8e9 in gp::defer_frame::gp_run (this=0x7fffffff9300) at /rt/src/defer.cpp:69\n" +
			"#3  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#4  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", ""},
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x00007ffff7a3b2c4 in qsort () from /lib/libc.so.6\n" +
			"#2  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#3  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", "frame #1 is qsort (no line information); the program's frames must come first"},
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x000055555555c8e9 in gp::detail::release (p=0x7fffffff9300) at /rt/include/gocpp/ptr.hpp:69\n" +
			"#2  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#3  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", "frame #1 is gp::detail::release at /rt/include/gocpp/ptr.hpp:69; the program's frames must come first"},
		{5, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:8\n" +
			"#1  0x000055555555c8e9 in gp::defer_frame::gp_run (this=0x7fffffff9300) at /rt/src/defer.cpp:69\n" +
			"#2  0x000055555555b2f1 in std::function<void ()>::operator() (this=0x7ff) at /usr/include/c++/15/bits/std_function.h:591\n" +
			"#3  0x00005555555573ed in gpkg_main::main () at /work/x.gp:12\n" +
			"#4  0x000055555555749e in main (argc=1, argv=0x7ff) at /tmp/w/x.gp.cpp:27\n", "frame #1 is gp::defer_frame::gp_run at /rt/src/defer.cpp:69; the program's frames must come first"},
		{7, "#0  gpkg_main::add (a=7, b=5) at /work/x.gp:9\n9\t}\n", ""},
		{len(steps) - 1, "k = 7\n[Inferior 1 (process 1034679) exited normally]\n", ""},
		{len(steps) - 1, "[Inferior 1 (process 1034679) exited with code 02]\n", "the program didn't exit normally"},
	}
	for _, tt := range tests {
		err := out(tt.step, tt.out)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("step %d (%s) on %q: %v", tt.step, steps[tt.step].cmd, tt.out, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("step %d (%s) on %q: error %v, want %q", tt.step, steps[tt.step].cmd, tt.out, err, tt.want)
		}
	}
}

func TestSplitGDBOutput(t *testing.T) {
	out := "noise\n\n" + gdbStepMark(0) + "\nfirst\n\n" + gdbStepMark(1) + "\nsecond\nline\n\n" + gdbStepMark(2) + "\n"
	got := splitGDBOutput(out, 4)
	want := []string{"first\n\n", "second\nline\n\n", "", ""}
	if !slices.Equal(got, want) {
		t.Errorf("splitGDBOutput = %q, want %q", got, want)
	}
}

func TestPtraceRx(t *testing.T) {
	// What gdb says when it may not trace the program (gdb/nat/linux-ptrace.c).
	for _, s := range []string{
		"warning: Error disabling address space randomization: Operation not permitted\nCould not trace the inferior process.\nwarning: ptrace: Operation not permitted\nDuring startup program exited with code 127.",
		"ptrace: Operation not permitted.",
	} {
		if !ptraceRx.MatchString(s) {
			t.Errorf("ptraceRx doesn't match %q", s)
		}
	}
	if ptraceRx.MatchString("warning: Error disabling address space randomization: Operation not permitted\n") {
		t.Error("ptraceRx matches the randomization warning alone, which gdb also prints where it can trace")
	}
	// Only a run that didn't stop is a skip.
	denied := "Could not trace the inferior process.\nwarning: ptrace: Operation not permitted\nDuring startup program exited with code 127.\n"
	if got := ptraceProblem(denied); got == "" {
		t.Errorf("ptraceProblem(%q) = \"\", want the ptrace message", denied)
	}
	stopped := "warning: ptrace: Operation not permitted\n\nTemporary breakpoint 1, gpkg_main::add (a=7, b=5) at /work/x.gp:8\n8\t...\n"
	if got := ptraceProblem(stopped); got != "" {
		t.Errorf("ptraceProblem(%q) = %q, want \"\": the program stopped at the breakpoint", stopped, got)
	}
	if got := ptraceProblem("Temporary breakpoint 1.2, gpkg_main::main () at /work/x.gp:12\nptrace: Operation not permitted.\n"); got != "" {
		t.Errorf("ptraceProblem after a stop at location 1.2 = %q, want \"\"", got)
	}
}
