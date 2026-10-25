package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The $GODEBUG a program starts with (internal/godebug's start-up value, runtime/src/shims.cpp
// at_start): tests/testdata/godebug/startenv.gp, built by gocpp and by real Go, runs with exact
// environments. exec.Cmd would keep only the last of two entries with one name, so a small
// launcher, built by real Go, starts the program with os.StartProcess, which passes the entries as
// they are. The two outputs must be the same.

// godebugLauncher is the launcher: launch PROG ENTRY... starts PROG with exactly the environment
// entries ENTRY..., waits and exits with its exit code.
const godebugLauncher = `package main

import "os"

func main() {
	p, err := os.StartProcess(os.Args[1], os.Args[1:2], &os.ProcAttr{
		Env:   append([]string{}, os.Args[2:]...),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		println(err.Error())
		os.Exit(100)
	}
	st, err := p.Wait()
	if err != nil {
		println(err.Error())
		os.Exit(100)
	}
	os.Exit(st.ExitCode())
}
`

// godebugEnvs are the environments of both systems: no GODEBUG, a value, the first of two entries
// wins (both ways), an empty value, and a name of another case (another variable on Linux).
var godebugEnvs = [][]string{
	nil,
	{"GODEBUG=randseednop=0"},
	{"GODEBUG=randseednop=0", "GODEBUG=randseednop=1"},
	{"GODEBUG=randseednop=1", "GODEBUG=randseednop=0"},
	{"GODEBUG="},
	{"GODEBUG=x=1,randseednop=0"},
	{"godebug=randseednop=0"},
}

// godebugWindowsEnvs are the ones of Windows only: names differ in case (one variable for the
// system, but a block can hold two or three, which Unsetenv removes one at a time).
var godebugWindowsEnvs = [][]string{
	{"GoDebug=x=1,randseednop=0"},
	{"godebug=randseednop=0", "GODEBUG=randseednop=1"},
	{"GODEBUG=randseednop=1", "godebug=randseednop=0"},
	{"SystemRoot=C:\\windows", "godebug=randseednop=0"},
	{"godebug=randseednop=0", "GODEBUG=randseednop=1", "GoDebug=x=1"},
}

// godebugCase names the subtest of the environment env.
func godebugCase(env []string) string {
	if len(env) == 0 {
		return "no_variables"
	}
	return strings.Join(env, " ")
}

// buildGodebugGo builds the Go source src (one main.go) for goos into dir/name and returns its path.
func buildGodebugGo(t *testing.T, c *config, goos, src, name string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module godebugtest\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, name)
	b := c.run(t, job{
		dir:      dir,
		env:      []string{"GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOAMD64=v1", "GOOS=" + goos, "GOARCH=amd64", "CGO_ENABLED=0"},
		name:     "go",
		args:     []string{"build", "-o", out, "."},
		show:     "GOOS=" + goos + " go build (" + name + ")",
		combined: true,
	})
	if b.problem != "" || b.exit != 0 {
		t.Fatalf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stdout)))
	}
	return out
}

// godebugPrograms builds startenv.gp for goos with gocpp and with real Go, and the launcher.
func godebugPrograms(t *testing.T, c *config, goos, suffix string) (gocpp, golang, launch string) {
	t.Helper()
	dir := filepath.Join(repoRoot, "tests", "testdata", "godebug")
	src, err := os.ReadFile(filepath.Join(dir, "startenv.gp"))
	if err != nil {
		t.Fatal(err)
	}
	golang = buildGodebugGo(t, c, goos, string(src), "startenv-go"+suffix)
	launch = buildGodebugGo(t, c, goos, godebugLauncher, "launch"+suffix)
	gocpp = filepath.Join(t.TempDir(), "startenv-gocpp"+suffix)
	args := []string{"build", "-o", gocpp, "startenv.gp"}
	if goos == "windows" {
		args = []string{"build", "-os", "windows", "-o", gocpp, "startenv.gp"}
	}
	b := c.run(t, gocppJob(dir, nil, "", 9, args...))
	if b.problem != "" || b.exit != 0 {
		t.Fatalf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
	}
	return gocpp, golang, launch
}

// compareGodebug checks gocpp's result against real Go's for the environment env.
func compareGodebug(t *testing.T, env []string, gp, golang result) {
	t.Helper()
	for _, r := range []result{gp, golang} {
		if r.problem != "" || r.exit != 0 {
			t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
		}
	}
	norm := func(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }
	if out := norm(golang.stdout); !strings.HasPrefix(out, "var: ") || strings.Count(out, "\n") != 8 {
		t.Fatalf("environment %q: real Go's program printed\n%s\nwant 8 lines, the first one \"var: ...\"", env, indent(out))
	}
	if norm(gp.stdout) != norm(golang.stdout) {
		t.Errorf("environment %q: gocpp's program printed\n%s\nreal Go's printed\n%s", env, indent(norm(gp.stdout)), indent(norm(golang.stdout)))
	}
	if len(gp.stderr) != 0 {
		t.Errorf("environment %q: gocpp's program wrote to stderr:\n%s", env, indent(string(gp.stderr)))
	}
}

func TestGodebugStartEnv(t *testing.T) {
	c := setup(t)
	if why := phaseGate(9, c.phase, c.phaseFrom); why != "" {
		t.Skip(why)
	}
	gocpp, golang, launch := godebugPrograms(t, c, "linux", "")
	for _, env := range godebugEnvs {
		t.Run(godebugCase(env), func(t *testing.T) {
			t.Parallel()
			run := func(prog string) result {
				return c.run(t, job{dir: t.TempDir(), name: launch, args: append([]string{prog}, env...), show: "launch " + prog + " " + strings.Join(env, " ")})
			}
			compareGodebug(t, env, run(gocpp), run(golang))
		})
	}
}

func TestGodebugStartEnvWindows(t *testing.T) {
	if os.Getenv("GOCPP_TEST_TARGET") != "windows" {
		t.Skip("GOCPP_TEST_TARGET=windows turns it on")
	}
	if windowsHost() != "" {
		t.Skip("GOCPP_WINDOWS_SSH is set: the launcher and the two programs would have to be copied to the PC together; this runs under Wine")
	}
	c := setup(t)
	if why := phaseGate(9, c.phase, c.phaseFrom); why != "" {
		t.Skip(why)
	}
	if why := windowsUnavailable(); why != "" {
		if os.Getenv("GOCPP_REQUIRE_WINE") == "1" {
			t.Fatalf("GOCPP_REQUIRE_WINE=1, but the Windows mode can't run: %s", why)
		}
		t.Skip(why)
	}
	w := winrunner(t)
	gocpp, golang, launch := godebugPrograms(t, c, "windows", ".exe")
	for _, env := range append(godebugEnvs[:len(godebugEnvs):len(godebugEnvs)], godebugWindowsEnvs...) {
		t.Run(godebugCase(env), func(t *testing.T) {
			t.Parallel()
			w.sem <- struct{}{}
			defer func() { <-w.sem }()
			run := func(prog string) result {
				// Wine's drive Z: is the Unix root.
				args := append([]string{`Z:` + strings.ReplaceAll(prog, "/", `\`)}, env...)
				return w.execute(t, c, t.TempDir(), launch, args, nil, "launch.exe "+strings.Join(args, " "), false)
			}
			compareGodebug(t, env, run(gocpp), run(golang))
		})
	}
}

// envCRTCases are the environments of TestWindowsEnvCRT: names that differ only in case, one to
// three of them.
var envCRTCases = [][]string{
	{"ZV=one"},
	{"zv=a", "ZV=b"},
	{"ZV=b", "zv=a"},
	{"zv=a", "ZV=b", "Zv=c"},
}

// TestWindowsEnvCRT runs tests/testdata/windows/envcrt.gp under Wine with exact environments (the
// launcher of TestGodebugStartEnvWindows) and checks, after every step, that C's getenv gives the
// block's first value of the name (NULL for none or ""), that an unset removed exactly one
// variable of the name (Go's syscall.Unsetenv removes the first), and that a set changed the first
// one.
func TestWindowsEnvCRT(t *testing.T) {
	if os.Getenv("GOCPP_TEST_TARGET") != "windows" {
		t.Skip("GOCPP_TEST_TARGET=windows turns it on")
	}
	if windowsHost() != "" {
		t.Skip("GOCPP_WINDOWS_SSH is set: the launcher and the program would have to be copied to the PC together; this runs under Wine")
	}
	c := setup(t)
	if why := windowsUnavailable(); why != "" {
		if os.Getenv("GOCPP_REQUIRE_WINE") == "1" {
			t.Fatalf("GOCPP_REQUIRE_WINE=1, but the Windows mode can't run: %s", why)
		}
		t.Skip(why)
	}
	w := winrunner(t)
	launch := buildGodebugGo(t, c, "windows", godebugLauncher, "launch.exe")
	prog := filepath.Join(t.TempDir(), "envcrt.exe")
	dir := filepath.Join(repoRoot, "tests", "testdata", "windows")
	if b := c.run(t, gocppJob(dir, nil, "", 14, "build", "-os", "windows", "-o", prog, "envcrt.gp")); b.problem != "" || b.exit != 0 {
		t.Fatalf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
	}
	for _, env := range envCRTCases {
		t.Run(strings.Join(env, " "), func(t *testing.T) {
			t.Parallel()
			w.sem <- struct{}{}
			defer func() { <-w.sem }()
			args := append([]string{`Z:` + strings.ReplaceAll(prog, "/", `\`), `SystemRoot=C:\windows`}, env...)
			r := w.execute(t, c, t.TempDir(), launch, args, nil, "launch.exe "+strings.Join(args, " "), false)
			if r.problem != "" || r.exit != 0 || len(r.stderr) != 0 {
				t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
			}
			checkEnvCRT(t, strings.ReplaceAll(string(r.stdout), "\r\n", "\n"))
		})
	}
}

// checkEnvCRT checks the lines of envcrt.gp (see TestWindowsEnvCRT).
func checkEnvCRT(t *testing.T, out string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 7 {
		t.Fatalf("%d lines, want 7:\n%s", len(lines), out)
	}
	var prev []string
	for i, line := range lines {
		f := strings.Split(line, "|")
		if len(f) != 4 {
			t.Fatalf("line %q: want 4 fields", line)
		}
		step, entries, cval := f[0], strings.Fields(f[2]), f[3]
		want := "<NULL>"
		if len(entries) > 0 {
			if _, v, _ := strings.Cut(entries[0], "="); v != "" {
				want = v
			}
		}
		if cval != want {
			t.Errorf("%s: C's getenv gives %q, want %q, the block's first value (block %q)", step, cval, want, entries)
		}
		if i > 0 && strings.HasPrefix(step, "unset") && len(prev) > 0 && len(entries) != len(prev)-1 {
			t.Errorf("%s: the block has %q after %q: an unset removes one variable", step, entries, prev)
		}
		if strings.HasPrefix(step, "set ") && (len(entries) == 0 || len(entries) != max(len(prev), 1)) {
			t.Errorf("%s: the block has %q after %q: a set changes the first variable (or adds one)", step, entries, prev)
		}
		prev = entries
	}
}
