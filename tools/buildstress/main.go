// Buildstress runs many `gocpp build` and `gocpp run` processes at the same time against ONE
// shared cache folder and reports every build that fails, a program that prints the wrong
// answer, and every folder a build leaves behind. It is the reproducer for the builds that
// failed at random when several gocpp processes shared a cache (an IDE next to a terminal, CI
// jobs, make -j): the object cache, the runtime library, the precompiled header, the folders of
// generated files, the up-to-date stamps and compile_commands.json are all shared state.
//
// Usage, in the dev shell (the compilers are found on PATH):
//
//	go run ./tools/buildstress -gocpp ./gocpp -procs 24 -builds 500
//
// A worker builds one of -programs small programs (each in a folder of its own) or, with
// -same-share, the same folder as the other workers (so they all write the same gen/<hash>), with
// a random compiler (-cxx), a random mode (default, -release, with -O0 flags in the environment),
// a random runtime key (-runtime-keys: GOCPP_CXXFLAGS=-DBUILDSTRESS_KEY=<k>, each key is a
// runtime library and a set of runtime objects of its own), and checks the program's output.
// The cache is replaced by a new empty one every -cold-every builds, with all the workers going
// on, so that cold-cache work (the runtime library, the precompiled header, the objects of the
// standard library) is raced too. -objsize makes the object cache tiny (GOCPP_OBJCACHE_SIZE) so
// that its trim runs all the time; -noobjcache turns it off for some builds; -taskset and -nice
// squeeze the processes onto few cores, which widens the races.
//
// The tool exits with status 1 when any build failed. Nothing is retried. Every child dies with
// the tool (Pdeathsig, and a kill of the process group when the tool ends), and the folders it
// made are removed unless -keep.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type config struct {
	gocpp       string
	work        string
	procs       int
	builds      int
	duration    time.Duration
	programs    int
	sameShare   float64
	cxxs        []string
	runtimeKeys int
	coldEvery   int
	objSize     string
	noObjCache  float64
	runFrac     float64
	releaseFrac float64
	taskset     string
	nice        int
	keep        bool
	seed        int64
	sharedTmp   bool
	verbose     bool
	maxShow     int
	stagger     time.Duration
	trimMarker  time.Duration
	wipeEvery   time.Duration
	wipeMode    string
}

// epoch is one cache folder, used by the builds that start while it is current.
type epoch struct {
	n     int
	cache string
	wg    sync.WaitGroup
}

type failure struct {
	desc   string
	output string
	kind   string
}

type stress struct {
	cfg     config
	ctx     context.Context
	started atomic.Int64
	done    atomic.Int64

	wipes     atomic.Int64 // work folders (or their contents) that the chaos cleaner removed
	recovered atomic.Int64 // builds that lost their work folder and built again (gocpp says so)

	mu       sync.Mutex
	cur      *epoch
	failures []failure
	kinds    map[string]int
	epochs   int
}

func main() {
	var cfg config
	var cxxs string
	flag.StringVar(&cfg.gocpp, "gocpp", "", "the gocpp binary (required)")
	flag.StringVar(&cfg.work, "work", "", "the work folder (default: a new folder in the temporary folder)")
	flag.IntVar(&cfg.procs, "procs", 16, "gocpp processes at the same time")
	flag.IntVar(&cfg.builds, "builds", 500, "stop after this many builds (0: only -duration)")
	flag.DurationVar(&cfg.duration, "duration", 3*time.Minute, "stop starting builds after this long")
	flag.IntVar(&cfg.programs, "programs", 6, "different small programs")
	flag.Float64Var(&cfg.sameShare, "same-share", 0.3, "fraction of builds of the one shared program folder (the same gen folder)")
	flag.StringVar(&cxxs, "cxx", "g++,clang++", "the compilers (GOCPP_CXX), comma separated")
	flag.IntVar(&cfg.runtimeKeys, "runtime-keys", 2, "different runtime keys besides the plain one (GOCPP_CXXFLAGS=-DBUILDSTRESS_KEY=k): each is a runtime library of its own")
	flag.IntVar(&cfg.coldEvery, "cold-every", 40, "a new empty cache after this many builds (0: one cache for all)")
	flag.StringVar(&cfg.objSize, "objsize", "", "GOCPP_OBJCACHE_SIZE of the builds (such as 2M: the object cache trims all the time)")
	flag.Float64Var(&cfg.noObjCache, "noobjcache", 0.1, "fraction of builds with GOCPP_NOOBJCACHE=1")
	flag.Float64Var(&cfg.runFrac, "run-frac", 0.25, "fraction of gocpp run instead of gocpp build")
	flag.Float64Var(&cfg.releaseFrac, "release-frac", 0.3, "fraction of builds with -release")
	flag.StringVar(&cfg.taskset, "taskset", "", "cpu list: run every gocpp under taskset -c LIST")
	flag.IntVar(&cfg.nice, "nice", 0, "nice value of the gocpp processes")
	flag.BoolVar(&cfg.keep, "keep", false, "keep the work folder")
	flag.Int64Var(&cfg.seed, "seed", 0, "random seed (0: from the time)")
	flag.BoolVar(&cfg.sharedTmp, "shared-tmp", false, "leave TMPDIR alone (default: a folder of the run, so that leftovers are counted)")
	flag.BoolVar(&cfg.verbose, "v", false, "print every build")
	flag.IntVar(&cfg.maxShow, "show", 12, "failures to show in full")
	flag.DurationVar(&cfg.trimMarker, "trim-marker-every", 0, "remove the object cache's .trimmed marker this often, so that the next store trims the cache (with -objsize: it trims all the time)")
	flag.DurationVar(&cfg.wipeEvery, "wipe-every", 0, "chaos: a cleaner removes the work folder of a running build (a gocpp-build* folder of the run's TMPDIR) this often, as a cleaner of /tmp would")
	flag.StringVar(&cfg.wipeMode, "wipe-mode", "folder", "what the cleaner removes: folder (the work folder), contents (what is in it), glob (what rm -rf folder/* removes: not the names that start with a dot) or tmp (all the run's TMPDIR)")
	flag.DurationVar(&cfg.stagger, "stagger", 0, "wait a random time up to this before each build")
	flag.Parse()
	cfg.cxxs = strings.Split(cxxs, ",")
	if cfg.gocpp == "" || cfg.procs < 1 {
		flag.Usage()
		os.Exit(2)
	}
	if abs, err := filepath.Abs(cfg.gocpp); err == nil {
		cfg.gocpp = abs
	}
	if cfg.seed == 0 {
		cfg.seed = time.Now().UnixNano()
	}
	os.Exit(run(cfg))
}

func run(cfg config) int {
	if cfg.work == "" {
		w, err := os.MkdirTemp("", "buildstress-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		cfg.work = w
	}
	if err := os.MkdirAll(cfg.work, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if !cfg.keep {
		defer os.RemoveAll(cfg.work)
	}
	setNice(cfg.nice)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s := &stress{cfg: cfg, ctx: ctx, kinds: map[string]int{}}
	if err := s.writePrograms(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if !cfg.sharedTmp {
		if err := os.MkdirAll(s.tmp(), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	fmt.Printf("buildstress: %d procs, %d builds, seed %d, work %s\n", cfg.procs, cfg.builds, cfg.seed, cfg.work)
	start := time.Now()
	s.newEpoch()
	if cfg.trimMarker > 0 {
		go func() {
			t := time.NewTicker(cfg.trimMarker)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					s.mu.Lock()
					c := s.cur.cache
					s.mu.Unlock()
					os.Remove(filepath.Join(c, "gocpp", "obj", ".trimmed"))
				}
			}
		}()
	}
	if cfg.wipeEvery > 0 && !cfg.sharedTmp {
		go s.cleaner(ctx)
	}
	var wg sync.WaitGroup
	for w := 0; w < cfg.procs; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(cfg.seed + int64(w)))
			for ctx.Err() == nil && time.Since(start) < cfg.duration {
				n := s.started.Add(1)
				if cfg.builds > 0 && n > int64(cfg.builds) {
					return
				}
				if cfg.coldEvery > 0 && n > 1 && (n-1)%int64(cfg.coldEvery) == 0 {
					s.newEpoch()
				}
				s.one(int(n), rng)
			}
		}(w)
	}
	wg.Wait()
	s.mu.Lock()
	last := s.cur
	s.mu.Unlock()
	last.wg.Wait()
	if !cfg.keep {
		os.RemoveAll(last.cache)
	}
	return s.report(time.Since(start))
}

func (s *stress) tmp() string { return filepath.Join(s.cfg.work, "tmp") }

// newEpoch makes a new empty cache the current one; the builds in the old one go on, and the old
// folder is removed when the last of them is done.
func (s *stress) newEpoch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epochs++
	e := &epoch{n: s.epochs, cache: filepath.Join(s.cfg.work, fmt.Sprintf("cache%d", s.epochs))}
	old := s.cur
	s.cur = e
	if old != nil && !s.cfg.keep {
		go func() {
			old.wg.Wait()
			os.RemoveAll(old.cache)
		}()
	}
}

func (s *stress) epoch() *epoch {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.cur
	e.wg.Add(1)
	return e
}

// source returns the text of program i: they differ in what they print and what they use.
func source(i int) string {
	switch i % 4 {
	case 0:
		return fmt.Sprintf("package main\n\nfunc main() {\n\tprintln(\"program %d\")\n}\n", i)
	case 1:
		return fmt.Sprintf("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"program\", %d)\n}\n", i)
	case 2:
		return fmt.Sprintf("package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Println(strings.Repeat(\"x\", 3), %d)\n}\n", i)
	}
	return fmt.Sprintf("package main\n\nimport \"fmt\"\n\ntype T struct{ a, b int }\n\nfunc main() {\n\tt := &T{%d, 2}\n\tfmt.Println(\"program\", t.a+t.b)\n}\n", i)
}

func want(i int) string {
	switch i % 4 {
	case 0:
		return fmt.Sprintf("program %d\n", i)
	case 1:
		return fmt.Sprintf("program %d\n", i)
	case 2:
		return fmt.Sprintf("xxx %d\n", i)
	}
	return fmt.Sprintf("program %d\n", 2+i)
}

// sharedIndex is the number of the program that the workers share a folder of (its folder is
// "shared", whatever program number it has).
const sharedIndex = 1001

func (s *stress) dir(i int) string {
	if i == sharedIndex {
		return filepath.Join(s.cfg.work, "src", "shared")
	}
	return filepath.Join(s.cfg.work, "src", fmt.Sprintf("p%d", i))
}

func (s *stress) writePrograms() error {
	for i := 0; i <= s.cfg.programs; i++ {
		n := i
		if i == s.cfg.programs {
			n = sharedIndex
		}
		d := s.dir(n)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(d, "main.gp"), []byte(source(n)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// one makes one build.
func (s *stress) one(n int, rng *rand.Rand) {
	cfg := s.cfg
	if cfg.stagger > 0 {
		time.Sleep(time.Duration(rng.Int63n(int64(cfg.stagger))))
	}
	prog := rng.Intn(cfg.programs)
	if rng.Float64() < cfg.sameShare {
		prog = sharedIndex
	}
	cxx := cfg.cxxs[rng.Intn(len(cfg.cxxs))]
	release := rng.Float64() < cfg.releaseFrac
	isRun := rng.Float64() < cfg.runFrac
	key := rng.Intn(cfg.runtimeKeys + 1) // 0: no flags of the environment (the precompiled header is used)
	noObj := rng.Float64() < cfg.noObjCache
	ep := s.epoch()
	defer ep.wg.Done()

	out := filepath.Join(cfg.work, "out", fmt.Sprintf("b%d", n))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return
	}
	defer os.Remove(out)
	args := []string{}
	if isRun {
		args = append(args, "run")
	} else {
		args = append(args, "build")
	}
	if release {
		args = append(args, "-release")
	}
	if !isRun {
		args = append(args, "-o", out)
	}
	args = append(args, ".")
	env := append(os.Environ(),
		"XDG_CACHE_HOME="+ep.cache,
		"GOCPP_CXX="+cxx,
		"GOCPP_MODCACHE=off",
		"GOCPP_OFFLINE=1",
		"GOCPP_GATE_PHASE=15",
	)
	if key > 0 {
		env = append(env, fmt.Sprintf("GOCPP_CXXFLAGS=-DBUILDSTRESS_KEY=%d", key))
	} else {
		env = append(env, "GOCPP_CXXFLAGS=")
	}
	if !cfg.sharedTmp {
		env = append(env, "TMPDIR="+s.tmp())
	}
	if cfg.wipeEvery == 0 {
		env = append(env, "GOCPP_WORKLOST=fail") // no cleaner here: a lost work folder is a bug of gocpp, not to be hidden
	}
	if cfg.objSize != "" {
		env = append(env, "GOCPP_OBJCACHE_SIZE="+cfg.objSize)
	}
	if noObj {
		env = append(env, "GOCPP_NOOBJCACHE=1")
	}
	argv := append([]string{cfg.gocpp}, args...)
	if cfg.taskset != "" {
		argv = append([]string{"taskset", "-c", cfg.taskset}, argv...)
	}
	desc := fmt.Sprintf("#%d epoch %d cxx=%s key=%d release=%v run=%v noobj=%v prog=%d: gocpp %s", n, ep.n, cxx, key, release, isRun, noObj, prog, strings.Join(args, " "))
	cctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	cmd.Dir = s.dir(prog)
	cmd.Env = env
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	setupCmd(cmd)
	cmd.WaitDelay = 5 * time.Second
	t0 := time.Now()
	err := cmd.Run()
	text := buf.String()
	if strings.Contains(text, "was removed while the build ran") {
		s.recovered.Add(1)
		var keep []string
		for _, l := range strings.SplitAfter(text, "\n") {
			if !strings.Contains(l, "was removed while the build ran") {
				keep = append(keep, l)
			}
		}
		text = strings.Join(keep, "")
	}
	switch {
	case err != nil:
		s.fail("build-failed", desc, firstLine(err.Error())+"\n"+tail(text))
	case isRun:
		if text != want(progNumber(prog)) {
			s.fail("wrong-output", desc, fmt.Sprintf("got %q, want %q", text, want(progNumber(prog))))
		}
	default:
		o, err := exec.Command(out).CombinedOutput()
		if err != nil {
			s.fail("program-failed", desc, err.Error()+"\n"+tail(string(o)))
		} else if string(o) != want(progNumber(prog)) {
			s.fail("wrong-output", desc, fmt.Sprintf("got %q, want %q", o, want(progNumber(prog))))
		}
	}
	done := s.done.Add(1)
	if cfg.verbose {
		fmt.Printf("%4d done %s (%.1fs)\n", done, desc, time.Since(t0).Seconds())
	} else if done%50 == 0 {
		fmt.Printf("%d builds done, %d failed\n", done, s.failed())
	}
}

func progNumber(prog int) int { return prog }

// wipe removes what a cleaner removes: the work folder w, its contents, its contents that a shell glob
// names (rm -rf w/*: the names with a dot first stay), or everything in tmp.
func wipe(w, tmp, mode string) {
	removeEntries := func(dir string, all bool) {
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			if all || !strings.HasPrefix(e.Name(), ".") {
				os.RemoveAll(filepath.Join(dir, e.Name()))
			}
		}
	}
	switch mode {
	case "contents":
		removeEntries(w, true)
	case "glob":
		removeEntries(w, false)
	case "tmp":
		removeEntries(tmp, true)
	default:
		os.RemoveAll(w)
	}
}

// cleaner is the chaos: it removes what a cleaner of the temporary folder would, while builds run.
func (s *stress) cleaner(ctx context.Context) {
	rng := rand.New(rand.NewSource(s.cfg.seed - 1))
	t := time.NewTicker(s.cfg.wipeEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		works, _ := filepath.Glob(filepath.Join(s.tmp(), "gocpp-build*"))
		if len(works) == 0 {
			continue
		}
		wipe(works[rng.Intn(len(works))], s.tmp(), s.cfg.wipeMode)
		s.wipes.Add(1)
	}
}

func (s *stress) fail(kind, desc, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, failure{desc, output, kind})
	s.kinds[classify(output, kind)]++
}

func (s *stress) failed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.failures)
}

// classify groups the failures by the first line that says what went wrong.
func classify(output, kind string) string {
	for _, l := range strings.Split(output, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "exit status") {
			continue
		}
		// leave out the random names
		if i := strings.Index(l, "/gocpp-build"); i >= 0 {
			l = l[:i] + "/gocpp-build<N>..."
		}
		if len(l) > 110 {
			l = l[:110]
		}
		return kind + ": " + l
	}
	return kind
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 25 {
		lines = append([]string{"..."}, lines[len(lines)-25:]...)
	}
	return strings.Join(lines, "\n")
}

func (s *stress) report(d time.Duration) int {
	total := int(s.done.Load())
	left := s.leftovers()
	fmt.Printf("\nbuildstress: %d builds in %.0fs, %d failed, %d work folders left behind (seed %d)\n", total, d.Seconds(), len(s.failures), len(left), s.cfg.seed)
	if s.cfg.wipeEvery > 0 {
		fmt.Printf("chaos: the cleaner removed %d work folders (%s); %d builds found theirs gone and built again\n", s.wipes.Load(), s.cfg.wipeMode, s.recovered.Load())
	}
	var ks []string
	for k := range s.kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("  %4d  %s\n", s.kinds[k], k)
	}
	for i, f := range s.failures {
		if i >= s.cfg.maxShow {
			fmt.Printf("... %d more failures\n", len(s.failures)-i)
			break
		}
		fmt.Printf("\nFAILURE %s\n%s\n", f.desc, f.output)
	}
	for _, l := range left {
		fmt.Println("LEFT BEHIND:", l)
	}
	if len(s.failures) > 0 || len(left) > 0 {
		return 1
	}
	return 0
}

// leftovers lists what the gocpp processes left in their temporary folder: a build removes its
// work folder, and every gen/<hash>-<pid>-<n> folder of a build that could not lock, when it succeeds.
func (s *stress) leftovers() []string {
	if s.cfg.sharedTmp {
		return nil
	}
	ents, _ := os.ReadDir(s.tmp())
	var out []string
	for _, e := range ents {
		// (A compiler's own temporary files, such as g++'s ccXXXX.ltrans0.o of a link that was
		// killed or lost its folder, are the compiler's: not counted.)
		if strings.HasPrefix(e.Name(), "gocpp") {
			out = append(out, filepath.Join(s.tmp(), e.Name()))
		}
	}
	return out
}
