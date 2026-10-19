package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain makes the test binary also the fake gocpp and the fake program: SANITIZE_FAKE=1 turns it
// into a program that "builds" by copying itself and "runs" by printing what the source file asks
// for (// fake-stderr:, // fake-exit:, // fake-sleep:, // fake-build-fail). The runner's logic is
// then tested without a C++ compiler, and without a wall clock except for the timeout test.
func TestMain(m *testing.M) {
	if os.Getenv("SANITIZE_FAKE") == "1" {
		os.Exit(fakeMain())
	}
	code := m.Run()
	if e2eDir != "" { // the end-to-end tests' gocpp and cache: hundreds of MB
		os.RemoveAll(e2eDir)
	}
	os.Exit(code)
}

func fakeMain() int {
	if len(os.Args) > 1 && os.Args[1] == "build" { // the fake gocpp: build [flags] -o exe file.gp
		var exe, src string
		for i := 2; i < len(os.Args); i++ {
			switch {
			case os.Args[i] == "-o" && i+1 < len(os.Args):
				exe = os.Args[i+1]
				i++
			case !strings.HasPrefix(os.Args[i], "-"):
				src = os.Args[i]
			}
		}
		b, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if bytes.Contains(b, []byte("// fake-build-fail")) {
			fmt.Fprintln(os.Stderr, "fake: build failed on purpose")
			return 1
		}
		self, _ := os.Executable()
		data, _ := os.ReadFile(self)
		if os.WriteFile(exe, data, 0o755) != nil || os.WriteFile(exe+".src", b, 0o644) != nil {
			return 1
		}
		// the arguments of the build are recorded for the tests
		var rec []string
		rec = append(rec, strings.Join(os.Args[1:], " "))
		for _, name := range []string{"GOCPP_CFLAGS", "GOCPP_LDFLAGS", "GOCPP_CXX", "GOCPP_GATE_PHASE", "GOCPP_MODCACHE", "GOCPP_OFFLINE", "GOCPP_WORKLOST", "GOCPP_FLAGS", "GOCPP_CC", "GOMAXPROCS", "GODEBUG", "LSAN_OPTIONS"} {
			rec = append(rec, name+"="+os.Getenv(name))
		}
		os.WriteFile(exe+".args", []byte(strings.Join(rec, "\n")), 0o644)
		return 0
	}
	if os.Getenv("SANITIZE_FAKE_SLEEP") == "1" { // the grandchild of // fake-orphan:
		time.Sleep(time.Minute)
		return 0
	}
	// the fake program
	b, err := os.ReadFile(os.Args[0] + ".src")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("ASAN_OPTIONS=%s\nUBSAN_OPTIONS=%s\nTSAN_OPTIONS=%s\n", os.Getenv("ASAN_OPTIONS"), os.Getenv("UBSAN_OPTIONS"), os.Getenv("TSAN_OPTIONS"))
	exit := 0
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "// fake-stderr: "):
			s := strings.ReplaceAll(strings.TrimPrefix(line, "// fake-stderr: "), `\n`, "\n")
			fmt.Fprintln(os.Stderr, s)
		case strings.HasPrefix(line, "// fake-stdout: "):
			fmt.Println(strings.ReplaceAll(strings.TrimPrefix(line, "// fake-stdout: "), `\n`, "\n"))
		case strings.HasPrefix(line, "// fake-flood: "): // that many bytes of lines on stderr
			var n int
			fmt.Sscan(strings.TrimPrefix(line, "// fake-flood: "), &n)
			chunk := strings.Repeat("x", 99) + "\n"
			for i := 0; i < n; i += len(chunk) {
				fmt.Fprint(os.Stderr, chunk)
			}
		case strings.HasPrefix(line, "// fake-orphan: "): // leave a process behind, and write its pid to the file
			self, _ := os.Executable()
			c := exec.Command(self)
			c.Env = append(os.Environ(), "SANITIZE_FAKE=1", "SANITIZE_FAKE_SLEEP=1")
			if c.Start() == nil {
				os.WriteFile(strings.TrimPrefix(line, "// fake-orphan: "), []byte(fmt.Sprint(c.Process.Pid)), 0o644)
			}
		case strings.HasPrefix(line, "// fake-exit: "):
			fmt.Sscan(strings.TrimPrefix(line, "// fake-exit: "), &exit)
		case strings.HasPrefix(line, "// fake-sleep"):
			time.Sleep(time.Minute)
		}
	}
	return exit
}

// ---------------------------------------------------------------------------------- Scan

const (
	asanText = "=================================================================\n" +
		"==12345==ERROR: AddressSanitizer: heap-buffer-overflow on address 0x7c1ff5be0020 at pc 0x555555a1ca89 bp 0x7fffffff98e0 sp 0x7fffffff98d8\n" +
		"WRITE of size 4 at 0x7c1ff5be0020 thread T0\n    #0 0x555555a1ca88 in overflow_heap canary_asan.h:11\n"
	ubsanText = "tools/sanitize/testdata/canary/canary_ubsan.h:8:14: runtime error: shift exponent 40 is too large for 32-bit type 'int'\n" +
		"    #0 0x55555559e2a3 in shift_too_far(int, int) canary_ubsan.h:8\n" +
		"SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior canary_ubsan.h:8:14\n"
	lsanText = "\n=================================================================\n" +
		"==777==ERROR: LeakSanitizer: detected memory leaks\n\nDirect leak of 77 byte(s) in 1 object(s) allocated from:\n" +
		"    #0 0x7ffff792271b in malloc (/lib/libasan.so.8+0x12271b)\n\n" +
		"SUMMARY: AddressSanitizer: 77 byte(s) leaked in 1 allocation(s).\n"
	tsanGoText  = "==================\nWARNING: DATA RACE\nWrite at 0x000000a0 by goroutine 7:\n  main.work()\n\nFound 1 data race(s)\n"
	tsanRawText = "==================\nWARNING: ThreadSanitizer: data race (pid=4242)\n  Write of size 8 at 0x55 by thread T2:\n"
	goPanic     = "panic: runtime error: index out of range [7] with length 3\n\ngoroutine 1 [running]:\nmain.main()\n\t/x/main.gp:9 +0x1d\nexit status 2\n"
	lsanNoiseTx = "==4242==Unable to get registers from thread 5.\n"
)

func TestScan(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string // Report.String()
	}{
		{"silent", "hello\nworld\n", nil},
		{"go panic is not a report", goPanic, nil},
		{"lsan thread noise is not a report", lsanNoiseTx, nil},
		{"asan", asanText, []string{"asan: heap-buffer-overflow on address 0x... at pc 0x... bp 0x... sp 0x..."}},
		{"ubsan", ubsanText, []string{"ubsan: shift exponent 40 is too large for 32-bit type 'int'"}},
		{"lsan", lsanText, []string{"lsan: detected memory leaks"}},
		{"tsan go form", tsanGoText, []string{"tsan: data race"}},
		{"tsan raw form", tsanRawText, []string{"tsan: data race (pid=4242)"}},
		{"tsan fatal", "FATAL: ThreadSanitizer: unexpected memory mapping 0x1-0x2\n", []string{"tsan: unexpected memory mapping 0x...-0x..."}},
		{"asan deadly signal", "AddressSanitizer:DEADLYSIGNAL\n", []string{"asan: DEADLYSIGNAL"}},
		{"two reports, once each", ubsanText + ubsanText + lsanText + asanText,
			[]string{"asan: heap-buffer-overflow on address 0x... at pc 0x... bp 0x... sp 0x...", "ubsan: shift exponent 40 is too large for 32-bit type 'int'", "lsan: detected memory leaks"}},
		{"an indented line is not UBSan's", "  x.gp:1:2: runtime error: y\n\t\tx.gp:1:2: runtime error: y\n", nil},
		{"ubsan only prints a summary", "SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior x.h:1:2\n", []string{"ubsan: SUMMARY: undefined-behavior x.h:1:2"}},
		{"ubsan segv (clang, UBSan alone)", "==5==ERROR: UndefinedBehaviorSanitizer: SEGV on unknown address 0x000000000000 (pc 0x1 bp 0x2 sp 0x3 T5)\n", []string{"ubsan: SEGV on unknown address 0x... (pc 0x... bp 0x... sp 0x... T5)"}},
		{"ubsan stack overflow", "==5==ERROR: UndefinedBehaviorSanitizer: stack-overflow on address 0x7ffc (pc 0x1 bp 0x2 sp 0x3 T0)\n", []string{"ubsan: stack-overflow on address 0x... (pc 0x... bp 0x... sp 0x... T0)"}},
		{"asan check failed", "==5==AddressSanitizer CHECK failed: x.cpp:1 \"a\" (0x0, 0x0)\n", []string{"asan: ==5==AddressSanitizer CHECK failed"}},
		{"tsan check failed", "ThreadSanitizer: CHECK failed: x.cpp:1\n", []string{"tsan: ThreadSanitizer: CHECK failed"}},
		{"tsan deadly signal", "ThreadSanitizer:DEADLYSIGNAL\n", []string{"tsan: DEADLYSIGNAL"}},
		{"lsan fatal", "==5==LeakSanitizer has encountered a fatal error.\n", []string{"lsan: ==5==LeakSanitizer has encountered a fatal error"}},
		{"asan summary alone", "SUMMARY: AddressSanitizer: heap-use-after-free x.cpp:1 in f\n", []string{"asan: SUMMARY: heap-use-after-free x.cpp:1 in f"}},
		{"a leak summary is lsan's, not asan's", "SUMMARY: AddressSanitizer: 5 byte(s) leaked in 1 allocation(s).\n", []string{"lsan: SUMMARY: 5 byte(s) leaked in 1 allocation(s)."}},
		{"a program that prints the words", "ERROR: AddressSanitizer is a fine tool\nruntime error: not at a position\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, r := range Scan(tt.text) {
				got = append(got, r.String())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Scan = %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestParseLeak(t *testing.T) {
	if l, ok := ParseLeak(lsanText); !ok || l != (Leak{77, 1}) {
		t.Errorf("ParseLeak = %v %v", l, ok)
	}
	if l, ok := ParseLeak("SUMMARY: LeakSanitizer: 5 byte(s) leaked in 2 allocation(s).\n"); !ok || l != (Leak{5, 2}) {
		t.Errorf("standalone LeakSanitizer: %v %v", l, ok)
	}
	if _, ok := ParseLeak(asanText); ok {
		t.Error("an AddressSanitizer error is not a leak summary")
	}
}

// ------------------------------------------------------------------------------- headers, corpus

func TestParseHeader(t *testing.T) {
	src := "// phase: 07\n// ExitCode: 2\n// lsan: cycle\n// Stderr: x\n//go:build linux\n\n// text\n// canary: ubsan\npackage main\n// phase: 99\n"
	h, err := ParseHeader(src)
	if err != nil {
		t.Fatal(err)
	}
	if h.Phase != 7 || h.ExitCode != 2 || !h.Cycle || h.Canary != UBSan || h.Build == nil || h.Pending || h.NeedsLibs {
		t.Errorf("header = %+v", h)
	}
	for _, bad := range []string{"// canary: msan\n", "// phase: x\n", "// ExitCode: -\n", "//go:build (\n"} {
		if _, err := ParseHeader(bad); err == nil {
			t.Errorf("ParseHeader(%q) accepted a bad line", bad)
		}
	}
}

func TestHeaderSkip(t *testing.T) {
	tests := []struct {
		src, want string
	}{
		{"// phase: 03\n", ""},
		{"// phase: 11\n", "phase"},
		{"// phase: 03\n// pending: step 9.9\n", "pending"},
		{"// phase: 03\n// pkg-config: sdl2\n", "needs-libs"},
		{"// phase: 03\n// display: required\n", "needs-display"},
		{"// phase: 03\n//go:build windows\n", "build-constraint"},
		{"// phase: 03\n//go:build linux || darwin\n", ""},
	}
	for _, tt := range tests {
		h, err := ParseHeader(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		// linux is the only target these tests can assume
		if strings.Contains(tt.src, "linux") && !targetTag("linux") {
			continue
		}
		if got := h.Skip(10, false); got != tt.want {
			t.Errorf("Skip(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
	h, _ := ParseHeader("// phase: 03\n// pending: x\n")
	if got := h.Skip(10, true); got != "" {
		t.Errorf("with pending: %q", got)
	}
}

// writeTree writes files (path -> text) under dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, text := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSelection(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tests/run/a_plain.gp":      "// phase: 03\npackage main\n",
		"tests/run/b_chan.gp":       "// phase: 08\npackage main\nfunc main() {\n\tgo f()\n}\n",
		"tests/run/c_future.gp":     "// phase: 12\npackage main\n",
		"tests/run/d_pending.gp":    "// phase: 03\n// pending: step 1\npackage main\n",
		"tests/run/e_sync.gp":       "// phase: 08\npackage main\nimport \"sync\"\n",
		"tests/run/f_libs.gp":       "// phase: 03\n// pkg-config: foo\npackage main\n",
		"tests/race/ok_silent.gp":   "// phase: 08\npackage main\n",
		"tests/race/race_noisy.gp":  "// phase: 08\npackage main\n",
		"tools/sanitize/sample.txt": "# comment\ntests/run/a_plain.gp  # trailing\n\ntests/race/ok_silent.gp\n",
	})
	names := func(ps []*Program) []string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Rel)
		}
		return out
	}
	pick := func(s Selection) ([]string, Skipped) {
		t.Helper()
		s.Root = root
		if s.Phase == 0 {
			s.Phase = 10
		}
		ps, sk, err := s.Pick()
		if err != nil {
			t.Fatal(err)
		}
		return names(ps), sk
	}
	got, sk := pick(Selection{Corpus: CorpusRun})
	if want := []string{"tests/run/a_plain.gp", "tests/run/b_chan.gp", "tests/run/e_sync.gp"}; !slices.Equal(got, want) {
		t.Errorf("run corpus = %v, want %v", got, want)
	}
	if sk["phase"] != 1 || sk["pending"] != 1 || sk["needs-libs"] != 1 {
		t.Errorf("skipped = %v", sk)
	}
	if got, _ := pick(Selection{Corpus: CorpusRun, Pending: true}); len(got) != 4 {
		t.Errorf("with pending: %v", got)
	}
	got, _ = pick(Selection{Corpus: CorpusConcurrency})
	if want := []string{"tests/race/ok_silent.gp", "tests/run/b_chan.gp", "tests/run/e_sync.gp"}; !slices.Equal(got, want) {
		t.Errorf("concurrency corpus = %v, want %v", got, want)
	}
	if got, _ := pick(Selection{Corpus: CorpusRace}); !slices.Equal(got, []string{"tests/race/ok_silent.gp"}) {
		t.Errorf("race corpus = %v", got)
	}
	if got, _ := pick(Selection{Corpus: CorpusSample}); !slices.Equal(got, []string{"tests/race/ok_silent.gp", "tests/run/a_plain.gp"}) {
		t.Errorf("sample corpus = %v", got)
	}
	if got, _ := pick(Selection{Paths: []string{"tests/run/b_chan.gp"}}); !slices.Equal(got, []string{"tests/run/b_chan.gp"}) {
		t.Errorf("paths = %v", got)
	}
	if got, _ := pick(Selection{Corpus: CorpusRun, Filter: regexp.MustCompile("chan")}); !slices.Equal(got, []string{"tests/run/b_chan.gp"}) {
		t.Errorf("filter = %v", got)
	}
	if got, _ := pick(Selection{Corpus: CorpusRun, Limit: 1}); len(got) != 1 {
		t.Errorf("limit = %v", got)
	}
	if _, _, err := (Selection{Root: root, Corpus: "nope"}).Pick(); err == nil {
		t.Error("an unknown corpus was accepted")
	}
	if _, _, err := (Selection{Root: root, Paths: []string{"tests/run/missing.gp"}}).Pick(); err == nil {
		t.Error("a missing path was accepted")
	}
}

func TestShards(t *testing.T) {
	var files []string
	for i := range 200 {
		files = append(files, fmt.Sprintf("tests/run/p%03d.gp", i))
	}
	count := map[int]int{}
	for _, f := range files {
		in := 0
		for i := range 4 {
			if inShard(f, i, 4) {
				in++
				count[i]++
			}
		}
		if in != 1 {
			t.Fatalf("%s is in %d of 4 shards", f, in)
		}
	}
	for i := range 4 {
		if count[i] < 25 {
			t.Errorf("shard %d has only %d of 200 programs", i, count[i])
		}
	}
	for _, bad := range []string{"1", "4/4", "-1/3", "a/b", "1/0"} {
		if _, _, err := parseShard(bad); err == nil {
			t.Errorf("parseShard(%q) accepted", bad)
		}
	}
}

// -------------------------------------------------------------------------------- runner (fake)

// fakeRunner returns a runner whose gocpp and programs are this test binary.
func fakeRunner(t *testing.T, mode string, cxx string) *Runner {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m, err := ModeByName(mode)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	r := NewRunner(Config{
		Root: work, Gocpp: self, Work: work, CXX: cxx, Mode: m, RunTimeout: 20 * time.Second,
		Env: append(cleanEnv(os.Environ()), "SANITIZE_FAKE=1"),
	})
	return r
}

// fakeProgram writes a program with the header lines and fake instructions, and returns it.
func fakeProgram(t *testing.T, name, text string) *Program {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name+".gp")
	if err := os.WriteFile(path, []byte(text+"\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := ParseHeader(text)
	if err != nil {
		t.Fatal(err)
	}
	return &Program{Path: path, Rel: "tests/run/" + name + ".gp", Header: h}
}

func TestRunProgramStatuses(t *testing.T) {
	esc := func(s string) string { return strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", `\n`) }
	tests := []struct {
		name, mode string
		src        string
		want       string
		kinds      []Kind
	}{
		{"clean", "asan", "// phase: 03\n", StatusOK, nil},
		{"asan report", "asan", "// fake-stderr: " + esc(asanText) + "\n// fake-exit: 1\n", StatusReport, []Kind{ASan}},
		{"report with exit 0 still fails", "asan", "// fake-stderr: " + esc(ubsanText) + "\n", StatusReport, []Kind{UBSan}},
		{"leak", "asan", "// fake-stderr: " + esc(lsanText) + "\n// fake-exit: 23\n", StatusReport, []Kind{LSan}},
		{"ubsan mode", "ubsan", "// fake-stderr: " + esc(ubsanText) + "\n", StatusReport, []Kind{UBSan}},
		{"tsan go form", "tsan", "// fake-stderr: " + esc(tsanGoText) + "\n// fake-exit: 66\n", StatusReport, []Kind{TSan}},
		{"tsan raw form", "tsan", "// fake-stderr: " + esc(tsanRawText) + "\n", StatusReport, []Kind{TSan}},
		{"expected exit code", "asan", "// ExitCode: 2\n// fake-stderr: " + esc(goPanic) + "\n// fake-exit: 2\n", StatusOK, nil},
		{"unexpected exit code", "asan", "// fake-exit: 3\n", StatusBadExit, nil},
		{"build failure", "asan", "// fake-build-fail\n", StatusBuildFailed, nil},
		{"canary reported", "asan", "// canary: asan\n// fake-stderr: " + esc(asanText) + "\n// fake-exit: 1\n", StatusOK, []Kind{ASan}},
		{"canary silent: the sanitizer is blind", "asan", "// canary: asan\n", StatusCanarySilent, nil},
		{"canary of another kind reported: still blind", "asan", "// canary: asan\n// fake-stderr: " + esc(ubsanText) + "\n", StatusCanarySilent, []Kind{UBSan}},
		{"canary the mode cannot make", "ubsan", "// canary: tsan\n// fake-sleep\n", StatusNotRun, nil},
		{"canary ubsan in the asan mode", "asan", "// canary: ubsan\n// fake-stderr: " + esc(ubsanText) + "\n", StatusOK, []Kind{UBSan}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fakeRunner(t, tt.mode, "g++")
			res := r.RunProgram(fakeProgram(t, "prog", tt.src))
			if res.Status != tt.want {
				t.Fatalf("status = %s, want %s\n%s\n%s", res.Status, tt.want, res.Detail, res.Output)
			}
			var kinds []Kind
			for _, rep := range res.Reports {
				kinds = append(kinds, rep.Kind)
			}
			if !slices.Equal(kinds, tt.kinds) {
				t.Errorf("report kinds = %v, want %v", kinds, tt.kinds)
			}
			if got := res.Failed(); got != (tt.want != StatusOK && tt.want != StatusNotRun) {
				t.Errorf("Failed() = %v for %s", got, tt.want)
			}
		})
	}
}

func TestRunProgramTimeout(t *testing.T) {
	r := fakeRunner(t, "asan", "g++")
	r.RunTimeout = 300 * time.Millisecond
	start := time.Now()
	res := r.RunProgram(fakeProgram(t, "hang", "// fake-sleep\n"))
	if res.Status != StatusTimeout {
		t.Fatalf("status = %s: %s", res.Status, res.Detail)
	}
	if time.Since(start) > 30*time.Second {
		t.Error("the program was not killed at the time limit")
	}
}

func TestBuildArgsAndEnvPerMode(t *testing.T) {
	read := func(t *testing.T, mode, cxx string) string {
		t.Helper()
		r := fakeRunner(t, mode, cxx)
		work := t.TempDir()
		r.Work = work
		p := fakeProgram(t, "prog", "// phase: 03\n")
		// keep the work folder: read what the fake gocpp recorded, through a second build into a known place
		exe := filepath.Join(work, "x")
		args := append(append([]string{"build", "-compdb=false"}, r.Mode.BuildArgs...), "-o", exe, filepath.Base(p.Path))
		res := r.exec(1<<20, filepath.Dir(p.Path), r.Env, r.gocppEnv(p, work), time.Minute, r.Gocpp, args...)
		if res.Code != 0 {
			t.Fatalf("fake build: %v %s", res.Err, res.Stderr)
		}
		b, err := os.ReadFile(exe + ".args")
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read(t, "asan", "clang++"); !strings.Contains(got, "build -compdb=false -asan -o ") || !strings.Contains(got, "GOCPP_CXX=clang++") {
		t.Errorf("asan build: %q", got)
	}
	if got := read(t, "tsan", "g++"); !strings.Contains(got, "-race") || strings.Contains(got, "-asan") {
		t.Errorf("tsan build: %q", got)
	}
	got := read(t, "ubsan", "g++")
	if strings.Contains(got, "-asan") || !strings.Contains(got, "GOCPP_CFLAGS=-fsanitize=undefined -fno-sanitize-recover=undefined") {
		t.Errorf("ubsan build: %q", got)
	}
}

func TestCycleProgramsRunWithoutLeakCheck(t *testing.T) {
	r := fakeRunner(t, "asan", "g++")
	cyc := r.RunProgram(fakeProgram(t, "cyc", "// lsan: cycle\n"))
	if !strings.Contains(cyc.Output, "detect_leaks=1:") || !strings.Contains(cyc.Output, ":detect_leaks=0") {
		t.Errorf("a cycle program's ASAN_OPTIONS: %q", cyc.Output)
	}
	plain := r.RunProgram(fakeProgram(t, "plain", "// phase: 03\n"))
	if strings.Contains(plain.Output, "detect_leaks=0") || !strings.Contains(plain.Output, "detect_leaks=1") {
		t.Errorf("a plain program's ASAN_OPTIONS: %q", plain.Output)
	}
	// the canary for leaks keeps leak checking even when it has the marker (the marker would hide it)
	can := r.RunProgram(fakeProgram(t, "can", "// lsan: cycle\n// canary: lsan\n"))
	if strings.Contains(can.Output, "detect_leaks=0") {
		t.Errorf("a canary lost its leak check: %q", can.Output)
	}
}

func TestSanitizerOptionsOfTheShellAreDropped(t *testing.T) {
	t.Setenv("ASAN_OPTIONS", "detect_leaks=0")
	t.Setenv("GORACE", "halt_on_error=1")
	env := cleanEnv(os.Environ())
	for _, kv := range env {
		if strings.HasPrefix(kv, "ASAN_OPTIONS=") || strings.HasPrefix(kv, "GORACE=") {
			t.Errorf("%s survived", kv)
		}
	}
}

// ------------------------------------------------------------------------------- command line

// runTool runs the tool with args and returns its exit code and output.
func runTool(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("SANITIZE_FAKE", "1")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	args = append(args[:1:1], append([]string{"-gocpp", self, "-work", t.TempDir()}, args[1:]...)...)
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunCommandFailsOnAnyReport(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	esc := func(s string) string { return strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", `\n`) }
	writeTree(t, root, map[string]string{
		"go.mod":                 "module x\n",
		"cmd/gocpp/main.go":      "package main\n",
		"tests/run/clean.gp":     "// phase: 03\npackage main\n",
		"tests/run/dirty.gp":     "// phase: 03\n// fake-stderr: " + esc(ubsanText) + "\npackage main\n",
		"tests/run/clean_two.gp": "// phase: 03\npackage main\n",
	})
	code, stdout, stderr := runTool(t, "run", "-root", root, "-out", out, "-mode", "asan", "-cxx", "g++,clang++", "-j", "3", "-corpus", "run")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s\n%s", code, stdout, stderr)
	}
	for _, want := range []string{"asan g++: 3 programs, ok 2, report 1", "asan clang++: 3 programs, ok 2, report 1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if !strings.Contains(stderr, "FAIL tests/run/dirty.gp [asan g++] report") || !strings.Contains(stderr, "ubsan: shift exponent 40") {
		t.Errorf("stderr:\n%s", stderr)
	}
	// the files of the run
	var sum Summary
	b, err := os.ReadFile(filepath.Join(out, "asan-g++.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Counts[StatusReport] != 1 || len(sum.Results) != 3 || sum.Corpus != "run" {
		t.Errorf("summary = %+v", sum.Counts)
	}
	logb, err := os.ReadFile(filepath.Join(out, "logs", "asan-clang++", "tests__run__dirty.log"))
	if err != nil || !strings.Contains(string(logb), "shift exponent 40") {
		t.Errorf("log: %v %q", err, logb)
	}
	// without the dirty program the run passes
	code, stdout, stderr = runTool(t, "run", "-root", root, "-out", out, "-mode", "asan", "-j", "3", "-filter", "clean")
	if code != 0 || !strings.Contains(stdout, "asan g++: 2 programs, ok 2") {
		t.Errorf("clean run: exit %d\n%s\n%s", code, stdout, stderr)
	}
	// usage errors are exit 2
	if code, _, _ := runTool(t, "run", "-root", root, "-mode", "msan"); code != 2 {
		t.Errorf("bad mode: exit %d", code)
	}
	if code, _, _ := runTool(t, "run", "-root", root, "-corpus", "nope"); code == 0 {
		t.Errorf("bad corpus accepted")
	}
	if code, _, _ := runTool(t, "run", "-root", root, "-filter", "zzz-none"); code == 0 {
		t.Errorf("an empty selection must not pass (nothing was checked)")
	}
}

func TestScanCommand(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"clean.txt": goPanic, "dirty.txt": asanText + lsanText})
	var out, errOut bytes.Buffer
	if code := run([]string{"scan", filepath.Join(dir, "clean.txt")}, &out, &errOut); code != 0 || out.Len() != 0 {
		t.Errorf("clean: exit %d %q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"scan", filepath.Join(dir, "dirty.txt")}, &out, &errOut); code != 1 || strings.Count(out.String(), "\n") != 2 {
		t.Errorf("dirty: exit %d %q", code, out.String())
	}
}

// ---------------------------------------------------------------------------------- leak audit

func TestParseFormatLeaks(t *testing.T) {
	text := "# c\n\ntests/run/a.gp | closure-self | g++=288/8 clang++=300/9 | a closure calls itself\n" +
		"tests/run/b.gp | cond-locker | g++=1/1 clang++=2/1 slack=32/0 | a Cond whose Locker is inside\n"
	es, err := ParseLeaks(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 || es[0].Ceiling["clang++"] != (Leak{300, 9}) || es[1].Slack != (Leak{32, 0}) || es[1].Reason != "a Cond whose Locker is inside" {
		t.Fatalf("entries = %+v", es)
	}
	back, err := ParseLeaks(strings.NewReader(FormatLeaks(es)))
	if err != nil || fmt.Sprint(back) != fmt.Sprint(es) {
		t.Errorf("round trip: %v\n%v\n%v", err, back, es)
	}
	for _, bad := range []string{"just a path\n", "a.gp | k | g++=1 | r\n", "a.gp | k | g++=1/1 g++=2/2 | r\n", "a.gp | k | msvc=1/1 | r\n"} {
		if _, err := ParseLeaks(strings.NewReader(bad)); err == nil {
			t.Errorf("ParseLeaks(%q) accepted", bad)
		}
	}
}

func TestCheckLeaks(t *testing.T) {
	good := func() LeakEntry {
		return LeakEntry{Path: "tests/run/a.gp", Kind: "closure-self", Ceiling: map[string]Leak{"g++": {1, 1}, "clang++": {1, 1}}, Reason: "a closure calls itself"}
	}
	tests := []struct {
		name    string
		entries []LeakEntry
		markers []string
		want    string // a substring of a problem, "" for none
	}{
		{"fine", []LeakEntry{good()}, []string{"tests/run/a.gp"}, ""},
		{"a marker with no line", nil, []string{"tests/run/a.gp"}, "is not in tools/sanitize/leaks.txt"},
		{"a line with no marker", []LeakEntry{good()}, nil, "does not exist or has no"},
		{"an unknown kind", []LeakEntry{func() LeakEntry { e := good(); e.Kind = "whatever"; return e }()}, []string{"tests/run/a.gp"}, "unknown kind"},
		{"no reason", []LeakEntry{func() LeakEntry { e := good(); e.Reason = "TODO"; return e }()}, []string{"tests/run/a.gp"}, "the reason is missing"},
		{"no ceiling for a compiler", []LeakEntry{func() LeakEntry { e := good(); delete(e.Ceiling, "clang++"); return e }()}, []string{"tests/run/a.gp"}, "no ceiling for clang++"},
		{"an allocation slack", []LeakEntry{func() LeakEntry { e := good(); e.Slack = Leak{10, 1}; return e }()}, []string{"tests/run/a.gp"}, "allocation slack must be 0"},
		{"a byte slack at the limit", []LeakEntry{func() LeakEntry { e := good(); e.Slack = Leak{maxByteSlack, 0}; return e }()}, []string{"tests/run/a.gp"}, ""},
		{"a big byte slack", []LeakEntry{func() LeakEntry { e := good(); e.Slack = Leak{maxByteSlack + 1, 0}; return e }()}, []string{"tests/run/a.gp"}, "make the program deterministic"},
		{"a ceiling of zero", []LeakEntry{func() LeakEntry { e := good(); e.Ceiling["g++"] = Leak{}; return e }()}, []string{"tests/run/a.gp"}, "has no marker"},
		{"listed twice", []LeakEntry{good(), good()}, []string{"tests/run/a.gp"}, "listed twice"},
		{"an absolute path", []LeakEntry{func() LeakEntry { e := good(); e.Path = "/tmp/a.gp"; return e }()}, []string{"/tmp/a.gp"}, "relative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probs := CheckLeaks(tt.entries, tt.markers)
			if tt.want == "" {
				if len(probs) != 0 {
					t.Errorf("problems: %v", probs)
				}
				return
			}
			if !strings.Contains(strings.Join(probs, "\n"), tt.want) {
				t.Errorf("problems %q lack %q", probs, tt.want)
			}
		})
	}
}

func TestJudgeLeak(t *testing.T) {
	ceil := Leak{288, 8}
	lsan := []Report{{LSan, "detected memory leaks"}}
	tests := []struct {
		name    string
		reports []Report
		leak    *Leak
		slack   Leak
		fail    bool
		tighten bool
		msg     string
	}{
		{"as measured", lsan, &Leak{288, 8}, Leak{}, false, false, ""},
		{"stale: no leak any more", nil, nil, Leak{}, true, false, "leaks nothing any more"},
		{"grew in bytes", lsan, &Leak{289, 8}, Leak{}, true, false, "more than the ceiling"},
		{"grew in allocations", lsan, &Leak{288, 9}, Leak{}, true, false, "more than the ceiling"},
		{"shrank", lsan, &Leak{200, 6}, Leak{}, false, true, "lower it"},
		{"another sanitizer", []Report{{LSan, "x"}, {ASan, "heap-use-after-free"}}, &Leak{288, 8}, Leak{}, true, false, "another sanitizer report"},
		{"a leak without a size", lsan, nil, Leak{}, true, false, "not its size"},
		{"within the slack", lsan, &Leak{300, 9}, Leak{32, 1}, false, false, ""},
		{"beyond the slack", lsan, &Leak{330, 8}, Leak{32, 1}, true, false, "more than the ceiling"},
		{"below with slack is not a note", lsan, &Leak{100, 3}, Leak{32, 1}, false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := JudgeLeak(tt.reports, tt.leak, ceil, tt.slack)
			if v.Fail != tt.fail || v.Tighten != tt.tighten || !strings.Contains(v.Message, tt.msg) {
				t.Errorf("verdict = %+v", v)
			}
		})
	}
}

// leakRoot makes a repository-like folder for the audit with the fake gocpp: programs and leaks.txt.
func leakRoot(t *testing.T, programs map[string]string, list string) string {
	root := t.TempDir()
	files := map[string]string{"go.mod": "module x\n", "cmd/gocpp/main.go": "package main\n", leaksFile: list}
	for p, s := range programs {
		files[p] = s
	}
	writeTree(t, root, files)
	return root
}

func TestFindMarkersSkipsTestdata(t *testing.T) {
	root := leakRoot(t, map[string]string{
		"tests/run/a.gp":                      "// phase: 03\n// lsan: cycle\npackage main\n",
		"tests/run/b.gp":                      "// phase: 03\npackage main\n",
		"tests/testdata/header/c.gp":          "// phase: 03\n// lsan: cycle\npackage main\n",
		"internal/cppgen/testdata/emit/d.gp":  "// lsan: cycle\npackage main\n",
		"internal/cppgen/testdata/other/e.gp": "// lsan: cycle\npackage main\n",
		"examples/f/main.gp":                  "// lsan: cycle\npackage main\n",
	}, "")
	got, err := FindMarkers(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"examples/f/main.gp", "internal/cppgen/testdata/emit/d.gp", "tests/run/a.gp"}
	if !slices.Equal(got, want) {
		t.Errorf("markers = %v, want %v", got, want)
	}
}

func leakProgram(bytes, allocs int) string {
	return fmt.Sprintf("// phase: 03\n// lsan: cycle\n// fake-stderr: ==1==ERROR: LeakSanitizer: detected memory leaks\\nSUMMARY: AddressSanitizer: %d byte(s) leaked in %d allocation(s).\npackage main\n", bytes, allocs)
}

func TestLeakAudit(t *testing.T) {
	list := func(b, a int) string {
		return fmt.Sprintf("tests/run/a.gp | closure-self | g++=%d/%d clang++=%d/%d | a closure calls itself\n", b, a, b, a)
	}
	tests := []struct {
		name    string
		program string
		list    string
		code    int
		stderr  string // a substring
		stdout  string
	}{
		{"as measured", leakProgram(288, 8), list(288, 8), 0, "", "leaks g++: 1 programs, ok 1"},
		{"a leak on top of the cycle", leakProgram(300, 9), list(288, 8), 1, "more than the ceiling", ""},
		{"the cycle is gone: a stale marker", "// phase: 03\n// lsan: cycle\npackage main\n", list(288, 8), 1, "leaks nothing any more", ""},
		{"it also corrupts memory", "// phase: 03\n// lsan: cycle\n// fake-stderr: ==1==ERROR: AddressSanitizer: heap-use-after-free on address 0x1\npackage main\n", list(288, 8), 1, "another sanitizer report", ""},
		{"it improved", leakProgram(100, 4), list(288, 8), 0, "", "below the ceiling 288 in 8: run leaks -update"},
		{"its build fails", "// phase: 03\n// lsan: cycle\n// fake-build-fail\npackage main\n", list(288, 8), 1, "build-failed", ""},
		{"a marker that is not listed", leakProgram(288, 8), "", 1, "is not in tools/sanitize/leaks.txt", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := leakRoot(t, map[string]string{"tests/run/a.gp": tt.program}, tt.list)
			code, stdout, stderr := runTool(t, "leaks", "-root", root, "-out", t.TempDir(), "-cxx", "g++")
			if code != tt.code || !strings.Contains(stderr, tt.stderr) || !strings.Contains(stdout, tt.stdout) {
				t.Errorf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.code, stdout, stderr)
			}
		})
	}
	// both compilers by default: the ceilings are per compiler
	root := leakRoot(t, map[string]string{"tests/run/a.gp": leakProgram(288, 8)}, list(288, 8))
	if code, stdout, stderr := runTool(t, "leaks", "-root", root, "-out", t.TempDir()); code != 0 || !strings.Contains(stdout, "leaks g++: 1 programs, ok 1") || !strings.Contains(stdout, "leaks clang++: 1 programs, ok 1") {
		t.Errorf("default compilers: exit %d\n%s\n%s", code, stdout, stderr)
	}
	// the check alone needs no builds
	root = leakRoot(t, map[string]string{"tests/run/a.gp": leakProgram(1, 1)}, list(1, 1))
	if code, stdout, _ := runTool(t, "leaks", "-check", "-root", root); code != 0 || !strings.Contains(stdout, "1 programs") {
		t.Errorf("-check: exit %d %s", code, stdout)
	}
	if code, _, stderr := runTool(t, "leaks", "-check", "-root", leakRoot(t, map[string]string{"tests/run/a.gp": leakProgram(1, 1)}, "")); code != 1 || !strings.Contains(stderr, "not in tools/sanitize/leaks.txt") {
		t.Errorf("-check with a missing line: exit %d %s", code, stderr)
	}
}

func TestLeakAuditUpdate(t *testing.T) {
	list := "tests/run/a.gp | closure-self | g++=288/8 clang++=288/8 | a closure calls itself\n"
	read := func(root string) string {
		b, err := os.ReadFile(filepath.Join(root, leaksFile))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	// lowers
	root := leakRoot(t, map[string]string{"tests/run/a.gp": leakProgram(200, 6)}, list)
	out := t.TempDir()
	if code, _, stderr := runTool(t, "leaks", "-update", "-root", root, "-out", out, "-cxx", "g++,clang++"); code != 0 {
		t.Fatalf("update: exit %d\n%s", code, stderr)
	}
	// an accepted entry is "ok" in the results, not the "report" of the leak it was allowed to have
	for _, cxx := range []string{"g++", "clang++"} {
		b, err := os.ReadFile(filepath.Join(out, "leaks-"+cxx+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var sum Summary
		if err := json.Unmarshal(b, &sum); err != nil || sum.Counts[StatusOK] != 1 || sum.Counts[StatusReport] != 0 {
			t.Errorf("%s: summary after -update: %v %v", cxx, err, sum.Counts)
		}
	}
	if got := read(root); !strings.Contains(got, "g++=200/6 clang++=200/6") {
		t.Errorf("not lowered:\n%s", got)
	}
	// does not raise without -allow-grow, and leaves the file as it was
	root = leakRoot(t, map[string]string{"tests/run/a.gp": leakProgram(400, 9)}, list)
	if code, _, stderr := runTool(t, "leaks", "-update", "-root", root, "-out", t.TempDir(), "-cxx", "g++,clang++"); code != 1 || !strings.Contains(stderr, "above the ceiling") {
		t.Errorf("update that grows: exit %d\n%s", code, stderr)
	}
	if got := read(root); got != list {
		t.Errorf("the file changed:\n%s", got)
	}
	if code, _, _ := runTool(t, "leaks", "-update", "-allow-grow", "-root", root, "-out", t.TempDir(), "-cxx", "g++,clang++"); code != 0 {
		t.Errorf("update -allow-grow: exit %d", code)
	}
	if got := read(root); !strings.Contains(got, "g++=400/9 clang++=400/9") {
		t.Errorf("not raised:\n%s", got)
	}
	// a new marker gets a line to complete
	root = leakRoot(t, map[string]string{"tests/run/a.gp": leakProgram(288, 8), "tests/run/new.gp": leakProgram(50, 1)}, list)
	if code, _, stderr := runTool(t, "leaks", "-update", "-root", root, "-out", t.TempDir(), "-cxx", "g++,clang++"); code != 0 {
		t.Fatalf("update with a new marker: exit %d\n%s", code, stderr)
	}
	if got := read(root); !strings.Contains(got, "tests/run/new.gp | mixed | g++=50/1 clang++=50/1 | TODO: say what the cycle is") {
		t.Errorf("new line missing:\n%s", got)
	}
	// ... and the TODO reason fails the check until somebody writes the reason
	if code, _, stderr := runTool(t, "leaks", "-check", "-root", root); code != 1 || !strings.Contains(stderr, "the reason is missing") {
		t.Errorf("-check after update: exit %d %s", code, stderr)
	}
}

// TestLeaksFileMatchesTheTree is the policy: every "// lsan: cycle" program of the repository is
// in tools/sanitize/leaks.txt with a kind, a reason and a ceiling, and every line has its marker.
func TestLeaksFileMatchesTheTree(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := loadLeaks(root)
	if err != nil {
		t.Fatal(err)
	}
	markers, err := FindMarkers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) == 0 {
		t.Fatal("found no \"// lsan: cycle\" program: the search is broken")
	}
	for _, p := range CheckLeaks(entries, markers) {
		t.Error(p)
	}
}

// TestSampleAndCanaryFilesExist keeps the corpora honest: every path of sample.txt is a program
// that the selection accepts, and the canaries cover each kind and have their controls.
func TestSampleAndCanaryFilesExist(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	ps, skipped, err := Selection{Root: root, Corpus: CorpusSample, Phase: 99}.Pick()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := readList(filepath.Join(root, "tools/sanitize/sample.txt"))
	if len(ps) != len(list) || len(ps) < 10 {
		t.Errorf("sample: %d of %d programs selected (skipped %s)", len(ps), len(list), skipped)
	}
	ps, _, err = Selection{Root: root, Corpus: CorpusCanary, Phase: 99, Pending: true}.Pick()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[Kind]bool{}
	clean := 0
	for _, p := range ps {
		if p.Header.Canary == "" {
			clean++
			continue
		}
		kinds[p.Header.Canary] = true
		if p.Header.Cycle {
			t.Errorf("%s: a canary must not have the leak marker", p.Rel)
		}
	}
	for _, k := range []Kind{ASan, UBSan, LSan, TSan} {
		if !kinds[k] {
			t.Errorf("no canary of kind %s", k)
		}
	}
	if clean < 2 {
		t.Errorf("only %d clean controls", clean)
	}
}

// ------------------------------------------------------------------------ the files that run the job

func TestNightlyFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"nightly", "-n", "-stages", "asan,leaks", "-deep", "-shard", "1/4", "-cxx", "g++"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if want := `stages leaks,asan,deep-sanitize,deep-race, compilers g++, phase <= 99, shard "1/4"`; !strings.Contains(out.String(), want) {
		t.Errorf("plan = %q", out.String())
	}
	for _, bad := range [][]string{{"-stages", "msan"}, {"-j", "0"}, {"extra"}, {"-nosuchflag"}} {
		var o, e bytes.Buffer
		if code := run(append([]string{"nightly", "-n"}, bad...), &o, &e); code != 2 {
			t.Errorf("nightly -n %v: exit %d, want 2 (%s)", bad, code, e.String())
		}
	}
}

// TestJobFilesUseRealFlags checks the files that start the job against the tool: every make target
// that the workflow calls exists, and the flags it passes (SANITIZEFLAGS="...") and the flags of
// nightly.sh are accepted by "sanitize nightly" (-n parses them and runs nothing).
func TestJobFilesUseRealFlags(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	makefile := read("Makefile")
	workflow := read(".github/workflows/sanitizers-nightly.yml")
	for _, target := range []string{"sanitize-nightly", "sanitize-smoke", "sanitize-leaks", "sanitize-selftest"} {
		if !regexp.MustCompile(`(?m)^` + target + `:`).MatchString(makefile) {
			t.Errorf("Makefile has no target %s", target)
		}
		if !regexp.MustCompile(`\.PHONY:.*\b` + target + `\b`).MatchString(makefile) {
			t.Errorf("%s is not .PHONY", target)
		}
	}
	for _, m := range regexp.MustCompile(`make (sanitize-[a-z]+)`).FindAllStringSubmatch(workflow, -1) {
		if !regexp.MustCompile(`(?m)^` + m[1] + `:`).MatchString(makefile) {
			t.Errorf("the workflow calls make %s, which does not exist", m[1])
		}
	}
	flags := regexp.MustCompile(`SANITIZEFLAGS="([^"]*)"`).FindAllStringSubmatch(workflow, -1)
	if len(flags) < 4 {
		t.Fatalf("found %d SANITIZEFLAGS in the workflow, want 4 jobs", len(flags))
	}
	for _, m := range flags {
		text := strings.ReplaceAll(m[1], "${{ matrix.shard }}", "1")
		text = regexp.MustCompile(`\$\{\{[^}]*\}\}`).ReplaceAllString(text, "")
		text = strings.ReplaceAll(strings.ReplaceAll(text, "$HOME", "/home/x"), "$EXTRA", "")
		args := append([]string{"nightly", "-n"}, strings.Fields(text)...)
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 0 {
			t.Errorf("the workflow's flags %q: exit %d: %s", m[1], code, errOut.String())
		}
	}
	script := read("tools/sanitize/nightly.sh")
	if !strings.Contains(script, "go build -o \"$bin\" ./tools/sanitize") || !strings.Contains(script, "\"$bin\" nightly") || !strings.Contains(script, "-gocpp-cache") || strings.Contains(script, "go run") {
		t.Error("nightly.sh does not run the tool as the docs say")
	}
	// the job does not silently skip the programs of later phases
	if regexp.MustCompile(`tools/sanitize nightly[^\n]*-phase`).MatchString(makefile) {
		t.Error("a sanitize target passes -phase: the nightly job runs every phase (the tool counts what it skips)")
	}
	if strings.Contains(script, "-phase 10") {
		t.Error("nightly.sh pins -phase 10")
	}
	// the Makefile's own flags
	for _, line := range []string{"nightly", "nightly -sample"} {
		var out, errOut bytes.Buffer
		if code := run(append([]string{}, append(strings.Fields(line)[:1], append([]string{"-n"}, strings.Fields(line)[1:]...)...)...), &out, &errOut); code != 0 {
			t.Errorf("%q: exit %d: %s", line, code, errOut.String())
		}
	}
}

// nightlyRoot makes a repository-like folder for the nightly job with the fake gocpp: one dirty and
// one clean program in tests/run, and canaries that the fake "reports".
func nightlyRoot(t *testing.T, dirty bool) string {
	esc := func(s string) string { return strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", `\n`) }
	files := map[string]string{
		"go.mod": "module x\n", "cmd/gocpp/main.go": "package main\n",
		leaksFile:                      "",
		"tests/run/clean_chan.gp":      "// phase: 08\npackage main\nfunc main() {\n\tgo f()\n}\n",
		"tools/sanitize/sample.txt":    "tests/run/clean_chan.gp\n",
		canaryPath("canary_asan"):      "// phase: 04\n// canary: asan\n// fake-stderr: " + esc(asanText) + "\n// fake-exit: 1\npackage main\n",
		canaryPath("canary_ubsan"):     "// phase: 04\n// canary: ubsan\n// fake-stderr: " + esc(ubsanText) + "\npackage main\n",
		canaryPath("canary_lsan"):      "// phase: 04\n// canary: lsan\n// fake-stderr: " + esc(lsanText) + "\n// fake-exit: 23\npackage main\n",
		canaryPath("canary_tsan"):      "// phase: 08\n// canary: tsan\n// fake-stderr: " + esc(tsanGoText) + "\n// fake-exit: 66\npackage main\n",
		canaryPath("clean_control_c"):  "// phase: 04\npackage main\n",
		canaryPath("clean_control_go"): "// phase: 08\npackage main\n",
	}
	if dirty {
		files["tests/run/dirty.gp"] = "// phase: 03\n// fake-stderr: " + esc(ubsanText) + "\npackage main\n"
	}
	root := t.TempDir()
	writeTree(t, root, files)
	return root
}

func canaryPath(name string) string { return "tools/sanitize/testdata/canary/" + name + ".gp" }

func TestNightlyRunsEveryStageAndFailsOnAny(t *testing.T) {
	root := nightlyRoot(t, true)
	out := t.TempDir()
	code, stdout, stderr := runTool(t, "nightly", "-root", root, "-out", out, "-cxx", "g++", "-j", "4", "-stages", "canary,leaks,asan,tsan")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s\n%s", code, stdout, stderr)
	}
	b, err := os.ReadFile(filepath.Join(out, "nightly.md"))
	if err != nil {
		t.Fatal(err)
	}
	report := string(b)
	for _, want := range []string{"| canary | passed |", "| leaks | passed |", "| asan | FAILED |", "| tsan | passed |"} {
		if !strings.Contains(report, want) {
			t.Errorf("nightly.md lacks %q:\n%s", want, report)
		}
	}
	// without the dirty program every stage passes
	root = nightlyRoot(t, false)
	if code, stdout, stderr := runTool(t, "nightly", "-root", root, "-out", t.TempDir(), "-cxx", "g++,clang++", "-j", "4", "-stages", "canary,leaks,asan,ubsan,tsan"); code != 0 {
		t.Errorf("clean tree: exit %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestNightlyFailsWhenACanaryIsSilent(t *testing.T) {
	root := nightlyRoot(t, false)
	// the ASan canary no longer makes a report: the sanitizer would be blind
	writeTree(t, root, map[string]string{canaryPath("canary_asan"): "// phase: 04\n// canary: asan\npackage main\n"})
	out := t.TempDir()
	code, _, stderr := runTool(t, "nightly", "-root", root, "-out", out, "-cxx", "g++", "-stages", "canary")
	if code != 1 || !strings.Contains(stderr, "canary-silent") || !strings.Contains(stderr, "the sanitizer is blind") {
		t.Errorf("exit %d\n%s", code, stderr)
	}
}

func TestNightlyLeaksStageChecksTheList(t *testing.T) {
	root := nightlyRoot(t, false)
	// a marker with no line in leaks.txt
	writeTree(t, root, map[string]string{"tests/run/cycle.gp": leakProgram(10, 1)})
	code, _, stderr := runTool(t, "nightly", "-root", root, "-out", t.TempDir(), "-cxx", "g++", "-stages", "leaks")
	if code != 1 || !strings.Contains(stderr, "is not in tools/sanitize/leaks.txt") {
		t.Errorf("exit %d\n%s", code, stderr)
	}
}

// ------------------------------------------------------------------------ review round 1

func TestCleanEnvDropsWhatChangesTheRun(t *testing.T) {
	drop := []string{"ASAN_OPTIONS", "UBSAN_OPTIONS", "TSAN_OPTIONS", "LSAN_OPTIONS", "MSAN_OPTIONS", "GORACE", "GOMAXPROCS", "GODEBUG",
		"GOCPP_FLAGS", "GOCPP_CFLAGS", "GOCPP_CXXFLAGS", "GOCPP_LDFLAGS", "GOCPP_CXX", "GOCPP_CC", "GOCPP_GATE_PHASE", "GOCPP_ALLOW_INTERNAL", "GOCPP_RACE_RAW", "GOCPP_ANYTHING_ELSE"}
	env := []string{"PATH=/bin", "HOME=/h", "GOFLAGS=-v", "TMPDIR=/t", "GOCPPX=1"}
	for _, d := range drop {
		env = append(env, d+"=1")
	}
	got := cleanEnv(env)
	if !slices.Equal(got, []string{"PATH=/bin", "HOME=/h", "GOFLAGS=-v", "TMPDIR=/t", "GOCPPX=1"}) {
		t.Errorf("cleanEnv = %v", got)
	}
}

func TestBuildEnvironmentIsPinned(t *testing.T) {
	r := fakeRunner(t, "asan", "g++")
	r.Env = append(r.Env, "GOMAXPROCS=1", "GOCPP_FLAGS=-race", "GOCPP_CC=cc", "LSAN_OPTIONS=exitcode=0", "GODEBUG=x=1") // not cleaned: the runner was given this Env
	p := fakeProgram(t, "prog", "// phase: 03\n")
	work := t.TempDir()
	exe := filepath.Join(work, "x")
	args := []string{"build", "-compdb=false", "-asan", "-o", exe, filepath.Base(p.Path)}
	env := r.gocppEnv(p, work)
	if res := r.exec(1<<20, filepath.Dir(p.Path), cleanEnv(r.Env), env, time.Minute, r.Gocpp, args...); res.Code != 0 {
		t.Fatalf("fake build: %v %s", res.Err, res.Stderr)
	}
	b, err := os.ReadFile(exe + ".args")
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"GOCPP_GATE_PHASE=15\n", "GOCPP_MODCACHE=off\n", "GOCPP_OFFLINE=1\n", "GOCPP_WORKLOST=fail\n", "GOCPP_CXX=g++\n", "GOCPP_FLAGS=\n", "GOCPP_CC=\n", "GOMAXPROCS=\n", "GODEBUG=\n", "LSAN_OPTIONS="} {
		if !strings.Contains(got+"\n", want) {
			t.Errorf("the build environment lacks %q:\n%s", want, got)
		}
	}
}

func TestModeOptionsArePinned(t *testing.T) {
	asan, _ := ModeByName("asan")
	for _, want := range []string{"ASAN_OPTIONS=detect_leaks=1:halt_on_error=1:abort_on_error=0:print_summary=1:symbolize=1", "UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1:print_summary=1"} {
		if !slices.Contains(asan.RunEnv, want) {
			t.Errorf("asan mode lacks %q: %v", want, asan.RunEnv)
		}
	}
	ubsan, _ := ModeByName("ubsan")
	if !slices.Contains(ubsan.RunEnv, "UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1:print_summary=1") {
		t.Errorf("ubsan mode: %v", ubsan.RunEnv)
	}
	if !slices.Contains(ubsan.BuildEnv, "GOCPP_LDFLAGS=-fsanitize=undefined") {
		t.Errorf("ubsan mode links without UBSan: %v", ubsan.BuildEnv)
	}
	tsan, _ := ModeByName("tsan")
	if len(tsan.RunEnv) != 0 {
		t.Errorf("tsan mode must leave gocpp's -race defaults alone: %v", tsan.RunEnv)
	}
}

func TestOptionKnobs(t *testing.T) {
	s := &session{commonFlags: commonFlags{asanOpts: "quarantine_size_mb=1", ubsanOpts: "print_stacktrace=0", tsanOpts: "detect_deadlocks=1 history_size=4"}}
	for mode, want := range map[string][]string{
		"asan": {"ASAN_OPTIONS=detect_leaks=1:halt_on_error=1:abort_on_error=0:print_summary=1:symbolize=1:quarantine_size_mb=1", "UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1:print_summary=1:print_stacktrace=0", "TSAN_OPTIONS=detect_deadlocks=1 history_size=4"},
	} {
		m, _ := ModeByName(mode)
		if got := s.runner(m, "g++").Extra; !slices.Equal(got, want) {
			t.Errorf("%s: Extra = %q, want %q", mode, got, want)
		}
	}
	m, _ := ModeByName("tsan")
	if got := (&session{}).runner(m, "g++").Extra; len(got) != 0 {
		t.Errorf("no knobs, but Extra = %q", got)
	}
}

func TestBigOutputKeepsTheReportAtTheEnd(t *testing.T) {
	r := fakeRunner(t, "asan", "g++")
	r.OutLimit = 2000
	res := r.RunProgram(fakeProgram(t, "flood", "// fake-flood: 200000\n// fake-stderr: "+strings.ReplaceAll(strings.TrimRight(ubsanText, "\n"), "\n", `\n`)+"\n"))
	if res.Status != StatusReport || len(res.Reports) != 1 || res.Reports[0].Kind != UBSan {
		t.Fatalf("status %s reports %v", res.Status, res.Reports)
	}
	if !strings.Contains(res.Output, "bytes of output cut") || len(res.Output) > 6000 {
		t.Errorf("output not cut to head and tail (%d bytes)", len(res.Output))
	}
}

func TestLimitBufKeepsHeadAndTail(t *testing.T) {
	l := &limitBuf{n: 10}
	for _, p := range []string{"abc", "defgh", "ijklmnop", "qrs"} {
		l.Write([]byte(p))
	}
	if got, want := l.String(), "abcde\n[9 bytes of output cut]\nopqrs"; got != want {
		t.Errorf("limitBuf = %q, want %q", got, want)
	}
	small := &limitBuf{n: 100}
	small.Write([]byte("hello"))
	if small.String() != "hello" {
		t.Errorf("small = %q", small.String())
	}
}

func TestStdoutIsScannedToo(t *testing.T) {
	r := fakeRunner(t, "asan", "g++")
	res := r.RunProgram(fakeProgram(t, "joined", "// fake-stdout: "+strings.ReplaceAll(strings.TrimRight(asanText, "\n"), "\n", `\n`)+"\n"))
	if res.Status != StatusReport || res.Reports[0].Kind != ASan {
		t.Errorf("status %s reports %v", res.Status, res.Reports)
	}
}

func TestHeaderCycleLineIsExact(t *testing.T) {
	for src, want := range map[string]bool{
		"// lsan: cycle\n":               true,
		"// lsan: cycle  \n":             true,
		"// lsan: cycles\n":              false,
		"// lsan: cycle-ish\n":           false,
		"// lsan:\n":                     false,
		"//lsan: cycle\n":                false,
		"// not lsan: cycle\n":           false,
		"package main\n// lsan: cycle\n": false, // after the package clause: not a header line
	} {
		h, err := ParseHeader(src)
		if err != nil || h.Cycle != want {
			t.Errorf("ParseHeader(%q): Cycle = %v (%v), want %v", src, h != nil && h.Cycle, err, want)
		}
	}
}

// TestConcurrencyCorpusMatchesTheShellScript: the Go regexp selects the same programs of tests/run as
// tests/race/concurrency.sh, which make ci-race uses, and the pattern covers each of its alternatives.
func TestConcurrencyCorpusMatchesTheShellScript(t *testing.T) {
	for _, src := range []string{"\tgo f()\n", "x := func() { go func() {}() }\n", "c := make(chan int)\n", "\tselect {\n", "import \"sync\"\n", "import \"time\"\n"} {
		if !concurrencyRx.MatchString(src) {
			t.Errorf("concurrencyRx does not select %q", src)
		}
	}
	for _, src := range []string{"fmt.Println(\"go\")\n", "// go fish\nx := 1\n"} {
		if concurrencyRx.MatchString(src) {
			t.Errorf("concurrencyRx selects %q", src)
		}
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("no grep")
	}
	out, err := exec.Command(sh, filepath.Join(root, "tests/race/concurrency.sh")).Output()
	if err != nil {
		t.Skipf("concurrency.sh: %v", err)
	}
	want := strings.Split(strings.TrimSpace(string(out)), "|")
	ps, _, err := Selection{Root: root, Corpus: CorpusConcurrency, Phase: 99, Pending: true}.Pick()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range ps {
		if name, ok := strings.CutPrefix(p.Rel, "tests/run/"); ok {
			got = append(got, strings.TrimSuffix(name, ".gp"))
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	// the script lists every program; the tool leaves out the ones its header skips (build constraints, libraries)
	for _, g := range got {
		if !slices.Contains(want, g) {
			t.Errorf("%s: selected by the tool, not by concurrency.sh", g)
		}
	}
	if missing := len(want) - len(got); missing > len(want)/20 {
		t.Errorf("the tool selects %d of the script's %d programs", len(got), len(want))
	}
}

func TestEmptyShardIsNotAnError(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": "module x\n", "cmd/gocpp/main.go": "package main\n", "tests/run/only.gp": "// phase: 03\npackage main\n"})
	empty := ""
	for i := 0; i < 8; i++ {
		if !inShard("tests/run/only.gp", i, 8) {
			empty = fmt.Sprintf("%d/8", i)
			break
		}
	}
	code, _, stderr := runTool(t, "run", "-root", root, "-out", t.TempDir(), "-shard", empty, "-corpus", "run")
	if code != 0 || !strings.Contains(stderr, "has no program") {
		t.Errorf("an empty shard: exit %d\n%s", code, stderr)
	}
	if code, _, _ := runTool(t, "run", "-root", root, "-out", t.TempDir(), "-filter", "zzz"); code == 0 {
		t.Error("an empty selection without a shard must fail")
	}
}

func TestNightlyPassesTheShardToEveryCorpusStage(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("tests/run/p%02d.gp", i)] = "// phase: 08\npackage main\nfunc main() {\n\tgo f()\n}\n" // in the concurrency corpus too
	}
	total := map[string]int{}
	for _, shard := range []string{"0/3", "1/3", "2/3"} {
		root := nightlyRoot(t, false)
		writeTree(t, root, files)
		out := t.TempDir()
		if code, stdout, stderr := runTool(t, "nightly", "-root", root, "-out", out, "-cxx", "g++", "-stages", "asan,ubsan,tsan", "-shard", shard); code != 0 {
			t.Fatalf("shard %s: exit %d\n%s\n%s", shard, code, stdout, stderr)
		}
		for _, stage := range []string{"asan", "ubsan", "tsan"} {
			b, err := os.ReadFile(filepath.Join(out, stage+"-g++.json"))
			if err != nil {
				t.Fatal(err)
			}
			var sum Summary
			if err := json.Unmarshal(b, &sum); err != nil {
				t.Fatal(err)
			}
			if len(sum.Results) >= 13 {
				t.Errorf("shard %s, stage %s: %d programs: the shard was not applied", shard, stage, len(sum.Results))
			}
			total[stage] += len(sum.Results)
		}
	}
	// the three shards together are the whole corpus: 12 programs and the one of nightlyRoot (13; tsan: + 0 ok_ programs)
	if total["asan"] != 13 || total["ubsan"] != 13 || total["tsan"] != 13 {
		t.Errorf("the shards add up to %v, want 13 each", total)
	}
}

func TestNightlyStagesDoNotOverwriteEachOther(t *testing.T) {
	root := nightlyRoot(t, false)
	out := t.TempDir()
	if code, stdout, stderr := runTool(t, "nightly", "-root", root, "-out", out, "-cxx", "g++", "-stages", "canary,leaks,asan"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	for _, name := range []string{"canary-asan-g++.json", "canary-ubsan-g++.json", "canary-tsan-g++.json", "asan-g++.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	var canary, asan Summary
	for name, sum := range map[string]*Summary{"canary-asan-g++.json": &canary, "asan-g++.json": &asan} {
		b, _ := os.ReadFile(filepath.Join(out, name))
		if err := json.Unmarshal(b, sum); err != nil {
			t.Fatal(err)
		}
	}
	if canary.Corpus != "canary" || asan.Corpus != "run" {
		t.Errorf("corpora: canary file %q, asan file %q", canary.Corpus, asan.Corpus)
	}
}

func TestDeepStageNames(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"nightly", "-n", "-stages", "asan,deep"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "asan,deep-sanitize,deep-race") {
		t.Errorf("deep: exit %d %q %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := run([]string{"nightly", "-n", "-stages", "tsan", "-deep"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "tsan,deep-sanitize,deep-race") {
		t.Errorf("-deep: exit %d %q", code, out.String())
	}
}

func TestInterruptCleansUp(t *testing.T) {
	work := filepath.Join(t.TempDir(), "w")
	self, _ := os.Executable()
	var stderr bytes.Buffer
	cf := commonFlags{gocpp: self, work: work, jobs: 1}
	s, err := cf.open(&stderr)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := ModeByName("asan")
	r := s.runner(m, "g++")
	code := -1
	s.exit = func(c int) { code = c }
	// a work folder of this run, as a build would leave it while it runs
	if err := os.MkdirAll(filepath.Join(work, "tmp", "sanitize-123"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.interrupt()
	if code != 130 {
		t.Errorf("exit code %d, want 130", code)
	}
	if !r.Stopped() {
		t.Error("the runner was not stopped")
	}
	// work was given by the caller: it stays, but the session removes what it made
	s2 := &session{}
	dir := t.TempDir()
	s2.work = dir
	s2.cleanups = append(s2.cleanups, func() { os.RemoveAll(dir) })
	s2.exit = func(int) {}
	s2.interrupt()
	if _, err := os.Stat(dir); err == nil {
		t.Error("interrupt left the work folder")
	}
}

func TestOwnWorkFolderIsRemovedOnInterrupt(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	self, _ := os.Executable()
	var stderr bytes.Buffer
	cf := commonFlags{gocpp: self, jobs: 1}
	s, err := cf.open(&stderr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.work); err != nil {
		t.Fatal(err)
	}
	s.exit = func(int) {}
	s.interrupt()
	if _, err := os.Stat(s.work); err == nil {
		t.Errorf("the work folder %s survived the interrupt", s.work)
	}
}

// TestCommandsEndWhenTheProgramLeavesProcessesBehind: a program that leaves a process behind (the
// process group is killed after the run) does not leave it running.
func TestOrphansAreKilled(t *testing.T) {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc")
	}
	r := fakeRunner(t, "asan", "g++")
	pidFile := filepath.Join(t.TempDir(), "pid")
	res := r.RunProgram(fakeProgram(t, "orphan", "// fake-orphan: "+pidFile+"\n"))
	if res.Status != StatusOK {
		t.Fatalf("status %s: %s", res.Status, res.Detail)
	}
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid := strings.TrimSpace(string(b))
	// the process is gone, or a zombie that has not been reaped yet
	for i := 0; i < 100; i++ {
		stat, err := os.ReadFile("/proc/" + pid + "/stat")
		if err != nil || strings.Contains(string(stat), ") Z ") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	exec.Command("kill", "-9", pid).Run()
	t.Errorf("process %s left behind by the program still runs", pid)
}

func TestLeakFilePolicyCatchesASmallInjectedLeak(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := loadLeaks(root)
	if err != nil {
		t.Fatal(err)
	}
	lsan := []Report{{LSan, "detected memory leaks"}}
	for _, e := range entries {
		for c, ceil := range e.Ceiling {
			// a 77-byte block that is never freed: one more allocation than the program leaks at most
			v := JudgeLeak(lsan, &Leak{ceil.Bytes + e.Slack.Bytes + 77, ceil.Allocs + e.Slack.Allocs + 1}, ceil, e.Slack)
			if !v.Fail {
				t.Errorf("%s [%s]: an injected 77-byte leak would not be caught", e.Path, c)
			}
			v = JudgeLeak(lsan, &Leak{ceil.Bytes, ceil.Allocs + 1}, ceil, e.Slack)
			if !v.Fail {
				t.Errorf("%s [%s]: one more allocation would not be caught (slack %v)", e.Path, c, e.Slack)
			}
			if e.Slack.Bytes > maxByteSlack {
				t.Errorf("%s: byte slack %d is larger than %d", e.Path, e.Slack.Bytes, maxByteSlack)
			}
		}
	}
}

// ------------------------------------------------------------------------ review round 2

func TestJoinOptions(t *testing.T) {
	for _, tt := range []struct{ base, more, sep, want string }{
		{"", "a=1", " ", "a=1"},
		{"x=1", "a=1", ":", "x=1:a=1"},
		{"x=1 y=2", "a=1", " ", "x=1 y=2 a=1"},
	} {
		if got := joinOptions(tt.base, tt.more, tt.sep); got != tt.want {
			t.Errorf("joinOptions(%q, %q, %q) = %q, want %q", tt.base, tt.more, tt.sep, got, tt.want)
		}
	}
}

func TestShardWhereEverythingIsSkippedSaysSo(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": "module x\n", "cmd/gocpp/main.go": "package main\n",
		"tests/run/late.gp": "// phase: 12\npackage main\n"})
	shard := ""
	for i := 0; i < 4; i++ {
		if inShard("tests/run/late.gp", i, 4) {
			shard = fmt.Sprintf("%d/4", i)
		}
	}
	code, _, stderr := runTool(t, "run", "-root", root, "-out", t.TempDir(), "-shard", shard, "-corpus", "run", "-phase", "10")
	if code != 0 || !strings.Contains(stderr, "all 1 of its programs were skipped") || !strings.Contains(stderr, "nothing was checked") || strings.Contains(stderr, "has no program") {
		t.Errorf("a shard whose program is skipped: exit %d\n%s", code, stderr)
	}
	other := ""
	for i := 0; i < 4; i++ {
		if !inShard("tests/run/late.gp", i, 4) {
			other = fmt.Sprintf("%d/4", i)
		}
	}
	code, _, stderr = runTool(t, "run", "-root", root, "-out", t.TempDir(), "-shard", other, "-corpus", "run", "-phase", "10")
	if code != 0 || !strings.Contains(stderr, "has no program") || strings.Contains(stderr, "were skipped") {
		t.Errorf("an empty shard: exit %d\n%s", code, stderr)
	}
}
