package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseThreads(t *testing.T) {
	got, err := parseThreads("", false)
	if err != nil || len(got) != 7 || got[0] != 1 || got[6] != 64 {
		t.Errorf("default: %v %v", got, err)
	}
	got, err = parseThreads("", true)
	if err != nil || len(got) != 2 || got[0] != 1 || got[1] != 8 {
		t.Errorf("smoke default: %v %v", got, err)
	}
	got, err = parseThreads("8, 1,4", false)
	if err != nil || len(got) != 3 || got[0] != 1 || got[1] != 4 || got[2] != 8 {
		t.Errorf("sorted: %v %v", got, err)
	}
	for _, bad := range []string{"0", "-1", "x", "1,1", "1,,x"} {
		if _, err := parseThreads(bad, false); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestParseFlavors(t *testing.T) {
	got, err := parseFlavors("all")
	if err != nil || strings.Join(got, ",") != "go,arc,gc" {
		t.Errorf("all: %v %v", got, err)
	}
	got, err = parseFlavors("gc, go,gc")
	if err != nil || strings.Join(got, ",") != "gc,go" {
		t.Errorf("list: %v %v", got, err)
	}
	for _, bad := range []string{"", "java", "go,nope"} {
		if _, err := parseFlavors(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestParsePrograms(t *testing.T) {
	got, err := parsePrograms("all")
	if err != nil || len(got) != 3 || got[0].name != "binarytrees" || got[2].name != "syscalls" {
		t.Errorf("all: %v %v", got, err)
	}
	got, err = parsePrograms("parks")
	if err != nil || len(got) != 1 || got[0].name != "parks" {
		t.Errorf("one: %v %v", got, err)
	}
	for _, bad := range []string{"", "nope", "parks,nope"} {
		if _, err := parsePrograms(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestSummarize(t *testing.T) {
	st := summarize([]int64{30, 10, 20})
	if st.N != 3 || st.Median != 20 || st.MAD != 10 || st.Min != 10 || st.Max != 30 {
		t.Errorf("odd: %+v", st)
	}
	st = summarize([]int64{10, 20, 30, 40})
	if st.Median != 25 || st.MAD != 10 {
		t.Errorf("even: %+v", st)
	}
	if got := st.relNoise(); got != 0.4 {
		t.Errorf("relNoise %v", got)
	}
	if st := summarize(nil); st != (stat{}) {
		t.Errorf("empty: %+v", st)
	}
	if st := summarize([]int64{7}); st.Median != 7 || st.MAD != 0 {
		t.Errorf("one: %+v", st)
	}
}

func TestChooseRSS(t *testing.T) {
	cases := []struct {
		rusage, runner, sampled int64
		wall                    time.Duration
		want                    int64
	}{
		{100, 50, 80, 30 * time.Millisecond, 100}, // rusage above the runner's floor: exact
		{40, 50, 80, 30 * time.Millisecond, 80},   // below it: the sampled VmHWM
		{40, 50, 80, 10 * time.Millisecond, 0},    // too short to sample
		{40, 50, 0, 30 * time.Millisecond, 0},     // no /proc
	}
	for _, c := range cases {
		if got := chooseRSS(c.rusage, c.runner, c.sampled, c.wall); got != c.want {
			t.Errorf("chooseRSS(%d, %d, %d, %v) = %d, want %d", c.rusage, c.runner, c.sampled, c.wall, got, c.want)
		}
	}
}

func TestNotAvailableNote(t *testing.T) {
	if got := notAvailableNote("flag provided but not defined: -gc-engine\nusage: gocpp build\n"); !strings.Contains(got, "not available yet (16.1d/16.1f)") {
		t.Errorf("flag: %q", got)
	}
	if got := notAvailableNote("gocpp build: the GC engine is not available yet (Phase 16)\n"); !strings.Contains(got, "not available yet (16.1d/16.1f)") {
		t.Errorf("GC engine message: %q", got)
	}
	// A gated feature of the program is a real build failure, not the GC stub.
	if got := notAvailableNote("main.gp:3:2: x is not available yet (Phase 09)\n"); got != "" {
		t.Errorf("gated feature: %q", got)
	}
	if got := notAvailableNote("main.gp:3:2: undefined: x\n"); got != "" {
		t.Errorf("genuine error: %q", got)
	}
}

func TestEnv(t *testing.T) {
	c := &config{work: t.TempDir()}
	t.Setenv("GOGC", "off")
	t.Setenv("GOMAXPROCS", "99")
	t.Setenv("GOCPP_GATE_PHASE", "12")
	env := envMap(c.gocppEnv())
	if env["GOMAXPROCS"] != "16" {
		t.Errorf("gocpp GOMAXPROCS %q", env["GOMAXPROCS"])
	}
	if _, ok := env["GOGC"]; ok {
		t.Errorf("gocpp env keeps GOGC=%q", env["GOGC"])
	}
	if env["GOCPP_GATE_PHASE"] != "12" {
		t.Errorf("a shell-set gate phase is not kept: %q", env["GOCPP_GATE_PHASE"])
	}
	os.Unsetenv("GOCPP_GATE_PHASE")
	if got := envMap(c.gocppEnv())["GOCPP_GATE_PHASE"]; got != "15" {
		t.Errorf("default gate phase %q", got)
	}
	genv := envMap(c.goEnv())
	if genv["GOMAXPROCS"] != "16" || genv["GOPROXY"] != "off" || genv["GOTOOLCHAIN"] != "local" {
		t.Errorf("go env: %v", genv)
	}
	if _, ok := genv["GOGC"]; ok {
		t.Errorf("go env keeps GOGC=%q", genv["GOGC"])
	}
}

// envMap turns KEY=VALUE entries into a map.
func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		m[k] = v
	}
	return m
}

func TestRunTimed(t *testing.T) {
	s, out, err := runTimed([]string{"/bin/sh", "-c", "echo hello; exit 0"}, nil, 10*time.Second)
	if err != nil || !strings.Contains(out, "hello") || s.Exit != 0 || s.WallNS <= 0 {
		t.Fatalf("run: %+v %q %v", s, out, err)
	}
	s, _, err = runTimed([]string{"/bin/sh", "-c", "exit 3"}, nil, 10*time.Second)
	if err == nil || s.Exit != 3 {
		t.Fatalf("failure: %+v %v", s, err)
	}
	if _, _, err := runTimed([]string{"/bin/sh", "-c", "sleep 5"}, nil, 100*time.Millisecond); err == nil {
		t.Fatal("timeout: no error")
	}
}

// fakeReport is a report with numbers that are easy to check by hand: Go and
// ARC agree on the checksum, the GC flavor is not available.
func fakeReport() *report {
	samples := []sample{
		{WallNS: 1_000_000_000, CPUNS: 900_000_000, RSSKB: 10_240, Exit: 0},
		{WallNS: 1_100_000_000, CPUNS: 950_000_000, RSSKB: 11_264, Exit: 0},
	}
	runs := func() []runResult {
		rs := []runResult{
			{Threads: 1, Status: runOK, Output: "checksum 42", Samples: samples},
			{Threads: 4, Status: runOK, Output: "checksum 42", Samples: samples},
		}
		for i := range rs {
			rs[i].Wall, rs[i].CPU, rs[i].RSS = statOf(rs[i].Samples)
		}
		return rs
	}
	return &report{
		Schema: schemaVersion, Tool: "gcbench", Generated: "2026-10-24T00:00:00Z", Reps: 2, Warmup: 1, Depth: 10,
		Threads: []int{1, 4}, Command: "gcbench run -bench binarytrees -flavor go,arc,gc",
		Commit: "abc1234", GatePhase: "15", TimeoutMS: 300000, Machine: machine{
			CPU: "test cpu", Cores: 8, MemGiB: 16, OS: "linux/amd64 test", Go: "go version test",
			Gocpp: "gocpp test", LoadBefore: [3]float64{1, 2, 3}, LoadAfter: [3]float64{4, 5, 6}, MaxLoad: 24,
		},
		Programs: []programResult{{
			Name: "binarytrees", About: "trees", Path: "bench/gc/binarytrees",
			Flavors: []flavorResult{
				{Flavor: "go", Status: statusOK, BuildCommand: "go build -o prog .", Runs: runs()},
				{Flavor: "arc", Status: statusOK, BuildCommand: "gocpp build -o prog .", Runs: runs()},
				{Flavor: "gc", Status: statusNotAvailable, Note: "not available yet (16.1d/16.1f)"},
			},
		}},
	}
}

func TestReportJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	r := fakeReport()
	if err := writeJSON(path, r); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) || !bytes.Contains(data, []byte("\n  \"schema\": 1,")) {
		t.Errorf("not indented JSON with a final newline:\n%s", data)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != 1 || got.Commit != "abc1234" || len(got.Programs) != 1 || len(got.Programs[0].Flavors) != 3 {
		t.Fatalf("round trip: %+v", got)
	}
	if got.GatePhase != "15" || got.TimeoutMS != 300000 {
		t.Errorf("gate phase/timeout: %q %d", got.GatePhase, got.TimeoutMS)
	}
	if got.Programs[0].Path != "bench/gc/binarytrees" {
		t.Errorf("path is not repo-relative: %q", got.Programs[0].Path)
	}
	rr := got.Programs[0].Flavors[0].Runs[0]
	if rr.Threads != 1 || rr.Output != "checksum 42" || rr.Wall.Median != 1_050_000_000 || len(rr.Samples) != 2 {
		t.Errorf("run: %+v", rr)
	}
	if got.Programs[0].Flavors[2].Status != statusNotAvailable {
		t.Errorf("gc status: %+v", got.Programs[0].Flavors[2])
	}
}

func TestMarkdown(t *testing.T) {
	r := fakeReport()
	md := markdown(r)
	for _, want := range []string{
		"# GC engine spike", "test cpu", "gcbench run -bench binarytrees",
		"checksum 42", "the flavors agree", "not yet (16.1d/16.1f)",
		"| 1 | 1050.0 ±4.8%", "#### Wall time", "#### CPU time", "#### Peak resident set",
		"gocpp gate phase", "timeout per run",
		"Wall time is not the comparison for `binarytrees`",
		"the split program's", "What job 16.1h adds",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown does not contain %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "**MISMATCH**") {
		t.Errorf("a matching report says MISMATCH:\n%s", md)
	}
}

func TestCompareOutputs(t *testing.T) {
	r := fakeReport()
	pr := &r.Programs[0]
	compareOutputs(pr, sameAcrossThreads(pr.Name), func(string, ...any) {})
	if got := pr.Flavors[1].Runs[0].Status; got != runOK {
		t.Fatalf("matching outputs marked %q", got)
	}
	pr.Flavors[1].Runs[0].Output = "checksum 43"
	compareOutputs(pr, sameAcrossThreads(pr.Name), func(string, ...any) {})
	if pr.Flavors[0].Runs[0].Status != runMismatch || pr.Flavors[1].Runs[0].Status != runMismatch {
		t.Errorf("mismatch not marked: %q %q", pr.Flavors[0].Runs[0].Status, pr.Flavors[1].Runs[0].Status)
	}
	if r.exitCode() != 1 {
		t.Error("a mismatch must fail the run")
	}
	if md := markdown(r); !strings.Contains(md, "**MISMATCH**") {
		t.Errorf("markdown hides the mismatch:\n%s", md)
	}
}

func TestThreadIndependentOutput(t *testing.T) {
	// binarytrees must print the same checksum at every thread count: a
	// partition bug that drops or double-counts work would print the same
	// wrong sum in Go and ARC, so the per-flavor comparison alone misses it.
	r := fakeReport()
	pr := &r.Programs[0]
	pr.Flavors[0].Runs[1].Output = "checksum 43"
	pr.Flavors[1].Runs[1].Output = "checksum 43"
	compareOutputs(pr, sameAcrossThreads(pr.Name), func(string, ...any) {})
	for _, fr := range pr.Flavors[:2] {
		for _, rr := range fr.Runs {
			if rr.Status != runMismatch || !strings.Contains(rr.Note, "same at every thread count") {
				t.Errorf("thread-dependent checksum not marked: %+v", rr)
			}
		}
	}
	if line := outputLine(*pr); !strings.Contains(line, "MISMATCH") {
		t.Errorf("outputLine: %q", line)
	}
	if r.exitCode() != 1 {
		t.Error("a thread-dependent checksum must fail the run")
	}
}

func TestExitCode(t *testing.T) {
	r := fakeReport()
	if code := r.exitCode(); code != 0 {
		t.Errorf("a GC flavor that is not available is not a failure: %d", code)
	}
	r.Programs[0].Flavors[0].Status = statusBuildFailed
	if code := r.exitCode(); code != 1 {
		t.Errorf("a build failure must fail the run: %d", code)
	}
}

func TestOutputLineThreadDependent(t *testing.T) {
	// parks and syscalls print the number of operations, which follows the
	// thread count: equal per thread count is a match, not a MISMATCH.
	r := fakeReport()
	pr := &r.Programs[0]
	pr.Name = "parks"
	pr.Flavors[0].Runs[1].Output = "ops 400"
	pr.Flavors[1].Runs[1].Output = "ops 400"
	line := outputLine(*pr)
	if !strings.Contains(line, "agree at every thread count") || strings.Contains(line, "MISMATCH") {
		t.Errorf("thread-dependent output: %q", line)
	}
	pr.Flavors[1].Runs[1].Output = "ops 401"
	line = outputLine(*pr)
	if !strings.Contains(line, "MISMATCH") {
		t.Errorf("a real mismatch: %q", line)
	}
	// A missing run at one thread count is not agreement at every count.
	pr.Flavors[1].Runs[1].Output = "ops 400"
	pr.Flavors[1].Runs[0].Status = runFailed
	line = outputLine(*pr)
	if strings.Contains(line, "agree at every thread count") {
		t.Errorf("a missing run was called agreement: %q", line)
	}
}

func TestCmdReport(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.json")
	if err := writeJSON(in, fakeReport()); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.md")
	var stdout, stderr bytes.Buffer
	if code := cmdReport([]string{"-in", in, "-md", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("report: exit %d: %s", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != markdown(fakeReport()) {
		t.Error("report wrote different markdown")
	}
	// without -md the markdown goes to stdout
	stdout.Reset()
	if code := cmdReport([]string{"-in", in}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "# GC engine spike") {
		t.Errorf("stdout report: exit %d, %q", code, stdout.String())
	}
	if code := cmdReport(nil, &stdout, &stderr); code != 2 {
		t.Errorf("missing -in: exit %d", code)
	}
	if code := cmdReport([]string{"-in", filepath.Join(dir, "nope.json")}, &stdout, &stderr); code != 2 {
		t.Errorf("missing file: exit %d", code)
	}
}

func TestBusyNote(t *testing.T) {
	m := machine{LoadBefore: [3]float64{30, 1, 1}, LoadAfter: [3]float64{2, 1, 1}, MaxLoad: 24}
	m.Busy = m.LoadBefore[0] > m.MaxLoad
	if note := m.busyNote(); !strings.Contains(note, "loaded") {
		t.Errorf("busy: %q", note)
	}
	m = machine{LoadBefore: [3]float64{1, 1, 1}, LoadAfter: [3]float64{1, 1, 1}, MaxLoad: 24}
	if note := m.busyNote(); note != "" {
		t.Errorf("quiet: %q", note)
	}
}
