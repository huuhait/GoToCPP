package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const panicOut = `fuzz: elapsed: 3s, execs: 100 (33/sec), new interesting: 1 (total: 11)
--- FAIL: FuzzCheck (0.12s)
    --- FAIL: FuzzCheck (0.00s)
        testing.go:1712: panic: runtime error: index out of range [7] with length 3
            goroutine 1 [running]:
            github.com/huuhait/gocpp/internal/check.(*checker).expr(0xc000123456, ...)
            	/x/internal/check/expr.go:123 +0x1f
            github.com/huuhait/gocpp/internal/check.(*checker).stmt(0xc0001, ...)
            github.com/huuhait/gocpp/tools/fuzz.FuzzCheck.func1(...)
    Failing input written to testdata/fuzz/FuzzCheck/abc123
`

func TestSignature(t *testing.T) {
	k1, s1 := signature([]byte(panicOut))
	if !strings.Contains(k1, "index out of range") {
		t.Errorf("kind %q", k1)
	}
	// The same bug with other numbers and addresses has the same signature.
	other := strings.NewReplacer("[7]", "[12]", "length 3", "length 9", "0xc000123456", "0xc0009999", ":123", ":124").Replace(panicOut)
	if _, s2 := signature([]byte(other)); s1 != s2 {
		t.Errorf("signatures differ for one bug: %s %s", s1, s2)
	}
	// Another frame in gocpp's code is another bug.
	diff := strings.Replace(panicOut, "check.(*checker).expr", "check.(*checker).call", 1)
	if _, s3 := signature([]byte(diff)); s1 == s3 {
		t.Error("signatures are equal for different frames")
	}
	// A message of t.Fatalf.
	k, _ := signature([]byte("--- FAIL: FuzzEmit\n    fuzz_test.go:99: the checker accepts the program and the emitter refuses it: boom\n"))
	if !strings.Contains(k, "the checker accepts") {
		t.Errorf("kind %q", k)
	}
	// A hang or a memory blowup.
	k, _ = signature([]byte("--- FAIL: FuzzLSP\nfuzz: HANG: FuzzLSP runs for more than 10s\ngoroutine 1\n"))
	if !strings.HasPrefix(k, "fuzz: HANG") {
		t.Errorf("kind %q", k)
	}
	k, _ = signature([]byte("--- FAIL: X\n    fuzzing process hung or terminated unexpectedly: exit status 3\n"))
	if !strings.HasPrefix(k, "fuzzing process hung") {
		t.Errorf("kind %q", k)
	}
}

func TestRecordDedupes(t *testing.T) {
	root := t.TempDir()
	tg := target{name: "Check", pkg: "./tools/fuzz", fn: "FuzzCheck"}
	dir := filepath.Join(root, "tools", "fuzz", "testdata", "fuzz", "FuzzCheck")
	os.MkdirAll(dir, 0o755)
	c := &campaign{root: root, out: filepath.Join(root, "out"), seen: map[string]*finding{}}
	write := func(name string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte("go test fuzz v1\n[]byte(\"x\")\n"), 0o644)
		return p
	}
	out := []byte(panicOut)
	p1 := failingInput(out, root, tg)
	if p1 != "" {
		t.Fatalf("an input that does not exist: %s", p1)
	}
	write("abc123")
	p1 = failingInput(out, root, tg)
	if p1 == "" {
		t.Fatal("no failing input found")
	}
	f1 := c.record(tg, out, p1)
	if f1 == nil || f1.Count != 1 {
		t.Fatalf("first record: %+v", f1)
	}
	if _, err := os.Stat(p1); err == nil {
		t.Error("the input stays in testdata/fuzz")
	}
	if _, err := os.Stat(f1.Input); err != nil {
		t.Errorf("the input is not saved: %v", err)
	}
	p2 := write("def456")
	f2 := c.record(tg, out, p2)
	if f2 != f1 || f2.Count != 2 || len(c.seen) != 1 {
		t.Errorf("second record of the same bug: %+v, %d findings", f2, len(c.seen))
	}
	if _, err := os.Stat(p2); err == nil {
		t.Error("the duplicate stays")
	}
	if err := c.writeSummary(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(c.summaryPath())
	if !strings.Contains(string(b), "index out of range") {
		t.Errorf("summary:\n%s", b)
	}
	// A second campaign knows the finding.
	c2 := &campaign{root: root, out: c.out, seen: map[string]*finding{}}
	c2.loadSummary()
	if len(c2.seen) != 1 {
		t.Errorf("loadSummary: %d findings", len(c2.seen))
	}
}

// TestRunsTheEngine runs a real campaign over a tiny module whose fuzz target panics on inputs that
// start with "fuzz": the runner starts go test -fuzz, finds the failure, saves the input out of
// testdata/fuzz, writes the summary, and leaves no process behind.
func TestRunsTheEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the fuzzing engine")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module tiny\n\ngo 1.25\n"), 0o644)
	os.WriteFile(filepath.Join(root, "tiny_test.go"), []byte(`package tiny

import (
	"strings"
	"testing"
)

func FuzzTiny(f *testing.F) {
	f.Add("fuz")
	f.Fuzz(func(t *testing.T, s string) {
		if strings.HasPrefix(s, "fuzz") {
			panic("found it")
		}
	})
}
`), 0o644)
	c := &campaign{root: root, out: filepath.Join(root, "out"), parallel: 1, maxFind: 1, seen: map[string]*finding{}}
	defer func() {
		if t.Failed() {
			logs, _ := filepath.Glob(filepath.Join(root, "out", "logs", "*"))
			for _, l := range logs {
				b, _ := os.ReadFile(l)
				t.Logf("%s:\n%s", l, b)
			}
		}
	}()
	c.runTarget(t.Context(), target{name: "Tiny", pkg: ".", fn: "FuzzTiny"}, 60*time.Second, 0)
	if len(c.seen) != 1 {
		b, _ := os.ReadFile(c.summaryPath())
		t.Fatalf("%d findings, want 1; summary:\n%s", len(c.seen), b)
	}
	for _, f := range c.seen {
		if !strings.Contains(f.Kind, "found it") {
			t.Errorf("kind %q", f.Kind)
		}
		if _, err := os.Stat(f.Input); err != nil {
			t.Errorf("input not saved: %v", err)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(root, "testdata", "fuzz", "FuzzTiny", "*")); len(m) != 0 {
		t.Errorf("the engine's input stays in testdata/fuzz: %v", m)
	}
	b, err := os.ReadFile(c.summaryPath())
	if err != nil || !strings.Contains(string(b), "found it") || !strings.Contains(string(b), "Tiny") {
		t.Errorf("summary (%v):\n%s", err, b)
	}
}

// The text of a hang (fuzzkit.Guard's stack dump, which the engine does not show) reaches the
// finding: the runner runs the input alone, and takes the kind and the frames from that run. Two
// hangs in different places are two findings; the same one found twice is one.
func TestSignatureOfHangs(t *testing.T) {
	dump := func(frame string) []byte {
		return []byte("(the engine: fuzzing process hung or terminated unexpectedly; the input run alone:)\n" +
			"fuzz: HANG: FuzzCheck runs for more than 10s\ngoroutine 5 [running]:\n" +
			"github.com/huuhait/gocpp/tools/fuzz/fuzzkit.die(...)\n" +
			"goroutine 9 [running]:\n" +
			"github.com/huuhait/gocpp/internal/check." + frame + "(0xc000)\n" +
			"github.com/huuhait/gocpp/internal/check.(*checker).stmt(0xc001)\n")
	}
	k1, s1 := signature(dump("(*gateChecker).visit"))
	k2, s2 := signature(dump("(*gateChecker).visit"))
	_, s3 := signature(dump("(*checker).call"))
	if !strings.HasPrefix(k1, "fuzz: HANG") || k1 != k2 {
		t.Errorf("kind %q %q", k1, k2)
	}
	if s1 != s2 || s1 == s3 {
		t.Errorf("signatures %s %s %s: want the first two equal and the third different", s1, s2, s3)
	}
}

func TestHangIsReported(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the fuzzing engine")
	}
	t.Setenv("GOCPP_FUZZ_HANG", "1")
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	c := &campaign{root: root, out: filepath.Join(t.TempDir(), "out"), parallel: 1, maxFind: 1, seen: map[string]*finding{}}
	tg := target{name: "Hang", pkg: "./tools/fuzz/campaign/testdata/hangtarget", fn: "FuzzHang"}
	c.runTarget(t.Context(), tg, 120*time.Second, 0)
	if len(c.seen) != 1 {
		b, _ := os.ReadFile(c.summaryPath())
		t.Fatalf("%d findings, want 1; summary:\n%s", len(c.seen), b)
	}
	for _, f := range c.seen {
		if !strings.HasPrefix(f.Kind, "fuzz: HANG") {
			t.Errorf("kind %q, want the HANG text of the guard", f.Kind)
		}
		if !strings.Contains(f.Head, "goroutine") || !strings.Contains(f.Head, "spinForever") {
			t.Errorf("the finding has no stack of the hang:\n%s", f.Head)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(root, "tools", "fuzz", "campaign", "testdata", "hangtarget", "testdata", "fuzz", "FuzzHang", "*")); len(m) != 0 {
		t.Errorf("the engine's input stays in testdata/fuzz: %v", m)
	}
}
