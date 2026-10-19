package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// Config is everything a run of the suite needs. The tool names are
// configurable so that the tests can use fake executables.
type Config struct {
	Root     string   // the repository (for building gocpp and finding bench/)
	BenchDir string   // bench/
	Work     string   // the work folder: sources, binaries, the caches of gocpp and Go
	Gocpp    string   // the gocpp binary ("": built from Root/cmd/gocpp into Work)
	GoBin    string   // the go command
	Gxx      string   // g++
	Clang    string   // clang++
	Strip    string   // strip ("": no stripped sizes)
	Taskset  string   // taskset
	Impls    []string // the implementations to build and measure
	Reps     int      // measured runs per implementation
	Warmup   int      // runs before them that are dropped
	Smoke    bool     // the smoke sizes
	Timeout  time.Duration
	Jobs     int      // programs built at the same time
	PinList  string   // the cpu list for taskset ("": not pinned)
	CXXFlags string   // GOCPP_CXXFLAGS of the "O2" flavor
	Flavors  []string // the flavors of gocpp builds that are compared (compile-time runs too)
	Release  bool     // gocpp knows `build -release`
	Retries  int      // how often a failed gocpp build is tried again (concurrent builds in one cache have failed at random)
	Gate     string   // GOCPP_GATE_PHASE of the gocpp builds ("": 15, every feature gate open)
	Log      func(format string, args ...any)
}

func (c *Config) logf(format string, args ...any) {
	if c.Log != nil {
		c.Log(format, args...)
	}
}

// DefaultCXXFlags are the flags of the "O2" flavor: optimized and without
// debug info (the default build is -O0 -g today; Phase 06 step 6.11 adds
// `-release`, -O2 with LTO, which is the "release" flavor). -g does not change
// the speed. The C++ twins get -O2 as well.
const DefaultCXXFlags = "-O2 -g0"

// The compiler of an implementation ("g++" or "clang++" with the path
// configured in the Config), "" for Go.
func (c *Config) cxxOf(impl string) string {
	cxx := impl
	if x, _, ok := benchdata.SplitGocpp(impl); ok {
		cxx = x
	} else if impl == benchdata.ImplCppGxx {
		cxx = "g++"
	} else if impl == benchdata.ImplCppClang {
		cxx = "clang++"
	}
	switch cxx {
	case "g++":
		return c.Gxx
	case "clang++":
		return c.Clang
	}
	return ""
}

// flavorFlags returns the GOCPP_CXXFLAGS and the extra arguments of `gocpp
// build` of a flavor.
func (c *Config) flavorFlags(flavor string) (cxxflags string, args []string) {
	switch flavor {
	case "default":
		return "", nil
	case "release":
		return "", []string{"-release"}
	}
	return c.CXXFlags, nil
}

func (c *Config) size(p *Program) int {
	if c.Smoke {
		return p.Smoke
	}
	return p.Size
}

// buildDir is the folder in which one implementation of a program is built.
func (c *Config) buildDir(program, impl string) string {
	return filepath.Join(c.Work, "build", program, strings.ReplaceAll(impl, ":", "-"))
}

// artifact is a built program: the binary of one implementation.
type artifact struct {
	impl    string
	bin     string
	result  benchdata.Result
	head    string // the first bytes of stdout of the last run
	skipAll bool   // the program needs a package gocpp does not have yet
}

func (a *artifact) ok() bool { return a.result.Status == benchdata.StatusOK }

// gocppEnv is the environment of a gocpp build for the compiler cxx.
func (c *Config) gocppEnv(cxx string, extra ...string) []string {
	gate := c.Gate
	if gate == "" {
		gate = "15"
	}
	env := []string{
		"GOCPP_CXX=" + cxx,
		"XDG_CACHE_HOME=" + filepath.Join(c.Work, "cache"),
		"GOCPP_CXXFLAGS=" + c.CXXFlags,
		"GOCPP_MODCACHE=off",
		"GOCPP_OFFLINE=1",
	}
	if os.Getenv("GOCPP_GATE_PHASE") == "" {
		env = append(env, "GOCPP_GATE_PHASE="+gate)
	}
	return envOver(cleanEnv(os.Environ(), true), append(env, extra...)...)
}

// goEnv is the environment of a Go build: no network, no toolchain download,
// no workspace, no flags from the environment (the Go build cache is the
// user's unless extra says GOCACHE=..., as the compile-time runs do).
func (c *Config) goEnv(extra ...string) []string {
	return envOver(cleanEnv(os.Environ(), true), append([]string{
		"GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GO111MODULE=on", "CGO_ENABLED=0",
	}, extra...)...)
}

// shimFor reads bench/shim/<name>.tmpl and fills in the size.
func (c *Config) shimFor(name string, n int) (string, error) {
	data, err := os.ReadFile(filepath.Join(c.BenchDir, "shim", name+".tmpl"))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(data), "@N@", strconv.Itoa(n)), nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// build builds the implementation impl of p. It never returns an error: a
// failure is the artifact's status (build-failed, or skipped when the program
// needs a package that gocpp does not have yet).
func (c *Config) build(p *Program, impl string) *artifact {
	a := &artifact{impl: impl}
	a.result = benchdata.Result{Program: p.Name, Category: p.Category, Impl: impl}
	fail := func(format string, args ...any) *artifact {
		a.result.Status = benchdata.StatusBuildFailed
		a.result.Note = fmt.Sprintf(format, args...)
		return a
	}
	dir := c.buildDir(p.Name, impl)
	if err := os.RemoveAll(dir); err != nil {
		return fail("%v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail("%v", err)
	}
	n := c.size(p)
	a.bin = filepath.Join(dir, "prog")
	start := time.Now()
	var out string
	var err error
	_, flavor, isGocpp := benchdata.SplitGocpp(impl)
	switch {
	case isGocpp:
		if flavor == "release" && !c.Release {
			a.result.Status = benchdata.StatusSkipped
			a.result.Note = "gocpp has no build -release yet"
			return a
		}
		if err = copyFile(p.Path, filepath.Join(dir, "main.gp")); err != nil {
			return fail("%v", err)
		}
		shim, err2 := c.shimFor("shim.gp", n)
		if err2 != nil {
			return fail("%v", err2)
		}
		if err = os.WriteFile(filepath.Join(dir, "shim.gp"), []byte(shim), 0o644); err != nil {
			return fail("%v", err)
		}
		flags, extra := c.flavorFlags(flavor)
		argv := append(append([]string{c.Gocpp, "build"}, extra...), "-o", a.bin, ".")
		out, err = capture(dir, c.gocppEnv(c.cxxOf(impl), "GOCPP_CXXFLAGS="+flags), 20*time.Minute, argv...)
		if err != nil && len(p.Needs) > 0 && notAvailableLine(out) != "" {
			a.result.Status = benchdata.StatusSkipped
			a.result.Note = "needs " + strings.Join(p.Needs, ", ") + ": " + notAvailableLine(out)
			a.skipAll = true
			return a
		}
	case impl == benchdata.ImplGo:
		if err = copyFile(p.Path, filepath.Join(dir, "main.go")); err != nil {
			return fail("%v", err)
		}
		shim, err2 := c.shimFor("shim.go", n)
		if err2 != nil {
			return fail("%v", err2)
		}
		if err = os.WriteFile(filepath.Join(dir, "shim.go"), []byte(shim), 0o644); err != nil {
			return fail("%v", err)
		}
		if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module bench\n\ngo 1.25\n"), 0o644); err != nil {
			return fail("%v", err)
		}
		out, err = capture(dir, c.goEnv(), 10*time.Minute, c.GoBin, "build", "-o", a.bin, ".")
	case impl == benchdata.ImplCppGxx || impl == benchdata.ImplCppClang:
		if !p.Twin {
			a.result.Status = benchdata.StatusSkipped
			a.result.Note = "no C++ twin"
			return a
		}
		out, err = capture(dir, nil, 10*time.Minute, c.cxxOf(impl), "-O2", "-std=c++20", "-pthread",
			"-DBENCH_N="+strconv.Itoa(n), "-o", a.bin, p.CppPath)
	default:
		return fail("unknown implementation %q", impl)
	}
	a.result.BuildMS = time.Since(start).Milliseconds()
	if err != nil {
		return fail("%v: %s", err, firstLine(lastErrorLine(out)))
	}
	if st, err := os.Stat(a.bin); err != nil {
		return fail("the build wrote no binary")
	} else {
		a.result.BinSize = st.Size()
	}
	a.result.Status = benchdata.StatusOK
	if c.Strip != "" {
		stripped := a.bin + ".stripped"
		if _, err := capture(dir, nil, time.Minute, c.Strip, "-o", stripped, a.bin); err == nil {
			if st, err := os.Stat(stripped); err == nil {
				a.result.Stripped = st.Size()
			}
		}
	}
	return a
}

// lastErrorLine picks the line of a compiler's output that tells the most: the
// first line with "error", else the first line.
func lastErrorLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "error") || strings.Contains(l, "internal error") {
			return l
		}
	}
	return out
}

// buildAll builds every selected implementation of every program, c.Jobs
// programs at a time (the first one alone, so that gocpp builds its runtime
// once). The result maps program name to its artifacts in impl order.
func (c *Config) buildAll(ps []*Program) map[string][]*artifact {
	res := map[string][]*artifact{}
	var mu sync.Mutex
	buildOne := func(p *Program) {
		var arts []*artifact
		skipAll := ""
		// gocpp first: a program that needs a package gocpp lacks is skipped whole.
		for _, impl := range buildOrder(c.Impls) {
			if skipAll != "" {
				arts = append(arts, &artifact{impl: impl, skipAll: true, result: benchdata.Result{
					Program: p.Name, Category: p.Category, Impl: impl, Status: benchdata.StatusSkipped, Note: skipAll}})
				continue
			}
			t := time.Now()
			a := c.build(p, impl)
			for try := 0; try < c.Retries && benchdata.IsGocpp(impl) && a.result.Status == benchdata.StatusBuildFailed; try++ {
				first := a.result.Note
				c.logf("built %-22s %-14s failed (%s): trying again", p.Category+"/"+p.Name, impl, firstLine(first))
				a = c.build(p, impl)
				if a.result.Status == benchdata.StatusOK {
					a.result.Note = "built on try " + strconv.Itoa(try+2) + "; the first build failed: " + firstLine(first)
				}
			}
			if a.skipAll {
				skipAll = a.result.Note
			}
			c.logf("built %-22s %-14s %-12s %5.1fs %s", p.Category+"/"+p.Name, impl, a.result.Status, time.Since(t).Seconds(), a.result.Note)
			arts = append(arts, a)
		}
		if skipAll != "" {
			for _, a := range arts {
				a.skipAll = true
				a.result.Status, a.result.Note = benchdata.StatusSkipped, skipAll
			}
		}
		sortArtifacts(arts)
		mu.Lock()
		res[p.Name] = arts
		mu.Unlock()
	}
	if len(ps) == 0 {
		return res
	}
	buildOne(ps[0])
	jobs := max(1, c.Jobs)
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for _, p := range ps[1:] {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			buildOne(p)
		}()
	}
	wg.Wait()
	return res
}

// buildOrder puts the gocpp implementations first.
func buildOrder(impls []string) []string {
	var out []string
	for _, i := range impls {
		if benchdata.IsGocpp(i) {
			out = append(out, i)
		}
	}
	for _, i := range impls {
		if !benchdata.IsGocpp(i) {
			out = append(out, i)
		}
	}
	return out
}

func sortArtifacts(arts []*artifact) {
	sort.SliceStable(arts, func(i, j int) bool {
		return benchdata.ImplRank(arts[i].impl) < benchdata.ImplRank(arts[j].impl)
	})
}

// notAvailableLine returns the line of out that says something is not built
// into gocpp yet ("is not available yet", "not supported yet"), or "".
func notAvailableLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "is not available yet") || strings.Contains(l, "not supported yet") {
			if _, rest, ok := strings.Cut(l, " "); ok {
				return firstLine(rest)
			}
			return firstLine(l)
		}
	}
	return ""
}
