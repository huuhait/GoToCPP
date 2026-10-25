package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
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

// needMingw skips a test of the GC engine's windows/amd64 build when this machine has no
// MinGW-w64 cross compiler, and the reason names the missing program (a skip, never a pass).
func needMingw(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return // the compiler of the host is the Windows one
	}
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
		t.Skip("no MinGW-w64 cross compiler (x86_64-w64-mingw32-g++): the GC engine's windows/amd64 build needs it")
	}
}

// gcWindowsBuild builds the folder dir's GC program for windows/amd64 with gocpp build -gc-engine
// and returns the path of the .exe, failing the test with the driver's output when it does not
// build.
func gcWindowsBuild(t *testing.T, dir string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "prog.exe")
	r := gocppRun("", false, "build", "-gc-engine", "-os", "windows", "-arch", "amd64", "-o", exe, dir)
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("gocpp build -gc-engine -os windows -arch amd64 = exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	return exe
}

// peNM returns a program that reads the symbols of a PE file, and whether one is installed: the
// cross toolchain's nm first, then llvm-nm, then the system's nm (binutils with the PE targets).
func peNM() (string, bool) {
	for _, name := range []string{"x86_64-w64-mingw32-nm", "llvm-nm", "nm"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, true
		}
	}
	return "", false
}

// TestGCWindowsCrossBuild is the cross build of job 16.1d-win, without the PC: gocpp build
// -gc-engine -os windows -arch amd64 compiles the generated C++ and the GC engine's sources (with
// GOCPP_GC_BDWGC and bdwgc's header flags), builds the vendored bdwgc for windows/amd64 through
// internal/build.Builder.Bdwgc, and links an .exe with the Windows link flags (-static, the 1 GiB
// stack, -pthread). The .exe is a PE file that contains the GC engine's and bdwgc's symbols; the
// running of it is TestGCRunVsGoWindows (the real PC) and runtime.TestGCHeapWindows (Wine and the
// PC). Skipped, with that reason, when the machine has no MinGW-w64 cross compiler.
func TestGCWindowsCrossBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the cross build is from Linux to windows/amd64; a Windows host builds the GC engine natively")
	}
	needMingw(t)
	// GOCPP_TOOLCHAIN=local never downloads a toolchain: the dev shell's MinGW-w64 is the
	// compiler of this test (and the only one the build may use).
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	dir := gcDir(t, map[string]string{"main.go": gcMilestoneSrc})
	exe := gcWindowsBuild(t, ".")
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	// "MZ": the linker wrote a PE file, not an ELF one. (Its PE magic at 0x3c is checked too.)
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		t.Fatalf("%s is not a PE file (%d bytes)", exe, len(data))
	}
	if off := int(uint32(data[0x3c]) | uint32(data[0x3d])<<8 | uint32(data[0x3e])<<16 | uint32(data[0x3f])<<24); off < 0 || off+4 > len(data) || string(data[off:off+4]) != "PE\x00\x00" {
		t.Fatalf("%s has no PE header at 0x%x", exe, off)
	}
	// The GC engine's runtime and the collector are linked in: the program is not a stub that
	// the cross compiler accepted. A PE symbol reader is the cross toolchain's nm, llvm-nm, or a
	// binutils nm built with the PE targets; when none is there the check is skipped with a log
	// line, not silently (only the PE header was then checked).
	if nm, ok := peNM(); ok {
		syms := nmSymbols(t, nm, exe)
		for _, want := range []string{"_ZN2gp2gc", "GC_malloc", "GC_gcollect", "GC_register_my_thread"} {
			if !bytes.Contains(syms, []byte(want)) {
				t.Errorf("the windows/amd64 program has no %s symbol (the GC engine or the collector is not linked in)", want)
			}
		}
	} else {
		t.Logf("no x86_64-w64-mingw32-nm, llvm-nm or nm in PATH: the PE file's symbols are not checked (the PE header was)")
	}

	// The program's name is an .exe also when -o names a folder: outputPath joins the folder and
	// the program's name, and the GC path adds the target's suffix (gcmode.go).
	t.Run("output naming", func(t *testing.T) {
		out := t.TempDir() // a folder: the program keeps its name and gets the target's .exe
		r := gocppRun("", false, "build", "-gc-engine", "-os", "windows", "-arch", "amd64", "-o", out, dir)
		if r.code != 0 || r.stderr != "" {
			t.Fatalf("gocpp build -o <folder> = exit %d\n%s", r.code, r.stderr)
		}
		if want := filepath.Join(out, filepath.Base(dir)+".exe"); !isFile(want) {
			t.Errorf("gocpp build -o <folder> wrote no %s", want)
		}
	})

	// gocpp run of a cross build goes through GOCPP_EXEC, like the ARC engine's (a runner is
	// needed: the .exe does not run on this system). Without it, the driver says so and builds
	// nothing; with it, the runner starts the .exe.
	t.Run("run needs a runner", func(t *testing.T) {
		t.Setenv("GOCPP_EXEC", "")
		r := gocppRun("", false, "run", "-gc-engine", "-os", "windows", "-arch", "amd64", dir)
		if r.code != exitErrors || !strings.Contains(r.stderr, "GOCPP_EXEC") {
			t.Errorf("gocpp run -gc-engine -os windows without GOCPP_EXEC = exit %d: %q, want the GOCPP_EXEC hint", r.code, r.stderr)
		}
	})
	t.Run("run under a runner", func(t *testing.T) {
		t.Setenv("GOCPP_EXEC", "true") // /bin/true: the runner, not the program, does the work
		r := gocppRun("", false, "run", "-gc-engine", "-os", "windows", "-arch", "amd64", dir)
		if r.code != 0 || r.stdout != "" || r.stderr != "" {
			t.Errorf("gocpp run -gc-engine -os windows with GOCPP_EXEC=true = exit %d, stdout %q, stderr %q, want 0 and nothing", r.code, r.stdout, r.stderr)
		}
	})
}

// TestGCTargetErrors checks gocpp run -gc-engine's order: the target comes before GOCPP_EXEC, as
// in the ARC engine's run, so a pair that the GC engine cannot build gets that error and not the
// hint to set a runner (review of job 16.1d-win). No compiler is needed: both cases fail before
// any build.
func TestGCTargetErrors(t *testing.T) {
	gcDir(t, map[string]string{"main.go": gcMilestoneSrc})
	t.Setenv("GOCPP_EXEC", "")
	for _, tc := range []struct {
		goos, goarch, want string
	}{
		// Not a target that gocpp builds at all (build.Targets).
		{"windows", "arm64", "building for windows/arm64 is not supported"},
		// A target that gocpp builds, but not with the GC engine.
		{"linux", "arm64", "cross-compiles for windows/amd64 only"},
	} {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			if isHostTarget(tc.goos, tc.goarch) {
				t.Skipf("%s/%s is this machine", tc.goos, tc.goarch)
			}
			r := gocppRun("", false, "run", "-gc-engine", "-os", tc.goos, "-arch", tc.goarch, ".")
			if r.code != exitErrors || strings.Contains(r.stderr, "GOCPP_EXEC") || !strings.Contains(r.stderr, tc.want) {
				t.Errorf("gocpp run -gc-engine -os %s -arch %s = exit %d: %q, want exit %d and %q, without the GOCPP_EXEC hint",
					tc.goos, tc.goarch, r.code, r.stderr, exitErrors, tc.want)
			}
		})
	}
}

// TestGCRunVsGoWindows is the differential test of job 16.1d-win on the real Windows PC: the GC
// engine's programs are built for windows/amd64 (as TestGCWindowsCrossBuild does), uploaded to the
// machine of GOCPP_WINDOWS_SSH=user@host, run there, and must print exactly what go run prints on
// this machine, with the same exit code. Skipped without the variable, with that reason; with it,
// an unreachable PC is a failure (never a silent pass).
func TestGCRunVsGoWindows(t *testing.T) {
	host := os.Getenv("GOCPP_WINDOWS_SSH")
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host runs the GC engine's windows/amd64 programs on the real PC; the cross build alone is TestGCWindowsCrossBuild")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the cross build is from Linux to windows/amd64")
	}
	needMingw(t)
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	remote, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is unreachable: %v", err)
	}
	defer remote.Close()
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"milestone", gcMilestoneSrc},
		{"loop", gcLoopSrc},
		{"order", gcOrderSrc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := gcDir(t, map[string]string{"main.go": tc.src})
			want, ok := goRun(t, dir)
			if !ok {
				t.Skip("no go command, so the differential test cannot run")
			}
			res := gcRunOnPC(t, remote, gcWindowsBuild(t, "."))
			if res.Code != want.code || string(res.Stdout) != want.stdout || string(res.Stderr) != want.stderr {
				t.Errorf("on the PC: exit %d, stdout %q, stderr %q\nwant go run's: exit %d, stdout %q, stderr %q",
					res.Code, res.Stdout, res.Stderr, want.code, want.stdout, want.stderr)
			}
		})
	}
	// The panic cases, as TestGCPanicVsGo compares them on this machine: the spike prints Go's
	// panic line and exits with 2; the stack trace after the line is step 16.6, so the comparison
	// stops at the panic line's newline. The Windows run must not add a "\r" (the panic line goes
	// through the GC engine's write_stderr, which sets the binary mode), and it is the case a
	// program that panics before its first println hits.
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"divzero", gcDivZeroSrc},
		{"negshift", gcNegShiftSrc},
	} {
		t.Run(tc.name+" panic", func(t *testing.T) {
			dir := gcDir(t, map[string]string{"main.go": tc.src})
			want, prefix := goPanicWant(t, dir)
			res := gcRunOnPC(t, remote, gcWindowsBuild(t, "."))
			if res.Code != want.code || string(res.Stdout) != want.stdout || string(res.Stderr) != prefix {
				t.Errorf("on the PC: exit %d, stdout %q, stderr %q\nwant exit %d, stdout %q and stderr %q (the panic line; the stack trace is step 16.6)",
					res.Code, res.Stdout, res.Stderr, want.code, want.stdout, prefix)
			}
		})
	}
}

// gcRunOnPC runs exe on the Windows machine and returns what it printed.
func gcRunOnPC(t *testing.T, remote *winssh.Remote, exe string) winssh.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	res, err := remote.Run(ctx, 2*time.Minute, exe, nil, nil, false)
	if err != nil {
		t.Fatalf("running on the PC: %v", err)
	}
	return res
}

// goPanicWant builds and runs the Go program of dir for this machine and returns its result and
// the part of its stderr up to and including the newline of Go's panic line (Go's stack trace
// after that line is step 16.6).
func goPanicWant(t *testing.T, dir string) (result, string) {
	t.Helper()
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command, so the differential test cannot run")
	}
	bin := filepath.Join(t.TempDir(), "go_prog")
	build := exec.Command(goCmd, "build", "-o", bin, "main.go")
	build.Dir = dir
	var bout bytes.Buffer
	build.Stdout, build.Stderr = &bout, &bout
	if err := build.Run(); err != nil {
		t.Fatalf("go build main.go: %v\n%s", err, bout.String())
	}
	want := runBinary(t, bin, dir)
	at := strings.Index(want.stderr, "panic: runtime error:")
	if at < 0 {
		t.Fatalf("the Go program did not panic: %q", want.stderr)
	}
	end := strings.IndexByte(want.stderr[at:], '\n')
	if end < 0 {
		t.Fatalf("the Go program's panic line has no newline: %q", want.stderr)
	}
	return want, want.stderr[:at+end+1]
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
				dir := gcDir(t, map[string]string{"main.go": tc.src})
				// Go's stderr is the output before the panic, the panic line, and then the stack
				// trace (or nothing, with GOTRACEBACK=none). The spike prints the same bytes up to
				// the panic line's newline (goPanicWant is the oracle of the Windows test too).
				want, wantPrefix := goPanicWant(t, dir)
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
