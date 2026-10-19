// Command campaign runs the fuzz targets of gocpp for a time each, one after the other, and keeps
// what they find (Phase 15 step 15.8).
//
//	go run ./tools/fuzz/campaign -time 60s                      every target, a minute each
//	go run ./tools/fuzz/campaign -time 20m -targets Check,Emit  two targets
//	go run ./tools/fuzz/campaign -list
//
// make fuzz-short and make fuzz-long TARGET= TIME= are this command.
//
// A target runs as "go test -fuzz" in a process group of its own, at the lowest priority, with at
// most -parallel workers (4). When the engine finds a failing input, the runner reads the failure
// for a signature (the kind of the failure and the first stack frames in gocpp's code, without
// numbers), moves the input out of testdata/fuzz (where the engine wrote it, and where it would
// stop the next run at once) to fuzz-findings/<target>/, adds the finding to fuzz-findings.md, and
// lets the target go on for the rest of its time. A second input with a signature that is already
// known is dropped, counted in the summary.
//
// fuzz-findings/ and fuzz-findings.md are not committed. To keep a finding as a regression seed,
// copy its input to testdata/fuzz/<FuzzName>/ in the package of the target (docs/guides/fuzzing.md).
//
// The runner ends the process groups it started when it ends, also on SIGINT and SIGTERM.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

type target struct {
	name string // the name on the command line
	pkg  string // the package, relative to the repository
	fn   string // the fuzz function
	note string
	// warm is added to the time of the target: the engine first runs every seed (2400 to 2800 programs
	// for the pipeline targets, 30 to 50 seconds on a loaded machine), and that counts as fuzzing time.
	warm time.Duration
}

var targets = []target{
	{"Parse", "./tools/fuzz", "FuzzParse", "the parser: no panic, sane positions", 30 * time.Second},
	{"Format", "./tools/fuzz", "FuzzFormat", "gocpp fmt: parses, same program, idempotent", 30 * time.Second},
	{"Check", "./tools/fuzz", "FuzzCheck", "the checker: no panic, no internal error on a valid syntax", 90 * time.Second},
	{"Lower", "./tools/fuzz", "FuzzLower", "check, lower and emit: no panic, no refusal of a checked program", 90 * time.Second},
	{"Emit", "./tools/fuzz", "FuzzEmit", "emit: deterministic, #line, the sampled C++ compile", 90 * time.Second},
	{"Gen", "./tools/fuzz", "FuzzGen", "generated programs: they check, emit and compile", 0},
	{"LSP", "./tools/fuzz", "FuzzLSP", "the language server: edits and requests", 0},
	{"CppErrors", "./cmd/gocpp", "FuzzCppErrors", "the mapper of the C++ compilers' errors", 0},
	{"GoMod", "./internal/load/gocppmod", "FuzzParse", "gocpp.mod: parse, format", 0},
	{"GoModRecipe", "./internal/load/gocppmod", "FuzzParseRecipe", "gocpp.mod recipes", 0},
	{"GoModVendor", "./internal/load/gocppmod", "FuzzParseVendor", "vendor/modules.txt", 0},
}

func main() {
	var (
		list     = flag.Bool("list", false, "list the targets and end")
		names    = flag.String("targets", "", "the targets, comma separated (default: all)")
		dur      = flag.Duration("time", time.Minute, "the fuzzing time of each target (the pipeline targets get 30 to 90 s more, for the seeds that the engine runs first)")
		parallel = flag.Int("parallel", 4, "workers of the engine for a target (at most 4)")
		every    = flag.Int("compile-every", 0, "compile one program in N with g++ and clang++ (GOCPP_FUZZ_COMPILE_EVERY); 0: 1 in 40 for Emit and Gen, off for the others")
		outDir   = flag.String("out", "", "the folder for the findings (default: fuzz-findings in the repository)")
		maxFind  = flag.Int("max-findings", 20, "stop a target after this many different findings")
	)
	flag.Parse()
	if *list {
		for _, t := range targets {
			fmt.Printf("%-12s %-28s %s %s\n", t.name, t.pkg, t.fn, t.note)
		}
		return
	}
	if *parallel > 4 || *parallel < 1 {
		fmt.Fprintln(os.Stderr, "campaign: -parallel must be 1 to 4")
		os.Exit(2)
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "campaign:", err)
		os.Exit(2)
	}
	var run []target
	if *names == "" {
		run = targets
	} else {
		for _, n := range strings.Split(*names, ",") {
			n = strings.TrimSpace(strings.TrimPrefix(n, "Fuzz"))
			found := false
			for _, t := range targets {
				if strings.EqualFold(t.name, n) {
					run, found = append(run, t), true
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "campaign: no target %q (campaign -list)\n", n)
				os.Exit(2)
			}
		}
	}
	if *outDir == "" {
		*outDir = filepath.Join(root, "fuzz-findings")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c := &campaign{root: root, out: *outDir, parallel: *parallel, maxFind: *maxFind, seen: map[string]*finding{}}
	if err := c.loadSummary(); err != nil {
		fmt.Fprintln(os.Stderr, "campaign:", err)
		os.Exit(2)
	}
	for _, t := range run {
		if ctx.Err() != nil {
			break
		}
		n := *every
		if n == 0 && (t.name == "Emit" || t.name == "Gen") {
			n = 40
		}
		c.runTarget(ctx, t, *dur, n)
	}
	if err := c.writeSummary(); err != nil {
		fmt.Fprintln(os.Stderr, "campaign:", err)
	}
	fmt.Printf("campaign: %d findings in %s\n", len(c.seen), filepath.Join(root, "fuzz-findings.md"))
	if ctx.Err() != nil {
		os.Exit(130)
	}
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
	}
}

// A finding is one failure of a target, by signature.
type finding struct {
	Target string
	Sig    string
	Kind   string // the first line of the failure
	Input  string // the saved input
	Count  int
	Head   string // the start of the failure's output
	When   string
}

type campaign struct {
	root     string
	out      string
	parallel int
	maxFind  int
	seen     map[string]*finding // by target and signature
	order    []string
}

type runInfo struct {
	target string
	time   time.Duration
	execs  string // the last progress line
	found  int
	note   string
}

func (c *campaign) runTarget(ctx context.Context, t target, dur time.Duration, compileEvery int) {
	deadline := time.Now().Add(dur + t.warm)
	info := runInfo{target: t.name}
	start := time.Now()
	logDir := filepath.Join(c.out, "logs")
	os.MkdirAll(logDir, 0o755)
	newFound := 0
	for ctx.Err() == nil {
		rem := time.Until(deadline).Round(time.Second)
		if rem < 5*time.Second {
			break
		}
		if newFound >= c.maxFind {
			info.note = "stopped: too many findings"
			break
		}
		logPath := filepath.Join(logDir, fmt.Sprintf("%s-%s.log", t.name, time.Now().Format("150405")))
		out, err := c.fuzz(ctx, t, rem, compileEvery, logPath)
		info.execs = lastProgress(out)
		if err == nil {
			os.Remove(logPath)
			break
		}
		path := failingInput(out, c.root, t)
		if path == "" && bytes.Contains(out, []byte("context deadline exceeded")) && !bytes.Contains(out, []byte("--- FAIL")) {
			break // the engine's own time ran out while it was working on an input
		}
		if path == "" && ctx.Err() != nil {
			info.note = "interrupted"
			os.Remove(logPath)
			break
		}
		if path == "" {
			// The engine failed without an input (the package does not build, a setup error).
			info.note = "failed without an input: see " + logPath
			fmt.Fprintf(os.Stderr, "campaign: %s: %v (log %s)\n", t.name, err, logPath)
			break
		}
		if kind, _ := signature(out); strings.HasPrefix(kind, "fuzzing process hung or terminated") {
			// The engine does not show what a worker said when it died: the text and the stack of
			// fuzzkit.Guard (a hang, a memory blowup) or of a fatal error are in the output of the input
			// run alone, as a test. If it passes alone, the worker was killed from outside (the memory
			// limit of the machine, a signal): not a finding.
			failed, again := c.reproduces(ctx, t, path)
			if !failed {
				fmt.Printf("campaign: %s: the worker died on an input that passes alone: dropped (%s)\n", t.name, filepath.Base(path))
				os.Remove(path)
				info.note += " unreproduced worker death;"
				os.Remove(logPath)
				continue
			}
			// The signature and the text of the finding come from the run alone (kind HANG or MEMORY
			// BLOWUP, or a panic, and the stacks).
			out = append([]byte("(the engine: fuzzing process hung or terminated unexpectedly; the input run alone:)\n"), again...)
		}
		if f := c.record(t, out, path); f != nil && f.Count == 1 {
			newFound++
		}
		os.Remove(logPath)
	}
	info.time = time.Since(start).Round(time.Second)
	info.found = newFound
	if f, err := os.OpenFile(filepath.Join(c.out, "runs.tsv"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "%s\t%s\t%v\t%d\t%s\t%s\n", time.Now().Format("2006-01-02 15:04"), info.target, info.time, info.found,
			strings.ReplaceAll(strings.TrimPrefix(info.execs, "fuzz: "), "\t", " "), info.note)
		f.Close()
	}
	c.writeSummary()
}

// reproduces runs the input at path (in the folder of the target's package, as the engine wrote it)
// as an ordinary test, and reports whether it fails (or hangs).
func (c *campaign) reproduces(ctx context.Context, t target, path string) (failed bool, out []byte) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-timeout", "150s", "-run", "^"+t.fn+"$/^"+regexp.QuoteMeta(filepath.Base(path))+"$", t.pkg)
	cmd.Dir = c.root
	cmd.Env = append(os.Environ(), "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail", "GOCPP_FUZZ_COMPILE_EVERY=1", "GOCPP_FUZZ_CACHE="+filepath.Join(c.out, "cache"))
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	build.InGroup(cmd)
	err := cmd.Run()
	build.KillGroup(cmd)
	out = buf.Bytes()
	if len(out) > 1<<20 {
		out = out[len(out)-1<<20:]
	}
	return err != nil, out
}

// fuzz runs the engine for one target for at most rem, and returns its output.
func (c *campaign) fuzz(ctx context.Context, t target, rem time.Duration, compileEvery int, logPath string) ([]byte, error) {
	args := []string{"-n", "19", "go", "test", "-run", "^$", "-fuzz", "^" + t.fn + "$",
		"-fuzztime", rem.String(), "-parallel", fmt.Sprint(c.parallel), "-fuzzminimizetime", "20s", "-timeout", "0", t.pkg}
	cmd := exec.Command("nice", args...)
	cmd.Dir = c.root
	cmd.Env = append(os.Environ(),
		"GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail",
		fmt.Sprintf("GOCPP_FUZZ_COMPILE_EVERY=%d", compileEvery),
		"GOCPP_FUZZ_CACHE="+filepath.Join(c.out, "cache"),
	)
	var buf bytes.Buffer
	lf, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	defer lf.Close()
	w := io.MultiWriter(&buf, lf)
	cmd.Stdout, cmd.Stderr = w, w
	build.InGroup(cmd)
	cmd.Cancel = nil // InGroup is for CommandContext; the runner kills the group itself
	setPdeathsig(cmd)
	fmt.Printf("campaign: %s for %v (log %s)\n", t.name, rem, logPath)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		build.KillGroup(cmd)
		err = <-done
		if err == nil {
			err = ctx.Err()
		}
	case <-time.After(rem + 10*time.Minute):
		// The engine's own limit is the fuzz time; this is the last resort.
		build.KillGroup(cmd)
		err = fmt.Errorf("timed out")
		<-done
	}
	build.KillGroup(cmd)
	return buf.Bytes(), err
}

var (
	failingRx  = regexp.MustCompile(`(?m)Failing input written to (\S+)`)
	progressRx = regexp.MustCompile(`(?m)^fuzz: elapsed: .*$`)
	frameRx    = regexp.MustCompile(`(?m)^\s*(github\.com/huuhait/gocpp/\S+?)\((?:0x[0-9a-f]+|\.\.\.|\)|\d)`)
	digitsRx   = regexp.MustCompile(`0x[0-9a-f]+|\d+`)
)

func lastProgress(out []byte) string {
	m := progressRx.FindAll(out, -1)
	if len(m) == 0 {
		return ""
	}
	return string(m[len(m)-1])
}

// failingInput returns the path of the input that the engine wrote, as an absolute path.
func failingInput(out []byte, root string, t target) string {
	m := failingRx.FindSubmatch(out)
	if m == nil {
		return ""
	}
	p := string(m[1])
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, strings.TrimPrefix(t.pkg, "./"), p)
	}
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// signature returns the kind of the failure in out, and a hash of it with the frames of gocpp's
// code that lead to it.
func signature(out []byte) (kind, sig string) {
	s := string(out)
	if i := strings.Index(s, "--- FAIL"); i >= 0 {
		s = s[i:]
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(nil, 4<<20)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	for i, l := range lines {
		tl := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(tl, "panic:"), strings.HasPrefix(tl, "fatal error:"), strings.HasPrefix(tl, "fuzz: HANG"),
			strings.HasPrefix(tl, "fuzz: MEMORY"), strings.HasPrefix(tl, "fuzzing process hung"), strings.HasPrefix(tl, "fuzzing process terminated"):
			kind = tl
		case kind == "" && regexp.MustCompile(`^\w+\.go:\d+: `).MatchString(tl):
			// t.Fatalf: the message, and the line after it that continues it
			kind = tl
			if i+1 < len(lines) && len(kind) < 40 {
				kind += " " + strings.TrimSpace(lines[i+1])
			}
		}
		if kind != "" {
			break
		}
	}
	if kind == "" {
		kind = "failure"
	}
	if i := strings.Index(kind, " goroutine "); i > 0 {
		kind = kind[:i]
	}
	if len(kind) > 160 {
		kind = kind[:160]
	}
	norm := digitsRx.ReplaceAllString(kind, "N")
	h := sha256.New()
	h.Write([]byte(norm))
	n := 0
	for _, m := range frameRx.FindAllStringSubmatch(s, -1) {
		if strings.Contains(m[1], "/tools/fuzz") || strings.Contains(m[1], "testing.") {
			continue
		}
		h.Write([]byte(m[1]))
		if n++; n == 3 {
			break
		}
	}
	return kind, hex.EncodeToString(h.Sum(nil))[:10]
}

// record adds the failure of out, whose input is at path, to the findings, and moves the input out
// of the folder where the engine wrote it.
func (c *campaign) record(t target, out []byte, path string) *finding {
	kind, sig := signature(out)
	key := t.name + "/" + sig
	if f := c.seen[key]; f != nil {
		f.Count++
		os.Remove(path)
		fmt.Printf("campaign: %s: known finding %s (%d times): %s\n", t.name, sig, f.Count, kind)
		return f
	}
	dir := filepath.Join(c.out, t.name)
	os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, sig+"-"+filepath.Base(path))
	data, err := os.ReadFile(path)
	if err == nil {
		err = os.WriteFile(dst, data, 0o644)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "campaign: saving %s: %v\n", path, err)
		dst = path
	} else {
		os.Remove(path)
	}
	head := string(out)
	if i := strings.Index(head, "--- FAIL"); i >= 0 {
		head = head[i:]
	}
	if len(head) > 3000 {
		head = head[:3000] + "\n..."
	}
	f := &finding{Target: t.name, Sig: sig, Kind: kind, Input: dst, Count: 1, Head: head, When: time.Now().Format("2006-01-02 15:04")}
	os.WriteFile(dst+".txt", []byte(kind+"\n"+head), 0o644)
	c.seen[key] = f
	c.order = append(c.order, key)
	fmt.Printf("campaign: %s: NEW finding %s: %s\n  input %s\n", t.name, sig, kind, dst)
	c.writeSummary()
	return f
}

func (c *campaign) summaryPath() string { return filepath.Join(c.root, "fuzz-findings.md") }

// loadSummary reads the signatures of the findings of earlier campaigns (the folders of fuzz-findings
// hold their inputs), so that a second campaign reports only what is new.
func (c *campaign) loadSummary() error {
	entries, err := os.ReadDir(c.out)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "logs" || e.Name() == "cache" {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(c.out, e.Name()))
		for _, f := range files {
			sig, _, ok := strings.Cut(f.Name(), "-")
			if !ok || strings.HasSuffix(f.Name(), ".txt") {
				continue
			}
			key := e.Name() + "/" + sig
			if c.seen[key] == nil {
				fd := &finding{Target: e.Name(), Sig: sig, Kind: "(from an earlier campaign)", Input: filepath.Join(c.out, e.Name(), f.Name()), Count: 0}
				if b, err := os.ReadFile(fd.Input + ".txt"); err == nil {
					fd.Kind, fd.Head, _ = strings.Cut(string(b), "\n")
				}
				c.seen[key] = fd
				c.order = append(c.order, key)
			}
		}
	}
	return nil
}

func (c *campaign) writeSummary() error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Fuzz findings\n\nWritten by tools/fuzz/campaign at %s.\n\n", time.Now().Format("2006-01-02 15:04"))
	b.WriteString("## Runs\n\n| when | target | time | new findings | last progress |\n|---|---|---|---|---|\n")
	if data, err := os.ReadFile(filepath.Join(c.out, "runs.tsv")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if p := strings.Split(line, "\t"); len(p) == 6 { // (a line ends with a tab when its note is empty: no trimming)
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s %s |\n", p[0], p[1], p[2], p[3], p[4], p[5])
			}
		}
	}
	b.WriteString("\n## Findings\n\n")
	keys := append([]string(nil), c.order...)
	sort.Strings(keys)
	for _, k := range keys {
		f := c.seen[k]
		fmt.Fprintf(&b, "### %s %s: %s\n\n- found %s, %d time(s)\n- input: %s\n", f.Target, f.Sig, f.Kind, f.When, f.Count, f.Input)
		if f.Head != "" {
			fmt.Fprintf(&b, "\n```\n%s\n```\n", strings.TrimSpace(f.Head))
		}
		b.WriteString("\n")
	}
	return os.WriteFile(c.summaryPath(), []byte(b.String()), 0o644)
}
