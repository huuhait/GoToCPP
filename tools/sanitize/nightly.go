package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The nightly job (step 15.11): every sanitizer over the corpora with both compilers, the canaries
// that prove the sanitizers still see an error, the leak audit, and Go's own tests of the standard
// packages. It runs every stage, also after one failed, and exits 1 when any failed, so that one
// night's report has everything.
// Stages:
//
//	canary  programs that are wrong on purpose (testdata/canary) must be reported, clean controls must be silent
//	leaks   the leak audit of leaks.txt (LeakSanitizer ON for every "// lsan: cycle" program, under its ceiling)
//	asan    tests/run with -asan: AddressSanitizer, LeakSanitizer and UndefinedBehaviorSanitizer
//	ubsan   tests/run with UBSan alone, any report ends the program
//	tsan    the concurrency family of tests/run and the silent programs of tests/race with -race
//	stdlib  make stdlib-nightly: go run ./tools/stdgocount -check and rangegen_shards.py (step 9.10,
//	        ADR-113; no sanitizer; its time is not measured yet); -sample leaves it out unless -stages names it
//	deep-sanitize  make ci-sanitize: the interop, diff, zoo and example programs and the emitter's own tests
//	               with -asan (only with -deep or by name: it takes hours)
//	deep-race      make ci-race: tests/race with both compilers, the concurrency programs with -race, the
//	               runtime's ThreadSanitizer unit tests (the same)
var nightlyStages = []string{"canary", "leaks", "asan", "ubsan", "tsan", "stdlib", "deep-sanitize", "deep-race"}

type stageResult struct {
	Name   string
	Err    error
	Took   time.Duration
	Stdout string
}

func cmdNightly(args []string, stdout, stderr io.Writer) error {
	var cf commonFlags
	fs := newFlagSet("nightly", stderr)
	cf.register(fs)
	cf.cxx = "g++,clang++"
	fs.Lookup("cxx").DefValue = cf.cxx
	stages := fs.String("stages", "canary,leaks,asan,ubsan,tsan,stdlib", "the stages to run (\"deep\" is both deep stages), in this order: "+strings.Join(nightlyStages, ", "))
	deep := fs.Bool("deep", false, "also run the deep stages (make ci-sanitize and make ci-race)")
	sample := fs.Bool("sample", false, "run the sanitizer stages on the short sample corpus (tools/sanitize/sample.txt), without the stdlib stage unless -stages names it: a smoke test of the job")
	phase := fs.Int("phase", 99, "the highest phase (// phase: NN) to run (default: all; the skipped programs are counted in the output)")
	pending := fs.Bool("pending", false, "also run the programs that say // pending:")
	shard := fs.String("shard", "", "i/n: only the programs of shard i of n in the asan, ubsan and tsan stages")
	limit := fs.Int("limit", 0, "at most this many programs in each corpus stage")
	dry := fs.Bool("n", false, "check the flags, print the stages and the compilers, and exit")
	if err := fs.Parse(args); err != nil {
		return usageError("%v", err)
	}
	if fs.NArg() > 0 {
		return usageError("nightly takes no arguments")
	}
	if cf.jobs < 1 {
		return usageError("-j %d: want 1 or more", cf.jobs)
	}
	want := strings.Split(*stages, ",")
	named := false // -stages was given
	fs.Visit(func(f *flag.Flag) { named = named || f.Name == "stages" })
	if *sample && !named {
		// The smoke test of the job takes minutes; Go's tests of the standard packages are not part of it.
		want = slices.DeleteFunc(want, func(w string) bool { return w == "stdlib" })
	}
	for i, w := range want { // "deep" names both deep stages
		if w == "deep" {
			want = slices.Replace(want, i, i+1, "deep-sanitize", "deep-race")
			break
		}
	}
	if *deep {
		for _, d := range []string{"deep-sanitize", "deep-race"} {
			if !slices.Contains(want, d) {
				want = append(want, d)
			}
		}
	}
	for _, w := range want {
		if !slices.Contains(nightlyStages, w) {
			return usageError("unknown stage %q (want %s)", w, strings.Join(nightlyStages, ", "))
		}
	}
	if *dry {
		var run []string
		for _, name := range nightlyStages {
			if slices.Contains(want, name) {
				run = append(run, name)
			}
		}
		fmt.Fprintf(stdout, "nightly: stages %s, compilers %s, phase <= %d, shard %q, sample %v, -j %d\n",
			strings.Join(run, ","), nameOr(cf.cxx, "default"), *phase, *shard, *sample, cf.jobs)
		return nil
	}
	s, err := cf.open(stderr)
	if err != nil {
		return err
	}
	defer s.close()
	corpus := func(mode, normal string) Selection {
		sel := Selection{Corpus: normal, Phase: *phase, Pending: *pending, Shard: *shard, Limit: *limit}
		if *sample {
			sel.Corpus = CorpusSample
		}
		return sel
	}
	var results []stageResult
	for _, name := range nightlyStages {
		if !slices.Contains(want, name) {
			continue
		}
		fmt.Fprintf(stderr, "sanitize nightly: stage %s\n", name)
		s.stage = name
		start := time.Now()
		var out strings.Builder
		mw := io.MultiWriter(&out, stdout)
		var err error
		switch name {
		case "canary":
			for _, m := range ModeNames() {
				mode, _ := ModeByName(m)
				if e := runCorpus(s, mode, Selection{Corpus: CorpusCanary, Phase: 99, Pending: true}, mw, stderr); e != nil && err == nil {
					err = fmt.Errorf("%s: %v", m, e)
				}
			}
		case "leaks":
			err = nightlyLeaks(s, mw, stderr)
		case "asan":
			mode, _ := ModeByName("asan")
			err = runCorpus(s, mode, corpus("asan", CorpusRun), mw, stderr)
		case "ubsan":
			mode, _ := ModeByName("ubsan")
			err = runCorpus(s, mode, corpus("ubsan", CorpusRun), mw, stderr)
		case "tsan":
			mode, _ := ModeByName("tsan")
			err = runCorpus(s, mode, corpus("tsan", CorpusConcurrency), mw, stderr)
		case "stdlib":
			err = nightlyDeep(s, "stdlib-nightly", mw, stderr)
		case "deep-sanitize":
			err = nightlyDeep(s, "ci-sanitize", mw, stderr)
		case "deep-race":
			err = nightlyDeep(s, "ci-race", mw, stderr)
		}
		results = append(results, stageResult{name, err, time.Since(start).Round(time.Second), out.String()})
	}
	text := nightlyReport(results, &cf)
	fmt.Fprint(stdout, text)
	if err := os.MkdirAll(cf.out, 0o755); err == nil {
		os.WriteFile(filepath.Join(cf.out, "nightly.md"), []byte(text), 0o644)
	}
	failed := 0
	for _, r := range results {
		if r.Err != nil {
			failed++
		}
	}
	if failed > 0 {
		return exitError{1, fmt.Sprintf("nightly: %d of %d stages failed", failed, len(results))}
	}
	return nil
}

// nightlyLeaks runs the static check and then the audit.
func nightlyLeaks(s *session, stdout, stderr io.Writer) error {
	entries, err := loadLeaks(s.root)
	if err != nil {
		return err
	}
	markers, err := FindMarkers(s.root)
	if err != nil {
		return err
	}
	if probs := CheckLeaks(entries, markers); len(probs) > 0 {
		for _, p := range probs {
			fmt.Fprintln(stderr, "leaks:", p)
		}
		return fmt.Errorf("%d problems in %s", len(probs), leaksFile)
	}
	return auditLeaks(s, entries, markers, nil, false, false, stdout, stderr)
}

// nightlyDeep runs the make target of a stage: the deep sanitizer gates, and stdlib-nightly.
func nightlyDeep(s *session, target string, stdout, stderr io.Writer) error {
	cmd := exec.Command("make", target)
	cmd.Dir = s.root
	cmd.Stdout, cmd.Stderr = stderr, stderr
	inGroup(cmd)
	err := cmd.Start()
	if err == nil {
		s.deep.track(cmd, true)
		err = cmd.Wait()
		s.deep.track(cmd, false)
	}
	killGroup(cmd)
	if err != nil {
		return fmt.Errorf("make %s: %v", target, err)
	}
	fmt.Fprintf(stdout, "make %s: passed\n", target)
	return nil
}

// nightlyReport renders the stages as a Markdown table, then the failures.
func nightlyReport(rs []stageResult, cf *commonFlags) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Nightly sanitizers (compilers: %s)\n\n| stage | result | time |\n|---|---|---|\n", nameOr(cf.cxx, "default"))
	for _, r := range rs {
		res := "passed"
		if r.Err != nil {
			res = "FAILED"
		}
		fmt.Fprintf(&b, "| %s | %s | %v |\n", r.Name, res, r.Took)
	}
	for _, r := range rs {
		if r.Err != nil {
			fmt.Fprintf(&b, "\n%s: %v\n", r.Name, r.Err)
		}
	}
	return b.String()
}
