package probe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// The tests of the cache with a compiler that is the test binary itself (the helper-process
// pattern, so that they run on every system, Windows included): when GOCPP_FAKE_CXX is set and
// the arguments are a compiler's (-MF), TestMain runs fakeCompiler instead of the tests. It counts
// its runs, reads the headers a probe file includes ("x.hpp" in FAKE_INC) and writes them to the
// dependency file as clang does, answers every probe line with a type that names the function
// of the line, and fails the lines that say ERR.

// fakeCompiler is the compiler. The environment (set by newFake): FAKE_COUNTER (a file that gets a
// line per run: "run", or "E" for a preprocessor-only run), FAKE_INC (the folder of the headers),
// FAKE_SYS (the compiler's own include folder, for -v), FAKE_SLEEP (seconds of a probe run), FAKE_DI (hang, cut, long or noenter: what the -dI run does wrong),
// FAKE_HELPER (a line number that the notes of an error point at), FAKE_BIG (print dump for ever), FAKE_BARRIER
// (a file: the compile waits for the first -E -dI run to create it). FAKE_DI_BLOCKED is written after
// the includes output is flushed; the run then waits for FAKE_DI_RELEASE or cancellation. FAKE_DI_HUNG
// (a file) gets a line when a FAKE_DI=hang run has printed its lines and waits for the time limit.
func fakeCompiler(args []string) int {
	var dep, src string
	mode, verbose, mg, di := "run", false, false, false
	memberCompletion := false
	for i, a := range args {
		if strings.HasPrefix(a, "-code-completion-at=") {
			memberCompletion = true
		}
		if a == "c++-header" && i > 0 && args[i-1] == "-x" {
			return fakeHeaderBuild(args)
		}
		switch a {
		case "-E", "-M":
			mode = "E"
		case "-MG":
			mg = true
		case "-dI":
			di = true
		case "-v":
			verbose = true
		case "-MF":
			if i+1 < len(args) {
				dep = args[i+1]
			}
		}
		src = a
	}
	if memberCompletion {
		mode = "member"
	}
	if f, err := os.OpenFile(os.Getenv("FAKE_COUNTER"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		if verbose {
			fmt.Fprintln(f, "V") // the toolchain's and the search path's questions
		} else if mg {
			fmt.Fprintln(f, "MG")
		} else if di {
			fmt.Fprintln(f, "D") // the question about the includes of the preamble
		} else {
			fmt.Fprintln(f, mode)
		}
		f.Close()
	}
	if code := fakeRendezvous(args, src); code != 0 {
		return code
	}
	if barrier := os.Getenv("FAKE_BARRIER"); barrier != "" {
		// A compile waits until the includes question has been asked (a run of -E -dI creates the file): a
		// question that is asked only after the compile ends makes the compile fail.
		if di {
			os.WriteFile(barrier, nil, 0o644)
		} else if mode == "run" {
			// (a minute: a bound that only a question that never comes reaches)
			for deadline := time.Now().Add(time.Minute); ; {
				if _, err := os.Stat(barrier); err == nil {
					break
				}
				if time.Now().After(deadline) {
					fmt.Fprintln(os.Stderr, "clang: error: the includes question did not come while the compile ran")
					return 2
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
	}
	if mode == "run" && strings.Contains(string(mustRead(src)), "FAILNOW") {
		fmt.Fprintln(os.Stderr, "clang: error: the compiler fell over")
		return 2
	}
	if mode == "run" {
		if p := os.Getenv("FAKE_START"); p != "" {
			if f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				fmt.Fprintln(f, "start")
				f.Close()
			}
		}
		if secs, err := strconv.ParseFloat(os.Getenv("FAKE_SLEEP"), 64); err == nil && secs > 0 {
			time.Sleep(time.Duration(secs * float64(time.Second)))
		}
	}
	text, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "clang: error: no such file:", src)
		return 2
	}
	if edit := os.Getenv("FAKE_EDIT"); edit != "" && mode == "run" {
		// A header is edited while the compiler runs (once), after the compiler has read it:
		// "path|contents", FAKE_EDIT_KEEP_MTIME=1 puts the old modification time back.
		defer func() {
			path, contents, _ := strings.Cut(edit, "|")
			once := path + ".edited"
			if _, err := os.Stat(once); err == nil {
				return
			}
			os.WriteFile(once, nil, 0o644)
			st, _ := os.Stat(path)
			os.WriteFile(path, []byte(contents), 0o644)
			if os.Getenv("FAKE_EDIT_KEEP_MTIME") != "" && st != nil {
				os.Chtimes(path, st.ModTime(), st.ModTime())
			}
		}()
	}
	// The headers: the ones that the file includes, and those they include (the headers of FAKE_INC).
	var deps, missingNames []string
	seen := map[string]bool{}
	// With -dI the standard output has the directives and the markers of the files, as clang -E -dI writes them.
	dout := bufio.NewWriter(os.Stdout)
	if !di {
		dout = bufio.NewWriter(io.Discard)
	}
	var scan func(text []byte, file string)
	scan = func(text []byte, file string) {
		for _, m := range regexp.MustCompile(`(?m)^#include "(.*)"\r?$`).FindAllStringSubmatch(string(text), -1) {
			p := filepath.Join(os.Getenv("FAKE_INC"), m[1])
			fmt.Fprintf(dout, "#include \"%s\" /* clang -E -dI */\n", m[1])
			if seen[p] {
				continue
			}
			seen[p] = true
			if b, err := os.ReadFile(p); err == nil {
				deps = append(deps, p)
				if os.Getenv("FAKE_DI") != "noenter" { // (an output with no file entered although headers were read)
					fmt.Fprintf(dout, "# 1 \"%s\" 1\n", markerPath(p))
				}
				scan(b, p)
				if os.Getenv("FAKE_DI") != "noenter" {
					fmt.Fprintf(dout, "# 2 \"%s\" 2\n", markerPath(file))
				}
			} else {
				missingNames = append(missingNames, m[1])
			}
		}
	}
	fmt.Fprintf(dout, "# 1 \"%s\"\n", markerPath(src))
	scan(text, src)
	dout.Flush()
	if di {
		if blocked := os.Getenv("FAKE_DI_BLOCKED"); blocked != "" {
			os.WriteFile(blocked, nil, 0o644)
			deadline := time.Now().Add(time.Minute)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(os.Getenv("FAKE_DI_RELEASE")); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	switch os.Getenv("FAKE_DI") {
	case "hang": // a compiler that is stopped at the time limit, after whole lines
		if di {
			dout.Flush()
			if hung := os.Getenv("FAKE_DI_HUNG"); hung != "" {
				// (tells the test that the lines are out and the run waits for its limit)
				if f, err := os.OpenFile(hung, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
					f.WriteString("hung\n")
					f.Close()
				}
			}
			time.Sleep(time.Minute)
		}
	case "cut": // an output that ends in the middle of a line
		if di {
			fmt.Fprint(dout, "# 1 \"cut")
		}
	case "long": // a directive line that no real output has
		if di {
			fmt.Fprintf(dout, "#%s\n", strings.Repeat("x", 2<<20))
		}
	}
	defer dout.Flush()
	missing := ""
	if len(missingNames) > 0 {
		missing = missingNames[0]
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "#include <...> search starts here:\n")
		for _, d := range strings.Split(os.Getenv("FAKE_SYS"), "|") {
			fmt.Fprintf(os.Stderr, " %s\n", d)
		}
		fmt.Fprintf(os.Stderr, "End of search list.\n")
	}
	if missing != "" && !mg {
		fmt.Fprintf(os.Stderr, "gp_probe.cpp:3:10: fatal error: '%s' file not found\n", missing)
		return 1
	}
	if os.Getenv("FAKE_BIG") != "" && mode == "run" {
		line := []byte(`{"name":"gp_d0_Big","pad":"` + strings.Repeat("x", 64) + `"}` + "\n")
		for {
			if _, err := os.Stdout.Write(line); err != nil {
				return 1
			}
		}
	}
	if dep != "" {
		os.WriteFile(dep, []byte("gp_probe: "+strings.Join(append(deps, missingNames...), " ")+"\n"), 0o644)
	}
	if memberCompletion {
		return fakeMemberCompletion()
	}
	if slices.Contains(args, "-ast-list") {
		for _, header := range deps {
			fmt.Print(string(mustRead(header)))
		}
		return 0
	}
	if slices.Contains(args, "-dM") && len(deps) > 0 {
		fmt.Println("#define HeaderMacro 1")
	}
	if mode == "E" {
		return 0
	}
	if strings.Contains(string(text), "__debug dump") {
		return fakeLocateDump(string(text), deps)
	}
	for _, arg := range args {
		// (the JSON dump of the probes has a filter too: only the text dump is a tag's)
		name, ok := strings.CutPrefix(arg, "-ast-dump-filter=")
		if ok && slices.Contains(args, "-ast-dump") {
			return fakeTagDump(name, deps)
		}
	}
	if strings.Contains(string(text), "gocpp_member_validation_complete") {
		return fakeMemberValidation(string(text), src)
	}
	if strings.Contains(string(text), "gocpp_index_validation_complete") {
		line := strings.Count(string(text), "\n")
		fmt.Fprintf(os.Stderr, "%s:%d:1: error: gocpp_index_validation_complete\n", src, line)
		return 1
	}
	helper, _ := strconv.Atoi(os.Getenv("FAKE_HELPER"))
	probeRx := regexp.MustCompile(`^using (gp_probe_\d+(?:_alt)?|gp_decl_[0-9a-f]+) = `)
	fnRx := regexp.MustCompile(`::f[0-9A-Za-z]+`)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for n, line := range strings.Split(string(text), "\n") {
		m := probeRx.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if strings.Contains(line, "ERR") {
			fmt.Fprintf(os.Stderr, "gp_probe.cpp:%d:5: error: boom in %s\n", n+1, m[1])
			fmt.Fprintf(os.Stderr, "gp_probe.cpp:%d:2: note: candidate here\n", n+1)
			fmt.Fprintf(os.Stderr, "gp_probe.cpp:%d:3: note: helper note\n", helper)
			fmt.Fprintf(os.Stderr, "gp_probe.cpp:%d:4: note: next line\n", n+2)
			fmt.Fprintf(os.Stderr, "other.hpp:9:1: note: in a header\n")
			continue
		}
		if strings.HasPrefix(m[1], "gp_decl_") {
			continue // an alias has no probe of its own
		}
		typ := "int"
		if f := fnRx.FindString(line); f != "" {
			typ = "R_" + f[2:]
			if h := os.Getenv("FAKE_HDR_TYPE"); h != "" {
				// the type has the letters of a header's contents: a result that depends on it
				b, _ := os.ReadFile(filepath.Join(os.Getenv("FAKE_INC"), h))
				typ += "_" + regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(string(b), "")
			}
		}
		fmt.Fprintf(out, `{"id":"0x1","kind":"TypeAliasDecl","name":"%s","type":{"desugaredQualType":"gocpp_probe_internal_c<%s>","qualType":"gocpp_probe_internal_c<decltype((x))>"}}`+"\n", m[1], typ)
	}
	return 0
}

// fakeRendezvous makes the runs of one product wait for each other: with FAKE_RENDEZVOUS (a
// folder), FAKE_RENDEZVOUS_RUNS (a number) and FAKE_RENDEZVOUS_KIND ("index" for the four runs of a
// header index, "member" for the completion and the validation of a member query), a matching
// run leaves a file in the folder and waits until that many runs have left theirs. A run is only
// let go when the others have started, so a product that runs them one after the other fails
// (the wait ends with an error) instead of passing slowly.
func fakeRendezvous(args []string, src string) int {
	dir := os.Getenv("FAKE_RENDEZVOUS")
	want, _ := strconv.Atoi(os.Getenv("FAKE_RENDEZVOUS_RUNS"))
	if dir == "" || want == 0 || !rendezvousRun(os.Getenv("FAKE_RENDEZVOUS_KIND"), args, src) {
		return 0
	}
	os.WriteFile(filepath.Join(dir, strconv.Itoa(os.Getpid())), nil, 0o644)
	deadline := time.Now().Add(time.Minute) // (a bound that only runs that never meet reach)
	for time.Now().Before(deadline) {
		if entries, _ := os.ReadDir(dir); len(entries) >= want {
			return 0
		}
		time.Sleep(5 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "clang: error: the other runs of the product did not start")
	return 2
}

func rendezvousRun(kind string, args []string, src string) bool {
	switch kind {
	case "index":
		return slices.Contains(args, "-ast-list") || slices.Contains(args, "-dM") ||
			slices.Contains(args, "-P")
	case "member":
		if slices.ContainsFunc(args, func(a string) bool {
			return strings.HasPrefix(a, "-code-completion-at=")
		}) {
			return true
		}
		text := string(mustRead(src))
		return slices.Contains(args, "-fsyntax-only") &&
			!strings.Contains(text, "gocpp_member_validation_complete") &&
			!strings.Contains(text, "__debug dump")
	}
	return false
}

// markerPath is a file name as clang writes it in a line marker: with the backslashes (Windows) and the
// quotes escaped.
func markerPath(p string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(p)
}

func mustRead(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

// isFakeCompiler reports whether this process was started as the fake compiler.
func isFakeCompiler() bool {
	return os.Getenv("GOCPP_FAKE_CXX") != "" && slices.Contains(os.Args, "-MF")
}

type fake struct {
	cxx     build.CXX
	counter string
	inc     string // the folder of the headers the probe files include
	cache   string // the cache folder (typed-v1 inside)
	dir     string // the package folder
}

// goCXX makes a compiler of the test binary: a hard link to it (a copy where links are not
// possible) with a name of its own, so that every test has a compiler of its own.
func goCXX(t testing.TB) build.CXX {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "clang++"+filepath.Ext(exe))
	// (Windows cannot delete a link to a running program, so there it is a copy.)
	if runtime.GOOS == "windows" || os.Link(exe, p) != nil {
		copyFile(t, exe, p)
	}
	return build.CXX{Path: p, Family: build.Clang, Version: "fake"}
}

func copyFile(t testing.TB, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o755); err != nil {
		t.Fatal(err)
	}
}

// newFake makes a fake compiler, a cache folder, and sets the environment of the tests.
func newFake(t *testing.T) *fake {
	t.Helper()
	f := &fake{counter: filepath.Join(t.TempDir(), "count"), inc: t.TempDir(), dir: t.TempDir()}
	f.cxx = goCXX(t)
	f.cache = filepath.Join(t.TempDir(), "typed-v1")
	// A test that hangs (a lock that is never let go) fails the run at once.
	watchdog := time.AfterFunc(3*time.Minute, func() { panic("the test hung: " + t.Name()) })
	t.Cleanup(func() { watchdog.Stop() })
	// A lock that a run leaves behind would make the next run wait for it: fail at once.
	t.Cleanup(func() {
		if entries, _ := os.ReadDir(filepath.Join(f.cache, "locks")); len(entries) != 0 {
			t.Errorf("locks left behind: %v", entries)
		}
	})
	t.Setenv("GOCPP_FAKE_CXX", "1")
	t.Setenv("FAKE_COUNTER", f.counter)
	t.Setenv("FAKE_INC", f.inc)
	t.Setenv("FAKE_SYS", filepath.Join(t.TempDir(), "sys"))
	t.Setenv("FAKE_SLEEP", "")
	t.Setenv("FAKE_BIG", "")
	t.Setenv("FAKE_HDR_TYPE", "")
	t.Setenv("GOCPP_PROBE_BATCH", "100000")
	return f
}

// limitEndsWhenHung makes the time limit of an auxiliary compiler run end when a FAKE_DI=hang run
// reports (FAKE_DI_HUNG) that it printed its lines and waits, not after a wall-clock time that a
// loaded machine or the race detector can exceed before the compiler printed anything. A run that
// does not hang is not limited (a minute only stops a test that is stuck).
func limitEndsWhenHung(t *testing.T) {
	t.Helper()
	hung := filepath.Join(t.TempDir(), "hung")
	t.Setenv("FAKE_DI_HUNG", hung)
	restore := auxRunContext
	t.Cleanup(func() { auxRunContext = restore })
	auxRunContext = func(ctx context.Context, cfg Config) (context.Context, context.CancelFunc) {
		limited, cancel := context.WithTimeout(ctx, time.Minute)
		seen := hungSize(hung)
		go func() {
			ticker := time.NewTicker(2 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-limited.Done():
					return
				case <-ticker.C:
					if hungSize(hung) > seen {
						cancel()
						return
					}
				}
			}
		}()
		return limited, cancel
	}
}

func hungSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (f *fake) cfg() Config {
	// (a time limit that makes the tests that wait for a lock fail in seconds, not hang)
	return Config{CXX: f.cxx, Dir: f.dir, CacheDir: f.cache, Timeout: 10 * time.Second}
}

// count returns the number of runs of a mode ("run" or "E").
func (f *fake) count(mode string) int {
	b, _ := os.ReadFile(f.counter)
	n := 0
	for _, l := range strings.Fields(string(b)) {
		if l == mode {
			n++
		}
	}
	return n
}

func (f *fake) runs() int { return f.count("run") }

func (f *fake) header(t *testing.T, name, text string) {
	t.Helper()
	p := filepath.Join(f.inc, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(p, old, old) // not racy
}

func fnSet(names ...string) *collect.Set {
	var refs []*collect.Ref
	for _, n := range names {
		refs = append(refs, ref(call(pth(n))))
	}
	return collect.NewSet(refs...)
}

func fnNames(n int) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, "f"+strconv.Itoa(i))
	}
	return out
}

func kindsOf(names ...string) map[string]interop.Kind {
	k := map[string]interop.Kind{}
	for _, n := range names {
		k[n] = interop.Value
	}
	return k
}

func gc(names ...string) GenConfig {
	return GenConfig{Includes: []string{`"a.hpp"`}, Kinds: kindsOf(names...)}
}

// stable is the JSON of a table without what describes the run.
func stable(t *testing.T, tab *Table) string {
	t.Helper()
	c := *tab
	c.Timing, c.Stats, c.Cached = Timing{}, Stats{}, false
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// answers is the JSON of what a table answers (not where the headers were).
func answers(t *testing.T, tab *Table) string {
	t.Helper()
	b, err := json.Marshal(struct {
		R []Result
		H []Diag
	}{tab.Results, tab.HeaderErrors})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func probeWith(t *testing.T, f *fake, rn *Runner, set *collect.Set, names ...string) *Table {
	t.Helper()
	tab, err := rn.Probe(context.Background(), f.cfg(), set, gc(names...))
	if err != nil {
		t.Fatal(err)
	}
	return tab
}

func TestCacheWarmRunsNoCompiler(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "// a\n")
	names := []string{"f0", "f1", "f2", "fERR3"}
	set := fnSet(names...)
	cold := probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 1 || cold.Cached || cold.Cache.Ran != 4 || cold.Cache.Hits != 0 || cold.Cache.Batches != 1 {
		t.Fatalf("cold: %d runs, cached %v, %+v", f.runs(), cold.Cached, cold.Cache)
	}
	if r := cold.Results[0]; !r.OK || r.Type != "R_f0" {
		t.Errorf("result %+v", r)
	}
	// A new Runner has no memory: the answers come from the disk, with no compiler run.
	ResetCacheCounters()
	warm := probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 1 || !warm.Cached || warm.Cache.Hits != 4 || warm.Cache.Ran != 0 || warm.Cache.Batches != 0 {
		t.Fatalf("warm: %d runs, cached %v, %+v", f.runs(), warm.Cached, warm.Cache)
	}
	if c := CacheCounters(); c.DiskHits == 0 || c.Runs != 0 {
		t.Errorf("counters %+v", c)
	}
	if a, b := stable(t, cold), stable(t, warm); a != b {
		t.Errorf("cold and warm differ:\n%s\n%s", a, b)
	}
	if warm.Key != cold.Key || len(warm.Deps) < 1 {
		t.Errorf("key %v vs %v, deps %v", warm.Key, cold.Key, warm.Deps)
	}
	// The same Runner serves it from memory.
	rn := new(Runner)
	probeWith(t, f, rn, set, names...)
	probeWith(t, f, rn, set, names...)
	if f.runs() != 1 || rn.Runs() != 0 {
		t.Errorf("%d runs", f.runs())
	}
}

func TestCacheHeaderEditInvalidates(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "// one\n")
	f.header(t, "b.hpp", "// other\n")
	names := []string{"f0", "f1"}
	set := fnSet(names...)
	probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 1 {
		t.Fatalf("%d runs", f.runs())
	}
	// A touch without a change of the contents keeps the entries.
	p := filepath.Join(f.inc, "a.hpp")
	now := time.Now().Add(-30 * time.Minute)
	os.Chtimes(p, now, now)
	probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 1 {
		t.Errorf("a touch made a run: %d", f.runs())
	}
	// An edit of the same size, with the modification time put back to what it was (a check
	// by size and time would not see it; the contents and the change time do).
	st0, _ := os.Stat(p)
	old := st0.ModTime()
	os.WriteFile(p, []byte("// two\n"), 0o644)
	os.Chtimes(p, old, old)
	st, _ := os.Stat(p)
	if st.Size() != int64(len("// one\n")) || !st.ModTime().Equal(old) {
		t.Fatal("not the same size and time")
	}
	tab := probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 2 || tab.Cached {
		t.Errorf("after the edit: %d runs", f.runs())
	}
	// Back to the old contents: the old variant is still in the context entry, and its
	// units are in the cache.
	os.WriteFile(p, []byte("// one\n"), 0o644)
	os.Chtimes(p, old, old)
	tab = probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 2 || !tab.Cached {
		t.Errorf("after the revert: %d runs (cached %v)", f.runs(), tab.Cached)
	}
	// The other header is another context: not touched by the edit of the first.
	other := GenConfig{Includes: []string{`"b.hpp"`}, Kinds: kindsOf(names...)}
	if _, err := new(Runner).Probe(context.Background(), f.cfg(), set, other); err != nil {
		t.Fatal(err)
	}
	runs := f.runs()
	os.WriteFile(p, []byte("// 333\n"), 0o644)
	os.Chtimes(p, old, old)
	if _, err := new(Runner).Probe(context.Background(), f.cfg(), set, other); err != nil {
		t.Fatal(err)
	}
	if f.runs() != runs {
		t.Errorf("an edit of a.hpp made b.hpp's probes run again")
	}
}

func TestCacheIncremental(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	const n = 200
	names := fnNames(n)
	cold := probeWith(t, f, new(Runner), fnSet(names...), names...)
	if f.runs() != 1 || cold.Cache.Ran != n {
		t.Fatalf("cold: %d runs, %+v", f.runs(), cold.Cache)
	}
	// One reference changes (f77 becomes f77x): the other 199 come from the cache.
	edited := append([]string(nil), names...)
	edited[77] = "f77x"
	tab := probeWith(t, f, new(Runner), fnSet(edited...), edited...)
	if f.runs() != 2 || tab.Cache.Ran != 1 || tab.Cache.Hits != n-1 || tab.Cache.Batches != 1 {
		t.Fatalf("edit: %d runs, %+v", f.runs(), tab.Cache)
	}
	b, _ := os.ReadFile(f.counter)
	_ = b
	// The batch had the one unit only.
	for _, r := range tab.Results {
		if r.Key == `call(::f77x|)` && (!r.OK || r.Type != "R_f77x") {
			t.Errorf("the new result: %+v", r)
		}
	}
	// And the whole table is the one a cold run makes.
	g := newFake(t)
	g.header(t, "a.hpp", "//\n")
	want := probeWith(t, g, new(Runner), fnSet(edited...), edited...)
	// (the Key has the compiler's path: compare the results)
	rj, _ := json.Marshal(tab.Results)
	wj, _ := json.Marshal(want.Results)
	if string(rj) != string(wj) {
		t.Error("the incremental table differs from a cold one")
	}
}

// A result does not depend on its batch neighbours: the same references in one batch, in
// several batches and one by one give the same results, errors and places included.
func TestCacheBatchIndependence(t *testing.T) {
	names := append(fnNames(30), "fERR1", "fERR2", "f9ERR")
	var want string
	for _, minb := range []string{"100000", "5", "1"} {
		f := newFake(t)
		t.Setenv("GOCPP_PROBE_BATCH", minb)
		t.Setenv("GOCPP_PROBE_JOBS", "6")
		f.header(t, "a.hpp", "//\n")
		src := Generate(fnSet(names...), gc(names...))
		t.Setenv("FAKE_HELPER", strconv.Itoa(src.HelperFrom+2))
		tab := probeWith(t, f, new(Runner), fnSet(names...), names...)
		wantRuns := map[string]int{"100000": 1, "5": 6, "1": 6}[minb]
		if f.runs() != wantRuns {
			t.Errorf("batch %s: %d runs, want %d", minb, f.runs(), wantRuns)
		}
		got := answers(t, tab)
		if want == "" {
			want = got
		} else if got != want {
			t.Errorf("batch %s: the table differs from the single batch's:\n%s\n%s", minb, want, got)
		}
		var errs int
		for _, r := range tab.Results {
			if strings.Contains(r.Key, "ERR") {
				errs++
				if r.OK || r.ErrorAt == nil || r.ErrorAt.File != "gp_probe.cpp" || r.ErrorAt.Line == 0 {
					t.Errorf("error result %+v", r)
				}
				// The error is at the line of the unit in the full file.
				if l := src.Lines[r.ErrorAt.Line]; l.Ref != r.ID {
					t.Errorf("%s: error at line %d, which is ref %d's", r.Key, r.ErrorAt.Line, l.Ref)
				}
				if len(r.Notes) != 4 || r.Notes[0].Line != r.ErrorAt.Line || r.Notes[1].Line != src.HelperFrom+2 || r.Notes[2].Line != 0 || r.Notes[3].File != "other.hpp" || r.Notes[3].Line != 9 {
					t.Errorf("%s: notes %+v", r.Key, r.Notes)
				}
			}
		}
		if errs != 3 {
			t.Errorf("%d errors", errs)
		}
	}
}

func TestCacheCorruptEntries(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	names := fnNames(5)
	set := fnSet(names...)
	cold := probeWith(t, f, new(Runner), set, names...)
	var files []string
	filepath.WalkDir(f.cache, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) != 7 { // the toolchain's, the context's and five units
		t.Fatalf("%d files", len(files))
	}
	corrupt := []func(p string){
		func(p string) { os.WriteFile(p, nil, 0o644) },                                  // empty
		func(p string) { b, _ := os.ReadFile(p); os.WriteFile(p, b[:len(b)/2], 0o644) }, // cut off
		func(p string) { b, _ := os.ReadFile(p); b[len(b)-3] ^= 0x55; os.WriteFile(p, b, 0o644) },
		func(p string) { os.WriteFile(p, []byte("garbage without a newline"), 0o644) },
		func(p string) { // a good checksum over text that is not an entry
			body := []byte(`{"format":1,"kind":"u","result":{"r":{"ok":true,"errorAt":{"severity":"error","msg":"x"}},"ea":{"k":"o","n":99}}}`)
			os.WriteFile(p, append([]byte(checksum(body)+"\n"), body...), 0o644)
		},
	}
	for i, p := range files {
		corrupt[i%len(corrupt)](p)
	}
	ResetCacheCounters()
	tab := probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 2 || tab.Cached {
		t.Errorf("%d runs after the damage", f.runs())
	}
	if c := CacheCounters(); c.Corrupt == 0 {
		t.Errorf("counters %+v", c)
	}
	if a, b := stable(t, cold), stable(t, tab); a != b {
		t.Error("the table after the damage differs")
	}
	// The damage was repaired.
	probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 2 {
		t.Errorf("%d runs: the entries were not written again", f.runs())
	}
}

// A change that leaves a valid entry (a letter of a type) is caught by the checksum.
func TestCacheChecksumCatchesQuietDamage(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	names := fnNames(4)
	set := fnSet(names...)
	cold := probeWith(t, f, new(Runner), set, names...)
	changed := 0
	filepath.WalkDir(f.cache, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") {
			b, _ := os.ReadFile(p)
			if nb := bytes.ReplaceAll(b, []byte("R_f"), []byte("R_g")); len(nb) == len(b) && !bytes.Equal(nb, b) {
				os.WriteFile(p, nb, 0o644)
				changed++
			}
		}
		return nil
	})
	if changed != 4 {
		t.Fatalf("%d files changed", changed)
	}
	tab := probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 2 || stable(t, tab) != stable(t, cold) {
		t.Errorf("%d runs; the damaged entries were served: %v", f.runs(), tab.Results[0].Type)
	}
}

func TestCacheDisabled(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	cfg := f.cfg()
	cfg.CacheDir = "off"
	set := fnSet("f0")
	for i := 0; i < 2; i++ {
		if _, err := new(Runner).Probe(context.Background(), cfg, set, gc("f0")); err != nil {
			t.Fatal(err)
		}
	}
	if f.runs() != 2 {
		t.Errorf("%d runs with the cache off", f.runs())
	}
	if _, err := os.Stat(f.cache); err == nil {
		t.Error("a folder was made")
	}
	// The environment: off, a relative name, and the variable of the kind probe.
	for _, v := range []string{"off", "OFF", "0", "false", "relative/dir"} {
		t.Setenv(EnvProbeCache, v)
		if CacheDir() != "" {
			t.Errorf("GOCPP_PROBE_CACHE=%s gives %q", v, CacheDir())
		}
	}
	t.Setenv(EnvNoProbeCache, "")
	abs := t.TempDir()
	t.Setenv(EnvProbeCache, abs)
	if CacheDir() != filepath.Join(abs, cacheSub) {
		t.Errorf("CacheDir %q", CacheDir())
	}
	t.Setenv(EnvNoProbeCache, "1")
	if CacheDir() != "" {
		t.Errorf("GOCPP_NOPROBECACHE=1 gives %q", CacheDir())
	}
}

// Two processes (here: two Runners, which only the lock files connect) that need the same
// answer: one compiler run.
func TestCacheLockOneRunForTwoRunners(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "1")
	names := fnNames(6)
	set := fnSet(names...)
	const n = 5
	var wg sync.WaitGroup
	tabs := make([]*Table, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tab, err := new(Runner).Probe(context.Background(), f.cfg(), set, gc(names...))
			if err != nil {
				t.Error(err)
				return
			}
			tabs[i] = tab
		}()
	}
	wg.Wait()
	if f.runs() != 1 {
		t.Errorf("%d compiler runs for %d runners", f.runs(), n)
	}
	fresh := 0
	for _, tab := range tabs {
		if tab != nil && !tab.Cached {
			fresh++
		}
		if tab != nil && stable(t, tab) != stable(t, tabs[0]) {
			t.Error("the tables differ")
		}
	}
	if fresh != 1 {
		t.Errorf("%d tables ran the compiler", fresh)
	}
	// No lock is left.
	if entries, _ := os.ReadDir(filepath.Join(f.cache, "locks")); len(entries) != 0 {
		t.Errorf("locks left: %v", entries)
	}
}

// A lock of a process that was killed does not stop others for ever.
func TestCacheStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := lockFile(dir, "k")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("1\n"), 0o600)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(path, old, old)
	release, waited := acquireLock(context.Background(), dir, "k", time.Second)
	if release == nil {
		t.Fatal("the stale lock was not taken over")
	}
	_ = waited
	release()
	if _, err := os.Stat(path); err == nil {
		t.Error("the lock is still there")
	}
	// A live lock is waited for, up to the limit, and the context stops the wait: without it the
	// wait would last an hour (lockWaitCap is lifted), so the minute only tells the two apart.
	rel, _ := acquireLock(context.Background(), dir, "k", time.Second)
	defer rel()
	defer func(c time.Duration) { lockWaitCap = c }(lockWaitCap)
	lockWaitCap = 0
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	t0 := time.Now()
	r2, waited := acquireLock(ctx, dir, "k", time.Hour)
	if r2 != nil || !waited || time.Since(t0) > time.Minute {
		t.Errorf("held lock: release %v waited %v after %v", r2 != nil, waited, time.Since(t0))
	}
}

func TestCachePrune(t *testing.T) {
	dir := filepath.Join(t.TempDir(), cacheSub)
	now := time.Now()
	var names []string
	for i := 0; i < 10; i++ {
		name := entryName("u", "x", strconv.Itoa(i))
		if !writeEntry(filepath.Join(dir, name), []byte(strings.Repeat("x", 100))) {
			t.Fatal("write")
		}
		mod := now.Add(-time.Duration(10-i) * time.Hour) // 0 is the oldest
		os.Chtimes(filepath.Join(dir, name), mod, mod)
		names = append(names, name)
	}
	stale := filepath.Join(dir, "u", ".tmp-123")
	os.WriteFile(stale, []byte("x"), 0o600)
	os.Chtimes(stale, now.Add(-3*time.Hour), now.Add(-3*time.Hour))
	exists := func(name string) bool { _, err := os.Stat(filepath.Join(dir, name)); return err == nil }
	// By count: the least recently used go.
	if n := prune(dir, Limits{MaxEntries: 7}, now); n != 3 {
		t.Errorf("removed %d", n)
	}
	for i, name := range names {
		if exists(name) != (i >= 3) {
			t.Errorf("entry %d: exists %v", i, exists(name))
		}
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("the stale temporary file stayed")
	}
	// By size: each file is 100 bytes plus the checksum line.
	u := CacheUsage(dir)
	if u.Entries != 7 || u.Kinds["u"] != 7 {
		t.Errorf("usage %+v", u)
	}
	per := u.Bytes / 7
	if n := prune(dir, Limits{MaxBytes: 4*per + 1}, now); n != 3 {
		t.Errorf("removed %d by size", n)
	}
	// By age.
	if n := prune(dir, Limits{MaxAge: 3*time.Hour + 30*time.Minute}, now); n != 1 {
		t.Errorf("removed %d by age", n)
	}
	if u := CacheUsage(dir); u.Entries != 3 {
		t.Errorf("%d entries left", u.Entries)
	}
	// A read refreshes the order: the entry that was read lives longer than a younger one.
	old := now.Add(-100 * time.Hour)
	for _, name := range names[7:] {
		os.Chtimes(filepath.Join(dir, name), old, old)
	}
	if _, ok := readEntry(filepath.Join(dir, names[8])); !ok {
		t.Fatal("read")
	}
	prune(dir, Limits{MaxEntries: 1}, now)
	if !exists(names[8]) || exists(names[7]) || exists(names[9]) {
		t.Errorf("after the read: %v %v %v", exists(names[7]), exists(names[8]), exists(names[9]))
	}
}

func TestCacheStatsAndClean(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	probeWith(t, f, new(Runner), fnSet("f0", "f1"), "f0", "f1")
	other := filepath.Join(filepath.Dir(f.cache), "k-1.json") // the kind probe's file, next to ours
	os.WriteFile(other, []byte("x"), 0o644)
	u := CacheUsage(f.cache)
	if u.Entries != 4 || u.Kinds["c"] != 1 || u.Kinds["t"] != 1 || u.Kinds["u"] != 2 || u.Dir != f.cache {
		t.Errorf("usage %+v", u)
	}
	if _, err := CleanCache(filepath.Dir(f.cache)); err == nil {
		t.Error("CleanCache removed a folder that is not the typed cache's")
	}
	got, err := CleanCache(f.cache)
	if err != nil || got.Entries != 4 {
		t.Errorf("clean: %+v %v", got, err)
	}
	if _, err := os.Stat(f.cache); err == nil {
		t.Error("the folder is still there")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("the kind probe's file was removed")
	}
}

// ---------------------------------------------------------------- the key

// Every component of the key matters: a change of any of them makes the compiler run, and a
// change of nothing (a touch) does not.
func TestCacheKeyComponents(t *testing.T) {
	names := []string{"f0", "f1"}
	set := fnSet(names...)
	type variation struct {
		name string
		do   func(t *testing.T, f *fake, cfg *Config, g *GenConfig)
		runs bool
	}
	vars := []variation{
		{"nothing", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) {}, false},
		{"touch", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) {
			now := time.Now().Add(-time.Minute)
			os.Chtimes(filepath.Join(f.inc, "a.hpp"), now, now)
		}, false},
		{"a macro flag", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.Flags = []string{"-DX=1"} }, true},
		{"the order of -I flags", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.Flags = []string{"-I", "/b", "-I", "/a"} }, true},
		{"the compiler's version", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.CXX.Version = "other" }, true},
		{"the compiler's arguments", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.CXX.Args = []string{"--x"} }, true},
		{"the target's system", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.CXX.GOOS = "windows" }, true},
		{"the target's architecture", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.CXX.GOARCH = "arm64" }, true},
		{"the cache's version", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) {
			old := keyVersion
			keyVersion = "another"
			t.Cleanup(func() { keyVersion = old })
		}, true},
		{"gocpp itself", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) {
			old := selfID
			selfID = func() string { return old() + "-another build" }
			t.Cleanup(func() { selfID = old })
		}, true},
		{"the package folder", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.Dir = t.TempDir() }, true},
		{"the includes", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) {
			f.header(t, "b.hpp", "//\n")
			g.Includes = []string{`"a.hpp"`, `"b.hpp"`}
		}, true},
		{"the string conversion", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { g.StringToCStr = true }, true},
		{"the environment", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { t.Setenv("CPATH", "/somewhere") }, true},
		{"the largest declaration", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.MaxDecl = 12345 }, true},
		{"the dump limit", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { cfg.MaxJSON = 1 << 20 }, true},
		{"the header's contents", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) {
			f.header(t, "a.hpp", "// changed\n")
		}, true},
		{"the kinds (so the text of a probe)", func(t *testing.T, f *fake, cfg *Config, g *GenConfig) { delete(g.Kinds, "f0") }, true},
	}
	for _, v := range vars {
		t.Run(strings.ReplaceAll(v.name, " ", "_"), func(t *testing.T) {
			f := newFake(t)
			f.header(t, "a.hpp", "//\n")
			cfg, g := f.cfg(), gc(names...)
			if _, err := new(Runner).Probe(context.Background(), cfg, set, g); err != nil {
				t.Fatal(err)
			}
			if f.runs() != 1 {
				t.Fatalf("%d runs", f.runs())
			}
			v.do(t, f, &cfg, &g)
			if _, err := new(Runner).Probe(context.Background(), cfg, set, g); err != nil {
				t.Fatal(err)
			}
			if ran := f.runs() == 2; ran != v.runs {
				t.Errorf("%s: ran %v, want %v", v.name, ran, v.runs)
			}
		})
	}
}

// A compiler program that was replaced (same path, another size and time) is another compiler.
func TestCacheKeyCompilerProgram(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	copyFile(t, os.Args[0], f.cxx.Path+".new") // a copy of its own: the test binary itself cannot be written
	os.Remove(f.cxx.Path)
	if err := os.Rename(f.cxx.Path+".new", f.cxx.Path); err != nil {
		t.Fatal(err)
	}
	set := fnSet("f0")
	probeWith(t, f, new(Runner), set, "f0")
	probeWith(t, f, new(Runner), set, "f0")
	if f.runs() != 1 {
		t.Fatalf("%d runs", f.runs())
	}
	b, _ := os.ReadFile(f.cxx.Path)
	var err error
	for i := 0; i < 100; i++ { // "text file busy" while a forked process still has the file
		if err = os.WriteFile(f.cxx.Path, append(b, "\nanother build\n"...), 0o755); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	probeWith(t, f, new(Runner), set, "f0")
	if f.runs() != 2 {
		t.Errorf("%d runs: a compiler that was replaced was not noticed", f.runs())
	}
}

// ---------------------------------------------------------------- missing headers

func TestCacheMissingHeaderIsCachedAndInvalidated(t *testing.T) {
	f := newFake(t)
	names := []string{"f0"}
	set := fnSet(names...)
	sys := os.Getenv("FAKE_SYS")
	os.MkdirAll(sys, 0o755)
	probe := func() *Table { return probeWith(t, f, new(Runner), set, names...) }
	tab := probe()
	if tab.Results[0].OK || len(tab.HeaderErrors) == 0 || !strings.Contains(tab.HeaderErrors[0].Msg, "file not found") {
		t.Fatalf("the result: %+v %+v", tab.Results[0], tab.HeaderErrors)
	}
	runs := f.runs()
	// The negative answer is cached, with the places that were searched.
	tab2 := probe()
	if f.runs() != runs || !tab2.Cached {
		t.Fatalf("not cached: %d runs", f.runs())
	}
	if len(tab.Deps) == 0 {
		t.Fatal("no deps")
	}
	has := func(p string) bool {
		for _, d := range tab.Deps {
			if d == p {
				return true
			}
		}
		return false
	}
	if !has(filepath.Join(f.dir, "a.hpp")) || !has(filepath.Join(sys, "a.hpp")) {
		t.Errorf("deps %v", tab.Deps)
	}
	if f.count("V") == 0 {
		t.Error("the search path was not asked for")
	}
	// The header appears in the package folder: another answer.
	os.WriteFile(filepath.Join(f.dir, "a.hpp"), []byte("//\n"), 0o644)
	os.Chtimes(filepath.Join(f.dir, "a.hpp"), time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	t.Setenv("FAKE_INC", f.dir)
	tab3 := probe()
	if f.runs() != runs+1 || !tab3.Results[0].OK {
		t.Errorf("after the header appeared in the folder: %d runs, %+v", f.runs(), tab3.Results[0])
	}
}

func TestCacheMissingHeaderInASystemFolder(t *testing.T) {
	f := newFake(t)
	names := []string{"f0"}
	set := fnSet(names...)
	sys := os.Getenv("FAKE_SYS")
	os.MkdirAll(sys, 0o755)
	probeWith(t, f, new(Runner), set, names...)
	runs := f.runs()
	probeWith(t, f, new(Runner), set, names...)
	if f.runs() != runs {
		t.Fatal("not cached")
	}
	// It appears in the compiler's own include folder (a library is installed).
	p := filepath.Join(sys, "a.hpp")
	os.WriteFile(p, []byte("//\n"), 0o644)
	os.Chtimes(p, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	t.Setenv("FAKE_INC", sys)
	tab := probeWith(t, f, new(Runner), set, names...)
	if f.runs() != runs+1 || !tab.Results[0].OK {
		t.Errorf("%d runs, %+v", f.runs(), tab.Results[0])
	}
}

func TestCacheMissingHeaderInAnIncludeFolder(t *testing.T) {
	f := newFake(t)
	names := []string{"f0"}
	set := fnSet(names...)
	inc := t.TempDir()
	cfg := f.cfg()
	cfg.Flags = []string{"-I", inc}
	run := func() *Table {
		tab, err := new(Runner).Probe(context.Background(), cfg, set, gc(names...))
		if err != nil {
			t.Fatal(err)
		}
		return tab
	}
	run()
	runs := f.runs()
	run()
	if f.runs() != runs {
		t.Fatal("not cached")
	}
	p := filepath.Join(inc, "a.hpp")
	os.WriteFile(p, []byte("//\n"), 0o644)
	os.Chtimes(p, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	t.Setenv("FAKE_INC", inc)
	if tab := run(); f.runs() != runs+1 || !tab.Results[0].OK {
		t.Errorf("%d runs, %+v", f.runs(), tab.Results[0])
	}
}

// A header that was modified just before the run (or while it ran) may have changed again within the
// clock's tick: the result is not kept, not in memory either, until a run that is not racy.
func TestCacheRacyHeaderIsNotKept(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	old := racyWindow
	racyWindow = time.Hour // (the header was written a moment ago, however slow the machine is)
	t.Cleanup(func() { racyWindow = old })
	now := time.Now()
	os.Chtimes(filepath.Join(f.inc, "a.hpp"), now, now) // modified a moment ago, by both times
	names := []string{"f0"}
	set := fnSet(names...)
	rn := new(Runner)
	probeWith(t, f, rn, set, names...)
	probeWith(t, f, rn, set, names...)
	if f.runs() != 2 {
		t.Errorf("a racy result was kept in memory: %d runs", f.runs())
	}
	probeWith(t, f, new(Runner), set, names...)
	if f.runs() != 3 {
		t.Errorf("a racy result was written to disk: %d runs", f.runs())
	}
	if u := CacheUsage(f.cache); u.Kinds["u"] != 0 || u.Kinds["c"] != 0 {
		t.Errorf("%+v", u)
	}
	racyWindow = 0 // not racy now
	probeWith(t, f, rn, set, names...)
	probeWith(t, f, rn, set, names...)
	if f.runs() != 4 {
		t.Errorf("%d runs after the window closed", f.runs())
	}
}

// ---------------------------------------------------------------- parallelism and cancellation

func TestProbeJobs(t *testing.T) {
	t.Setenv(EnvJobs, "3")
	if Jobs() != 3 {
		t.Errorf("Jobs %d", Jobs())
	}
	t.Setenv(EnvJobs, "")
	if j := Jobs(); j < 1 || j > 8 {
		t.Errorf("Jobs %d", j)
	}
	t.Setenv(EnvJobs, "zero")
	if j := Jobs(); j < 1 || j > 8 {
		t.Errorf("Jobs %d", j)
	}
}

// The bound is global to the process: calls that overlap (each with its own Runner, each
// with batches) never run more compilers than Jobs.
func TestGlobalBoundAcrossCalls(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "0.3")
	t.Setenv(EnvJobs, "2")
	t.Setenv("GOCPP_PROBE_BATCH", "2")
	gate.resetPeak()
	var wg sync.WaitGroup
	for c := 0; c < 4; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			names := make([]string, 8)
			for i := range names {
				names[i] = fmt.Sprintf("f%d_%d", c, i)
			}
			if _, err := new(Runner).Probe(context.Background(), f.cfg(), fnSet(names...), gc(names...)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if f.runs() != 8 { // 4 calls, 2 batches each (at most Jobs)
		t.Errorf("%d runs", f.runs())
	}
	if now, peak := gate.running(); now != 0 || peak != 2 {
		t.Errorf("running %d, peak %d (want 0 and 2)", now, peak)
	}
}

// The results do not depend on how the batches were scheduled.
func TestParallelBatchesAreDeterministic(t *testing.T) {
	names := fnNames(40)
	var want string
	for _, jobs := range []string{"1", "2", "7"} {
		f := newFake(t)
		f.header(t, "a.hpp", "//\n")
		t.Setenv(EnvJobs, jobs)
		t.Setenv("GOCPP_PROBE_BATCH", "3")
		tab := probeWith(t, f, new(Runner), fnSet(names...), names...)
		got := answers(t, tab)
		if want == "" {
			want = got
		} else if got != want {
			t.Errorf("jobs %s: the table differs:\n%s\n%s", jobs, want, got)
		}
		if jobs == "7" && f.runs() != 7 {
			t.Errorf("%d runs for 40 units, batch 3, jobs 7", f.runs())
		}
	}
}

func TestEachBatchHasItsTimeout(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "180") // a run that is not stopped at its timeout takes three minutes
	t.Setenv("GOCPP_PROBE_BATCH", "2")
	cfg := f.cfg()
	cfg.Timeout = 200 * time.Millisecond
	names := fnNames(4)
	t0 := time.Now()
	_, err := new(Runner).Probe(context.Background(), cfg, fnSet(names...), gc(names...))
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error %v", err)
	}
	if time.Since(t0) > time.Minute {
		t.Errorf("took %v", time.Since(t0))
	}
}

// ---------------------------------------------------------------- the declarations

func TestDeclCacheRefusalIsRemembered(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_BIG", "1") // a dump with no end
	cfg := f.cfg()
	dc := DeclConfig{Includes: []string{`"a.hpp"`}, MaxJSON: 1 << 20}
	q := []DeclQuery{{DeclType, "Big"}}
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, q)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tab.Errors["type Big"], "larger than the limit") || f.runs() != 1 {
		t.Fatalf("errors %v, %d runs", tab.Errors, f.runs())
	}
	if f.count("E") != 1 {
		t.Errorf("the headers were asked for %d times", f.count("E"))
	}
	// Another process: the refusal is on disk, nothing runs.
	tab2, err := new(Runner).Decls(context.Background(), cfg, dc, q)
	if err != nil {
		t.Fatal(err)
	}
	if f.runs() != 1 || !tab2.Cached || tab2.Errors["type Big"] != tab.Errors["type Big"] {
		t.Errorf("%d runs, cached %v, %v", f.runs(), tab2.Cached, tab2.Errors)
	}
	// An edit of the header lifts it: the refusal depends on the headers' contents.
	f.header(t, "a.hpp", "// edited\n")
	if _, err := new(Runner).Decls(context.Background(), cfg, dc, q); err != nil {
		t.Fatal(err)
	}
	if f.runs() != 2 {
		t.Errorf("%d runs after the edit", f.runs())
	}
}

// The declaration parts are keyed by gocpp's own identity as the probe results are.
func TestDeclCacheKeyHasGocpp(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	dc := DeclConfig{Includes: []string{`"a.hpp"`}}
	q := []DeclQuery{{DeclType, "A"}}
	decls := func() { // a new Runner: the answer comes from the disk or from the compiler
		if _, err := new(Runner).Decls(context.Background(), f.cfg(), dc, q); err != nil {
			t.Fatal(err)
		}
	}
	decls()
	decls()
	if f.runs() != 1 {
		t.Fatalf("%d runs", f.runs())
	}
	old := selfID
	selfID = func() string { return old() + "-another build" }
	t.Cleanup(func() { selfID = old })
	decls()
	if f.runs() != 2 {
		t.Errorf("%d runs: another gocpp got the old answer", f.runs())
	}
}

func TestDeclCacheFakeHitMiss(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	cfg := f.cfg()
	dc := DeclConfig{Includes: []string{`"a.hpp"`}}
	qs := []DeclQuery{{DeclType, "A"}, {DeclType, "B"}, {DeclFunc, "g"}}
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Runs != 3 || f.runs() != 3 {
		t.Fatalf("%d runs, %d", tab.Runs, f.runs())
	}
	tab2, err := new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	if tab2.Runs != 0 || !tab2.Cached || f.runs() != 3 || jsonOf(tab.Errors) != jsonOf(tab2.Errors) {
		t.Errorf("warm: %d runs, cached %v", f.runs(), tab2.Cached)
	}
	// Only the changed name runs.
	qs[1].Name = "C"
	tab3, err := new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	if tab3.Runs != 1 || f.runs() != 4 {
		t.Errorf("one name changed: %d runs (%d total)", tab3.Runs, f.runs())
	}
}

// ---------------------------------------------------------------- more

// A local variable's alias is declared once, before the first line that uses it, but belongs to every
// reference that uses it: the references' results do not depend on who else is in the file.
func TestCacheAliasUnits(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	def := &collect.Ref{Expr: &collect.Decay{X: call(pth("fERR9"))}}
	d0 := collect.NewSet(def).Refs[0] // for its key
	dr1, dr2 := &collect.DeclRef{Def: d0}, &collect.DeclRef{Def: d0}
	use1 := ref(call(sel(lv(dr1), "size")))
	use2 := ref(call(sel(lv(dr2), "capacity")))
	kinds := kindsOf("fERR9")
	// set makes the set of refs and points the uses at the definition's reference in it.
	set := func(refs ...*collect.Ref) *collect.Set {
		s := collect.NewSet(refs...)
		for _, r := range s.Refs {
			if r.Key == d0.Key {
				dr1.Def, dr2.Def = r, r
			}
		}
		return s
	}
	run := func(refs ...*collect.Ref) map[string]Result {
		tab, err := new(Runner).Probe(context.Background(), f.cfg(), set(refs...), GenConfig{Includes: []string{`"a.hpp"`}, Kinds: kinds})
		if err != nil {
			t.Fatal(err)
		}
		m := make(map[string]Result)
		for _, r := range tab.Results {
			m[r.Key] = r
		}
		return m
	}
	all := run(def, use1, use2)
	defOnly, use2Only := run(def), run(use2)
	if len(all) != 3 || len(defOnly) != 1 || len(use2Only) != 1 {
		t.Fatalf("%d, %d, %d results", len(all), len(defOnly), len(use2Only))
	}
	// (the lines of the probe file are those of each file: a position moves with the file)
	noLines := func(r Result) Result {
		r.ID = 0
		if r.ErrorAt != nil {
			d := *r.ErrorAt
			d.Line = 0
			r.ErrorAt = &d
		}
		r.Notes = slices.Clone(r.Notes)
		for i := range r.Notes {
			r.Notes[i].Line = 0
		}
		return r
	}
	same := func(a, b Result, what string) {
		ja, jb := jsonOf(noLines(a)), jsonOf(noLines(b))
		if ja != jb {
			t.Errorf("%s depends on the other references:\n%s\n%s", what, ja, jb)
		}
	}
	for k, r := range defOnly {
		if r.OK || r.ErrorAt == nil || !strings.Contains(r.Error, "gp_probe_") {
			t.Errorf("the definition: %+v", r)
		}
		same(all[k], r, "the definition's result")
	}
	for k, r := range use2Only {
		if !r.OK {
			t.Errorf("a use: %+v", r)
		}
		same(all[k], r, "a use's result")
	}
	// The same from the disk: a program that has one of the uses is answered by the entries of another.
	f2 := newFake(t)
	f2.header(t, "a.hpp", "//\n")
	for _, refs := range [][]*collect.Ref{{def, use1, use2}, {use2}, {def}} {
		if _, err := new(Runner).Probe(context.Background(), f2.cfg(), set(refs...), GenConfig{Includes: []string{`"a.hpp"`}, Kinds: kinds}); err != nil {
			t.Fatal(err)
		}
	}
	if f2.runs() != 1 {
		t.Errorf("%d runs for three programs that share their references", f2.runs())
	}
}

// The hash of a unit is that of the lines it uses and of the aliases they use.
func TestUnitHashHasTheAliases(t *testing.T) {
	lines := []Line{{Ref: 3, Role: RoleMain, Form: FormExpr, Text: "using gp_probe_3 = gocpp_probe_internal_c<decltype((x))>;"}}
	a := []Line{{Role: RoleAlias, Text: "using gp_decl_a = int;"}}
	b := []Line{{Role: RoleAlias, Text: "using gp_decl_a = long;"}}
	if unitHash(a, lines) == unitHash(b, lines) || unitHash(a, lines) == unitHash(nil, lines) {
		t.Error("the aliases are not in the hash")
	}
	other := []Line{{Ref: 77, Role: RoleMain, Form: FormExpr, Text: "using gp_probe_77 = gocpp_probe_internal_c<decltype((x))>;"}}
	if unitHash(a, lines) != unitHash(a, other) {
		t.Error("the reference's number is in the hash")
	}
	alt := []Line{{Ref: 3, Role: RoleAlt, Form: FormExpr, Text: lines[0].Text}}
	if unitHash(a, lines) == unitHash(a, alt) {
		t.Error("the role is not in the hash")
	}
}

func TestEnumeratorJSONRoundTrip(t *testing.T) {
	huge, _ := new(big.Int).SetString("-123456789012345678901234567890", 10)
	in := []Enumerator{{"A", big.NewInt(0)}, {"B", big.NewInt(12345)}, {"C", huge}, {"D", nil}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out []Enumerator
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	for i := range in {
		if out[i].Name != in[i].Name || (in[i].Value == nil) != (out[i].Value == nil) || in[i].Value != nil && in[i].Value.Cmp(out[i].Value) != 0 {
			t.Errorf("%v became %v", in[i], out[i])
		}
	}
	if err := json.Unmarshal([]byte(`[{"name":"X","value":"1x"}]`), &out); err == nil {
		t.Error("a bad value was accepted")
	}
}

// A write that cannot finish leaves no temporary file.
func TestCacheFailedWriteLeavesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), cacheSub)
	path := filepath.Join(dir, entryName("u", "x"))
	if err := os.MkdirAll(path, 0o755); err != nil { // a folder where the file should go: the rename fails
		t.Fatal(err)
	}
	writeDown.Delete(dir)
	if writeEntry(path, []byte("x")) {
		t.Fatal("the write succeeded")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("left: %s", e.Name())
		}
	}
	writeDown.Delete(dir)
}

// A caller that looked at the cache just before another finished the same run does not run it
// again: it looks once more when it holds the flight.
func TestCacheLooksAgainBeforeRunning(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "0.3") // the first run is slow enough for the second caller to look first
	names := fnNames(3)
	set := fnSet(names...)
	rn := new(Runner)
	var calls atomic.Int32
	second := make(chan struct{})
	release := make(chan struct{})
	hookRunGroup = func([]string) {
		if calls.Add(1) == 2 {
			close(second)
			<-release // held until the first caller has finished
		}
	}
	t.Cleanup(func() { hookRunGroup = nil })
	results := make(chan *Table, 2)
	go func() { results <- probeWith(t, f, rn, set, names...) }()
	go func() { results <- probeWith(t, f, rn, set, names...) }()
	<-second
	first := <-results // the one that was not held
	close(release)
	last := <-results
	if f.runs() != 1 || rn.Runs() != 1 {
		t.Errorf("%d compiler runs (%d)", f.runs(), rn.Runs())
	}
	if first.Cached || !last.Cached {
		t.Errorf("cached: %v and %v", first.Cached, last.Cached)
	}
}

// A batch that fails stops the others.
func TestCacheFailureStopsTheOtherBatches(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "180") // a batch that is not stopped takes three minutes
	t.Setenv("GOCPP_PROBE_BATCH", "2")
	t.Setenv(EnvJobs, "4")
	names := []string{"f0", "f1", "f2", "f3", "fFAILNOW", "f5", "f6", "f7"}
	t0 := time.Now()
	_, err := new(Runner).Probe(context.Background(), f.cfg(), fnSet(names...), gc(names...))
	if err == nil || !strings.Contains(err.Error(), "fell over") {
		t.Fatalf("error %v", err)
	}
	if time.Since(t0) > time.Minute {
		t.Errorf("took %v: the other batches ran to the end", time.Since(t0))
	}
	if now, _ := gate.running(); now != 0 {
		t.Errorf("%d slots still taken", now)
	}
}

// fakeHeaderBuild is the fake compiler asked for a precompiled header (-x c++-header): it counts the build
// ("H"), appends "start" to FAKE_PCH_START, sleeps FAKE_PCH_SLEEP seconds (appending a line to
// FAKE_PCH_BEAT every 10 ms, so that a test sees whether the process still lives), reads FAKE_PCH_DEP as the one
// header it read, writes FAKE_PCH_BYTES to the -o file, and last appends "end" to FAKE_PCH_END (a build that
// was killed never writes it).
func fakeHeaderBuild(args []string) int {
	var dep, out string
	for i, a := range args {
		switch a {
		case "-MF":
			if i+1 < len(args) {
				dep = args[i+1]
			}
		case "-o":
			if i+1 < len(args) {
				out = args[i+1]
			}
		}
	}
	appendLine := func(env, line string) {
		if p := os.Getenv(env); p != "" {
			if f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				fmt.Fprintln(f, line)
				f.Close()
			}
		}
	}
	appendLine("FAKE_COUNTER", "H")
	appendLine("FAKE_PCH_START", "start")
	if secs, err := strconv.ParseFloat(os.Getenv("FAKE_PCH_SLEEP"), 64); err == nil && secs > 0 {
		for end := time.Now().Add(time.Duration(secs * float64(time.Second))); time.Now().Before(end); {
			appendLine("FAKE_PCH_BEAT", "beat")
			time.Sleep(10 * time.Millisecond)
		}
	}
	bytes := os.Getenv("FAKE_PCH_BYTES")
	if bytes == "" {
		bytes = "fake precompiled header"
	}
	if out != "" {
		os.WriteFile(out, []byte(bytes), 0o644)
	}
	if dep != "" {
		os.WriteFile(dep, []byte("gp_pch.o: "+os.Getenv("FAKE_PCH_DEP")+"\n"), 0o644)
	}
	appendLine("FAKE_PCH_END", "end")
	return 0
}
