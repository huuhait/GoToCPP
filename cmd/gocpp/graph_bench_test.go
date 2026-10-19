package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The benchmarks of the incremental build (Phase 06 steps 6.5 to 6.7, exit criterion: a build with
// nothing to do takes less than 200 ms for a program of four packages). They run gocpp build as a
// process of its own (the test binary with GOCPP_TEST_MAIN=1), so the time includes starting it, and
// report the median of the runs in ms next to Go's mean:
//
//	go test -run '^$' -bench 'Noop|BodyChange' -benchtime 10x ./cmd/gocpp
//
// The machine is shared and noisy: the median of 10 is the number to look at, not one run.

// chainProject returns a module of n library packages p0 ... p(n-1), each importing the one before,
// and a main package that uses the last. (With n = 3 that is the program of four packages.) Every
// package has a constant, a type with a method, and two functions.
func chainProject(n int, stamp string) map[string]string {
	files := map[string]string{"gocpp.mod": "module example.com/m\n"}
	for i := range n {
		imp, call := "", fmt.Sprint(i)
		if i > 0 {
			imp = fmt.Sprintf("import \"example.com/m/p%d\"\n\n", i-1)
			call = fmt.Sprintf("p%d.F() + %d", i-1, i)
		}
		files[fmt.Sprintf("p%d/p%d.gp", i, i)] = fmt.Sprintf(`package p%d

%sconst K = %d

type T struct{ A, B int }

func (t T) Sum() int { return t.A + t.B }

func F() int { return %s }

func G(s string) string { return s + "x" }
`, i, imp, i, call)
	}
	last := n - 1
	files["main.gp"] = fmt.Sprintf(`package main

import (
	"fmt"

	"example.com/m/p%d"
)

var stamp = %q

func main() { fmt.Println(p%d.F(), p%d.T{A: 1, B: 2}.Sum(), p%d.G("a"), stamp) }
`, last, stamp, last, last, last)
	return files
}

// gocppProcess runs the test binary as gocpp build -o prog . in dir, with the extra environment env,
// and returns how long it took.
func gocppProcess(b *testing.B, dir string, env ...string) time.Duration {
	b.Helper()
	args := []string{"build", "-o", "prog", "."}
	if os.Getenv("GOCPP_BENCH_X") != "" { // to see why a build was not up to date
		args = slices.Insert(args, 1, "-x")
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), append([]string{"GOCPP_TEST_MAIN=1", "GOCPP_GATE_PHASE=06", "GOCPP_NOOBJCACHE=0"}, env...)...)
	cmd.WaitDelay = time.Minute
	start := time.Now()
	out, err := cmd.CombinedOutput()
	d := time.Since(start)
	if err != nil {
		b.Fatalf("gocpp build: %v\n%s", err, out)
	}
	if os.Getenv("GOCPP_BENCH_X") != "" {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(l, "# not up to date") || strings.HasPrefix(l, "# up to date") || strings.HasPrefix(l, "# not stamped") || strings.HasPrefix(l, "# actions") {
				b.Log(l)
			}
		}
	}
	return d
}

func median(ds []time.Duration) time.Duration {
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[len(s)/2]
}

func benchProject(b *testing.B, n int) string {
	b.Helper()
	anyCXXb(b)
	dir := b.TempDir()
	stamp := fmt.Sprintf("bench-%d-%d", n, time.Now().UnixNano())
	for name, text := range chainProject(n, stamp) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil {
			os.Chtimes(p, old, old)
		}
		return nil
	})
	return dir
}

// anyCXXb sets GOCPP_CXX for a benchmark: the first compiler that works.
func anyCXXb(b *testing.B) {
	b.Helper()
	for _, name := range []string{"g++", "clang++"} {
		if _, err := exec.LookPath(name); err == nil {
			b.Setenv("GOCPP_CXX", name)
			b.Setenv("CXX", "")
			return
		}
	}
	b.Skip("no C++ compiler")
}

// BenchmarkNoopRebuild: gocpp build of a program that was built and has not changed (the up-to-date
// check), for programs of 4, 20 and 100 packages. The 200 ms of the exit criterion are for four.
func BenchmarkNoopRebuild(b *testing.B) {
	for _, n := range []int{3, 19, 99} {
		b.Run(fmt.Sprintf("pkgs=%d", n+1), func(b *testing.B) {
			dir := benchProject(b, n)
			gocppProcess(b, dir) // the first build, which fills the cache
			b.ResetTimer()
			var ds []time.Duration
			for range b.N {
				ds = append(ds, gocppProcess(b, dir))
			}
			b.ReportMetric(float64(median(ds))/float64(time.Millisecond), "median-ms")
		})
	}
}

// BenchmarkNoopRebuildNoStamp: the same without the up-to-date check (GOCPP_NOUPTODATE=1): load, check,
// emit, the object cache's answer for every unit, and the link. This is what the build did before.
func BenchmarkNoopRebuildNoStamp(b *testing.B) {
	for _, n := range []int{3, 19, 99} {
		b.Run(fmt.Sprintf("pkgs=%d", n+1), func(b *testing.B) {
			dir := benchProject(b, n)
			gocppProcess(b, dir, "GOCPP_NOUPTODATE=1")
			b.ResetTimer()
			var ds []time.Duration
			for range b.N {
				ds = append(ds, gocppProcess(b, dir, "GOCPP_NOUPTODATE=1"))
			}
			b.ReportMetric(float64(median(ds))/float64(time.Millisecond), "median-ms")
		})
	}
}

// BenchmarkBodyChange: a function body of the first package changes before each build: the front end
// runs, one generated file is written, one unit compiles, the program is linked.
func BenchmarkBodyChange(b *testing.B) {
	for _, n := range []int{3, 19, 99} {
		b.Run(fmt.Sprintf("pkgs=%d", n+1), func(b *testing.B) {
			dir := benchProject(b, n)
			gocppProcess(b, dir)
			file := filepath.Join(dir, "p0", "p0.gp")
			orig, err := os.ReadFile(file)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			var ds []time.Duration
			for i := range b.N {
				b.StopTimer()
				text := strings.Replace(string(orig), `return s + "x"`, fmt.Sprintf(`return s + "x%d"`, i), 1)
				if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
					b.Fatal(err)
				}
				old := time.Now().Add(-time.Hour)
				os.Chtimes(file, old, old)
				b.StartTimer()
				ds = append(ds, gocppProcess(b, dir))
			}
			b.ReportMetric(float64(median(ds))/float64(time.Millisecond), "median-ms")
		})
	}
}
