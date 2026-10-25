package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tests of the GC engine's walking skeleton (phase-16 README §2.3 rules 1-2 and 7, §2.7;
// job 16.1c): the hidden -gc-engine switch, the differential test against real Go, the folder
// that mixes .gp and .go files, and the link without gp::gc symbols for a .gp program.

// gcMilestoneSrc is the ticket's milestone program.
const gcMilestoneSrc = `package main

func add(a, b int) int {
	return a + b
}

func main() {
	println("hello", add(40, 2))
}
`

// gcLoopSrc is the second program: a loop, an if, a bool result and a call.
const gcLoopSrc = `package main

func even(n int) bool {
	return n%2 == 0
}

func sum(n int) int {
	s := 0
	for i := 1; i <= n; i++ {
		if even(i) {
			s += i
		}
	}
	return s
}

func main() {
	println("sum of even numbers to", 10, "=", sum(10))
}
`

// gcOrderSrc is the third program: side-effecting call arguments (Go's left-to-right order),
// shift counts of 63/64/65, INT64_MIN / -1, and string comparison and +=.
const gcOrderSrc = `package main

func next(n int) int {
	println("arg", n)
	return n
}

func pair(a, b int) int {
	return a*10 + b
}

func sh(x, n int) int {
	return x << n
}

func shr(x, n int) int {
	return x >> n
}

func d(x, y int) int {
	return x / y
}

func r(x, y int) int {
	return x % y
}

func main() {
	println("order", pair(next(1), next(2)))
	println("sh", sh(1, 64), sh(-1, 64), sh(-1, 63), sh(1, 63))
	println("shr", shr(8, 65), shr(-8, 65), shr(-1, 100))
	println("d", d(-9223372036854775808, -1), d(7, -2), d(-7, 2))
	println("r", r(-9223372036854775808, -1), r(7, -2), r(-7, 2))
	s := "abc"
	s += "d"
	println("str", s, s < "abd", s <= "abcd", s > "ab", s >= "abcd", "a" < "b")
	if x := next(3) + next(4); x > 0 {
		println("if", x)
	}
	x := next(5) + next(6)
	println("scope", x)
	for i := next(1) + next(1); i < 4; i++ {
		println("loop", i)
	}
	if s != "" && len(s) > 0 {
		println("len", len(s))
	}
	if len(s) == 4 || s == "" {
		println("or")
	}
	println("min", -(9223372036854775808), 0-9223372036854775808)
	p := 10
	(p) /= 2
	println("paren", p)
	for i := 8; i > 0; (i) /= 2 {
		println("ploop", i)
	}
}
`

// gcDivZeroSrc and gcNegShiftSrc panic, which is a differential case of its own: the spike
// prints Go's panic line and exits with status 2, and the stack trace comes with step 16.6.
const gcDivZeroSrc = `package main

func d(x, y int) int {
	return x / y
}

func main() {
	println("before")
	println(d(1, 0))
	println("after")
}
`

const gcNegShiftSrc = `package main

func sh(x, n int) int {
	return x << n
}

func main() {
	println(sh(1, -1))
}
`

// gcDir writes the files into a new folder and makes it the current folder for the test. Unlike
// inDir it writes no .gp copy of a .go file: writeFiles does that for the fmt tests, and it
// would make every GC folder a mixed folder (README §2.3 rule 1).
func gcDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	return dir
}

// goRun runs "go run main.go" in dir and returns the program's output and exit code. It skips
// the test when there is no go command: the differential test compares gocpp with real Go.
func goRun(t *testing.T, dir string) (result, bool) {
	t.Helper()
	goCmd, err := exec.LookPath("go")
	if err != nil {
		return result{}, false
	}
	cmd := exec.Command(goCmd, "run", "main.go")
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("go run main.go: %v", err)
		}
		code = exit.ExitCode()
	}
	return result{code, out.String(), errOut.String()}, true
}

// TestGCEngineSwitchHidden checks that the switch is hidden: neither gocpp help nor gocpp
// build -h names -gc-engine, while the commands still accept it (the other tests).
func TestGCEngineSwitchHidden(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"build", "-h"}} {
		r := gocppRun("", false, args...)
		if strings.Contains(r.stdout, "gc-engine") || strings.Contains(r.stderr, "gc-engine") {
			t.Errorf("gocpp %v names the hidden switch:\n%s%s", args, r.stdout, r.stderr)
		}
	}
}

// TestGCRunVsGo is the differential test of job 16.1c: the milestone program and a program with
// a loop and a function call print what real Go prints, with the same exit code.
func TestGCRunVsGo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the GC engine's walking skeleton builds for the host only (job 16.1c)")
	}
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"milestone", gcMilestoneSrc},
		{"loop", gcLoopSrc},
		{"order", gcOrderSrc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, _ testCXX) {
				dir := gcDir(t, map[string]string{"main.go": tc.src})
				want, ok := goRun(t, dir)
				if !ok {
					t.Skip("no go command, so the differential test cannot run")
				}
				got := gocppRun("", false, "run", "-gc-engine", ".")
				if got.code != want.code || got.stdout != want.stdout || got.stderr != want.stderr {
					t.Errorf("gocpp run -gc-engine . = %+v, want the result of go run: %+v", got, want)
				}
			})
		})
	}
}

// runBinary runs a built program in dir and returns its output and exit code.
func runBinary(t *testing.T, bin, dir string) result {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("%s: %v", bin, err)
		}
		code = exit.ExitCode()
	}
	return result{code, out.String(), errOut.String()}
}

// TestGCPanicVsGo is the differential test of the two panic cases of the subset: a zero divisor
// and a negative shift count. The spike prints Go's panic line and exits with status 2; Go's
// stack trace after that line is step 16.6, so the comparison stops at the panic line. The Go
// side is a built binary, not `go run`, whose own exit status is 1.
func TestGCPanicVsGo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the GC engine's walking skeleton builds for the host only (job 16.1c)")
	}
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"divzero", gcDivZeroSrc},
		{"negshift", gcNegShiftSrc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, _ testCXX) {
				goCmd, err := exec.LookPath("go")
				if err != nil {
					t.Skip("no go command, so the differential test cannot run")
				}
				dir := gcDir(t, map[string]string{"main.go": tc.src})
				bin := filepath.Join(t.TempDir(), "go_prog")
				build := exec.Command(goCmd, "build", "-o", bin, "main.go")
				build.Dir = dir
				var bout bytes.Buffer
				build.Stdout, build.Stderr = &bout, &bout
				if err := build.Run(); err != nil {
					t.Fatalf("go build main.go: %v\n%s", err, bout.String())
				}
				want := runBinary(t, bin, dir)
				// Go's stderr is the output before the panic, the panic line, and then the stack
				// trace (or nothing, with GOTRACEBACK=none). The spike prints the same bytes up to
				// the panic line's newline.
				at := strings.Index(want.stderr, "panic: runtime error:")
				if at < 0 {
					t.Fatalf("the Go program did not panic: %q", want.stderr)
				}
				end := strings.IndexByte(want.stderr[at:], '\n')
				if end < 0 {
					t.Fatalf("the Go program's panic line has no newline: %q", want.stderr)
				}
				wantPrefix := want.stderr[:at+end+1]
				got := gocppRun("", false, "run", "-gc-engine", ".")
				if got.code != want.code || got.stdout != want.stdout || got.stderr != wantPrefix {
					t.Errorf("gocpp run -gc-engine . = %+v, want exit %d, stdout %q and stderr %q (the panic line; the stack trace is step 16.6)",
						got, want.code, want.stdout, wantPrefix)
				}
			})
		})
	}
}

// TestGCEngineSwitchAfterProgram checks that the hidden switch is a flag of the command, not an
// argument of the program: gocpp run main.gp -gc-engine runs the ARC program (which takes
// -gc-engine as its own argument), and the switch before the program takes the GC path.
func TestGCEngineSwitchAfterProgram(t *testing.T) {
	forEachCXX(t, func(t *testing.T, _ testCXX) {
		gcDir(t, map[string]string{"main.gp": "package main\n\nfunc main() {\n\tprintln(\"arc\")\n}\n"})
		r := gocppRun("", false, "run", "main.gp", "-gc-engine")
		if r.code != 0 || r.stderr != "arc\n" {
			t.Errorf("gocpp run main.gp -gc-engine = %+v, want the ARC program's output (the argument is the program's)", r)
		}
		r = gocppRun("", false, "run", "-gc-engine", "main.gp")
		if r.code != exitErrors || !strings.Contains(r.stderr, "not a folder") {
			t.Errorf("gocpp run -gc-engine main.gp = %+v, want the GC path's folder error", r)
		}
	})
}

// TestGCEngineSwitchFlagValues checks the switch after flags that take a separate value, which
// the scan must copy with its flag: gocpp build -o out -gc-engine dir, gocpp run -p 4
// -gc-engine dir, and gocpp run -tags foo -gc-engine=false main.gp (the ARC path).
func TestGCEngineSwitchFlagValues(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the GC engine's walking skeleton builds for the host only (job 16.1c)")
	}
	forEachCXX(t, func(t *testing.T, _ testCXX) {
		const gcSrc = "package main\n\nfunc main() { println(\"gc\") }\n"
		t.Run("build -o out -gc-engine", func(t *testing.T) {
			dir := gcDir(t, map[string]string{"main.go": gcSrc})
			out := filepath.Join(dir, "prog")
			r := gocppRun("", false, "build", "-o", out, "-gc-engine", ".")
			if r.code != 0 {
				t.Fatalf("gocpp build -o out -gc-engine . = exit %d: %s", r.code, r.stderr)
			}
			got := runBinary(t, out, dir)
			if got.code != 0 || got.stderr != "gc\n" {
				t.Errorf("the built program = %+v, want exit 0 and stderr %q", got, "gc\n")
			}
		})
		t.Run("run -p 4 -gc-engine", func(t *testing.T) {
			gcDir(t, map[string]string{"main.go": gcSrc})
			r := gocppRun("", false, "run", "-p", "4", "-gc-engine", ".")
			if r.code != 0 || r.stderr != "gc\n" {
				t.Errorf("gocpp run -p 4 -gc-engine . = %+v, want exit 0 and stderr %q", r, "gc\n")
			}
		})
		t.Run("run -tags foo -gc-engine=false", func(t *testing.T) {
			gcDir(t, map[string]string{"main.gp": "package main\n\nfunc main() {\n\tprintln(\"arc\")\n}\n"})
			r := gocppRun("", false, "run", "-tags", "foo", "-gc-engine=false", "main.gp")
			if r.code != 0 || r.stderr != "arc\n" {
				t.Errorf("gocpp run -tags foo -gc-engine=false main.gp = %+v, want the ARC program's output", r)
			}
		})
	})
}

// TestGCEngineMixedFolder checks phase-16 README §2.3 rule 1: a folder with both .gp and .go
// files is an error, and a folder of .gp files is the ARC engine's.
func TestGCEngineMixedFolder(t *testing.T) {
	t.Run("both", func(t *testing.T) {
		gcDir(t, map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
			"main.gp": "package main\n\nfunc main() {}\n",
		})
		r := gocppRun("", false, "run", "-gc-engine", ".")
		if r.code != exitErrors {
			t.Errorf("gocpp run -gc-engine . in a folder with both kinds = exit %d, want %d", r.code, exitErrors)
		}
		if !strings.Contains(r.stderr, "both .gp and .go files") || !strings.Contains(r.stderr, "rule 1") {
			t.Errorf("the error does not name the rule: %q", r.stderr)
		}
	})
	t.Run("gp only", func(t *testing.T) {
		gcDir(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"})
		r := gocppRun("", false, "run", "-gc-engine", ".")
		if r.code != exitErrors {
			t.Errorf("gocpp run -gc-engine . in a folder of .gp files = exit %d, want %d", r.code, exitErrors)
		}
		if !strings.Contains(r.stderr, "no .go files") || !strings.Contains(r.stderr, "rule 1") {
			t.Errorf("the error does not name the rule: %q", r.stderr)
		}
	})
}

// TestGCEngineSubsetErrors checks that a construct outside the walking skeleton's subset is an
// error with its .go position, not silently different C++.
func TestGCEngineSubsetErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{"import", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"x\") }\n", "imports are not supported by the GC engine yet"},
		{"switch", "package main\n\nfunc main() {\n\tswitch 1 {\n\t}\n}\n", "does not support a switch statement"},
		{"float", "package main\n\nfunc main() {\n\tx := 1.5\n\tprintln(x)\n}\n", "supports only int, string and bool"},
		{"results", "package main\n\nfunc f() (int, int) {\n\treturn 1, 2\n}\n\nfunc main() {\n\tprintln(f())\n}\n", "does not support multiple results"},
		{"init", "package main\n\nfunc init() {}\n\nfunc main() {}\n", "does not support func init"},
		{"main signature", "package main\n\nfunc main(x int) {\n\tprintln(x)\n}\n", "main must have no"},
		{"huge div", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 / 2)\n}\n", "cannot represent the integer literal"},
		{"huge rem", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 % 3)\n}\n", "cannot represent the integer literal"},
		{"huge shr", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 >> 1)\n}\n", "cannot represent the integer literal"},
		{"huge sub", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 - 1)\n}\n", "cannot represent the integer literal"},
		{"huge 2^64", "package main\n\nfunc main() {\n\tprintln(18446744073709551616 - 18446744073709551616)\n}\n", "cannot represent the integer literal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gcDir(t, map[string]string{"main.go": tc.src})
			r := gocppRun("", false, "run", "-gc-engine", ".")
			if r.code != exitErrors {
				t.Errorf("gocpp run -gc-engine . = exit %d, want %d (stderr %q)", r.code, exitErrors, r.stderr)
			}
			if !strings.Contains(r.stderr, tc.want) {
				t.Errorf("stderr = %q, want it to contain %q", r.stderr, tc.want)
			}
			if !strings.Contains(r.stderr, "main.go:") {
				t.Errorf("stderr = %q, want the .go position", r.stderr)
			}
		})
	}
}

// TestGCEngineNoGCSymbols checks phase-16 README §2.3 rule 7: a .gp program links no gp::gc
// symbol and no bdwgc (job 16.1d links the vendored collector into the GC engine's programs
// only). The GC engine's own program does link both, which makes the check meaningful.
func TestGCEngineNoGCSymbols(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads an ELF program's symbols with nm")
	}
	nm, err := exec.LookPath("nm")
	if err != nil {
		t.Skip("no nm command")
	}
	forEachCXX(t, func(t *testing.T, _ testCXX) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		gpProg := filepath.Join(dir, "gp_prog")
		if r := gocppRun("", false, "build", "-o", gpProg, "main.gp"); r.code != 0 {
			t.Fatalf("gocpp build main.gp = exit %d: %s", r.code, r.stderr)
		}
		if syms := nmSymbols(t, nm, gpProg); bytes.Contains(syms, []byte("_ZN2gp2gc")) {
			t.Errorf("a .gp program has a gp::gc symbol:\n%s", gcSymbols(syms))
		} else if bdwgcSyms := bdwgcSymbols(syms); len(bdwgcSyms) != 0 {
			t.Errorf("a .gp program has a bdwgc symbol:\n%s", bdwgcSyms)
		}

		goDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(goDir, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gcProg := filepath.Join(goDir, "gc_prog")
		if r := gocppRun("", false, "build", "-gc-engine", "-o", gcProg, goDir); r.code != 0 {
			t.Fatalf("gocpp build -gc-engine = exit %d: %s", r.code, r.stderr)
		}
		if syms := nmSymbols(t, nm, gcProg); !bytes.Contains(syms, []byte("_ZN2gp2gc")) {
			t.Errorf("the GC engine's program links no gp::gc symbol, so the .gp check proves nothing")
		} else if len(bdwgcSymbols(syms)) == 0 {
			t.Errorf("the GC engine's program links no bdwgc symbol, so the .gp check proves nothing")
		}
	})
}

// nmSymbols returns the output of nm for the program prog.
func nmSymbols(t *testing.T, nm, prog string) []byte {
	t.Helper()
	out, err := exec.Command(nm, prog).Output()
	if err != nil {
		t.Fatalf("nm %s: %v", prog, err)
	}
	return out
}

// bdwgcSymbols returns the lines of nm's output that name a bdwgc symbol: the collector's C
// names (the archive defines GC_malloc, GC_gcollect, GC_register_my_thread, ...).
func bdwgcSymbols(out []byte) string {
	var b strings.Builder
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "GC_malloc") || strings.Contains(line, "GC_gcollect") ||
			strings.Contains(line, "GC_register_my_thread") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// gcSymbols returns the lines of nm's output that name a gp::gc symbol.
func gcSymbols(out []byte) string {
	var b strings.Builder
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "_ZN2gp2gc") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
