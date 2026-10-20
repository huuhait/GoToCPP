package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The speed of typed interop on the example packages (Phase 12 step 12.12, exit criteria "warm
// builds add < 50 ms; cold probing < 0.5 s per example package"). The benchmarks run
// `gocpp check .` as a process of its own (this test binary with GOCPP_TEST_MAIN=1, as
// graph_bench_test.go does, which has no process group either: the child is a check that ends in
// seconds and cmd.WaitDelay bounds the wait), so the time includes starting it, and report the
// median of the runs in milliseconds:
//
//	go test -run '^$' -bench TypedCheck -benchtime 10x ./cmd/gocpp
//
// For each example that imports a header there are four numbers:
//
//	opaque-cold   no cache of any kind (a new cache folder for each run)
//	typed-cold    the same, in typed mode: typed-cold - opaque-cold is the cost of probing
//	opaque-warm   the caches of the first run
//	typed-warm    the same, in typed mode: typed-warm - opaque-warm is the warm overhead
//
// The machine is shared and noisy: the median of 10 is the number to look at, not one run. No test
// checks a time; TestTypedWarmRunsNoCompiler counts the compiler runs instead.

var headerImportLine = regexp.MustCompile(`(?m)^\s*"(<[^">]+>|\./[^"]*\.(h|hpp|hh|hxx))"`)

// headerExamples returns the folders of the examples that import a C/C++ header (in any of their
// files) and build for this system.
func headerExamples(tb testing.TB) []string {
	tb.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		tb.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		tb.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		main, err := os.ReadFile(filepath.Join(dir, "main.gp"))
		if err != nil || regexp.MustCompile(`(?m)^//go:build .*windows`).Match(main) {
			continue
		}
		found := false
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".gp") && !found {
				b, _ := os.ReadFile(path)
				found = headerImportLine.Match(b)
			}
			return nil
		})
		if found {
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) == 0 {
		tb.Fatal("no example imports a header")
	}
	return dirs
}

// checkProcess runs `gocpp check -interop=mode .` in dir with the cache folders cache, and returns
// its time and exit code.
func checkProcess(b *testing.B, dir, mode, cache string) time.Duration {
	b.Helper()
	cmd := exec.Command(os.Args[0], "check", "-interop="+mode, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOCPP_TEST_MAIN=1",
		"GOCPP_MODCACHE=off",
		"GOCPP_OFFLINE=1",
		"GOCPP_NOPROBECACHE=",
		// The tests turn the compiler-detection cache off (build_test.go TestMain); a user has it.
		"GOCPP_NOOBJCACHE=",
		// A test binary has no probe cache unless it is asked for one (the cache folders check
		// testing.Testing()); gocpp itself would use the same folder under XDG_CACHE_HOME.
		"GOCPP_PROBE_CACHE="+filepath.Join(cache, "probe"),
		"XDG_CACHE_HOME="+cache,
	)
	cmd.WaitDelay = time.Minute
	start := time.Now()
	out, err := cmd.CombinedOutput()
	d := time.Since(start)
	if err != nil {
		b.Fatalf("gocpp check -interop=%s in %s: %v\n%s", mode, dir, err, out)
	}
	return d
}

func BenchmarkTypedCheck(b *testing.B) {
	anyCXXb(b)
	if _, err := exec.LookPath(matrixClangProgram()); err != nil {
		b.Skip("no clang++ for the probes")
	}
	for _, dir := range headerExamples(b) {
		b.Run(filepath.Base(dir), func(b *testing.B) {
			for _, mode := range []string{"opaque", "typed"} {
				b.Run(mode+"-cold", func(b *testing.B) {
					var ds []time.Duration
					for range b.N {
						ds = append(ds, checkProcess(b, dir, mode, b.TempDir()))
					}
					b.ReportMetric(float64(median(ds))/float64(time.Millisecond), "median-ms")
				})
				b.Run(mode+"-warm", func(b *testing.B) {
					cache := b.TempDir()
					checkProcess(b, dir, mode, cache) // fills the caches
					b.ResetTimer()
					var ds []time.Duration
					for range b.N {
						ds = append(ds, checkProcess(b, dir, mode, cache))
					}
					b.ReportMetric(float64(median(ds))/float64(time.Millisecond), "median-ms")
				})
			}
		})
	}
}

// matrixClangProgram is the clang++ the typed tests probe with: GOCPP_CLANG, else clang++ in PATH.
func matrixClangProgram() string {
	if matrixClang != "" {
		return matrixClang
	}
	return "clang++"
}

// countingClang makes a script that logs the arguments of each call and runs the real clang++, sets
// GOCPP_CLANG to it, and returns a function that reports the calls that compile or preprocess a
// source (not the questions about the compiler itself: --version, -dM).
func countingClang(t *testing.T) func() int {
	t.Helper()
	real, err := exec.LookPath(matrixClangProgram())
	if err != nil {
		t.Skipf("no clang++: %v", err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	wrapper := filepath.Join(dir, "clang-counting")
	script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CLANG", wrapper)
	return func() int {
		b, err := os.ReadFile(log)
		if err != nil {
			return 0
		}
		n := 0
		for _, line := range strings.Split(string(b), "\n") {
			compiles := strings.Contains(line, "-fsyntax-only") ||
				strings.Contains(line, "-ast-dump") ||
				strings.Contains(line, " -E ")
			if compiles {
				n++
			}
		}
		return n
	}
}

// A warm typed check asks the C++ compiler nothing for the probes: every answer comes from the cache
// on disk, and its output is the cold run's. Two counts say so: the probe cache's own counter of
// compiler runs, and the calls that reach the clang++ program (a wrapper that logs them), which also
// see the runs that the cache does not count. These are counts, not times.
func TestTypedWarmRunsNoCompiler(t *testing.T) {
	for _, dir := range headerExamples(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			typedEnv(t)
			t.Setenv(probe.EnvProbeCache, t.TempDir())
			t.Setenv(probe.EnvNoProbeCache, "")
			t.Setenv("GOCPP_INTEROP", "typed")
			t.Setenv("GOCPP_CXX", "clang++")
			t.Chdir(dir)
			needClang(t)
			defer probe.SetRacyWindow(0)()
			if r := gocppRun("", false, "check", "-interop=opaque", "."); r.code != 0 {
				t.Skipf("the example does not check here (a missing library?): %s", r.stderr)
			}
			calls := countingClang(t)

			probe.ResetCacheCounters()
			cold := gocppRun("", true, "check", ".")
			if cold.code != 0 {
				t.Fatalf("cold typed check: exit %d\n%s", cold.code, cold.stdout)
			}
			coldCalls := calls()
			if probe.CacheCounters().Runs == 0 || coldCalls == 0 {
				t.Fatalf("the cold typed check made %d probe runs and %d clang++ calls: "+
					"the example is no test of the cache",
					probe.CacheCounters().Runs, coldCalls)
			}

			probe.ResetCacheCounters()
			warm := gocppRun("", true, "check", ".")
			counters := probe.CacheCounters()
			if warm.code != 0 || warm.stdout != cold.stdout {
				t.Errorf("warm typed check: exit %d, output %q, cold output %q",
					warm.code, warm.stdout, cold.stdout)
			}
			if counters.Runs != 0 || counters.DiskHits == 0 {
				t.Errorf("warm typed check: %d probe runs and %d answers from the disk; "+
					"want 0 runs and some answers",
					counters.Runs, counters.DiskHits)
			}
			if n := calls() - coldCalls; n != 0 {
				t.Errorf("warm typed check: %d calls reached clang++ (cold: %d); want 0",
					n, coldCalls)
			}
		})
	}
}

// The clang++ that tools/clang-matrix.sh asks for (GOCPP_CLANG) is the one the typed tests of this
// package probe with: typedEnv keeps it (matrixClang), and the compiler the probes find has the
// major version that the script expects in GOCPP_EXPECT_CLANG_MAJOR. Without the variable the test
// only logs.
func TestMatrixClangReachesTheDriver(t *testing.T) {
	typedEnv(t)
	cxx, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skipf("no clang++: %v", err)
	}
	t.Logf("the typed tests probe with %s %d (%s)", cxx.Name(), cxx.Major, cxx.Version)
	want := os.Getenv("GOCPP_EXPECT_CLANG_MAJOR")
	if want != "" && want != strconv.Itoa(cxx.Major) {
		t.Errorf("the matrix asked for clang %s, the typed tests use clang %d (%s)",
			want, cxx.Major, cxx.Version)
	}
}
