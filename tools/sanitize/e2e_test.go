package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The end-to-end tests build real programs with real gocpp and the real sanitizers: they prove that
// the job FAILS on an injected UBSan, ASan, LeakSanitizer and ThreadSanitizer error, and that the
// leak audit fails on a leak on top of a cycle and on a stale marker. They take a few minutes
// (ThreadSanitizer builds are big), so they run only with GOCPP_SANITIZE_E2E=1 (make sanitize-selftest).
// GOCPP_SANITIZE_E2E_CXX picks the compilers (default g++; g++,clang++ for both).

var (
	e2eOnce  sync.Once
	e2eDir   string // the folder of the built gocpp and its cache: removed by TestMain when the tests end
	e2eGocpp string
	e2eCache string
	e2eErr   error
)

// e2eSetup builds gocpp once and returns its path and a cache folder shared by the tests.
func e2eSetup(t *testing.T) (gocpp, cache, cxx string) {
	t.Helper()
	if os.Getenv("GOCPP_SANITIZE_E2E") != "1" {
		t.Skip("set GOCPP_SANITIZE_E2E=1 (make sanitize-selftest): builds real programs with the sanitizers")
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	e2eOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sanitize-e2e-")
		if err != nil {
			e2eErr = err
			return
		}
		e2eDir = dir
		e2eGocpp = filepath.Join(dir, "gocpp")
		e2eCache = filepath.Join(dir, "xdg")
		cmd := exec.Command("go", "build", "-o", e2eGocpp, "./cmd/gocpp")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			e2eErr = err
			t.Logf("%s", out)
		}
	})
	if e2eErr != nil {
		t.Fatal(e2eErr)
	}
	cxx = os.Getenv("GOCPP_SANITIZE_E2E_CXX")
	if cxx == "" {
		cxx = "g++"
	}
	return e2eGocpp, e2eCache, cxx
}

// e2eTool runs the tool against root with the real gocpp.
func e2eTool(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	gocpp, cache, cxx := e2eSetup(t)
	t.Setenv("SANITIZE_FAKE", "")
	full := append(args[:1:1], append([]string{"-gocpp", gocpp, "-gocpp-cache", cache, "-work", t.TempDir(), "-out", t.TempDir(), "-cxx", cxx, "-j", "4"}, args[1:]...)...)
	var out, errOut strings.Builder
	code := run(full, &out, &errOut)
	return code, out.String(), errOut.String()
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// stripCanary returns the program text without its "// canary:" line: the same wrong program, as a
// program that the job must fail.
func stripCanary(src string) string {
	var out []string
	for _, l := range strings.Split(src, "\n") {
		if !strings.HasPrefix(l, "// canary:") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

const canaryDir = "tools/sanitize/testdata/canary/"

// TestCanariesAreReported: the canary corpus passes in every mode: each wrong program is reported
// with its kind and the clean controls are silent.
func TestCanariesAreReported(t *testing.T) {
	for _, mode := range ModeNames() {
		t.Run(mode, func(t *testing.T) {
			code, stdout, stderr := e2eTool(t, "run", "-mode", mode, "-corpus", "canary")
			if code != 0 {
				t.Errorf("exit %d\n%s\n%s", code, stdout, stderr)
			}
		})
	}
}

// TestInjectedErrorsFail: the same wrong programs without the canary line are failures of the run,
// each with the report of its sanitizer.
func TestInjectedErrorsFail(t *testing.T) {
	tests := []struct {
		file, mode string
		want       string
	}{
		{"canary_asan", "asan", "asan: heap-buffer-overflow"},
		{"canary_ubsan", "asan", "ubsan: shift exponent 40 is too large"},
		{"canary_ubsan", "ubsan", "ubsan: shift exponent 40 is too large"},
		{"canary_lsan", "asan", "lsan: detected memory leaks"},
		{"canary_tsan", "tsan", "tsan: data race"},
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.mode, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{"go.mod": "module x\n", "cmd/gocpp/main.go": "package main\n"}
			files["tests/run/"+tt.file+".gp"] = stripCanary(readRepoFile(t, canaryDir+tt.file+".gp"))
			if h := strings.TrimPrefix(tt.file, "canary_"); tt.file != "canary_tsan" {
				files["tests/run/canary_"+h+".h"] = readRepoFile(t, canaryDir+"canary_"+h+".h")
			}
			writeTree(t, root, files)
			code, _, stderr := e2eTool(t, "run", "-root", root, "-mode", tt.mode, "-corpus", "run")
			if code != 1 || !strings.Contains(stderr, tt.want) {
				t.Errorf("exit %d, want 1 with %q\n%s", code, tt.want, stderr)
			}
		})
	}
}

// TestLeakAuditCatchesALeakOnTopOfACycle: closures_recursive.gp leaks a cycle (its ceiling is in
// leaks.txt). The same program with a C allocation that is never freed is over the ceiling, and a
// marker on a program that leaks nothing is stale: the audit fails on both and passes on the original.
func TestLeakAuditCatchesALeakOnTopOfACycle(t *testing.T) {
	cycle := readRepoFile(t, "tests/run/closures_recursive.gp")
	var line string
	for _, l := range strings.Split(readRepoFile(t, leaksFile), "\n") {
		if strings.HasPrefix(l, "tests/run/closures_recursive.gp |") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("closures_recursive.gp is not in leaks.txt")
	}
	entry := func(name string) string {
		return strings.Replace(line, "tests/run/closures_recursive.gp", "tests/run/"+name+".gp", 1) + "\n"
	}
	withLeak := strings.Replace(cycle, "package main\n\nimport \"fmt\"", "package main\n\nimport (\n\t\"fmt\"\n\n\t\"./canary_lsan.h\"\n)", 1)
	if withLeak == cycle {
		t.Fatal("the leak could not be injected: closures_recursive.gp changed")
	}
	withLeak = strings.Replace(withLeak, "func main() {\n", "func main() {\n\tcanary_lsan.leak_block()\n", 1)
	stale := "// phase: 03\n// lsan: cycle\n\npackage main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n"
	build := func(programs map[string]string, list string) string {
		root := leakRoot(t, programs, list)
		writeTree(t, root, map[string]string{"tests/run/canary_lsan.h": readRepoFile(t, canaryDir+"canary_lsan.h")})
		return root
	}

	t.Run("the cycle alone passes", func(t *testing.T) {
		root := build(map[string]string{"tests/run/cycle.gp": cycle}, entry("cycle"))
		if code, stdout, stderr := e2eTool(t, "leaks", "-root", root); code != 0 {
			t.Errorf("exit %d\n%s\n%s", code, stdout, stderr)
		}
	})
	t.Run("a leak on top of the cycle fails", func(t *testing.T) {
		root := build(map[string]string{"tests/run/cycle_plus_leak.gp": withLeak}, entry("cycle_plus_leak"))
		code, _, stderr := e2eTool(t, "leaks", "-root", root)
		if code != 1 || !strings.Contains(stderr, "more than the ceiling") {
			t.Errorf("exit %d\n%s", code, stderr)
		}
	})
	t.Run("a stale marker fails", func(t *testing.T) {
		root := build(map[string]string{"tests/run/stale.gp": stale}, entry("stale"))
		code, _, stderr := e2eTool(t, "leaks", "-root", root)
		if code != 1 || !strings.Contains(stderr, "leaks nothing any more") {
			t.Errorf("exit %d\n%s", code, stderr)
		}
	})
}

// injectLeak adds a call of canary_lsan.leak_block (a 77-byte block that is never freed) to the start of
// main of the program source.
func injectLeak(t *testing.T, src string) string {
	t.Helper()
	out := strings.Replace(src, "\nimport (\n", "\nimport (\n\t\"./canary_lsan.h\"\n", 1)
	out = strings.Replace(out, "\nfunc main() {\n", "\nfunc main() {\n\tcanary_lsan.leak_block()\n", 1)
	if out == src || !strings.Contains(out, "canary_lsan.leak_block()") || !strings.Contains(out, "\"./canary_lsan.h\"") {
		t.Fatal("the leak could not be injected")
	}
	return out
}

// TestSmallLeakInTheLooseEntriesIsCaught: the programs whose leak varies from run to run keep their
// own ceiling and slack, and a 77-byte block that is never freed on top of them fails the audit.
func TestSmallLeakInTheLooseEntriesIsCaught(t *testing.T) {
	for _, name := range []string{"sync_cond_queue", "context_gotest_x"} {
		t.Run(name, func(t *testing.T) {
			src := readRepoFile(t, "tests/run/"+name+".gp")
			var line string
			for _, l := range strings.Split(readRepoFile(t, leaksFile), "\n") {
				if strings.HasPrefix(l, "tests/run/"+name+".gp |") {
					line = l
				}
			}
			if line == "" {
				t.Fatal("not in leaks.txt")
			}
			root := leakRoot(t, map[string]string{"tests/run/" + name + ".gp": injectLeak(t, src)}, line+"\n")
			writeTree(t, root, map[string]string{"tests/run/canary_lsan.h": readRepoFile(t, canaryDir+"canary_lsan.h")})
			code, _, stderr := e2eTool(t, "leaks", "-root", root)
			if code != 1 || !strings.Contains(stderr, "more than the ceiling") {
				t.Errorf("exit %d\n%s", code, stderr)
			}
			// and the program itself, as listed, passes
			root = leakRoot(t, map[string]string{"tests/run/" + name + ".gp": src}, line+"\n")
			if code, _, stderr := e2eTool(t, "leaks", "-root", root); code != 0 {
				t.Errorf("the program as it is: exit %d\n%s", code, stderr)
			}
		})
	}
}
