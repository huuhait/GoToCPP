package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// RunConfig is everything a Runner needs besides the tests.
type RunConfig struct {
	Root         string        // the folder that has test/ (Go's tests, from the cache)
	Gocpp        string        // the gocpp program
	Go           string        // real Go, for runoutput and errorcheckoutput ("" = go in PATH)
	Work         string        // the folder of the temporary folders
	GocppCache   string        // XDG_CACHE_HOME of the gocpp commands (the runtime is compiled once into it)
	CXX          string        // GOCPP_CXX ("" = leave the environment's)
	ASan         bool          // build with -asan
	BuildTimeout time.Duration // one gocpp build or check
	RunTimeout   time.Duration // one test program (a test's -t flag raises it)
	OutLimit     int           // bytes of output kept per command
	Env          []string      // extra environment entries for the commands
	GateOutput   int           // runoutput tests that run at the same time (default 1)
}

// A Runner runs tests of Go's test/ folder with gocpp.
type Runner struct {
	RunConfig
	outGate chan struct{}

	mu      sync.Mutex
	running map[*exec.Cmd]bool
	stopped bool
}

// track records a running command; after StopAll a command that starts is
// killed at once.
func (r *Runner) track(cmd *exec.Cmd, runs bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running == nil {
		r.running = map[*exec.Cmd]bool{}
	}
	switch {
	case !runs:
		delete(r.running, cmd)
	case r.stopped:
		killGroup(cmd)
	default:
		r.running[cmd] = true
	}
}

// StopAll kills the process groups of the commands that run now, and of the
// ones that start later (a signal arrived: the tool is about to end).
func (r *Runner) StopAll() {
	r.mu.Lock()
	r.stopped = true
	cmds := make([]*exec.Cmd, 0, len(r.running))
	for c := range r.running {
		cmds = append(cmds, c)
	}
	r.mu.Unlock()
	for _, c := range cmds {
		killGroup(c)
	}
}

// Stopped reports whether StopAll was called.
func (r *Runner) Stopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopped
}

// NewRunner returns a runner with the defaults filled in.
func NewRunner(c RunConfig) *Runner {
	if c.BuildTimeout == 0 {
		c.BuildTimeout = 5 * time.Minute
	}
	if c.RunTimeout == 0 {
		c.RunTimeout = 20 * time.Second
	}
	if c.OutLimit == 0 {
		c.OutLimit = 1 << 20
	}
	if c.Go == "" {
		c.Go = "go"
	}
	if c.GateOutput == 0 {
		c.GateOutput = 1
	}
	if c.Work == "" {
		c.Work = os.TempDir()
	}
	noCoreFiles()
	return &Runner{RunConfig: c, outGate: make(chan struct{}, c.GateOutput)}
}

// limitBuf keeps the first n bytes written and drops the rest.
type limitBuf struct {
	mu   sync.Mutex
	b    bytes.Buffer
	n    int
	over bool
}

func (l *limitBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := l.n - l.b.Len(); room < len(p) {
		l.over = true
		if room > 0 {
			l.b.Write(p[:room])
		}
	} else {
		l.b.Write(p)
	}
	return len(p), nil
}

func (l *limitBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := l.b.String()
	if l.over {
		s += "\n[output cut]"
	}
	return s
}

// cmdResult is what a command did.
type cmdResult struct {
	Out      string
	Code     int // exit status; -1 when killed by a signal
	TimedOut bool
	Err      error // the command could not start
}

func (c cmdResult) ok() bool { return c.Err == nil && !c.TimedOut && c.Code == 0 }

// exec runs a command in a process group of its own, in dir, with stdout and
// stderr in one stream (as Go's runner does), and kills it and what it
// started when the time limit passes.
func (r *Runner) exec(dir string, env []string, timeout time.Duration, name string, args ...string) cmdResult {
	return r.execLimit(r.OutLimit, dir, env, timeout, name, args...)
}

// execLimit is exec with its own limit of kept output bytes (the program that
// a runoutput test writes is megabytes long).
func (r *Runner) execLimit(limit int, dir string, env []string, timeout time.Duration, name string, args ...string) cmdResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	inGroup(cmd)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	buf := &limitBuf{n: limit}
	cmd.Stdout, cmd.Stderr = buf, buf
	err := cmd.Start()
	if err == nil {
		r.track(cmd, true)
		err = cmd.Wait()
		r.track(cmd, false)
	}
	killGroup(cmd)
	res := cmdResult{Out: buf.String()}
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.TimedOut = true
		res.Code = -1
	case err == nil:
	default:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.Code = ee.ExitCode()
		} else {
			res.Err = err
			res.Code = -1
		}
	}
	return res
}

// gocppEnv is the environment of the gocpp commands: its own cache, no
// downloads, every language feature enabled (the features of later phases
// give an error naming the phase otherwise, which hides what the test is
// about).
func (r *Runner) gocppEnv() []string {
	env := []string{"GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail", "GOCPP_GATE_PHASE=15", "GOFLAGS=", "GOENV=off"}
	if r.GocppCache != "" {
		env = append(env, "XDG_CACHE_HOME="+r.GocppCache)
	}
	if r.CXX != "" {
		env = append(env, "GOCPP_CXX="+r.CXX)
	}
	return append(env, r.Env...)
}

func (r *Runner) gocpp(dir string, args ...string) cmdResult {
	return r.exec(dir, r.gocppEnv(), r.BuildTimeout, r.Gocpp, args...)
}

// A failure is a test that did not pass.
type failure struct {
	Stage  string
	Detail string
}

func (f *failure) String() string { return f.Stage + ": " + f.Detail }

func fail(stage, format string, args ...any) *failure {
	return &failure{stage, fmt.Sprintf(format, args...)}
}

// RunTest runs one applicable test and returns its result.
func (r *Runner) RunTest(t Test) Result {
	start := time.Now()
	res := Result{Path: t.Path, Directive: t.Directive}
	f := r.runTest(t)
	res.Millis = time.Since(start).Milliseconds()
	if f == nil {
		res.Outcome = Pass
		return res
	}
	res.Outcome = Fail
	stage := f.Stage
	if strings.HasPrefix(stage, "timeout-") {
		res.Outcome = Timeout
	}
	res.Stage = strings.TrimPrefix(stage, "timeout-")
	res.Cluster = clusterOf(stage, f.Detail)
	res.Detail = truncate(f.Detail, 1500)
	return res
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n[...]"
}

func (r *Runner) runTest(t Test) (f *failure) {
	src, err := os.ReadFile(filepath.Join(r.Root, "test", filepath.FromSlash(t.Path)))
	if err != nil {
		return fail("runner", "%v", err)
	}
	rec, err := ParseRecipe(string(src))
	if err != nil {
		return fail("runner", "%v", err)
	}
	work, err := os.MkdirTemp(r.Work, "gotest-")
	if err != nil {
		return fail("runner", "%v", err)
	}
	defer os.RemoveAll(work)
	defer func() {
		if p := recover(); p != nil {
			f = fail("runner", "internal error: %v", p)
		}
	}()
	tc := &testCase{r: r, t: t, rec: rec, src: string(src), work: work}
	return tc.run()
}

// A testCase is one test being run.
type testCase struct {
	r    *Runner
	t    Test
	rec  Recipe
	src  string
	work string
}

func (tc *testCase) stem() string    { return strings.TrimSuffix(path.Base(tc.t.Path), ".go") }
func (tc *testCase) testDir() string { return filepath.Join(tc.r.Root, "test") }

// progTimeout is the time limit of the program: the runner's, or the test's
// own (-t seconds, scaled like Go's GO_TEST_TIMEOUT_SCALE by what we need).
func (tc *testCase) progTimeout() time.Duration {
	d := tc.r.RunTimeout
	if tc.rec.Timeout > 0 {
		if t := time.Duration(tc.rec.Timeout) * time.Second; t > d {
			d = t
		}
	}
	return d
}

func (tc *testCase) run() *failure {
	switch tc.rec.Action {
	case "run":
		return tc.buildAndRun(tc.src, tc.stem()+".gp", tc.testDir())
	case "buildrun":
		return tc.buildAndRun(tc.src, tc.stem()+".gp", tc.work)
	case "build", "compile":
		return tc.compileFile()
	case "errorcheck":
		return tc.errorcheckFile()
	case "runoutput", "errorcheckoutput":
		return tc.viaGo()
	case "rundir", "compiledir", "errorcheckdir", "errorcheckandrundir", "builddir", "buildrundir", "runindir":
		return tc.dirTest()
	}
	return fail("runner", "directive %q is not supported by the runner", tc.rec.Action)
}

// writeGp writes src as work/name.
func (tc *testCase) writeGp(src, name string) (string, error) {
	p := filepath.Join(tc.work, name)
	return p, os.WriteFile(p, []byte(src), 0o644)
}

// buildExe builds gocpp's program from the arguments (.gp files or a folder).
func (tc *testCase) buildExe(dir string, args ...string) (string, *failure) {
	exe := filepath.Join(tc.work, "prog.exe")
	a := []string{"build", "-o", exe}
	if tc.r.ASan {
		a = append(a, "-asan")
	}
	res := tc.r.gocpp(dir, append(a, args...)...)
	switch {
	case res.Err != nil:
		return "", fail("runner", "%v", res.Err)
	case res.TimedOut:
		return "", fail("timeout-build", "gocpp build ran out of time (%v)\n%s", tc.r.BuildTimeout, res.Out)
	case res.Code != 0:
		return "", fail("build", "%s", res.Out)
	}
	return exe, nil
}

// runExe runs the program and compares its output with the .out file (a
// missing file means empty output): exit status 0 is required.
func (tc *testCase) runExe(exe, dir string) *failure {
	res := tc.r.exec(dir, nil, tc.progTimeout(), exe, tc.rec.Args...)
	switch {
	case res.Err != nil:
		return fail("runner", "%v", res.Err)
	case res.TimedOut:
		return fail("timeout-run", "the program ran out of time (%v)\n%s", tc.progTimeout(), res.Out)
	case res.Code != 0:
		return fail("run", "exit status %d\n%s", res.Code, res.Out)
	}
	return tc.compareOutput(res.Out)
}

// compareOutput compares output with the .out file of the test.
func (tc *testCase) compareOutput(got string) *failure {
	want, err := os.ReadFile(filepath.Join(tc.testDir(), filepath.FromSlash(strings.TrimSuffix(tc.t.Path, ".go")+".out")))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fail("runner", "%v", err)
	}
	got = strings.ReplaceAll(got, "\r\n", "\n")
	if got != string(want) {
		return fail("compare", "output differs from %s.out: %s", tc.stem(), diffSummary(string(want), got))
	}
	return nil
}

// diffSummary shows the first line where want and got differ.
func diffSummary(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b || i >= len(w) || i >= len(g) {
			return fmt.Sprintf("line %d: want %q, got %q (%d and %d lines)", i+1, truncate(a, 120), truncate(b, 120), len(w), len(g))
		}
	}
	return "equal lines"
}

func (tc *testCase) buildAndRun(src, name, runDir string) *failure {
	p, err := tc.writeGp(src, name)
	if err != nil {
		return fail("runner", "%v", err)
	}
	exe, f := tc.buildExe(tc.work, p)
	if f != nil {
		return f
	}
	return tc.runExe(exe, runDir)
}

var (
	pkgRx      = regexp.MustCompile(`(?m)^package ([\p{Lu}\p{Ll}\w]+)`)
	funcMainRx = regexp.MustCompile(`(?m)^func main\(\)`)
)

func packageName(src string) string {
	if m := pkgRx.FindStringSubmatch(src); m != nil {
		return m[1]
	}
	return ""
}

// compileFile is compile and build: the program must build with the C++
// compiler. A package other than main is imported by a small main of a
// module, so that its C++ is compiled and linked too.
func (tc *testCase) compileFile() *failure {
	name := tc.stem() + ".gp"
	if hasBodylessFunc(tc.src) {
		// Go compiles to an object file and never links: a function declared without
		// a body (implemented in assembly) is fine. gocpp checks the package only.
		p, err := tc.writeGp(tc.src, name)
		if err != nil {
			return fail("runner", "%v", err)
		}
		return tc.checkOnly(tc.work, "-e", p)
	}
	if packageName(tc.src) == "main" && funcMainRx.MatchString(tc.src) {
		p, err := tc.writeGp(tc.src, name)
		if err != nil {
			return fail("runner", "%v", err)
		}
		_, f := tc.buildExe(tc.work, p)
		return f
	}
	if packageName(tc.src) == "main" {
		// no func main: only the front end can look at it
		p, err := tc.writeGp(tc.src, name)
		if err != nil {
			return fail("runner", "%v", err)
		}
		return tc.checkOnly(tc.work, "-e", p)
	}
	mod := filepath.Join(tc.work, "mod")
	pdir := filepath.Join(mod, "p")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		return fail("runner", "%v", err)
	}
	for name, content := range map[string]string{
		"gocpp.mod":  "module tmod\n",
		"p/" + name:  tc.src,
		"zz_main.gp": "package main\n\nimport _ \"tmod/p\"\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(mod, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			return fail("runner", "%v", err)
		}
	}
	_, f := tc.buildExe(mod, ".")
	return f
}

// checkOnly runs gocpp check and fails when it reports anything.
func (tc *testCase) checkOnly(dir string, args ...string) *failure {
	res := tc.r.gocpp(dir, append([]string{"check"}, args...)...)
	switch {
	case res.Err != nil:
		return fail("runner", "%v", res.Err)
	case res.TimedOut:
		return fail("timeout-check", "gocpp check ran out of time\n%s", res.Out)
	case res.Code != 0:
		return fail("check", "%s", res.Out)
	}
	return nil
}

// errorcheckFile checks the diagnostics of gocpp check against the ERROR
// comments (see ErrorCheck).
func (tc *testCase) errorcheckFile() *failure {
	short := path.Base(tc.t.Path)
	p, err := tc.writeGp(tc.src, tc.stem()+".gp")
	if err != nil {
		return fail("runner", "%v", err)
	}
	res := tc.r.gocpp(tc.work, "check", "-e", p)
	return tc.errorcheckResult(res, []SrcFile{{Short: short, Full: short, Src: tc.src}},
		map[string]string{tc.stem() + ".gp": short}, tc.rec.WantError, hasFlag(tc.rec.Flags, "-auto"))
}

func hasFlag(flags []string, f string) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}

// errorcheckResult turns the output of gocpp check into a verdict.
func (tc *testCase) errorcheckResult(res cmdResult, files []SrcFile, names map[string]string, wantError, auto bool) *failure {
	if res.Err != nil {
		return fail("runner", "%v", res.Err)
	}
	if res.TimedOut {
		return fail("timeout-check", "gocpp check ran out of time\n%s", res.Out)
	}
	if wantError && res.Code == 0 {
		return fail("errorcheck", "succeeded\ngocpp check accepted the program, Go rejects it\n%s", res.Out)
	}
	if !wantError && res.Code != 0 {
		return fail("errorcheck", "failed\n%s", res.Out)
	}
	if res.Code > 1 {
		return fail("check", "gocpp check exit status %d\n%s", res.Code, res.Out)
	}
	ms, err := ErrorCheck(NormalizeGocpp(res.Out, names), auto, files)
	if err != nil {
		return fail("runner", "%v", err)
	}
	if len(ms) == 0 {
		return nil
	}
	return failErrorcheck(ms)
}

// failErrorcheck describes the mismatches: the first line of the detail is
// the kind of the most basic mismatch (a missing error comes before text
// differences, which come before extra errors).
func failErrorcheck(ms []Mismatch) *failure {
	kinds := map[string]int{}
	for _, m := range ms {
		kinds[m.Kind]++
	}
	kind := "unmatched"
	for _, k := range []string{"missing", "nomatch", "unmatched"} {
		if kinds[k] > 0 {
			kind = k
			break
		}
	}
	var b strings.Builder
	b.WriteString(kind + "\n")
	for i, m := range ms {
		if i == 6 {
			fmt.Fprintf(&b, "... and %d more\n", len(ms)-i)
			break
		}
		b.WriteString(m.String() + "\n")
	}
	return &failure{"errorcheck", b.String()}
}

// genLimit is how many bytes of the generated program of a runoutput test are
// kept (rotate0.go writes 1.5 MB).
const genLimit = 64 << 20

// viaGo is runoutput and errorcheckoutput: real Go runs the test, and its
// output is the program that gocpp then runs (or checks).
func (tc *testCase) viaGo() *failure {
	tc.r.outGate <- struct{}{}
	defer func() { <-tc.r.outGate }()
	genv := []string{"GOENV=off", "GOFLAGS="}
	// the path relative to test/, as in Go's runner: an argument that is a
	// .go file (index1.go gets ./index.go) must be in the test's folder
	rel := "./" + tc.t.Path
	res := tc.r.execLimit(genLimit, tc.testDir(), genv, 5*time.Minute, tc.r.Go, append([]string{"run", rel}, tc.rec.Args...)...)
	if !res.ok() {
		return fail("go", "go run %s: exit %d timeout=%v\n%s", tc.t.Path, res.Code, res.TimedOut, res.Out)
	}
	gen := res.Out
	if tc.rec.Action == "errorcheckoutput" {
		p, err := tc.writeGp(gen, "tmp__.gp")
		if err != nil {
			return fail("runner", "%v", err)
		}
		r2 := tc.r.gocpp(tc.work, "check", "-e", p)
		return tc.errorcheckResult(r2, []SrcFile{{Short: "tmp__.go", Full: "tmp__.go", Src: gen}},
			map[string]string{"tmp__.gp": "tmp__.go"}, tc.rec.WantError, false)
	}
	p, err := tc.writeGp(gen, "tmp__.gp")
	if err != nil {
		return fail("runner", "%v", err)
	}
	exe, f := tc.buildExe(tc.work, p)
	if f != nil {
		return f
	}
	return tc.runExe(exe, tc.testDir())
}

// A dirPkg is one package of a .dir folder.
type dirPkg struct {
	Name  string   // the package clause name
	Stem  string   // the first file without .go: the import path "./Stem"
	Files []string // file names, in order
}

// dirPackages groups the .go files of a .dir folder by package name (or
// one package per file with -s), in the order of the first file, as Go's
// runner does.
func dirPackages(files map[string]string, single bool) []*dirPkg {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var pkgs []*dirPkg
	by := map[string]*dirPkg{}
	for _, n := range names {
		pn := packageName(files[n])
		p, ok := by[pn]
		if single || !ok {
			p = &dirPkg{Name: pn, Stem: strings.TrimSuffix(n, ".go")}
			pkgs = append(pkgs, p)
			by[pn] = p
		}
		p.Files = append(p.Files, n)
	}
	return pkgs
}

var relImportRx = regexp.MustCompile(`(?m)^(\s*(?:import\s+)?(?:[A-Za-z_.][A-Za-z_0-9]*\s+)?)"\./([^"]+)"`)

// rewriteImports turns the relative imports of Go's tests ("./p") into
// imports of the module that holds the packages, and the module path of a
// runindir test (modName) too.
func rewriteImports(src, modName string) string {
	src = relImportRx.ReplaceAllString(src, `${1}"tmod/$2"`)
	if modName != "" {
		src = strings.ReplaceAll(src, `"`+modName+`/`, `"tmod/`)
	}
	return src
}

// readDir reads the .go files of the .dir folder of the test.
func (tc *testCase) readDir() (map[string]string, error) {
	dir := filepath.Join(tc.testDir(), filepath.FromSlash(strings.TrimSuffix(tc.t.Path, ".go")+".dir"))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		files[e.Name()] = string(b)
	}
	return files, nil
}

// materialize writes the packages of a .dir folder as a module: one folder
// per package, named after its first file.
func (tc *testCase) materialize(mod string, pkgs []*dirPkg, files map[string]string, modName string) error {
	if err := os.MkdirAll(mod, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(mod, "gocpp.mod"), []byte("module tmod\n"), 0o644); err != nil {
		return err
	}
	for _, p := range pkgs {
		d := filepath.Join(mod, p.Stem)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		for _, n := range p.Files {
			gp := strings.TrimSuffix(n, ".go") + ".gp"
			if err := os.WriteFile(filepath.Join(d, gp), []byte(rewriteImports(files[n], modName)), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// dirTest runs the tests with a .dir folder of packages.
func (tc *testCase) dirTest() *failure {
	files, err := tc.readDir()
	if err != nil {
		return fail("runner", "%v", err)
	}
	if len(files) == 0 {
		return fail("runner", "no .go files in the .dir folder")
	}
	act := tc.rec.Action
	mod := filepath.Join(tc.work, "mod")
	if act == "runindir" {
		return tc.runInDir(mod)
	}
	var pkgs []*dirPkg
	if act == "builddir" || act == "buildrundir" {
		// one main package of all the files
		all := &dirPkg{Name: "main", Stem: "app"}
		for n := range files {
			all.Files = append(all.Files, n)
		}
		sort.Strings(all.Files)
		pkgs = []*dirPkg{all}
	} else {
		pkgs = dirPackages(files, tc.rec.Single)
	}
	if err := tc.materialize(mod, pkgs, files, ""); err != nil {
		return fail("runner", "%v", err)
	}
	last := pkgs[len(pkgs)-1]
	pkgPath := func(p *dirPkg) string { return "./" + p.Stem }

	switch act {
	case "compiledir":
		for _, p := range pkgs {
			if f := tc.checkOnly(mod, "-e", pkgPath(p)); f != nil {
				return f
			}
		}
		for _, src := range files {
			if hasBodylessFunc(src) {
				return nil // checked above; Go never links a compiledir test
			}
		}
		return tc.buildAllPackages(mod, pkgs, files)
	case "builddir":
		_, f := tc.buildExe(mod, pkgPath(last))
		return f
	case "buildrundir":
		exe, f := tc.buildExe(mod, pkgPath(last))
		if f != nil {
			return f
		}
		return tc.runExe(exe, tc.work)
	}
	// rundir, errorcheckdir, errorcheckandrundir: the packages are checked in order
	errPkg := len(pkgs) - 1
	if tc.rec.WantError && act == "errorcheckandrundir" {
		errPkg--
	}
	if act == "rundir" {
		errPkg = -1
	}
	for i, p := range pkgs {
		if act == "rundir" {
			if f := tc.checkOnly(mod, "-e", pkgPath(p)); f != nil {
				return f
			}
			continue
		}
		res := tc.r.gocpp(mod, "check", "-e", pkgPath(p))
		var srcs []SrcFile
		names := map[string]string{}
		for _, n := range p.Files {
			short := n
			srcs = append(srcs, SrcFile{Short: short, Full: short, Src: files[n]})
			names[strings.TrimSuffix(n, ".go")+".gp"] = short
		}
		want := i == errPkg && tc.rec.WantError
		if i != errPkg && res.ok() {
			continue
		}
		if i != errPkg && !res.TimedOut && res.Err == nil {
			return fail("check", "package %s: %s", p.Stem, res.Out)
		}
		if f := tc.errorcheckResult(res, srcs, names, want, false); f != nil {
			return f
		}
	}
	if act == "errorcheckdir" {
		return nil
	}
	exe, f := tc.buildExe(mod, pkgPath(last))
	if f != nil {
		return f
	}
	return tc.runExe(exe, tc.work)
}

// buildAllPackages builds a program that links every package of a
// compiledir test: the main package if there is one, else a main that
// imports the others.
func (tc *testCase) buildAllPackages(mod string, pkgs []*dirPkg, files map[string]string) *failure {
	var others []*dirPkg
	for _, p := range pkgs {
		if p.Name == "main" && hasMain(p, files) {
			_, f := tc.buildExe(mod, "./"+p.Stem)
			return f
		}
		if p.Name != "main" {
			others = append(others, p)
		}
	}
	var b strings.Builder
	b.WriteString("package main\n\n")
	for _, p := range others {
		fmt.Fprintf(&b, "import _ \"tmod/%s\"\n", p.Stem)
	}
	b.WriteString("\nfunc main() {}\n")
	d := filepath.Join(mod, "zz_main")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return fail("runner", "%v", err)
	}
	if err := os.WriteFile(filepath.Join(d, "main.gp"), []byte(b.String()), 0o644); err != nil {
		return fail("runner", "%v", err)
	}
	_, f := tc.buildExe(mod, "./zz_main")
	return f
}

// hasBodylessFunc reports whether the source declares a function without a body
// (a declaration for assembly or linkname).
func hasBodylessFunc(src string) bool {
	if !strings.Contains(src, "func ") {
		return false
	}
	f, _ := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.SkipObjectResolution)
	if f == nil {
		return false
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body == nil {
			return true
		}
	}
	return false
}

// hasMain reports whether a main package declares func main (a compiledir
// test may have a package main that is only checked).
func hasMain(p *dirPkg, files map[string]string) bool {
	for _, n := range p.Files {
		if funcMainRx.MatchString(files[n]) {
			return true
		}
	}
	return false
}

// runInDir copies the .dir folder as a module (go run . there), with .go
// renamed to .gp.
func (tc *testCase) runInDir(mod string) *failure {
	src := filepath.Join(tc.testDir(), filepath.FromSlash(strings.TrimSuffix(tc.t.Path, ".go")+".dir"))
	modName := filepath.Base(src)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		dst := filepath.Join(mod, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".go") {
			dst = strings.TrimSuffix(dst, ".go") + ".gp"
			b = []byte(rewriteImports(string(b), modName))
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		return fail("runner", "%v", err)
	}
	if err := os.WriteFile(filepath.Join(mod, "gocpp.mod"), []byte("module tmod\n"), 0o644); err != nil {
		return fail("runner", "%v", err)
	}
	exe, f := tc.buildExe(mod, ".")
	if f != nil {
		return f
	}
	return tc.runExe(exe, mod)
}
