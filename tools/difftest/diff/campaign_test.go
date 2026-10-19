package diff

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

func lineOf(src, needle string) int {
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	return 1
}

func testConfig(t *testing.T, ref, sub *fakeBackend) Config {
	t.Helper()
	dir := t.TempDir()
	feat := gen.FeaturesForPhase(5)
	return Config{
		Seed: 5, N: 40, Batch: 20, Jobs: 2,
		Gen:      gen.Config{Features: feat, Scenarios: 2, FatalPercent: 30},
		Ref:      ref,
		Subjects: []Backend{sub},
		StateDir: filepath.Join(dir, "state"), FindingsDir: filepath.Join(dir, "findings"),
		FatalSample: 100, ReduceBudget: 25, MaxReduce: 1,
		Log: os.Stderr,
	}
}

// bugOnScenario makes the subject print "wrong" for programs that use the scenario.
func bugOnScenario(name string) func(string, int) string {
	return func(src string, id int) string {
		if strings.Contains(src, "// scenario: "+name+"\n") {
			return "wrong " + name
		}
		return ""
	}
}

func TestCampaignFindsResumesClassifies(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	ref, sub := fakeGo(), fakeCpp(bugOnScenario("ints"))
	cfg := testConfig(t, ref, sub)
	cfg.ReduceBudget = 100
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Programs < 35 || st.Batches != 2 {
		t.Fatalf("stats %+v", st)
	}
	if st.Mismatches == 0 || st.Unknown == 0 || st.Clusters == 0 {
		t.Fatalf("the injected bug was not found: %+v", st)
	}
	if st.ByKind["output"] != st.Mismatches {
		t.Fatalf("kinds %v", st.ByKind)
	}
	matches, _ := filepath.Glob(filepath.Join(cfg.FindingsDir, "output-*", "main.go"))
	if len(matches) == 0 {
		t.Fatal("no finding written")
	}
	data, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(data), "// scenario: ints") {
		t.Fatalf("the finding does not hold the buggy scenario:\n%s", data)
	}
	if len(data) > 3500 {
		t.Errorf("the finding was not minimized: %d bytes", len(data))
	}
	md, _ := filepath.Glob(filepath.Join(cfg.FindingsDir, "output-*", "finding.md"))
	if len(md) == 0 {
		t.Fatal("no finding.md")
	}

	// resuming runs nothing and gives the same totals
	runsBefore := sub.runs.Load() + ref.runs.Load()
	st2, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if after := sub.runs.Load() + ref.runs.Load(); after != runsBefore {
		t.Fatalf("a finished campaign ran %d more builds", after-runsBefore)
	}
	if st2.Programs != st.Programs || st2.Mismatches != st.Mismatches {
		t.Fatalf("totals changed: %+v vs %+v", st2, st)
	}

	// the same bug with a known-differences entry: no findings
	cfg2 := testConfig(t, fakeGo(), fakeCpp(bugOnScenario("ints")))
	cfg2.Known, err = ParseKnown(strings.NewReader("name: ints-bug\nkind: output\nsig: wrong ints\nreason: injected\ndoc: docs/guides/difftest.md\n"))
	if err != nil {
		t.Fatal(err)
	}
	st3, err := NewCampaign(cfg2).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st3.Unknown != 0 || st3.Known["ints-bug"] != st3.Mismatches || st3.Mismatches == 0 {
		t.Fatalf("classification: %+v", st3)
	}
	if st3.Clusters != 0 {
		t.Fatalf("a known difference must not write a finding: %+v", st3)
	}
}

func TestCampaignManyFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	cfg := testConfig(t, fakeGo(), fakeCpp(bugOnScenario("ints")))
	cfg.Files = 4
	cfg.ReduceBudget = 100
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mismatches == 0 || st.Programs < 35 || st.GenBugs != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestSplitFilesAndErrors(t *testing.T) {
	progs := []*gen.Program{progWith(1, "func p1_a() {}\n"), progWith(2, "func p2_a() {}\n"), progWith(3, "func p3_a() {}\n")}
	src, err := gen.BundleFiles(progs, gen.Recovered, 3)
	if err != nil {
		t.Fatal(err)
	}
	fs := SplitFiles(src)
	if len(fs) != 4 || fs[0].Name != "main" || fs[1].Name != "p0" {
		t.Fatalf("%+v", fs)
	}
	// an error in the file of program 2
	var p2 File
	for _, f := range fs {
		if strings.Contains(f.Text, "func p2_a") {
			p2 = f
		}
	}
	line := lineOf(p2.Text, "func p2_a")
	out := fmt.Sprintf("./%s.go:%d:6: boom\n./main.go:3:1: not a program\n", p2.Name, line)
	if ids := ProgsOfErrors(src, out); fmt.Sprint(ids) != "[2]" {
		t.Fatalf("ids %v", ids)
	}
	if e := ErrorOfProg(src, out, 2); !strings.Contains(e, "boom") || strings.Contains(e, "not a program") {
		t.Fatalf("%q", e)
	}
	if got := SplitFiles("package main\n"); len(got) != 1 || got[0].Name != "main" {
		t.Fatalf("%+v", got)
	}
}

func TestCampaignGeneratorBug(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	good := fakeGo()
	ref := &fakeBackend{name: "go", f: func(src string) Result {
		if strings.Contains(src, "func prog_3()") {
			return Result{BuildErr: fmt.Sprintf("./main.go:%d:1: declared and not used: x\n", lineOf(src, "// ---- program 3 ")+2)}
		}
		r, _ := good.Run(context.Background(), src)
		return r
	}}
	cfg := testConfig(t, ref, fakeCpp(func(string, int) string { return "" }))
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.GenBugs != 1 || st.Mismatches != 0 || st.Programs < 35 {
		t.Fatalf("stats %+v", st)
	}
	if _, err := os.Stat(filepath.Join(cfg.FindingsDir, "genbugs", "seed5-prog3.go.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignCrashAndBuildFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	inner := fakeCpp(func(string, int) string { return "" })
	var crashRuns int
	sub := &fakeBackend{name: "gocpp/fake", f: func(src string) Result {
		r, _ := inner.Run(context.Background(), src)
		// program 7 crashes the process; program 25 does not compile
		if strings.Contains(src, "func prog_25()") {
			return Result{BuildErr: fmt.Sprintf("main.gp:%d:3: internal error: generated C++ did not compile\n", lineOf(src, "// ---- program 25 ")+3), ICE: true}
		}
		if strings.Contains(src, "func prog_7()") {
			crashRuns++
			i := strings.Index(r.Stdout, "#### 7\n")
			r.Stdout = r.Stdout[:i+len("#### 7\n== prog\n")]
			r.Exit = 139
			r.Stderr = "Segmentation fault\n"
		}
		return r
	}}
	cfg := testConfig(t, fakeGo(), sub)
	cfg.MaxReduce = 1
	cfg.ReduceBudget = 5
	cfg.FatalSample = 0
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.ByKind["crash"] != 1 || st.ByKind["ice"] != 1 || st.Mismatches != 2 {
		t.Fatalf("stats %+v", st)
	}
	// the programs after the crash were run again, and matched
	if st.Programs < 35 {
		t.Fatalf("programs %d", st.Programs)
	}
}

func TestClusterKeyIgnoresNumbers(t *testing.T) {
	a := clusterKey(Mismatch{Kind: KindOutput, Sig: "- o12: 5\n+ o12: 6"})
	b := clusterKey(Mismatch{Kind: KindOutput, Sig: "- o99: 7\n+ o99: 8"})
	c := clusterKey(Mismatch{Kind: KindExit, Sig: "- o12: 5\n+ o12: 6"})
	if a != b || a == c {
		t.Fatalf("%s %s %s", a, b, c)
	}
}

func TestSampleIsDeterministic(t *testing.T) {
	n := 0
	for id := 0; id < 1000; id++ {
		first := sample(1, id, 25)
		if sample(1, id, 25) != first {
			t.Fatal("not deterministic")
		}
		if first {
			n++
		}
	}
	if n < 150 || n > 350 {
		t.Fatalf("25%% sample took %d of 1000", n)
	}
	if sample(1, 3, 0) || !sample(1, 3, 100) {
		t.Fatal("0 and 100 percent")
	}
}

// S3: the address filter looks at the whole output.
func TestCampaignAddressFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	long := strings.Repeat("x", 3000)
	isInts := func(src string) bool { return strings.Contains(src, "// scenario: ints\n") }
	// 1. a difference after byte 2000 behind a pointer: a finding, not a generator bug
	ref := fakeGoOut(func(src string, id int) string {
		if isInts(src) {
			return long + " 0xc000012345 tail A"
		}
		return ""
	})
	sub := fakeCpp(func(src string, id int) string {
		if isInts(src) {
			return long + " 0x555555564bf6 tail B"
		}
		return ""
	})
	cfg := testConfig(t, ref, sub)
	cfg.ReduceBudget = 30
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.GenBugs != 0 || st.Mismatches == 0 || st.Unknown == 0 {
		t.Fatalf("a real difference behind byte 2000 was filed as a generator bug: %+v", st)
	}
	// 2. a long hexadecimal value with a wrong digit: a finding
	ref = fakeGoOut(func(src string, id int) string {
		if isInts(src) {
			return "%#x 0xdeadbeef"
		}
		return ""
	})
	sub = fakeCpp(func(src string, id int) string {
		if isInts(src) {
			return "%#x 0xdeadbeee"
		}
		return ""
	})
	st, err = NewCampaign(testConfig(t, ref, sub)).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.GenBugs != 0 || st.Unknown == 0 {
		t.Fatalf("a wrong digit was hidden: %+v", st)
	}
	// 3. only addresses differ: a generator bug
	ref = fakeGoOut(func(src string, id int) string {
		if isInts(src) {
			return "p 0xc000012345 q"
		}
		return ""
	})
	sub = fakeCpp(func(src string, id int) string {
		if isInts(src) {
			return "p 0x555555564bf6 q"
		}
		return ""
	})
	st, err = NewCampaign(testConfig(t, ref, sub)).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.GenBugs == 0 || st.Mismatches != 0 || st.Unknown != 0 {
		t.Fatalf("an address-only difference must be a generator bug: %+v", st)
	}
}

// S2: a member of a known cluster must match the known entry itself.
func TestCampaignKnownClusterMemberIsChecked(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	bug := func(src string, id int) string {
		if strings.Contains(src, "// scenario: ints\n") || strings.Contains(src, "// scenario: strings\n") {
			return "wrong"
		}
		return ""
	}
	cfg := testConfig(t, fakeGo(), fakeCpp(bug))
	cfg.N, cfg.Batch, cfg.MaxReduce = 80, 40, 1
	var err error
	cfg.Known, err = ParseKnown(strings.NewReader("name: ints-only\nkind: output\nsig: wrong\nsrc: // scenario: ints\\n\nreason: injected\ndoc: docs/guides/difftest.md\n"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Known["ints-only"] == 0 || st.Unknown == 0 {
		t.Fatalf("both the known and the unknown bug must be seen: %+v", st)
	}
	if st.Known["ints-only"]+st.Unknown < st.Mismatches {
		t.Fatalf("%d mismatches, %d known, %d unknown", st.Mismatches, st.Known["ints-only"], st.Unknown)
	}
}

// S5: -redo runs the bundles with findings again.
func TestCampaignRedo(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	cfg := testConfig(t, fakeGo(), fakeCpp(bugOnScenario("ints")))
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil || st.Unknown == 0 {
		t.Fatalf("%v %+v", err, st)
	}
	// fixed: without Redo nothing runs, the old counters stay
	fixed := fakeCpp(func(string, int) string { return "" })
	cfg.Subjects = []Backend{fixed}
	st2, _ := NewCampaign(cfg).Run(context.Background())
	if fixed.runs.Load() != 0 || st2.Unknown != st.Unknown {
		t.Fatalf("a finished campaign ran again: %d runs, %+v", fixed.runs.Load(), st2)
	}
	cfg.Redo = true
	st3, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fixed.runs.Load() == 0 || st3.Unknown != 0 || st3.Mismatches != 0 {
		t.Fatalf("redo did not run the bundles with findings again: %d runs, %+v", fixed.runs.Load(), st3)
	}
}

// S1: the janitor.
func TestCleanStale(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Join(p, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(p, "sub", "f"), []byte("x"), 0o644)
		old := now.Add(-age)
		os.Chtimes(p, old, old)
	}
	mk("go-old", 2*time.Hour)
	mk("go-new", time.Minute)
	mk("cpp-old", 2*time.Hour)
	mk("keep-old", 2*time.Hour)
	if n := CleanStale(dir, time.Hour, now, "go-", "cpp-"); n != 2 {
		t.Fatalf("removed %d", n)
	}
	for name, want := range map[string]bool{"go-old": false, "cpp-old": false, "go-new": true, "keep-old": true} {
		_, err := os.Stat(filepath.Join(dir, name))
		if (err == nil) != want {
			t.Errorf("%s exists: %v, want %v", name, err == nil, want)
		}
	}
	// the campaign's sweep: tmp folders and the cache's gen and stamp
	cache := t.TempDir()
	for _, sub := range []string{"gen", "stamp", "obj", "rt"} {
		os.MkdirAll(filepath.Join(cache, sub, "x"), 0o755)
		old := now.Add(-3 * time.Hour)
		os.Chtimes(filepath.Join(cache, sub, "x"), old, old)
	}
	c := NewCampaign(Config{TmpDir: dir, CacheDir: cache})
	if n := c.Sweep(now); n != 2 {
		t.Fatalf("swept %d", n)
	}
	for sub, want := range map[string]bool{"gen": false, "stamp": false, "obj": true, "rt": true} {
		_, err := os.Stat(filepath.Join(cache, sub, "x"))
		if (err == nil) != want {
			t.Errorf("%s/x exists: %v, want %v (the object cache and the runtime are not swept)", sub, err == nil, want)
		}
	}
}

// A build of the generated C++ that fails once and works the second time (the machine's /tmp
// cleaner took the folder) is not a finding.
func TestCampaignRetriesAnIceOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks with the source importer")
	}
	inner := fakeCpp(func(string, int) string { return "" })
	var n atomic.Int64
	sub := &fakeBackend{name: "gocpp/fake", f: func(src string) Result {
		if n.Add(1) == 1 {
			return Result{BuildErr: "generated C++ did not compile with the flags below\n$GOCPP/gen/x.cpp:1:1: fatal error: opening dependency file /tmp/x.d: No such file or directory\n", ICE: true}
		}
		r, _ := inner.Run(context.Background(), src)
		return r
	}}
	cfg := testConfig(t, fakeGo(), sub)
	cfg.N, cfg.Batch, cfg.Jobs, cfg.FatalSample = 20, 20, 1, 0
	st, err := NewCampaign(cfg).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Retries != 1 || st.Mismatches != 0 || st.Unknown != 0 {
		t.Fatalf("%+v", st)
	}
	if got := iceSig("# math\ngenerated C++ did not compile with the flags below\n$GOCPP/gen/gpkg/math/pow.gp.cpp:142:1: fatal error: opening dependency file /tmp/a.d: No such file or directory\n"); got == "" || got == FirstError("# math\ngenerated C++ did not compile with the flags below") {
		t.Fatalf("the ice signature must be the compiler's error: %q", got)
	}
}
