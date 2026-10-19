// Command gobench runs the benchmark suite of gocpp (bench/): every program
// built by gocpp with g++ and with clang++, by real Go, and (the compute
// kernels) as a hand-written C++20 twin, measured several times, with the
// outputs compared, and writes JSON, a markdown report and a history line.
//
//	gobench [run] [flags]     measure the suite
//	gobench check [flags]     measure and compare with bench/baseline.json (the regression gate)
//	gobench validate          check the headers of the programs and their twins (no builds)
//	gobench list              list the programs
//	gobench report -in FILE   write the markdown report of a JSON file
//	gobench merge -base FILE -add FILE   replace the programs of FILE (a run of part of the suite) in the base report
//
// See docs/benchmarks/README.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

func main() {
	args := os.Args[1:]
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var code int
	switch cmd {
	case "run":
		code = cmdRun(args, false)
	case "check":
		code = cmdRun(args, true)
	case "validate":
		code = cmdValidate(args)
	case "list":
		code = cmdList(args)
	case "report":
		code = cmdReport(args)
	case "merge":
		code = cmdMerge(args)
	default:
		fmt.Fprintf(os.Stderr, "gobench: unknown command %q (run, check, validate, list, report, merge)\n", cmd)
		code = 2
	}
	os.Exit(code)
}

// findRoot returns the repository root: the folder with go.mod above dir.
func findRoot(dir string) (string, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "bench")); err == nil {
				return d, nil
			}
		}
		if p := filepath.Dir(d); p != d {
			d = p
			continue
		}
		return "", fmt.Errorf("no folder with go.mod and bench/ above %s (use -root)", dir)
	}
}

// parseFlavors parses the -flavors flag: a comma separated list of the flavors of benchdata.Flavors.
func parseFlavors(s string) ([]string, error) {
	var out []string
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if !slices.Contains(benchdata.Flavors, x) {
			return nil, fmt.Errorf("unknown flavor %q (have %v)", x, benchdata.Flavors)
		}
		if !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out, nil
}

// parseImpls parses the -impls flag: "all" (Go, the gocpp builds of the
// flavors, the twins) or a comma separated list of implementation names.
func parseImpls(s string, flavors []string) ([]string, error) {
	all := benchdata.ImplsFor(flavors)
	if s == "" || s == "all" {
		return all, nil
	}
	var out []string
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if !slices.Contains(all, x) {
			return nil, fmt.Errorf("unknown implementation %q (have %v)", x, all)
		}
		out = append(out, x)
	}
	return out, nil
}

type runFlags struct {
	root, work, gocpp, match, impls, flavors, out, md, history, pin, cxxflags, baseline string
	smoke, compile, allowFail, keep, requireQuiet, noDrift, trustBaseline               bool
	reps, warmup, jobs, compileReps, retries, buildRetries                              int
	maxLoad, threshold                                                                  float64
	timeout                                                                             time.Duration
}

func newFlags(name string, check bool) (*flag.FlagSet, *runFlags) {
	fs := flag.NewFlagSet("gobench "+name, flag.ContinueOnError)
	f := &runFlags{}
	fs.StringVar(&f.root, "root", ".", "the repository (a folder with go.mod and bench/ at or above it)")
	fs.StringVar(&f.work, "work", "", "work folder for sources, binaries and caches (default: a new temporary folder, removed at the end)")
	fs.BoolVar(&f.keep, "keep", false, "keep the work folder")
	fs.StringVar(&f.gocpp, "gocpp", "", "the gocpp binary (default: built from the repository)")
	fs.StringVar(&f.match, "match", "", "only the programs whose name, category or category/name match this regular expression")
	fs.StringVar(&f.flavors, "flavors", "", "flavors of the gocpp builds, comma separated: "+strings.Join(benchdata.Flavors, ", ")+" (default: O2 for check, default,O2 for -smoke, all three otherwise; release is skipped while gocpp has no build -release)")
	fs.StringVar(&f.impls, "impls", "all", "implementations, comma separated: go, gocpp-g++:FLAVOR, gocpp-clang++:FLAVOR, cpp-g++, cpp-clang++ (all: every one of the flavors)")
	fs.BoolVar(&f.smoke, "smoke", false, "tiny sizes: builds and runs everything and compares the outputs, no timing meaning")
	fs.IntVar(&f.reps, "reps", 10, "measured runs per implementation")
	fs.IntVar(&f.warmup, "warmup", 1, "warm-up runs before them, dropped")
	fs.IntVar(&f.jobs, "jobs", 4, "programs built at the same time")
	fs.IntVar(&f.buildRetries, "build-retries", 1, "how often a failed gocpp build is tried again (the retry is named in the result and in the report)")
	fs.DurationVar(&f.timeout, "timeout", 120*time.Second, "limit of one run of one program")
	fs.StringVar(&f.pin, "pin", "auto", "taskset: auto (idle cores when 4 are free), off, or a cpu list")
	fs.StringVar(&f.cxxflags, "cxxflags", DefaultCXXFlags, "GOCPP_CXXFLAGS of the O2 flavor")
	fs.Float64Var(&f.maxLoad, "max-load", 0, "1-minute load average above which the machine is called busy (0: an eighth of the cores, at least 4)")
	fs.BoolVar(&f.allowFail, "allow-failures", false, "a program that fails to build or run does not fail the command (a mismatch of outputs always does)")
	if check {
		fs.StringVar(&f.baseline, "baseline", "bench/baseline.json", "the baseline to compare with")
		fs.Float64Var(&f.threshold, "threshold", 0.05, "allowed slowdown")
		fs.IntVar(&f.retries, "retries", 2, "re-measurements of a suspected regression (it must reproduce in all of them)")
		fs.BoolVar(&f.requireQuiet, "require-quiet", false, "a busy machine fails the check instead of being inconclusive")
		fs.BoolVar(&f.trustBaseline, "trust-baseline", false, "compare even when the baseline was measured on a busy machine (its load average was over the limit)")
		fs.BoolVar(&f.noDrift, "no-drift", false, "do not correct for the machine's speed change measured with Go and the C++ twins")
		fs.StringVar(&f.out, "out", "", "write the JSON of the measurements here")
	} else {
		fs.StringVar(&f.out, "out", "bench/results/latest.json", "write the JSON here, and a copy named by the commit and the time (empty: none)")
		fs.StringVar(&f.md, "md", "", "write the markdown report here (empty: none)")
		fs.StringVar(&f.history, "history", "", "append the history line to this file, or to bench/history/<machine>.jsonl when the value is `auto` (empty: none)")
		fs.BoolVar(&f.compile, "compile", false, "also measure compile times (cold, warm, no-op) of generated programs")
		fs.IntVar(&f.compileReps, "compile-reps", 3, "runs of each compile-time measurement")
	}
	return fs, f
}

func (f *runFlags) config(root string, check bool) (*Config, error) {
	spec := f.flavors
	if spec == "" {
		switch {
		case check:
			spec = "O2"
		case f.smoke:
			spec = "default,O2"
		default:
			spec = strings.Join(benchdata.Flavors, ",")
		}
	}
	flavors, err := parseFlavors(spec)
	if err != nil {
		return nil, err
	}
	impls, err := parseImpls(f.impls, flavors)
	if err != nil {
		return nil, err
	}
	work := f.work
	if work == "" {
		work, err = os.MkdirTemp("", "gobench-")
		if err != nil {
			return nil, err
		}
	} else if work, err = filepath.Abs(work); err != nil {
		return nil, err
	}
	cfg := &Config{
		Root: root, BenchDir: filepath.Join(root, "bench"), Work: work, Gocpp: f.gocpp,
		GoBin: toolPath("go"), Gxx: toolPath("g++"), Clang: toolPath("clang++"), Strip: lookTool("strip"), Taskset: lookTool("taskset"),
		Impls: impls, Reps: f.reps, Warmup: f.warmup, Smoke: f.smoke, Timeout: f.timeout, Jobs: f.jobs, Retries: f.buildRetries, CXXFlags: f.cxxflags, Flavors: flavors,
		Log: func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) },
	}
	if f.smoke {
		cfg.Reps, cfg.Warmup = max(1, min(f.reps, 1)), 0
	}
	return cfg, nil
}

func lookTool(name string) string {
	if p := toolPath(name); p != name {
		return p
	}
	return ""
}

func cmdList(args []string) int {
	fs, f := newFlags("list", false)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, err := findRoot(f.root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	ps, err := LoadPrograms(filepath.Join(root, "bench"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	for _, p := range ps {
		twin := "-"
		if p.Twin {
			twin = "twin"
		}
		needs := ""
		if len(p.Needs) > 0 {
			needs = " needs: " + strings.Join(p.Needs, ",")
		}
		fmt.Printf("%-9s %-22s size %-10d smoke %-8d %s%s  %s\n", p.Category, p.Name, p.Size, p.Smoke, twin, needs, p.About)
	}
	return 0
}

// cmdValidate checks the programs without building anything.
func cmdValidate(args []string) int {
	fs, f := newFlags("validate", false)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, err := findRoot(f.root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	ps, err := LoadPrograms(filepath.Join(root, "bench"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	bad := 0
	twins := 0
	for _, p := range ps {
		if p.Smoke >= p.Size {
			fmt.Fprintf(os.Stderr, "%s: smoke %d is not smaller than size %d\n", p.Path, p.Smoke, p.Size)
			bad++
		}
		if p.Twin {
			twins++
		}
	}
	fmt.Printf("%d programs, %d with C++ twins\n", len(ps), twins)
	if bad > 0 {
		return 1
	}
	return 0
}

func cmdReport(args []string) int {
	fs := flag.NewFlagSet("gobench report", flag.ContinueOnError)
	in := fs.String("in", "bench/results/latest.json", "the JSON file")
	md := fs.String("md", "docs/benchmarks/runtime.md", "the markdown file to write")
	maxLoad := fs.Float64("max-load", 0, "1-minute load above which the run is called noisy (0: an eighth of the cores, at least 4)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	r, err := benchdata.ReadReport(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	ml := *maxLoad
	if ml == 0 {
		ml = benchdata.DefaultMaxLoad(r.Machine.Cores)
	}
	if err := os.WriteFile(*md, []byte(Markdown(r, ml)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	return 0
}

// cmdMerge puts the results of the programs of a partial run into a report,
// replacing the ones it has (for a program added to the suite after the
// baseline run, or measured again). The machine and the compile times of the
// base report stay.
func cmdMerge(args []string) int {
	fs := flag.NewFlagSet("gobench merge", flag.ContinueOnError)
	base := fs.String("base", "bench/baseline.json", "the report to change")
	add := fs.String("add", "", "the report whose programs replace or extend the base's")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *add == "" {
		fmt.Fprintln(os.Stderr, "gobench merge: -add FILE is required")
		return 2
	}
	b, err := benchdata.ReadReport(*base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	a, err := benchdata.ReadReport(*add)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	if a.Size != b.Size {
		fmt.Fprintf(os.Stderr, "gobench merge: the sizes differ (%s and %s)\n", b.Size, a.Size)
		return 2
	}
	benchdata.MergeResults(b, a)
	if err := benchdata.WriteReport(*base, b); err != nil {
		fmt.Fprintln(os.Stderr, "gobench:", err)
		return 2
	}
	return 0
}

func selectPrograms(root, match string) ([]*Program, error) {
	ps, err := LoadPrograms(filepath.Join(root, "bench"))
	if err != nil {
		return nil, err
	}
	if match != "" {
		re, err := regexp.Compile(match)
		if err != nil {
			return nil, fmt.Errorf("-match: %v", err)
		}
		ps = Select(ps, re.MatchString)
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("no program matches")
	}
	return ps, nil
}

// cmdRun runs the suite; with check set it compares with the baseline.
func cmdRun(args []string, check bool) int {
	name := "run"
	if check {
		name = "check"
	}
	fs, f := newFlags(name, check)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, "gobench: "+format+"\n", a...)
		return 2
	}
	root, err := findRoot(f.root)
	if err != nil {
		return fail("%v", err)
	}
	ps, err := selectPrograms(root, f.match)
	if err != nil {
		return fail("%v", err)
	}
	cfg, err := f.config(root, check)
	if err != nil {
		return fail("%v", err)
	}
	if f.work == "" && !f.keep {
		defer os.RemoveAll(cfg.Work)
		// an interrupted run must not leave its work folder (binaries and caches) in /tmp; the
		// benchmarked programs die with the runner (Pdeathsig, own process groups)
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sig
			os.RemoveAll(cfg.Work)
			os.Exit(130)
		}()
	}
	var base *benchdata.Report
	if check {
		base, err = benchdata.ReadReport(filepath.Join(root, f.baseline))
		if err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
				fmt.Fprintf(os.Stderr, "gobench check: no baseline (%v): nothing to compare with; make bench-baseline writes one\n", err)
				return 0
			}
			return fail("%v", err)
		}
		cfg.Reps, cfg.Warmup = max(cfg.Reps, base.Reps), base.Warmup
		if f.smoke {
			return fail("check does not run with -smoke")
		}
	}
	maxLoad := f.maxLoad
	if maxLoad == 0 {
		maxLoad = benchdata.DefaultMaxLoad(runtimeCores())
	}
	rep := &benchdata.Report{Date: time.Now().UTC().Format(time.RFC3339), Reps: cfg.Reps, Warmup: cfg.Warmup, Size: "full"}
	if f.smoke {
		rep.Size = "smoke"
	}
	rep.Commit, rep.Dirty = gitInfo(root)
	pinList, why := pinList(f.pin, 4)
	if pinList == "" && f.pin == "auto" {
		cfg.logf("not pinned: %s", why)
	}
	cfg.PinList = pinList
	rep.Machine.Pinned = pinList
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		return fail("%v", err)
	}
	mach := machineInfo(cfg.Gocpp)
	mach.Pinned = pinList
	mach.GoMaxProcs = childProcs(pinList)
	mach.LoadBefore = loadAvg()
	rep.Machine = mach
	if mach.LoadBefore[0] > maxLoad {
		cfg.logf("the 1-minute load average is %.1f, over %.1f: the figures are noisy", mach.LoadBefore[0], maxLoad)
	}
	rep.Results = suite.Run()
	if !check && f.compile {
		rep.Compile = runCompileBench(cfg, f.compileReps)
	}
	rep.Machine.LoadAfter = loadAvg()
	code := 0
	if lines := Failures(rep.Results); len(lines) > 0 {
		fmt.Fprintln(os.Stderr, "\ngobench: FAILURES")
		mismatch := false
		for _, l := range lines {
			fmt.Fprintln(os.Stderr, "  "+l)
			mismatch = mismatch || strings.Contains(l, "MISMATCH")
		}
		if mismatch || !f.allowFail {
			code = 2
		}
	}
	if check {
		return runCheck(cfg, suite, base, rep, f, maxLoad, code)
	}
	if err := writeOutputs(root, rep, f, maxLoad); err != nil {
		return fail("%v", err)
	}
	printSummary(rep)
	return code
}

func runtimeCores() int { return len(cpuTimes()) }

// writeOutputs writes the JSON, the markdown and the history line the flags ask for.
func writeOutputs(root string, rep *benchdata.Report, f *runFlags, maxLoad float64) error {
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, p)
	}
	if f.out != "" {
		if err := os.MkdirAll(filepath.Dir(abs(f.out)), 0o755); err != nil {
			return err
		}
		if err := benchdata.WriteReport(abs(f.out), rep); err != nil {
			return err
		}
		stamp := filepath.Join(filepath.Dir(abs(f.out)), fmt.Sprintf("%s-%s-%s.json", rep.Size, rep.Commit, time.Now().UTC().Format("20060102T150405")))
		if err := benchdata.WriteReport(stamp, rep); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s and %s\n", f.out, stamp)
	}
	if f.md != "" {
		if err := os.MkdirAll(filepath.Dir(abs(f.md)), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs(f.md), []byte(Markdown(rep, maxLoad)), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", f.md)
	}
	if f.history != "" && rep.Size == "full" {
		name := f.history
		if name == "auto" {
			name = filepath.Join("bench", "history", historyFileName(rep.Machine)+".jsonl")
		}
		if err := benchdata.AppendHistory(abs(name), benchdata.FromReport(rep, gitSubject(root))); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "appended a line to %s\n", name)
	}
	return nil
}

// historyFileName names the history file of a machine: the host name and the
// core count, so that two machines never share a history.
func historyFileName(m benchdata.Machine) string {
	host, _ := os.Hostname()
	s := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, host)
	if s == "" {
		s = "host"
	}
	return fmt.Sprintf("%s-%dc", s, m.Cores)
}

// printSummary prints the geometric means to stderr: gocpp over Go for every
// gocpp build, then the optimized builds over their twins.
func printSummary(r *benchdata.Report) {
	var all []string
	for _, i := range gocppImpls(r) {
		if hasImpl(r, i) { // a flavor gocpp cannot build yet has no column
			all = append(all, i)
		}
	}
	fmt.Fprintln(os.Stderr, "\ngeometric mean of time ratios (below 1.00: gocpp is faster)")
	line := func(title string, impls []string, ref func(string) string) {
		fmt.Fprintf(os.Stderr, "%-10s %4s", title, "n")
		for _, i := range impls {
			fmt.Fprintf(os.Stderr, " %18s", strings.TrimPrefix(implTitle(i), "gocpp "))
		}
		fmt.Fprintln(os.Stderr)
		for _, g := range append(slices.Clone(benchdata.Categories), "all") {
			n := 0
			cells := ""
			for _, i := range impls {
				geo, k, _, _ := groupRatios(r, g, i, ref(i))
				n = max(n, k)
				cells += fmt.Sprintf(" %18s", ratioStr(geo))
			}
			if n > 0 {
				fmt.Fprintf(os.Stderr, "%-10s %4d%s\n", g, n, cells)
			}
		}
	}
	line("over Go", all, func(string) string { return benchdata.ImplGo })
	line("over C++", slices.DeleteFunc(slices.Clone(all), func(i string) bool { return !benchdata.Optimized(i) }), benchdata.CppTwin)
}
