// Fmtdiff generates Go programs that format values with fmt (Sprintf and its relatives over a
// matrix of verbs, flags, widths, precisions and values), runs them with real Go and with gocpp
// and reports the cases where the two print something else. See README.md.
//
//	go run ./tools/fmtdiff -seed 1 -n 3000 -j 8 -gocpp ./gocpp -out /tmp/fmtdiff
//	go run ./tools/fmtdiff -seed 1 -print 3                       # the source of program 3
//	go run ./tools/fmtdiff -seed 1 -prog 3 -case 17,40 -emit-test fmt_gen_x   # a tests/run test
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fmtdiff:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		seed  = flag.Uint64("seed", 1, "seed")
		n     = flag.Int("n", 1000, "number of cases in all")
		per   = flag.Int("per", 250, "cases per program")
		first = flag.Int("first", 0, "index of the first program")
		jobs  = flag.Int("j", 6, "programs in flight (at most 12)")
		gocpp = flag.String("gocpp", "", "the gocpp binary")
		work  = flag.String("work", "",
			"scratch parent folder (a tool-owned child is created; default: temporary directory)")
		out      = flag.String("out", "", "folder of the report (default: <owned-child>/out)")
		xdg      = flag.String("xdg", "", "XDG_CACHE_HOME of gocpp (default: <owned-child>/xdg)")
		mode     = flag.String("mode", "both", "both: compare Go with gocpp; go: only check the programs with real Go")
		budget   = flag.Int("budget", 40, "runs per program for isolating a crash")
		timeout  = flag.Duration("timeout", 10*time.Minute, "time limit of one run")
		cxx      = flag.String("cxx", "", "GOCPP_CXX: the C++ compiler of gocpp (g++ or clang++)")
		cxxf     = flag.String("run-flags", "", "extra arguments of `gocpp run`, space separated (-asan)")
		noKnown  = flag.Bool("no-known", false, "count the mismatches of the known bugs (known.go) too")
		printAll = flag.Bool("print-all", false, "print a program that formats every value of the catalog once, and exit")
		printP   = flag.Int("print", -1, "print the program with this index and exit")
		prog     = flag.Int("prog", -1, "program index of -emit-test")
		cases    = flag.String("case", "", "case ids of -emit-test, comma separated")
		emit     = flag.String("emit-test", "", "write tests/run/NAME.gp (the output block from real Go) for the cases of -prog")
		testDir  = flag.String("test-dir", "tests/run", "folder of -emit-test")
		desc     = flag.String("desc", "", "comment of the emitted test")
	)
	flag.Parse()
	g := NewGen()
	if *printAll {
		p := &Program{Seed: *seed, Vals: g.Vals}
		for i, v := range g.Vals {
			p.Cases = append(p.Cases, Case{ID: i + 1, Expr: "fmt.Sprintf(\"%v|%+v|%#v|%T\", " + v.Op + ", " + v.Op + ", " + v.Op + ", " + v.Op + ")", Vals: []int{i}})
		}
		fmt.Print(g.Source(p, nil))
		return nil
	}
	if *printP >= 0 {
		p := g.Program(*seed, *printP, *per)
		fmt.Print(g.Source(p, nil))
		return nil
	}
	ownedWork, cleanupWork, err := newWorkDir(*work)
	if err != nil {
		return err
	}
	*work = ownedWork
	if *out == "" {
		*out = filepath.Join(*work, "out")
	}
	if *xdg == "" {
		*xdg = filepath.Join(*work, "xdg")
	}
	for _, d := range []string{*work, *out, *xdg} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	r := &Runner{Gocpp: *gocpp, Work: *work, XDG: *xdg, Timeout: *timeout, Flags: strings.Fields(*cxxf), Cxx: *cxx}
	if *emit != "" {
		return emitTest(g, r, *seed, *per, *prog, *cases, *emit, *testDir, *desc)
	}
	if *mode == "both" && *gocpp == "" {
		return fmt.Errorf("-gocpp is required")
	}
	if *jobs > 12 {
		*jobs = 12
	}
	nprog := (*n + *per - 1) / *per
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	var (
		mu       sync.Mutex
		all      []Mismatch
		genBugs  []Mismatch
		notes    []string
		cases_   int
		runs     int
		reportMu sync.Mutex
	)
	rf, err := os.Create(filepath.Join(*out, fmt.Sprintf("report-%d.txt", *seed)))
	if err != nil {
		return err
	}
	defer rf.Close()
	go func() {
		<-sig
		// Close our report before removing its directory (required on Windows). Notify was
		// installed before report creation, so an early signal stays buffered until here.
		rf.Close()
		cleanupWork()
		os.Exit(130)
	}()

	idx := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < *jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				p := g.Program(*seed, i, *per)
				var oc *Outcome
				if *mode == "go" {
					oc = r.checkGoOnly(g, p, *budget)
				} else {
					oc = r.Check(g, p, *budget)
				}
				mu.Lock()
				all = append(all, oc.Mismatches...)
				genBugs = append(genBugs, oc.GenBugs...)
				cases_ += oc.Cases
				runs += oc.Runs
				for _, nt := range oc.Notes {
					notes = append(notes, fmt.Sprintf("program %d: %s", i, nt))
				}
				mu.Unlock()
				reportMu.Lock()
				for _, m := range oc.Mismatches {
					fmt.Fprint(rf, formatMismatch(g, m))
				}
				for _, m := range oc.GenBugs {
					fmt.Fprint(rf, "GENERATOR BUG\n"+formatMismatch(g, m))
				}
				rf.Sync()
				reportMu.Unlock()
				fmt.Fprintf(os.Stderr, "program %d: %d cases, %d mismatches, %d generator bugs, %d runs\n", i, oc.Cases, len(oc.Mismatches), len(oc.GenBugs), oc.Runs)
			}
		}()
	}
	for i := 0; i < nprog; i++ {
		idx <- *first + i
	}
	close(idx)
	wg.Wait()
	var fresh []Mismatch
	knownN := map[string]int{}
	for _, m := range all {
		if k := known(g, m); k != "" && !*noKnown {
			knownN[k]++
		} else {
			fresh = append(fresh, m)
		}
	}
	for k, n := range knownN {
		fmt.Printf("known bug %s: %d mismatches (not counted)\n", k, n)
	}
	summary(g, fresh, genBugs, notes, cases_, runs)
	if len(fresh) > 0 || len(genBugs) > 0 {
		return fmt.Errorf("%d mismatches, %d generator bugs (report: %s)", len(fresh), len(genBugs), rf.Name())
	}
	return nil
}

// newWorkDir never takes ownership of the caller's directory: even an explicit -work is only
// a parent. Capture the newly created child in cleanup, not the flag or a caller-controlled path.
// Reports remain available after a normal run; only interruption invokes this cleanup.
func newWorkDir(parent string) (string, func(), error) {
	if parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", nil, err
		}
	}
	owned, err := os.MkdirTemp(parent, "fmtdiff-")
	if err != nil {
		return "", nil, err
	}
	return owned, func() { os.RemoveAll(owned) }, nil
}

func (r *Runner) checkGoOnly(g *Gen, p *Program, budget int) *Outcome {
	oc := &Outcome{Cases: len(p.Cases), Runs: 1}
	res := r.RunGo(g.Source(p, nil))
	if res.Failed() {
		oc.Notes = append(oc.Notes, "go failed: "+firstLine(res.Stderr))
		bad := r.isolate(&budget, oc, func() []int {
			var s []int
			for _, c := range p.Cases {
				s = append(s, c.ID)
			}
			return s
		}(), func(sub map[int]bool) bool { return r.RunGo(g.Source(p, sub)).Failed() })
		for _, id := range bad {
			oc.GenBugs = append(oc.GenBugs, Mismatch{Seed: p.Seed, Prog: p.Index, Vals: p.caseVals(p.Cases[id-1]), Case: p.Cases[id-1], Note: firstLine(res.Stderr)})
		}
	}
	return oc
}

func formatMismatch(g *Gen, m Mismatch) string {
	var b strings.Builder
	fmt.Fprintf(&b, "seed %d prog %d case %d [%s]\n  src:   %s\n", m.Seed, m.Prog, m.Case.ID, m.Case.Fn, m.Case.Expr)
	for i := range m.Case.Vals {
		fmt.Fprintf(&b, "  value: %s\n", declOrLit(m.Vals[i]))
	}
	if m.Note != "" {
		fmt.Fprintf(&b, "  note:  %s\n", m.Note)
	}
	if m.Go != "" || m.Gocpp != "" {
		fmt.Fprintf(&b, "  go:    %s\n  gocpp: %s\n", m.Go, m.Gocpp)
	}
	b.WriteString("\n")
	return b.String()
}

func declOrLit(v Val) string {
	if v.Decl == "" {
		return v.Op
	}
	return declLine(v)
}

func summary(g *Gen, all, genBugs []Mismatch, notes []string, cases, runs int) {
	fmt.Printf("%d cases, %d builds, %d mismatches, %d generator bugs\n", cases, runs, len(all), len(genBugs))
	count := func(title string, key func(Mismatch) string, ms []Mismatch) {
		m := map[string]int{}
		for _, x := range ms {
			m[key(x)]++
		}
		type kv struct {
			k string
			n int
		}
		var l []kv
		for k, n := range m {
			l = append(l, kv{k, n})
		}
		sort.Slice(l, func(i, j int) bool { return l[i].n > l[j].n || l[i].n == l[j].n && l[i].k < l[j].k })
		fmt.Println(title)
		for i, x := range l {
			if i >= 25 {
				break
			}
			fmt.Printf("  %5d  %s\n", x.n, x.k)
		}
	}
	if len(all) > 0 {
		count("by verb:", func(m Mismatch) string { return m.Case.Fn + " " + m.Case.Verb }, all)
		count("by value:", func(m Mismatch) string {
			var s []string
			for _, v := range m.Vals {
				s = append(s, v.Describe)
			}
			return strings.Join(s, " ; ")
		}, all)
	}
	if len(genBugs) > 0 {
		count("generator bugs:", func(m Mismatch) string { return m.Note }, genBugs)
	}
	for _, nt := range notes {
		fmt.Println("note:", nt)
	}
}

func emitTest(g *Gen, r *Runner, seed uint64, per, prog int, cases, name, dir, desc string) error {
	if prog < 0 || cases == "" {
		return fmt.Errorf("-emit-test needs -prog and -case")
	}
	p := g.Program(seed, prog, per)
	only := map[int]bool{}
	for _, f := range strings.Split(cases, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || id < 1 || id > len(p.Cases) {
			return fmt.Errorf("bad case %q", f)
		}
		only[id] = true
	}
	pruned, err := Prune(g.Source(p, only))
	if err != nil {
		return err
	}
	res := r.RunGo(pruned)
	if res.Failed() {
		return fmt.Errorf("real Go fails on the test: %v %s", res.Err, res.Stderr)
	}
	txt, err := testFile(pruned, res.Out, desc, seed, prog, cases)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name+".gp")
	if err := os.WriteFile(path, []byte(txt), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return nil
}

// testFile makes the text of a tests/run test: the program and its output from real Go.
func testFile(src, output, desc string, seed uint64, prog int, cases string) (string, error) {
	var b strings.Builder
	b.WriteString("// phase: 05\n\n")
	if desc == "" {
		desc = "A case that tools/fmtdiff found: the output is real Go's."
	}
	fmt.Fprintf(&b, "// %s (fmtdiff seed %d, program %d, cases %s)\n", desc, seed, prog, cases)
	b.WriteString(src)
	b.WriteString("\n// Output:\n")
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if strings.HasSuffix(line, " ") || strings.ContainsRune(line, 0) || !validUTF8(line) {
			return "", fmt.Errorf("the output line %q cannot be an Output comment", line)
		}
		if line == "" {
			b.WriteString("//\n")
		} else {
			b.WriteString("// " + line + "\n")
		}
	}
	return b.String(), nil
}

func validUTF8(s string) bool { return strings.ToValidUTF8(s, "\x00") == s }
