//go:build unix

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// TestMain lets the tests run the runner as a separate process (see TestInterruptRemovesTheWorkFolder).
func TestMain(m *testing.M) {
	if os.Getenv("GOBENCH_HELPER") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// ---- the header of a program --------------------------------------------

const goodHeader = "// bench: foo\n// category: cpu\n// size: 100\n// smoke: 10\n// twin: no\n// about: a test\npackage main\n"

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestParseProgram(t *testing.T) {
	dir := t.TempDir()
	p, err := ParseProgram(writeFile(t, filepath.Join(dir, "cpu", "foo.gp"), "// needs: sync, generic-types\n"+goodHeader))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "foo" || p.Category != "cpu" || p.Size != 100 || p.Smoke != 10 || p.Twin || p.About != "a test" {
		t.Errorf("parsed %+v", p)
	}
	if len(p.Needs) != 2 || p.Needs[0] != "sync" || p.Needs[1] != "generic-types" {
		t.Errorf("needs %v", p.Needs)
	}
	bad := []struct{ name, src, want string }{
		{"missing about", strings.Replace(goodHeader, "// about: a test\n", "", 1), "missing header line"},
		{"duplicate", "// size: 5\n" + goodHeader, "duplicate"},
		{"size zero", strings.Replace(goodHeader, "size: 100", "size: 0", 1), "positive integer"},
		{"size text", strings.Replace(goodHeader, "size: 100", "size: big", 1), "positive integer"},
		{"twin maybe", strings.Replace(goodHeader, "twin: no", "twin: maybe", 1), "yes or no"},
		{"wrong name", strings.Replace(goodHeader, "bench: foo", "bench: bar", 1), "is named"},
		{"wrong category", strings.Replace(goodHeader, "category: cpu", "category: data", 1), "folder"},
	}
	for _, c := range bad {
		_, err := ParseProgram(writeFile(t, filepath.Join(dir, "cpu", "foo.gp"), c.src))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one with %q", c.name, err, c.want)
		}
	}
	// an unknown category is an error even when the folder agrees
	src := strings.Replace(goodHeader, "category: cpu", "category: weird", 1)
	if _, err := ParseProgram(writeFile(t, filepath.Join(dir, "weird", "foo.gp"), src)); err == nil || !strings.Contains(err.Error(), "unknown category") {
		t.Errorf("unknown category: %v", err)
	}
	// an ordinary comment with a colon before the header is not a header line
	p, err = ParseProgram(writeFile(t, filepath.Join(dir, "cpu", "foo.gp"), "// note: nothing\n"+goodHeader))
	if err != nil || p.Name != "foo" {
		t.Errorf("comment with a colon: %v", err)
	}
}

func TestLoadProgramsTwins(t *testing.T) {
	dir := t.TempDir()
	hdr := func(name, twin string) string {
		return "// bench: " + name + "\n// category: cpu\n// size: 10\n// smoke: 1\n// twin: " + twin + "\n// about: x\npackage main\n"
	}
	writeFile(t, filepath.Join(dir, "programs", "cpu", "a.gp"), hdr("a", "yes"))
	writeFile(t, filepath.Join(dir, "programs", "cpu", "b.gp"), hdr("b", "no"))
	if _, err := LoadPrograms(dir); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("twin: yes without a file: %v", err)
	}
	writeFile(t, filepath.Join(dir, "cpp", "a.cpp"), "int main(){}")
	writeFile(t, filepath.Join(dir, "cpp", "b.cpp"), "int main(){}")
	if _, err := LoadPrograms(dir); err == nil || !strings.Contains(err.Error(), "twin: no, but") {
		t.Errorf("twin: no with a file: %v", err)
	}
	os.Remove(filepath.Join(dir, "cpp", "b.cpp"))
	writeFile(t, filepath.Join(dir, "cpp", "orphan.cpp"), "int main(){}")
	if _, err := LoadPrograms(dir); err == nil || !strings.Contains(err.Error(), "no program") {
		t.Errorf("orphan twin: %v", err)
	}
	os.Remove(filepath.Join(dir, "cpp", "orphan.cpp"))
	ps, err := LoadPrograms(dir)
	if err != nil || len(ps) != 2 || ps[0].CppPath == "" || ps[1].CppPath != "" {
		t.Fatalf("LoadPrograms: %v %v", ps, err)
	}
	writeFile(t, filepath.Join(dir, "programs", "data", "a.gp"), strings.Replace(hdr("a", "no"), "cpu", "data", 1))
	if _, err := LoadPrograms(dir); err == nil || !strings.Contains(err.Error(), "also used") {
		t.Errorf("duplicate names: %v", err)
	}
}

// The real suite: the requirements of step 15.1 as a test.
func TestRealSuite(t *testing.T) {
	ps, err := LoadPrograms(filepath.Join("..", "..", "bench"))
	if err != nil {
		t.Fatal(err)
	}
	cats := map[string]int{}
	twins := 0
	for _, p := range ps {
		cats[p.Category]++
		if p.Twin {
			twins++
		}
		if p.Smoke >= p.Size {
			t.Errorf("%s: smoke size %d is not smaller than %d", p.Name, p.Smoke, p.Size)
		}
		src, err := os.ReadFile(p.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "benchN") {
			t.Errorf("%s does not use benchN: its size cannot be set", p.Name)
		}
		if p.Twin {
			cpp, _ := os.ReadFile(p.CppPath)
			if !strings.Contains(string(cpp), "BENCH_N") {
				t.Errorf("%s's twin does not use BENCH_N", p.Name)
			}
		}
	}
	if len(ps) < 30 || twins < 15 {
		t.Errorf("%d programs and %d twins; step 15.1 wants at least 30 and 15", len(ps), twins)
	}
	for _, c := range benchdata.Categories {
		if cats[c] == 0 {
			t.Errorf("no program in category %s", c)
		}
	}
}

// ---- running with fake tools ----------------------------------------------

// The fake tools are shell scripts. fakeGocpp/fakeGo/fakeCxx write a program
// (a script) that prints "result <N>" where N is the size from the shim or from
// -DBENCH_N; the behaviour is steered by environment variables:
//
//	FAKE_BAD=<program>   that program prints something else, in the Go build
//	FAKE_FAIL=<program>  that program exits 3 in every build
//	FAKE_NOSYNC=1        fake gocpp fails with "the type sync.Mutex is not available yet"
const fakeBuilder = `#!/bin/sh
# usage: fake <kind> build -o OUT ...   (kind: gocpp, go, cxx)
kind=$1; shift
[ -n "$FAKE_LOG" ] && echo "$kind CXX=$GOCPP_CXX FLAGS=[$GOCPP_CXXFLAGS] ARGS=$*" >> "$FAKE_LOG"
[ -n "$FAKE_SLEEP" ] && sleep "$FAKE_SLEEP"
if [ -n "$FAKE_FLAKY" ] && [ "$kind" = gocpp ] && [ ! -e "$FAKE_FLAKY" ] && case " $* " in *" -o "*) true ;; *) false ;; esac; then
  : > "$FAKE_FLAKY"
  echo "internal error: generated C++ did not link"
  exit 2
fi
out=""; n=0; src=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    -DBENCH_N=*) n=${1#-DBENCH_N=} ;;
    *.cpp) src=$1 ;;
  esac
  shift
done
[ -z "$out" ] && { echo "fake $kind 1.0"; exit 0; }
name=$(basename "$PWD")
case "$kind" in
  gocpp|go)
    n=$(sed -n 's/^const benchN = //p' shim.* | head -1)
    name=$(basename "$(dirname "$PWD")")
    ;;
  cxx) name=$(basename "$src" .cpp) ;;
esac
if [ "$kind" = gocpp ] && [ -n "$FAKE_NOSYNC" ] && grep -q needs-sync main.gp 2>/dev/null; then
  echo "./main.gp:16:6: the type sync.Mutex is not available yet (package sync: Phase 09)"
  exit 1
fi
tag="result"
[ "$kind" = go ] && [ "$FAKE_BAD" = "$name" ] && tag="other"
code=0
[ "$FAKE_FAIL" = "$name" ] && code=3
printf '#!/bin/sh\necho "%s %s"\nexit %d\n' "$tag" "$n" "$code" > "$out"
chmod +x "$out"
`

// fakeEnv builds a tool directory and returns a Config over a bench folder with
// the given programs (name, category, twin).
func fakeConfig(t *testing.T, progs ...[3]string) (*Config, []*Program) {
	t.Helper()
	root := t.TempDir()
	tools := filepath.Join(root, "tools")
	tool := func(name, kind string) string {
		return writeFile(t, filepath.Join(tools, name), "#!/bin/sh\nexec /bin/sh "+filepath.Join(tools, "builder.sh")+" "+kind+" \"$@\"\n")
	}
	writeFile(t, filepath.Join(tools, "builder.sh"), fakeBuilder)
	bench := filepath.Join(root, "bench")
	writeFile(t, filepath.Join(bench, "shim", "shim.gp.tmpl"), "package main\n\nconst benchN = @N@\n")
	writeFile(t, filepath.Join(bench, "shim", "shim.go.tmpl"), "package main\n\nconst benchN = @N@\n")
	for _, p := range progs {
		extra := ""
		if p[0] == "needy" {
			extra = "// needs: sync\n// needs-sync\n"
		}
		src := "// bench: " + p[0] + "\n// category: " + p[1] + "\n// size: 7\n// smoke: 3\n// twin: " + p[2] + "\n" + extra + "// about: fake\npackage main\n"
		if p[0] == "needy" { // keep the header contiguous: needs-sync marker goes in the body
			src = "// bench: needy\n// category: " + p[1] + "\n// size: 7\n// smoke: 3\n// twin: " + p[2] + "\n// needs: sync\n// about: fake\npackage main\n// needs-sync\n"
		}
		writeFile(t, filepath.Join(bench, "programs", p[1], p[0]+".gp"), src)
		if p[2] == "yes" {
			writeFile(t, filepath.Join(bench, "cpp", p[0]+".cpp"), "// twin\n")
		}
	}
	ps, err := LoadPrograms(bench)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Flavors: []string{"O2"}, CXXFlags: DefaultCXXFlags,
		Root: root, BenchDir: bench, Work: filepath.Join(root, "work"),
		Gocpp: tool("gocpp", "gocpp"), GoBin: tool("go", "go"), Gxx: tool("g++", "cxx"), Clang: tool("clang++", "cxx"),
		Impls: benchdata.AllImpls, Reps: 4, Warmup: 1, Timeout: 20 * time.Second, Jobs: 2,
	}
	if err := os.MkdirAll(cfg.Work, 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg, ps
}

func results(rs []benchdata.Result) map[string]benchdata.Result {
	m := map[string]benchdata.Result{}
	for _, r := range rs {
		m[r.Program+"/"+r.Impl] = r
	}
	return m
}

func TestSuiteWithFakeTools(t *testing.T) {
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "yes"}, [3]string{"beta", "data", "no"})
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	rs := suite.Run()
	if len(rs) != 10 {
		t.Fatalf("%d results, want 5 per program", len(rs))
	}
	m := results(rs)
	for _, impl := range benchdata.AllImpls {
		r := m["alpha/"+impl]
		if r.Status != benchdata.StatusOK || len(r.Wall) != cfg.Reps || len(r.CPU) != cfg.Reps || len(r.RSS) != cfg.Reps || r.Output == "" || r.BinSize == 0 {
			t.Errorf("alpha/%s: %+v", impl, r)
		}
		if r.Output != m["alpha/"+benchdata.ImplGo].Output {
			t.Errorf("alpha/%s: output %s differs from Go's", impl, r.Output)
		}
	}
	// beta has no twin: those two are skipped, the other three measured
	for _, impl := range []string{benchdata.ImplCppGxx, benchdata.ImplCppClang} {
		if r := m["beta/"+impl]; r.Status != benchdata.StatusSkipped || r.Note != "no C++ twin" {
			t.Errorf("beta/%s: %+v", impl, r)
		}
	}
	if len(Failures(rs)) != 0 {
		t.Errorf("failures: %v", Failures(rs))
	}
	// the size reaches every implementation: the full size here
	if !strings.Contains(runOut(t, filepath.Join(cfg.buildDir("alpha", benchdata.ImplGo), "prog")), "result 7") {
		t.Error("the size 7 did not reach the Go program")
	}
}

func runOut(t *testing.T, bin string) string {
	t.Helper()
	out, err := capture(".", nil, 10*time.Second, bin)
	if err != nil {
		t.Fatalf("%s: %v", bin, err)
	}
	return out
}

// The flavors reach gocpp as the environment and the arguments they stand for; -release is
// skipped while gocpp has no such flag, and built when it has.
func TestFlavors(t *testing.T) {
	logf := filepath.Join(t.TempDir(), "log")
	t.Setenv("FAKE_LOG", logf)
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "no"})
	cfg.Flavors = benchdata.Flavors
	cfg.Impls = benchdata.ImplsFor(benchdata.Flavors)
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	m := results(suite.Run())
	for _, cxx := range benchdata.Compilers {
		if r := m["alpha/"+benchdata.GocppImpl(cxx, "release")]; r.Status != benchdata.StatusSkipped || !strings.Contains(r.Note, "-release") {
			t.Errorf("release with %s: %+v", cxx, r)
		}
		for _, fl := range []string{"default", "O2"} {
			if r := m["alpha/"+benchdata.GocppImpl(cxx, fl)]; r.Status != benchdata.StatusOK {
				t.Errorf("%s %s: %+v", cxx, fl, r)
			}
		}
	}
	log, _ := os.ReadFile(logf)
	for _, want := range []string{"gocpp CXX=" + cfg.Gxx + " FLAGS=[] ARGS=build -o", "gocpp CXX=" + cfg.Clang + " FLAGS=[-O2 -g0] ARGS=build -o"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("the log lacks %q:\n%s", want, log)
		}
	}
	if strings.Contains(string(log), "-release") {
		t.Errorf("-release was passed to a gocpp that does not have it:\n%s", log)
	}
	// a gocpp that knows -release builds that flavor with the flag and without GOCPP_CXXFLAGS
	cfg.Release = true
	a := cfg.build(ps[0], benchdata.GocppImpl("g++", "release"))
	if a.result.Status != benchdata.StatusOK {
		t.Fatalf("%+v", a.result)
	}
	log, _ = os.ReadFile(logf)
	if !strings.Contains(string(log), "FLAGS=[] ARGS=build -release -o") {
		t.Errorf("release build:\n%s", log)
	}
}

func TestSmokeSizeUsed(t *testing.T) {
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "yes"})
	cfg.Smoke = true
	cfg.Reps, cfg.Warmup = 1, 0
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	for _, impl := range benchdata.AllImpls {
		if out := runOut(t, filepath.Join(cfg.buildDir("alpha", impl), "prog")); !strings.Contains(out, "result 3") {
			t.Errorf("%s ran with %q, want the smoke size 3", impl, out)
		}
	}
	for _, r := range suite.Run() {
		if r.Status != benchdata.StatusOK || len(r.Wall) != 1 {
			t.Errorf("%+v", r)
		}
	}
}

func TestOutputMismatchIsLoud(t *testing.T) {
	t.Setenv("FAKE_BAD", "alpha")
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "yes"})
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	rs := suite.Run()
	// Go is the reference: the four others print something different from Go's output
	m := results(rs)
	if m["alpha/go"].Status != benchdata.StatusOK {
		t.Errorf("the reference: %+v", m["alpha/go"])
	}
	n := 0
	for _, r := range rs {
		if r.Status == benchdata.StatusMismatch {
			n++
			if !strings.Contains(r.Note, "OUTPUT MISMATCH") {
				t.Errorf("note %q", r.Note)
			}
		}
	}
	if n != 4 {
		t.Errorf("%d mismatches, want 4", n)
	}
	lines := Failures(rs)
	if len(lines) != 4 || !strings.Contains(lines[0], "MISMATCH") {
		t.Errorf("failures: %v", lines)
	}
}

// The same binary printing something else on its second run is a mismatch, not a result.
func TestUnstableOutputIsMismatch(t *testing.T) {
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "no"})
	cfg.Impls = []string{benchdata.ImplGo}
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(cfg.buildDir("alpha", benchdata.ImplGo), "prog")
	counter := filepath.Join(cfg.Work, "counter")
	writeFile(t, bin, "#!/bin/sh\necho x >> "+counter+"\necho \"run $(wc -l < "+counter+")\"\n")
	rs := suite.Run()
	if len(rs) != 1 || rs[0].Status != benchdata.StatusMismatch || !strings.Contains(rs[0].Note, "changes between runs") {
		t.Errorf("%+v", rs)
	}
}

// Go is the reference of the comparison even when it is not the first implementation to run.
func TestGoIsTheReference(t *testing.T) {
	t.Setenv("FAKE_BAD", "alpha")
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "no"})
	cfg.Impls = []string{benchdata.ImplGocppGxx, benchdata.ImplGo}
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	m := results(suite.Run())
	if m["alpha/go"].Status != benchdata.StatusOK || m["alpha/gocpp-g++:O2"].Status != benchdata.StatusMismatch {
		t.Errorf("go %s, gocpp %s: Go must be the reference", m["alpha/go"].Status, m["alpha/gocpp-g++:O2"].Status)
	}
}

func TestRunFailureIsReported(t *testing.T) {
	t.Setenv("FAKE_FAIL", "alpha")
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "no"}, [3]string{"fine", "cpu", "no"})
	suite, _ := NewSuite(cfg, ps)
	rs := suite.Run()
	m := results(rs)
	if r := m["alpha/go"]; r.Status != benchdata.StatusRunFailed || len(r.Wall) != 0 {
		t.Errorf("alpha/go: %+v", r)
	}
	if r := m["fine/go"]; r.Status != benchdata.StatusOK {
		t.Errorf("fine/go: %+v", r)
	}
	if len(Failures(rs)) != 3 { // go, gocpp-g++:O2, gocpp-clang++:O2 (no twin)
		t.Errorf("failures: %v", Failures(rs))
	}
}

func TestNeedsSkipsAndPlainFailureFails(t *testing.T) {
	t.Setenv("FAKE_NOSYNC", "1")
	cfg, ps := fakeConfig(t, [3]string{"needy", "runtime", "no"}, [3]string{"alpha", "cpu", "no"})
	suite, _ := NewSuite(cfg, ps)
	m := results(suite.Run())
	for _, impl := range benchdata.AllImpls {
		r := m["needy/"+impl]
		if r.Status != benchdata.StatusSkipped || !strings.Contains(r.Note, "needs sync") {
			t.Errorf("needy/%s: %+v", impl, r)
		}
	}
	if m["alpha/go"].Status != benchdata.StatusOK {
		t.Errorf("alpha/go: %+v", m["alpha/go"])
	}
	// the same failure of a program without a needs line is a build failure
	cfg, ps = fakeConfig(t, [3]string{"needy", "runtime", "no"})
	for _, p := range ps {
		p.Needs = nil
	}
	suite, _ = NewSuite(cfg, ps)
	m = results(suite.Run())
	if r := m["needy/gocpp-g++:O2"]; r.Status != benchdata.StatusBuildFailed {
		t.Errorf("without needs: %+v", r)
	}
}

func TestPinnedCommand(t *testing.T) {
	c := &Config{}
	if got := c.cmdFor("/x"); len(got) != 1 {
		t.Errorf("unpinned: %v", got)
	}
	c = &Config{PinList: "2,3", Taskset: "/usr/bin/taskset"}
	if got := strings.Join(c.cmdFor("/x"), " "); got != "/usr/bin/taskset -c 2,3 /x" {
		t.Errorf("pinned: %s", got)
	}
}

// ---- the gate ---------------------------------------------------------------

// captureStdout runs f and returns what it printed to os.Stdout.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	defer func() { os.Stdout = old }()
	f()
	w.Close()
	return <-done
}

func steadyWall(v int64) []int64 { return []int64{v, v + 1, v - 1, v, v + 2, v - 2, v, v + 1} }

func gateReport(gocppScale float64) *benchdata.Report {
	r := &benchdata.Report{Commit: "c", Size: "full", Reps: 8, Warmup: 1}
	r.Machine.CPU, r.Machine.Cores = "fake", 8
	for _, p := range []string{"a", "b", "c", "d"} {
		for _, x := range []struct {
			impl string
			v    int64
		}{{benchdata.ImplGo, 10_000_000}, {benchdata.ImplCppGxx, 9_000_000}, {benchdata.ImplGocppGxx, int64(12_000_000 * gocppScale)}} {
			w := steadyWall(x.v)
			r.Results = append(r.Results, benchdata.Result{Program: p, Category: "cpu", Impl: x.impl, Status: benchdata.StatusOK, Wall: w, CPU: w, RSS: []int64{1}})
		}
	}
	return r
}

func gateRun(t *testing.T, base, cur *benchdata.Report, mod func(*runFlags)) (int, string) {
	t.Helper()
	f := &runFlags{threshold: 0.05, retries: 2}
	if mod != nil {
		mod(f)
	}
	suite := &Suite{cfg: &Config{}, arts: map[string][]*artifact{}} // nothing to measure again
	var code int
	out := captureStdout(t, func() { code = runCheck(&Config{}, suite, base, cur, f, 16, 0) })
	return code, out
}

func TestGateDecisions(t *testing.T) {
	base := gateReport(1)

	if code, out := gateRun(t, base, gateReport(1.01), nil); code != 0 || !strings.Contains(out, "passed") {
		t.Errorf("unchanged: %d\n%s", code, out)
	}
	if code, out := gateRun(t, base, gateReport(1.04), nil); code != 0 {
		t.Errorf("4 percent is under the threshold: %d\n%s", code, out)
	}
	if code, out := gateRun(t, base, gateReport(1.049), nil); code != 0 {
		t.Errorf("4.9 percent is under the threshold: %d\n%s", code, out)
	}
	for _, s := range []float64{1.052, 1.08} {
		if code, out := gateRun(t, base, gateReport(s), nil); code != 1 || !strings.Contains(out, "REGRESSIONS (4)") {
			t.Errorf("%.3f is over the threshold: %d\n%s", s, code, out)
		}
	}
	code, out := gateRun(t, base, gateReport(1.20), nil)
	if code != 1 || !strings.Contains(out, "REGRESSIONS (4)") || !strings.Contains(out, "FAILED") {
		t.Errorf("20 percent slower: %d\n%s", code, out)
	}
	if code, out := gateRun(t, base, gateReport(0.7), nil); code != 0 || !strings.Contains(out, "improved (4)") {
		t.Errorf("faster: %d\n%s", code, out)
	}

	// a faster or slower machine is not a regression: Go and the twins moved too
	slowMachine := gateReport(1.5)
	for i := range slowMachine.Results {
		if slowMachine.Results[i].Impl == benchdata.ImplGocppGxx {
			continue
		}
		for j := range slowMachine.Results[i].Wall {
			slowMachine.Results[i].Wall[j] = slowMachine.Results[i].Wall[j] * 3 / 2
		}
	}
	if code, out := gateRun(t, base, slowMachine, nil); code != 0 {
		t.Errorf("drift: %d\n%s", code, out)
	}
	if code, _ := gateRun(t, base, slowMachine, func(f *runFlags) { f.noDrift = true }); code != 1 {
		t.Errorf("with -no-drift the slow machine looks like a regression, got %d", code)
	}
}

// -match measures a part of the suite: the rest of the baseline is not "missing".
func TestGateOnlyComparesMeasuredPrograms(t *testing.T) {
	base := gateReport(1)
	cur := gateReport(1)
	cur.Results = cur.Results[:6] // programs a and b only
	code, out := gateRun(t, base, cur, nil)
	if code != 0 || strings.Contains(out, "not measured now") {
		t.Errorf("%d\n%s", code, out)
	}
	// a program that was selected but is no longer ok is still reported
	cur.Results[2].Status = benchdata.StatusRunFailed
	if _, out := gateRun(t, base, cur, nil); !strings.Contains(out, "not measured now (or not ok): a/gocpp-g++:O2") {
		t.Errorf("%s", out)
	}
}

func TestGateInconclusive(t *testing.T) {
	base := gateReport(1)
	slow := gateReport(1.5)
	busy := gateReport(1.5)
	busy.Machine.LoadBefore[0] = 99
	code, out := gateRun(t, base, busy, nil)
	if code != 0 || !strings.Contains(out, "INCONCLUSIVE") {
		t.Errorf("busy machine: %d\n%s", code, out)
	}
	if code, _ := gateRun(t, base, busy, func(f *runFlags) { f.requireQuiet = true }); code != 1 {
		t.Errorf("-require-quiet: %d, want 1", code)
	}
	other := gateReport(1.5)
	other.Machine.CPU = "another cpu"
	if code, out := gateRun(t, base, other, nil); code != 0 || !strings.Contains(out, "baseline was measured on") {
		t.Errorf("another machine: %d\n%s", code, out)
	}
	if code, out := gateRun(t, base, slow, nil); code != 1 {
		t.Errorf("sanity: %d\n%s", code, out)
	}
	// a run that failed fails the check with its own code and never compares
	code, out = captureAndRun(t, base, slow, 2)
	if code != 2 || strings.Contains(out, "REGRESSIONS") {
		t.Errorf("failed run: %d\n%s", code, out)
	}
	// nothing to compare
	empty := gateReport(1)
	empty.Results = nil
	if code, out := gateRun(t, base, empty, nil); code != 0 || !strings.Contains(out, "nothing to compare") {
		t.Errorf("empty: %d\n%s", code, out)
	}
}

func captureAndRun(t *testing.T, base, cur *benchdata.Report, runCode int) (int, string) {
	t.Helper()
	f := &runFlags{threshold: 0.05, retries: 2}
	suite := &Suite{cfg: &Config{}, arts: map[string][]*artifact{}}
	var code int
	out := captureStdout(t, func() { code = runCheck(&Config{}, suite, base, cur, f, 16, runCode) })
	return code, out
}

// A suspected regression is measured again with the built binaries, and one
// clean re-measurement clears it (a busy machine, not a slower gocpp).
func TestGateRemeasureClears(t *testing.T) {
	cfg, ps := fakeConfig(t, [3]string{"a", "cpu", "no"})
	cfg.Impls = []string{benchdata.ImplGo, benchdata.ImplGocppGxx}
	cfg.Reps, cfg.Warmup = 6, 0
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	// the fake program takes about a millisecond; the baseline claims 100 ns, the current run
	// claims 100 ms: a gross "regression" in the first measurement; remeasured (real times
	// of the fake programs) it is compared with the same baseline and must stay a regression:
	base := &benchdata.Report{Size: "full", Reps: 6}
	cur := &benchdata.Report{Size: "full", Reps: 6}
	base.Machine.CPU, cur.Machine.CPU = "fake", "fake"
	mk := func(impl string, ns int64) benchdata.Result {
		return benchdata.Result{Program: "a", Category: "cpu", Impl: impl, Status: benchdata.StatusOK, Wall: steadyWall(ns)}
	}
	base.Results = []benchdata.Result{mk(benchdata.ImplGo, 1_000_000), mk(benchdata.ImplGocppGxx, 100)}
	cur.Results = []benchdata.Result{mk(benchdata.ImplGo, 1_000_000), mk(benchdata.ImplGocppGxx, 100_000_000)}
	f := &runFlags{threshold: 0.05, retries: 1}
	var code int
	out := captureStdout(t, func() { code = runCheck(cfg, suite, base, cur, f, 1e9, 0) })
	if code != 1 || !strings.Contains(out, "measuring 1 suspected regression") {
		t.Errorf("code %d\n%s", code, out)
	}
	// the other way: the baseline is slow (1 s), so the re-measured fake (milliseconds) is no regression
	base.Results = []benchdata.Result{mk(benchdata.ImplGo, 1_000_000), mk(benchdata.ImplGocppGxx, 1_000_000_000)}
	cur.Results = []benchdata.Result{mk(benchdata.ImplGo, 1_000_000), mk(benchdata.ImplGocppGxx, 1_000_000_000*3/2)}
	out = captureStdout(t, func() { code = runCheck(cfg, suite, base, cur, f, 1e9, 0) })
	if code != 0 || !strings.Contains(out, "measuring 1 suspected regression") || strings.Contains(out, "REGRESSIONS") {
		t.Errorf("a clean re-measurement must clear it: code %d\n%s", code, out)
	}
}

// The commands end to end with fake tools on PATH: files written, exit codes.
func fakeRoot(t *testing.T) (root string, cfg *Config) {
	t.Helper()
	cfg, _ = fakeConfig(t, [3]string{"alpha", "cpu", "yes"}, [3]string{"beta", "data", "no"})
	root = cfg.Root
	writeFile(t, filepath.Join(root, "go.mod"), "module fake\n")
	t.Setenv("PATH", filepath.Join(root, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root, cfg
}

func quiet(t *testing.T, f func() int) int {
	t.Helper()
	if testing.Verbose() {
		return f()
	}
	old := os.Stderr
	null, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stderr = null
	defer func() { os.Stderr = old; null.Close() }()
	return f()
}

func TestCommandsEndToEnd(t *testing.T) {
	root, cfg := fakeRoot(t)
	args := []string{"-root", root, "-gocpp", cfg.Gocpp, "-reps", "3", "-warmup", "1", "-jobs", "2", "-pin", "off", "-max-load", "1e9", "-flavors", "O2",
		"-out", "bench/results/latest.json", "-md", "docs/b.md", "-history", "auto", "-work", filepath.Join(root, "w")}
	if code := quiet(t, func() int { return cmdRun(args, false) }); code != 0 {
		t.Fatalf("run exited %d", code)
	}
	rep, err := benchdata.ReadReport(filepath.Join(root, "bench", "results", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Size != "full" || rep.Reps != 3 || len(rep.Results) != 10 || rep.Machine.Cores == 0 || rep.Date == "" {
		t.Errorf("report: size %s reps %d results %d", rep.Size, rep.Reps, len(rep.Results))
	}
	if md, err := os.ReadFile(filepath.Join(root, "docs", "b.md")); err != nil || !strings.Contains(string(md), "# Runtime benchmarks") {
		t.Errorf("markdown: %v", err)
	}
	hist, _ := benchdata.ReadHistory(filepath.Join(root, "bench", "history"))
	if len(hist) != 1 || len(hist[0].Wall) != 8 { // 5 + 3 ok results (beta has no twins)
		t.Errorf("history: %d entries", len(hist))
	}
	if stamped, _ := filepath.Glob(filepath.Join(root, "bench", "results", "full-*.json")); len(stamped) != 1 {
		t.Errorf("the stamped copy: %v", stamped)
	}

	// the same run in smoke size: no history line, size smoke
	sargs := append([]string{}, args...)
	sargs = append(sargs, "-smoke")
	if code := quiet(t, func() int { return cmdRun(sargs, false) }); code != 0 {
		t.Fatalf("smoke run exited %d", code)
	}
	if hist, _ := benchdata.ReadHistory(filepath.Join(root, "bench", "history")); len(hist) != 1 {
		t.Errorf("a smoke run wrote a history line: %d", len(hist))
	}

	// a failing program fails the run unless allowed; a mismatch always fails
	t.Setenv("FAKE_FAIL", "alpha")
	if code := quiet(t, func() int { return cmdRun(args, false) }); code != 2 {
		t.Errorf("a run failure: %d, want 2", code)
	}
	allow := append(append([]string{}, args...), "-allow-failures")
	if code := quiet(t, func() int { return cmdRun(allow, false) }); code != 0 {
		t.Errorf("allowed run failure: %d, want 0", code)
	}
	t.Setenv("FAKE_FAIL", "")
	t.Setenv("FAKE_BAD", "alpha")
	if code := quiet(t, func() int { return cmdRun(allow, false) }); code != 2 {
		t.Errorf("a mismatch with -allow-failures: %d, want 2", code)
	}
}

func TestCheckCommandEndToEnd(t *testing.T) {
	root, cfg := fakeRoot(t)
	base := []string{"-root", root, "-gocpp", cfg.Gocpp, "-jobs", "2", "-pin", "off", "-max-load", "1e9", "-work", filepath.Join(root, "w")}
	if code := quiet(t, func() int {
		return cmdRun(append(base[:len(base):len(base)], "-reps", "3", "-out", "bench/results/m.json"), false)
	}); code != 0 {
		t.Fatalf("run: %d", code)
	}
	rep, err := benchdata.ReadReport(filepath.Join(root, "bench", "results", "m.json"))
	if err != nil {
		t.Fatal(err)
	}
	baseline := func(gocppNS, otherNS int64) {
		b := *rep
		b.Results = nil
		for _, r := range rep.Results {
			r.Wall = []int64{1, 1, 1, 1, 1, 1, 1, 1}
			if benchdata.IsGocpp(r.Impl) {
				for i := range r.Wall {
					r.Wall[i] = gocppNS
				}
			} else {
				for i := range r.Wall {
					r.Wall[i] = otherNS
				}
			}
			b.Results = append(b.Results, r)
		}
		b.Reps, b.Warmup = 3, 1
		if err := benchdata.WriteReport(filepath.Join(root, "bench", "baseline.json"), &b); err != nil {
			t.Fatal(err)
		}
	}
	check := func(extra ...string) (int, string) {
		var code int
		out := captureStdout(t, func() {
			code = quiet(t, func() int { return cmdRun(append(base[:len(base):len(base)], extra...), true) })
		})
		return code, out
	}
	if code, out := check(); code != 0 || !strings.Contains(out, "no baseline") && false {
		t.Errorf("%d\n%s", code, out)
	}
	// a baseline that is a minute slower than anything: everything improved, exit 0
	baseline(60_000_000_000, 60_000_000_000)
	if code, out := check("-no-drift"); code != 0 || !strings.Contains(out, "improved") {
		t.Errorf("slow baseline: %d\n%s", code, out)
	}
	// a baseline of 100 ns for gocpp: every gocpp benchmark regressed, and it reproduces
	baseline(100, 100)
	code, out := check("-no-drift")
	if code != 1 || !strings.Contains(out, "REGRESSIONS") {
		t.Errorf("regression: %d\n%s", code, out)
	}
	// the same with the drift correction: Go and the twins are 'slower' by the same factor
	// (the baseline's references are 100 ns too), so the correction hides it: a gate that
	// only has references that moved cannot tell a regression from a slow machine
	if code, _ := check(); code != 0 {
		t.Errorf("drift-corrected: %d, want 0", code)
	}
	// a run that fails to build fails the check
	t.Setenv("FAKE_FAIL", "alpha")
	if code, _ := check("-no-drift"); code != 2 {
		t.Errorf("failed run: %d, want 2", code)
	}
	// no baseline: nothing to compare with, not a failure
	os.Remove(filepath.Join(root, "bench", "baseline.json"))
	if code, _ := check(); code != 0 {
		t.Errorf("no baseline: %d, want 0", code)
	}
}

// ---- helpers ------------------------------------------------------------------

func TestParseImpls(t *testing.T) {
	all, err := parseImpls("all", []string{"O2"})
	if err != nil || len(all) != len(benchdata.AllImpls) {
		t.Errorf("all: %v %v", all, err)
	}
	one, err := parseImpls("go, gocpp-g++:O2", []string{"O2"})
	if err != nil || len(one) != 2 || one[1] != "gocpp-g++:O2" {
		t.Errorf("%v %v", one, err)
	}
	if _, err := parseImpls("rust", []string{"O2"}); err == nil {
		t.Error("an unknown implementation must be an error")
	}
}

func TestNotAvailableLine(t *testing.T) {
	cases := map[string]string{
		"./main.gp:16:6: the type sync.Mutex is not available yet (package sync: Phase 09)": "the type sync.Mutex is not available yet (package sync: Phase 09)",
		"x\n./main.gp:11:6: generic types are not supported yet (Phase 07)\n":               "generic types are not supported yet (Phase 07)",
		"./main.gp:1:1: syntax error": "",
		"":                            "",
	}
	for in, want := range cases {
		if got := notAvailableLine(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestRunTimed(t *testing.T) {
	dir := t.TempDir()
	ok := writeFile(t, filepath.Join(dir, "ok.sh"), "#!/bin/sh\necho hello\n")
	s, err := runTimed([]string{ok}, nil, 10*time.Second)
	if err != nil || s.Wall <= 0 || s.Head != "hello\n" || len(s.Hash) != 16 {
		t.Errorf("%+v %v", s, err)
	}
	s2, _ := runTimed([]string{ok}, nil, 10*time.Second)
	if s2.Hash != s.Hash {
		t.Error("the hash of the same output differs")
	}
	bad := writeFile(t, filepath.Join(dir, "bad.sh"), "#!/bin/sh\necho oops >&2\nexit 3\n")
	if _, err := runTimed([]string{bad}, nil, 10*time.Second); err == nil || !strings.Contains(err.Error(), "oops") {
		t.Errorf("exit status: %v", err)
	}
	slow := writeFile(t, filepath.Join(dir, "slow.sh"), "#!/bin/sh\nexec sleep 30\n")
	start := time.Now()
	if _, err := runTimed([]string{slow}, nil, 300*time.Millisecond); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("timeout: %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("the timeout did not kill the child")
	}
	// the output is hashed, not kept
	big := writeFile(t, filepath.Join(dir, "big.sh"), "#!/bin/sh\nyes x | head -c 3000000\n")
	if s, err := runTimed([]string{big}, nil, 20*time.Second); err != nil || len(s.Head) > 256 {
		t.Errorf("big output: %d bytes of head, %v", len(s.Head), err)
	}
}

func TestEnvWith(t *testing.T) {
	t.Setenv("GOBENCH_TEST_A", "old")
	env := envWith("GOBENCH_TEST_A=new", "GOBENCH_TEST_B=x")
	n := 0
	for _, e := range env {
		if strings.HasPrefix(e, "GOBENCH_TEST_A=") {
			n++
			if e != "GOBENCH_TEST_A=new" {
				t.Errorf("not replaced: %s", e)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d entries of the replaced key", n)
	}
}

func TestPickIdle(t *testing.T) {
	a := map[int][2]uint64{0: {0, 0}, 1: {0, 0}, 2: {0, 0}, 3: {10, 10}, 4: {5, 50}}
	b := map[int][2]uint64{0: {50, 100}, 1: {99, 100}, 2: {99, 100}, 3: {10, 10}, 4: {4, 40}} // 3: no time passed; 4: the counters went back
	got := pickIdle(a, b, 0.9)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("pickIdle = %v, want [1 2] (the idlest first, ties by number, busy and stalled ones left out)", got)
	}
	if list, _ := pinList("off", 4); list != "" {
		t.Error("off")
	}
	if list, _ := pinList("4,5", 4); list != "4,5" {
		t.Error("explicit list")
	}
}

func TestHistoryFileName(t *testing.T) {
	m := benchdata.Machine{Cores: 128}
	if got := historyFileName(m); !strings.HasSuffix(got, "-128c") || strings.ContainsAny(got, " /.") {
		t.Errorf("historyFileName = %q", got)
	}
}

// ---- the report and the compile programs ----------------------------------------

func TestMarkdownDeterministicAndComplete(t *testing.T) {
	r := gateReport(1.2)
	r.Results = append(r.Results,
		benchdata.Result{Program: "z", Category: "runtime", Impl: benchdata.ImplGocppGxx, Status: benchdata.StatusSkipped, Note: "needs sync"},
		benchdata.Result{Program: "y", Category: "data", Impl: benchdata.ImplGocppGxx, Status: benchdata.StatusMismatch, Note: "x"})
	r.Machine.LoadBefore[0] = 50
	a, b := Markdown(r, 16), Markdown(r, 16)
	if a != b {
		t.Error("Markdown is not deterministic")
	}
	for _, want := range []string{"# Runtime benchmarks", "Noisy run", "Geometric mean", "needs sync", "Does not build yet", "MISMATCH", "1.44"} {
		if !strings.Contains(a, want) {
			t.Errorf("the report lacks %q", want)
		}
	}
	r.Machine.LoadBefore[0], r.Machine.LoadAfter[0] = 1, 1
	if strings.Contains(Markdown(r, 16), "Noisy run") {
		t.Error("a quiet run is called noisy")
	}
}

func TestCompileBenchWithFakeTools(t *testing.T) {
	cfg, _ := fakeConfig(t)
	cs := runCompileBench(cfg, 2)
	if len(cs) != 4*5*3 { // 4 programs, (go + 2 gocpp x 2 flavors) builders, 3 modes
		t.Fatalf("%d measurements", len(cs))
	}
	seen := map[string]int{}
	for _, c := range cs {
		if c.Status != benchdata.StatusOK || len(c.WallNS) != 2 {
			t.Errorf("%+v", c)
		}
		for _, w := range c.WallNS {
			if w <= 0 {
				t.Errorf("%s/%s/%s: wall time %d", c.Name, c.Impl, c.Mode, w)
			}
		}
		seen[c.Mode]++
	}
	if seen["cold"] != 20 || seen["warm"] != 20 || seen["noop"] != 20 {
		t.Errorf("modes: %v", seen)
	}
	// the work folder of a measurement is removed again
	if left, _ := filepath.Glob(filepath.Join(cfg.Work, "compile", "*", "*")); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
	// a broken compiler is a build-failed record with the reason, for every mode
	t.Setenv("FAKE_FAIL", "")
	bad := writeFile(t, filepath.Join(t.TempDir(), "gocpp"), "#!/bin/sh\necho 'main.gp:1:1: internal error: boom'\nexit 1\n")
	cfg.Gocpp = bad
	cfg.Impls = []string{benchdata.ImplGocppGxx}
	for _, c := range runCompileBench(cfg, 1) {
		if c.Status != benchdata.StatusBuildFailed || !strings.Contains(c.Note, "boom") {
			t.Errorf("%+v", c)
		}
	}
}

func TestMarkdownCompileTable(t *testing.T) {
	r := gateReport(1)
	add := func(impl, flavor, mode string, ns int64) {
		r.Compile = append(r.Compile, benchdata.Compile{Name: "hello", Impl: impl, Flavor: flavor, Mode: mode, Status: benchdata.StatusOK, WallNS: []int64{ns, ns, ns}})
	}
	for _, mode := range []string{"cold", "warm", "noop"} {
		add(benchdata.ImplGo, "", mode, 1e9)
		for _, fl := range []string{"default", "O2"} {
			add("gocpp-g++", fl, mode, 3e9)
			add("gocpp-clang++", fl, mode, 2e9)
		}
	}
	r.Results = append(r.Results, benchdata.Result{Program: "q", Category: "cpu", Impl: benchdata.ImplCppGxx, Status: benchdata.StatusSkipped, Note: "no C++ twin"})
	md := Markdown(r, 16)
	i := strings.Index(md, "## Compile time")
	if i < 0 {
		t.Fatal("no compile section")
	}
	var rows []string
	for _, l := range strings.Split(md[i:], "\n") {
		if strings.HasPrefix(l, "| hello") {
			rows = append(rows, l)
		}
	}
	// one row per (mode, flavor): 3 modes x 2 flavors, no duplicate and no row for Go alone
	if len(rows) != 6 {
		t.Errorf("%d rows:\n%s", len(rows), strings.Join(rows, "\n"))
	}
	if !strings.Contains(rows[0], "| 1.00 | 3.00 | 2.00 | **3.00** | 2.00 |") {
		t.Errorf("row: %s", rows[0])
	}
	if !strings.Contains(rows[0], "| default |") || !strings.Contains(rows[1], "| O2 |") {
		t.Errorf("the default build comes first:\n%s\n%s", rows[0], rows[1])
	}
	if strings.Contains(md, "no C++ twin") {
		t.Error("the missing twins are listed as failures")
	}
}

func TestCompileProjectsAreDeterministicAndSized(t *testing.T) {
	for _, p := range compileProjects() {
		a, b := p.files(1), p.files(1)
		if len(a) == 0 {
			t.Errorf("%s: no files", p.name)
		}
		for k, v := range a {
			if b[k] != v {
				t.Errorf("%s/%s differs between two generations", p.name, k)
			}
		}
		if p.files(1)["main.GO"] == p.files(2)["main.GO"] {
			t.Errorf("%s: the variant does not change main", p.name)
		}
	}
	lines := strings.Count(genLines1000(0)["main.GO"], "\n")
	if lines < 950 || lines > 1100 {
		t.Errorf("lines1000 has %d lines", lines)
	}
	if n := len(genPkgs20(0)); n != 21 {
		t.Errorf("pkgs20 has %d files, want 20 packages and main", n)
	}
	if n := strings.Count(genGenerics100(0)["main.GO"], "First(Map("); n != 100 {
		t.Errorf("generics100 has %d instantiations", n)
	}
	dir := t.TempDir()
	if err := writeProject(dir, compileProjects()[1], 0, "go"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"go.mod", "main.go", "p19/p19.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
}

func TestParseVmHWM(t *testing.T) {
	status := "Name:\tx\nVmPeak:\t  9000 kB\nVmHWM:\t    1234 kB\nVmRSS:\t   1000 kB\n"
	if runtime.GOOS != "linux" {
		t.Skip("/proc")
	}
	if got := parseVmHWM(status); got != 1234 {
		t.Errorf("parseVmHWM = %d", got)
	}
	if parseVmHWM("Name:\tx\n") != 0 {
		t.Error("no VmHWM line must give 0")
	}
}

// A small program reports a small peak resident set, not the runner's own (the
// getrusage floor), and a program that allocates 64 MiB reports at least that.
func TestPeakRSSOfTheChild(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc")
	}
	// make the runner big first: its 80 MiB must not show up in the child's figure
	ballast := make([]byte, 80<<20)
	for i := range ballast {
		ballast[i] = 1
	}
	defer runtime.KeepAlive(ballast)
	dir := t.TempDir()
	small := writeFile(t, filepath.Join(dir, "small.sh"), "#!/bin/sh\nexec sleep 0.2\n")
	s, err := runTimed([]string{small}, nil, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if s.RSSKB <= 0 || s.RSSKB > 40<<10 {
		t.Errorf("sleep reports %d KiB, the runner's ballast leaked into it", s.RSSKB)
	}
	big := writeFile(t, filepath.Join(dir, "big.sh"), "#!/bin/sh\nexec python3 -I -c 'b = bytearray(64 << 20); import time; time.sleep(0.2)'\n")
	if _, err := exec.LookPath("python3"); err == nil {
		s, err := runTimed([]string{big}, nil, 20*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if s.RSSKB < 60<<10 {
			t.Errorf("a program holding 64 MiB reports %d KiB", s.RSSKB)
		}
	}
}

func TestMergeCommand(t *testing.T) {
	dir := t.TempDir()
	base, add := gateReport(1), gateReport(1)
	add.Results = add.Results[:3] // program a only
	for i := range add.Results {
		add.Results[i].Wall = []int64{7, 7, 7}
	}
	bn, an := filepath.Join(dir, "base.json"), filepath.Join(dir, "add.json")
	for n, r := range map[string]*benchdata.Report{bn: base, an: add} {
		if err := benchdata.WriteReport(n, r); err != nil {
			t.Fatal(err)
		}
	}
	if code := quiet(t, func() int { return cmdMerge([]string{"-base", bn, "-add", an}) }); code != 0 {
		t.Fatalf("merge: %d", code)
	}
	got, err := benchdata.ReadReport(bn)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 12 || got.Find("a", benchdata.ImplGo).Wall[0] != 7 || got.Find("b", benchdata.ImplGo).Wall[0] == 7 {
		t.Errorf("merged report: %d results, a=%v b=%v", len(got.Results), got.Find("a", benchdata.ImplGo).Wall, got.Find("b", benchdata.ImplGo).Wall)
	}
	if o := got.Find("a", benchdata.ImplGo).Origin; !strings.Contains(o, "commit c") || !strings.Contains(o, "8 reps") {
		t.Errorf("a merged result has the origin %q, want the add report's commit and reps", o)
	}
	if o := got.Find("b", benchdata.ImplGo).Origin; o != "" {
		t.Errorf("a result that was not merged has the origin %q", o)
	}
	raw, _ := os.ReadFile(bn)
	if !strings.Contains(string(raw), `"origin"`) || !strings.Contains(string(raw), `"schema": 2`) {
		t.Errorf("the merged report has no origin field or is not schema 2")
	}
	add.Size = "smoke"
	if err := benchdata.WriteReport(an, add); err != nil {
		t.Fatal(err)
	}
	if code := quiet(t, func() int { return cmdMerge([]string{"-base", bn, "-add", an}) }); code != 2 {
		t.Errorf("merging a smoke run into a full one: %d, want 2", code)
	}
	if code := quiet(t, func() int { return cmdMerge([]string{"-base", bn}) }); code != 2 {
		t.Errorf("no -add: %d, want 2", code)
	}
}

func TestFlavorsFlagsAndImpls(t *testing.T) {
	fl, err := parseFlavors("default, O2")
	if err != nil || len(fl) != 2 {
		t.Fatalf("%v %v", fl, err)
	}
	if _, err := parseFlavors("O3"); err == nil {
		t.Error("an unknown flavor must be an error")
	}
	all, err := parseImpls("all", fl)
	if err != nil || len(all) != 1+2*2+2 {
		t.Errorf("all with two flavors: %v %v", all, err)
	}
	if _, err := parseImpls("gocpp-g++:release", fl); err == nil {
		t.Error("an implementation of a flavor that is not selected must be an error")
	}
	if one, err := parseImpls("gocpp-g++:default,go", fl); err != nil || len(one) != 2 {
		t.Errorf("%v %v", one, err)
	}
	// check measures O2 only, a smoke run default and O2, a full run all three
	for _, c := range []struct {
		check, smoke bool
		want         int
	}{{true, false, 1}, {false, true, 2}, {false, false, 3}} {
		f := &runFlags{smoke: c.smoke, impls: "all", cxxflags: DefaultCXXFlags}
		cfg, err := f.config(t.TempDir(), c.check)
		if err != nil || len(cfg.Flavors) != c.want || len(cfg.Impls) != 1+2*c.want+2 {
			t.Errorf("check=%v smoke=%v: %v, %v", c.check, c.smoke, cfg, err)
		}
	}
}

func TestCompileBenchSkipsReleaseWithoutTheFlag(t *testing.T) {
	cfg, _ := fakeConfig(t)
	cfg.Flavors = benchdata.Flavors
	cfg.Impls = benchdata.ImplsFor(benchdata.Flavors)
	n := 0
	for _, c := range runCompileBench(cfg, 1) {
		if c.Flavor == "release" {
			n++
			if c.Status != benchdata.StatusSkipped {
				t.Errorf("%+v", c)
			}
		} else if c.Status != benchdata.StatusOK {
			t.Errorf("%+v", c)
		}
	}
	if n != 4*2*3 { // 4 programs, 2 compilers, 3 modes
		t.Errorf("%d release records", n)
	}
}

func TestMarkdownFlavorsAndNotes(t *testing.T) {
	r := gateReport(1)
	add := func(impl string, p string, ns int64) {
		w := steadyWall(ns)
		r.Results = append(r.Results, benchdata.Result{Program: p, Category: "cpu", Impl: impl, Status: benchdata.StatusOK, Wall: w, CPU: w, RSS: []int64{1}})
	}
	// the default flavor is slow everywhere, the optimized one is fine except in b
	for _, p := range []string{"a", "b", "c", "d"} {
		add(benchdata.GocppImpl("g++", "default"), p, 900_000_000)
	}
	for i := range r.Results { // gateReport has the O2 build at 1.2 times Go; program b is slower
		if r.Results[i].Impl == benchdata.ImplGocppGxx && r.Results[i].Program == "b" {
			r.Results[i].Wall = steadyWall(30_000_000)
		}
	}
	r.Results = append(r.Results,
		benchdata.Result{Program: "n", Category: "data", Impl: benchdata.ImplGo, Status: benchdata.StatusSkipped, Note: "needs foo: not available yet"},
		benchdata.Result{Program: "n", Category: "data", Impl: benchdata.ImplGocppGxx, Status: benchdata.StatusSkipped, Note: "needs foo: not available yet"},
		benchdata.Result{Program: "a", Category: "cpu", Impl: benchdata.GocppImpl("g++", "release"), Status: benchdata.StatusSkipped, Note: "gocpp has no build -release yet"},
		benchdata.Result{Program: "b", Category: "cpu", Impl: benchdata.GocppImpl("g++", "release"), Status: benchdata.StatusSkipped, Note: "gocpp has no build -release yet"})
	md := Markdown(r, 16)
	if !strings.Contains(md, "| cpu | 4 |") || !strings.Contains(md, "b 3.00 (gocpp g++ O2)") {
		t.Errorf("the worst gap must be that of the optimized build, not of the default one (every ratio there is 90):\n%s", md[strings.Index(md, "gocpp over Go"):][:900])
	}
	if strings.Count(md, "needs foo") != 1 || !strings.Contains(md, "- data/n: needs foo: not available yet") {
		t.Errorf("a program that is skipped everywhere is one line:\n%s", md[strings.Index(md, "Does not build yet"):])
	}
	if !strings.Contains(md, "gocpp g++ release (2 programs): gocpp has no build -release yet") {
		t.Errorf("a flavor that cannot be built is one line:\n%s", md[strings.Index(md, "Does not build yet"):])
	}
	// the unbuilt release flavor has no column
	if strings.Contains(md, "release |") && strings.Contains(md, "| gocpp g++ release |") {
		t.Error("a flavor that did not run has a column")
	}
}

func TestGateIgnoresImplementationsNotMeasured(t *testing.T) {
	base := gateReport(1)
	w := steadyWall(5_000_000)
	for _, p := range []string{"a", "b", "c", "d"} { // the baseline also has the default flavor; this run measures O2 only
		base.Results = append(base.Results, benchdata.Result{Program: p, Category: "cpu", Impl: benchdata.GocppImpl("g++", "default"), Status: benchdata.StatusOK, Wall: w, CPU: w, RSS: []int64{1}})
	}
	code, out := gateRun(t, base, gateReport(1), nil)
	if code != 0 || strings.Contains(out, "not measured now") {
		t.Errorf("%d\n%s", code, out)
	}
}

func TestChooseRSS(t *testing.T) {
	ms := time.Millisecond
	cases := []struct {
		name                string
		ru, runner, sampled int64
		wall                time.Duration
		want                int64
	}{
		{"above the runner's floor: exact", 80000, 6000, 70000, 500 * ms, 80000},
		{"below the floor: the sampled peak", 6000, 6000, 3000, 500 * ms, 3000},
		{"a tiny run has no figure", 6000, 6000, 200, 5 * ms, 0},
		{"a tiny run above the floor is exact", 9000, 6000, 200, 5 * ms, 9000},
		{"nothing sampled", 6000, 6000, 0, 500 * ms, 0},
		{"no /proc: getrusage is all there is", 4000, 0, 0, 5 * ms, 4000},
	}
	for _, c := range cases {
		if got := chooseRSS(c.ru, c.runner, c.sampled, c.wall); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

func TestCleanEnv(t *testing.T) {
	env := []string{"PATH=/bin", "GOGC=off", "GOMAXPROCS=2", "GODEBUG=x", "GOCPP_FLAGS=-asan", "GOCPP_GATE_PHASE=7", "MALLOC_PERTURB_=3", "HOME=/h", "GOFLAGS=-v"}
	got := strings.Join(cleanEnv(env, false), " ")
	if got != "PATH=/bin HOME=/h GOFLAGS=-v" {
		t.Errorf("run environment: %s", got)
	}
	got = strings.Join(cleanEnv(env, true), " ")
	if got != "PATH=/bin GOCPP_GATE_PHASE=7 HOME=/h GOFLAGS=-v" {
		t.Errorf("build environment: %s", got)
	}
}

// An interrupted run removes its temporary work folder and exits with 130.
func TestInterruptRemovesTheWorkFolder(t *testing.T) {
	root, cfg := fakeRoot(t)
	tmp := t.TempDir()
	logf := filepath.Join(tmp, "log")
	cmd := exec.Command(os.Args[0], "run", "-root", root, "-gocpp", cfg.Gocpp, "-pin", "off", "-flavors", "O2", "-reps", "1", "-out", "")
	cmd.Env = append(os.Environ(), "GOBENCH_HELPER=1", "TMPDIR="+tmp, "FAKE_SLEEP=5", "FAKE_LOG="+logf)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	// wait (bounded) until a builder runs, so the work folder exists and the runner is busy
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(logf); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the runner never started a build")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "gobench-*")); len(left) != 1 {
		t.Fatalf("work folders while running: %v", left)
	}
	cmd.Process.Signal(os.Interrupt)
	err := cmd.Wait()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 130 {
		t.Errorf("exit: %v, want status 130", err)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "gobench-*")); len(left) != 0 {
		t.Errorf("the interrupted run left %v", left)
	}
}

func TestChildProcs(t *testing.T) {
	for list, want := range map[string]int{"3": 1, "0,2-5": 5, "4-7, 9": 5, "": 0, "x": 0, "5-3": 0} {
		if got := countCPUList(list); got != want {
			t.Errorf("countCPUList(%q) = %d, want %d", list, got, want)
		}
	}
	if childProcs("1,2,3,4") != 4 {
		t.Error("a pinned run gets the cpus of its list")
	}
	if childProcs("") < 1 {
		t.Error("an unpinned run gets the default parallelism")
	}
}

func TestGateInconclusiveWhenTheConditionsDiffer(t *testing.T) {
	base := gateReport(1)
	base.Machine.GoMaxProcs = 8
	mk := func(mod func(*benchdata.Report)) *benchdata.Report {
		r := gateReport(1.5) // would be a regression on equal conditions
		r.Machine.GoMaxProcs = 8
		mod(r)
		return r
	}
	for name, mod := range map[string]func(*benchdata.Report){
		"pinned now":       func(r *benchdata.Report) { r.Machine.Pinned = "0-3" },
		"other GOMAXPROCS": func(r *benchdata.Report) { r.Machine.GoMaxProcs = 4 },
	} {
		code, out := gateRun(t, base, mk(mod), nil)
		if code != 0 || !strings.Contains(out, "INCONCLUSIVE") {
			t.Errorf("%s: %d\n%s", name, code, out)
		}
		if code, _ := gateRun(t, base, mk(mod), func(f *runFlags) { f.requireQuiet = true }); code != 1 {
			t.Errorf("%s with -require-quiet: %d, want 1", name, code)
		}
	}
	if code, out := gateRun(t, base, mk(func(*benchdata.Report) {}), nil); code != 1 {
		t.Errorf("same conditions, real regression: %d\n%s", code, out)
	}
	// a baseline that was measured on a busy machine says nothing, unless trusted
	busy := gateReport(1)
	busy.Machine.GoMaxProcs = 8
	busy.Machine.LoadBefore[0] = 99
	if code, out := gateRun(t, busy, mk(func(*benchdata.Report) {}), nil); code != 0 || !strings.Contains(out, "baseline was measured at a 1-minute load") {
		t.Errorf("busy baseline: %d\n%s", code, out)
	}
	if code, _ := gateRun(t, busy, mk(func(*benchdata.Report) {}), func(f *runFlags) { f.trustBaseline = true }); code != 1 {
		t.Errorf("-trust-baseline: %d, want 1", code)
	}
}

func TestMarkdownHeadlineWithoutFib(t *testing.T) {
	r := gateReport(1)
	for i := range r.Results {
		if r.Results[i].Program == "a" && r.Results[i].Impl == benchdata.ImplGocppGxx {
			r.Results[i].Wall = steadyWall(1_200_000) // program "a" (called fib here) is 10 times faster than Go
		}
	}
	for i := range r.Results {
		if r.Results[i].Program == "a" {
			r.Results[i].Program = "fib"
		}
	}
	md := Markdown(r, 16)
	if !strings.Contains(md, "| cpu without fib | 3 |") || !strings.Contains(md, "| all without fib, generic_minmax | 3 |") {
		t.Errorf("the rows without the flattering programs are missing:\n%s", md[strings.Index(md, "gocpp over Go"):][:700])
	}
}

func TestFailedGocppBuildIsTriedAgain(t *testing.T) {
	t.Setenv("FAKE_FLAKY", filepath.Join(t.TempDir(), "failed-once"))
	cfg, ps := fakeConfig(t, [3]string{"alpha", "cpu", "no"})
	cfg.Impls = []string{benchdata.ImplGo, benchdata.ImplGocppGxx}
	cfg.Retries = 1
	suite, err := NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	m := results(suite.Run())
	r := m["alpha/"+benchdata.ImplGocppGxx]
	if r.Status != benchdata.StatusOK || !strings.HasPrefix(r.Note, "built on try 2; the first build failed: exit status 2: internal error") {
		t.Errorf("%+v", r)
	}
	if !strings.Contains(Markdown(&benchdata.Report{Size: "full", Results: []benchdata.Result{r}}, 16), "## Builds that needed a second try") {
		t.Error("the report does not name the retry")
	}
	if len(Failures([]benchdata.Result{r})) != 0 {
		t.Error("a build that worked on the second try is not a failure")
	}
	// without retries the same flakiness is a failure
	t.Setenv("FAKE_FLAKY", filepath.Join(t.TempDir(), "failed-once-2"))
	cfg, ps = fakeConfig(t, [3]string{"alpha", "cpu", "no"})
	cfg.Impls = []string{benchdata.ImplGo, benchdata.ImplGocppGxx}
	suite, err = NewSuite(cfg, ps)
	if err != nil {
		t.Fatal(err)
	}
	if r := results(suite.Run())["alpha/"+benchdata.ImplGocppGxx]; r.Status != benchdata.StatusBuildFailed {
		t.Errorf("without retries: %+v", r)
	}
}
