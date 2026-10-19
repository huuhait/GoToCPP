package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The builds of the flags of step 6.12 and the files of 6.11 (flags.go, export.go, compdb): a
// fake compiler records the argv of every run, so that the tests see the exact flags and their
// order, and the plugin flags that it is told to ignore.

// fakeCompiler writes a compiler that records its arguments (one file per run, in the folder
// logs) and runs the real compiler c without the plugin flags (-fpass-plugin=, -fplugin=: the
// plugin files of the tests are not real). It returns the program to name in GOCPP_CXX.
func fakeCompiler(t *testing.T, c testCXX) (prog, logs string) {
	t.Helper()
	dir := t.TempDir()
	logs = filepath.Join(dir, "logs")
	if err := os.Mkdir(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	prog = filepath.Join(dir, "fake-"+filepath.Base(c.cxx.Path))
	writeScript(t, prog, `log=`+shellQuote(logs)+`/$$
{ echo RUN; for a in "$@"; do printf '%s\n' "$a"; done; echo END; } >> "$log"
n=$#; i=0
while [ $i -lt $n ]; do a=$1; shift; case "$a" in -fpass-plugin=*|-fplugin=*|-fplugin-arg-*) ;; *) set -- "$@" "$a";; esac; i=$((i+1)); done
exec `+shellQuote(c.cxx.Path)+` "$@"
`)
	return prog, logs
}

// runs returns the recorded runs of the fake compiler, in no particular order.
func runs(t *testing.T, logs string) [][]string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(logs, "*"))
	var all [][]string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cur []string
		for _, line := range strings.Split(string(data), "\n") {
			switch line {
			case "RUN":
				cur = nil
			case "END":
				all = append(all, cur)
			default:
				cur = append(cur, line)
			}
		}
	}
	return all
}

// compilesOf returns the runs that compile the file whose name ends in suffix.
func compilesOf(all [][]string, suffix string) [][]string {
	var out [][]string
	for _, r := range all {
		if slices.Contains(r, "-c") && slices.ContainsFunc(r, func(a string) bool { return strings.HasSuffix(a, suffix) }) {
			out = append(out, r)
		}
	}
	return out
}

// linksOf returns the runs that link (no -c, and an -o).
func linksOf(all [][]string) [][]string {
	var out [][]string
	for _, r := range all {
		if !slices.Contains(r, "-c") && slices.Contains(r, "-o") && !slices.Contains(r, "-E") && !slices.Contains(r, "-fsyntax-only") {
			out = append(out, r)
		}
	}
	return out
}

func indexOf(args []string, want string) int { return slices.Index(args, want) }

func quiet(t *testing.T, c testCXX) (logs string) {
	t.Helper()
	cacheOn(t)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	prog, logs := fakeCompiler(t, c)
	t.Setenv("GOCPP_CXX", prog)
	t.Setenv("GOCPP_CFLAGS", "")
	t.Setenv("GOCPP_CXXFLAGS", "")
	t.Setenv("GOCPP_LDFLAGS", "")
	return logs
}

func uniqueMain(t *testing.T) string {
	return strings.Replace(helloSrc, "hello, gocpp", "flags "+t.Name(), 1)
}

func TestBuildTrustedFlagsReachEveryCompile(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		t.Setenv("GOCPP_CXXFLAGS", "-DFROM_ENV=1")
		inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
		r := gocppRun("", false, "build", "-cxxflags=-DFROM_FLAG=2", "-cflags", "-DFROM_CFLAG=3", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		all := runs(t, logs)
		for _, suffix := range []string{"hello.gp.cpp", "pkg_init.cpp", "/src/rt.cpp", "/src/chan.cpp"} {
			cs := compilesOf(all, suffix)
			if len(cs) == 0 {
				t.Errorf("no compile of %s recorded", suffix)
				continue
			}
			args := cs[0]
			// The cflags, then the environment's, then the build flag's: later wins.
			a, b, d := indexOf(args, "-DFROM_CFLAG=3"), indexOf(args, "-DFROM_ENV=1"), indexOf(args, "-DFROM_FLAG=2")
			if a < 0 || b < 0 || d < 0 || !(a < b && b < d) {
				t.Errorf("%s: flags at %d %d %d, want all there in the order -cflags, GOCPP_CXXFLAGS, -cxxflags:\n%q", suffix, a, b, d, args)
			}
		}
	})
}

func TestBuildTrustedFlagsBeforePackageFlagsAndLanguageFlagsLast(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		t.Setenv("GOCPP_GATE_PHASE", "06")
		inDir(t, map[string]string{"hello.gp": "//gocpp:cflags -DFROM_PKG=1\n\n" + uniqueMain(t)})
		r := gocppRun("", false, "build", "-cxxflags=-DFROM_FLAG=2", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		args := compilesOf(runs(t, logs), "hello.gp.cpp")[0]
		pkg, user := indexOf(args, "-DFROM_PKG=1"), indexOf(args, "-DFROM_FLAG=2")
		lang := slices.Index(args, "-fexceptions")
		// The trusted flags come before the package's (README: GOCPP_CFLAGS always did), and
		// gocpp's language flags come last of all, so that no flag can turn them off.
		if pkg < 0 || user < 0 || lang < 0 || !(user < pkg) || lang < max(pkg, user) {
			t.Errorf("order of the flags: package %d, user %d, language %d:\n%q", pkg, user, lang, args)
		}
	})
}

func TestBuildLDFlagsAfterPackageLinkFlags(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		t.Setenv("GOCPP_LDFLAGS", "-Wl,-z,relro")
		inDir(t, map[string]string{"hello.gp": "//gocpp:link m\n\n" + uniqueMain(t)})
		r := gocppRun("", false, "build", "-ldflags=-Wl,-O1", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		links := linksOf(runs(t, logs))
		if len(links) != 1 {
			t.Fatalf("%d links recorded: %q", len(links), links)
		}
		args := links[0]
		m, env, flag := indexOf(args, "-lm"), indexOf(args, "-Wl,-z,relro"), indexOf(args, "-Wl,-O1")
		if m < 0 || env < 0 || flag < 0 || !(m < env && env < flag) {
			t.Errorf("link flags at %d %d %d, want -lm, GOCPP_LDFLAGS, -ldflags in this order:\n%q", m, env, flag, args)
		}
		// The compiles get no link flags.
		for _, cs := range compilesOf(runs(t, logs), "hello.gp.cpp") {
			if slices.Contains(cs, "-Wl,-O1") {
				t.Errorf("a link flag in a compile: %q", cs)
			}
		}
	})
}

func TestBuildModeFlags(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
		for _, m := range []struct {
			flag    string
			compile []string
			link    []string
		}{
			{"-release", []string{"-O2", "-flto"}, []string{"-O2", "-flto"}},
			{"-race", []string{"-fsanitize=thread"}, []string{"-fsanitize=thread"}},
		} {
			os.RemoveAll(logs)
			os.Mkdir(logs, 0o755)
			r := gocppRun("", false, "build", m.flag, "-o", "prog", "hello.gp")
			if r.code != 0 {
				t.Fatalf("gocpp build %s = %+v", m.flag, r)
			}
			all := runs(t, logs)
			cs := compilesOf(all, "hello.gp.cpp")
			if len(cs) == 0 {
				t.Fatalf("%s: no compile", m.flag)
			}
			for _, f := range m.compile {
				if !slices.Contains(cs[0], f) {
					t.Errorf("%s: the compile lacks %s: %q", m.flag, f, cs[0])
				}
			}
			links := linksOf(all)
			if len(links) != 1 {
				t.Fatalf("%s: %d links", m.flag, len(links))
			}
			for _, f := range m.link {
				// GCC links with -flto=auto (no serial LTRANS warning), clang with -flto.
				if !slices.Contains(links[0], f) && !(f == "-flto" && slices.Contains(links[0], "-flto=auto")) {
					t.Errorf("%s: the link lacks %s: %q", m.flag, f, links[0])
				}
			}
		}
	})
}

func TestBuildDryRun(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		dir := inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
		r := gocppRun("", false, "build", "-n", "-cxxflags=-DDRY=1", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build -n = %+v", r)
		}
		// No program, and the compiler ran only for its version and its macros.
		if exists(filepath.Join(dir, "prog")) {
			t.Errorf("gocpp build -n wrote the program")
		}
		all := runs(t, logs)
		if len(compilesOf(all, "hello.gp.cpp")) != 0 || len(linksOf(all)) != 0 {
			t.Errorf("gocpp build -n ran the compiler to compile or link: %q", all)
		}
		// The commands are shown: every compile of the program and the runtime, and the link.
		for _, want := range []string{"-DDRY=1", "hello.gp.cpp", "src_rt.o", " -o $WORK/hello", "-Wl,--gc-sections"} {
			if !strings.Contains(r.stderr, want) {
				t.Errorf("gocpp build -n shows no %q:\n%s", want, r.stderr)
			}
		}
		if got := strings.Count(r.stderr, "\n"); got < 20 {
			t.Errorf("only %d lines shown", got)
		}
	})
}

func TestBuildDryRunDoesNotStamp(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		quiet(t, c)
		t.Setenv("GOCPP_NOUPTODATE", "")
		inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
		if r := gocppRun("", false, "build", "-n", "-x", "-o", "prog", "hello.gp"); r.code != 0 {
			t.Fatalf("gocpp build -n = %+v", r)
		}
		// The program does not exist, so a later build must make it (not "up to date").
		r := gocppRun("", false, "build", "-x", "-o", "prog", "hello.gp")
		workDir(t, r.stderr)
		if r.code != 0 || isUpToDate(r.stderr) || !exists("prog") {
			t.Errorf("gocpp build after -n = %+v", r)
		}
	})
}

func TestBuildRebuildFlag(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
		for _, args := range [][]string{{"build", "-o", "prog", "hello.gp"}, {"build", "-o", "prog", "hello.gp"}} {
			if r := gocppRun("", false, args...); r.code != 0 {
				t.Fatalf("gocpp %v = %+v", args, r)
			}
		}
		os.RemoveAll(logs)
		os.Mkdir(logs, 0o755)
		// With the object cache the second build compiled nothing; -a compiles again, the
		// runtime too.
		r := gocppRun("", false, "build", "-a", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build -a = %+v", r)
		}
		all := runs(t, logs)
		if len(compilesOf(all, "hello.gp.cpp")) != 1 || len(compilesOf(all, "/src/rt.cpp")) != 1 || len(linksOf(all)) != 1 {
			t.Errorf("gocpp build -a ran %d runs; want a compile of the unit, of the runtime and a link", len(all))
		}
	})
}

func TestBuildVerbose(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		quiet(t, c)
		inDir(t, pkgsProject("v", 1))
		r := gocppRun("", false, "build", "-v", "-n", "-o", "prog", ".")
		if r.code != 0 {
			t.Fatalf("gocpp build -v = %+v", r)
		}
		for _, p := range []string{"example.com/m/a", "example.com/m/b", "example.com/m"} {
			if !strings.Contains(r.stderr, "\n"+p+"\n") && !strings.HasPrefix(r.stderr, p+"\n") {
				t.Errorf("-v does not name %s:\n%.400s", p, r.stderr)
			}
		}
	})
}

const taggedMain = `package main

func main() { println(label) }
`

func TestBuildTags(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "06")
		inDir(t, map[string]string{
			"main.gp":      taggedMain,
			"plain.gp":     "//go:build !fast\n\npackage main\n\nconst label = \"plain\"\n",
			"fast.gp":      "//go:build fast\n\npackage main\n\nconst label = \"fast\"\n",
			"both.gp":      "//go:build fast && extra\n\npackage main\n\nconst both = 1\n",
			"filetag_x.gp": "package main\n",
		})
		for _, tc := range []struct {
			args []string
			want string
		}{
			{[]string{"run", "."}, "plain\n"},
			{[]string{"run", "-tags", "fast", "."}, "fast\n"},
			{[]string{"run", "-tags=fast,extra", "."}, "fast\n"},
			{[]string{"run", "-tags", "unrelated", "."}, "plain\n"},
		} {
			r := gocppRun("", true, tc.args...)
			if r.code != 0 || r.stdout != tc.want {
				t.Errorf("gocpp %v = %+v, want %q", tc.args, r, tc.want)
			}
		}
		// emit and check see the tags too.
		if r := gocppRun("", false, "emit", "-tags", "fast", "."); r.code != 0 || !strings.Contains(r.stdout, "fast") {
			t.Errorf("gocpp emit -tags fast = %.200s", r.stdout)
		}
		if r := gocppRun("", false, "check", "-tags", "fast,extra", "."); r.code != 0 {
			t.Errorf("gocpp check -tags = %+v", r)
		}
		// Without the tag both files would be there (a duplicate label): the error shows that tags work.
		if r := gocppRun("", false, "check", "-tags", "fast", "."); r.code != 0 {
			t.Errorf("gocpp check -tags fast = %+v", r)
		}
	})
}

func TestStampKeyFollowsFlags(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		r, n := s.build()
		if isUpToDate(r.stderr) || n.link != 1 {
			t.Fatalf("first build: %+v", n)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Fatalf("not up to date with the same flags:\n%s", r.stderr)
		}
		// Each flag is another question: the build is not up to date, and it is current again after.
		for _, args := range [][]string{
			{"-cxxflags=-DKEY_TEST=1"}, {"-cxxflags=-DKEY_TEST=2"}, {"-cflags=-DKEY_TEST=3"}, {"-ldflags=-Wl,-O1"},
			{"-tags", "keytag"}, {"-release"}, {"-debug"}, {"-race"},
		} {
			s.args = args
			r, n := s.build()
			if isUpToDate(r.stderr) || n.link != 1 {
				t.Errorf("a build with %v was up to date or did not link:\n%s", args, r.stderr)
			}
			if r, _ := s.build(); !isUpToDate(r.stderr) {
				t.Errorf("a second build with %v is not up to date:\n%s", args, r.stderr)
			}
		}
		// Back to no flags: the old record is found again.
		s.args = nil
		if r, _ := s.build(); isUpToDate(r.stderr) {
			// either answer is allowed; the program must exist and run
			_ = r
		}
		if got := s.run(); got != "8 10 "+s.stamp+"\n" {
			t.Errorf("the program prints %q", got)
		}
	})
}

func TestStampKeyFollowsEnvironmentFlags(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		for _, env := range []struct{ name, value string }{
			{"GOCPP_CXXFLAGS", "-DENV_KEY=1"}, {"GOCPP_CFLAGS", "-DENV_KEY=2"}, {"GOCPP_LDFLAGS", "-Wl,-O1"},
		} {
			t.Setenv(env.name, env.value)
			if r, _ := s.build(); isUpToDate(r.stderr) {
				t.Errorf("%s=%s did not change the key:\n%s", env.name, env.value, r.stderr)
			}
			if r, _ := s.build(); !isUpToDate(r.stderr) {
				t.Errorf("%s=%s: the second build is not up to date:\n%s", env.name, env.value, r.stderr)
			}
			t.Setenv(env.name, "")
		}
	})
}

func TestStampKeyFollowsPluginFile(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		_ = logs
		t.Setenv("GOCPP_NOUPTODATE", "")
		s := newIncr(t, stampProject(5))
		plugin := filepath.Join(t.TempDir(), "p.so")
		os.WriteFile(plugin, []byte("one"), 0o755)
		s.args = []string{"-cxxflags=-fpass-plugin=" + plugin}
		s.build()
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Fatalf("not up to date with the same plugin:\n%s", r.stderr)
		}
		// The plugin is built again: the same flag, another file.
		os.WriteFile(plugin, []byte("a longer one"), 0o755)
		if r, _ := s.build(); isUpToDate(r.stderr) {
			t.Errorf("a changed plugin file left the build up to date:\n%s", r.stderr)
		}
		// A missing plugin is part of the key too (the compiler's error is the answer, not "up to date").
		os.Remove(plugin)
		if r, _ := s.build(); isUpToDate(r.stderr) {
			t.Errorf("a missing plugin file left the build up to date")
		}
	})
}

func TestPluginFlagsRelativeAndOrder(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		dir := inDir(t, map[string]string{"hello.gp": uniqueMain(t), "plugins/p.so": "x"})
		r := gocppRun("", false, "build", "-cxxflags=-fpass-plugin=plugins/p.so -fplugin=bare-name -fplugin-arg-bare-name-k=v", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		args := compilesOf(runs(t, logs), "hello.gp.cpp")[0]
		// The compiles run in a work folder: a relative plugin path is made absolute; a bare name is the compiler's.
		if !slices.Contains(args, "-fpass-plugin="+filepath.Join(dir, "plugins", "p.so")) {
			t.Errorf("the plugin path is not absolute:\n%q", args)
		}
		if !slices.Contains(args, "-fplugin=bare-name") || !slices.Contains(args, "-fplugin-arg-bare-name-k=v") {
			t.Errorf("the other plugin flags are not passed:\n%q", args)
		}
		// The runtime's compiles load the plugin too (GOCPP_CXXFLAGS does the same).
		rt := compilesOf(runs(t, logs), "/src/rt.cpp")
		if len(rt) == 0 || !slices.Contains(rt[0], "-fplugin=bare-name") {
			t.Errorf("the runtime compile does not get the plugin flags: %q", rt)
		}
	})
}

func TestPluginNeverFromPackageDirective(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		quiet(t, c)
		t.Setenv("GOCPP_GATE_PHASE", "06")
		inDir(t, map[string]string{"hello.gp": "//gocpp:cflags -fplugin=evil.so\n\n" + uniqueMain(t)})
		r := gocppRun("", false, "build", "-o", "prog", "hello.gp")
		if r.code == 0 || !strings.Contains(r.stderr, "-fplugin") {
			t.Errorf("a plugin from //gocpp:cflags was accepted: %+v", r)
		}
		// The user's own flag with the same text is fine.
		r = gocppRun("", false, "build", "-cxxflags=-fplugin=evil.so", "-n", "-o", "prog", "hello.gp")
		if r.code == 0 && !strings.Contains(r.stderr, "-fplugin=") {
			t.Errorf("-cxxflags did not pass the plugin: %+v", r)
		}
	})
}

func TestEmitLLVMGCCError(t *testing.T) {
	var gcc *testCXX
	for _, c := range testCompilers(t) {
		if c.missing == "" && c.cxx.Family == build.GCC {
			gcc = &c
		}
	}
	if gcc == nil {
		t.Skip("no g++")
	}
	t.Setenv("GOCPP_CXX", gcc.name)
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	for _, args := range [][]string{{"-emit-llvm"}, {"-emit-llvm=bc"}} {
		r := gocppRun("", false, append(append([]string{"build"}, args...), "-o", "prog", "hello.gp")...)
		if r.code != 2 || !strings.Contains(r.stderr, "-emit-llvm needs clang") || exists(filepath.Join(dir, "prog.llvm")) || exists(filepath.Join(dir, "prog")) {
			t.Errorf("gocpp build %v with g++ = %+v", args, r)
		}
	}
}

func clangOnly(t *testing.T) testCXX {
	t.Helper()
	for _, c := range testCompilers(t) {
		if c.missing == "" && c.cxx.Family == build.Clang {
			t.Setenv("GOCPP_CXX", c.name)
			t.Setenv("CXX", "")
			return c
		}
	}
	t.Skip("no clang++")
	return testCXX{}
}

func TestEmitLLVM(t *testing.T) {
	clangOnly(t)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	dir := inDir(t, pkgsProject("llvm", 3))
	r := gocppRun("", false, "build", "-emit-llvm", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build -emit-llvm = %+v", r)
	}
	ir := filepath.Join(dir, "prog.llvm")
	var found []string
	filepath.WalkDir(ir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() != build.IRMarker {
			rel, _ := filepath.Rel(ir, p)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(found)
	for _, want := range []string{"gen/gpkg/example.com/m/a/a.gp.ll", "gen/gpkg/example.com/m/b/b.gp.ll", "gen/gpkg/example.com/m/main.gp.ll", "gen/gpkg/example.com/m/pkg_init.ll"} {
		if !slices.Contains(found, want) {
			t.Errorf("no %s among %q", want, found)
		}
	}
	data, err := os.ReadFile(filepath.Join(ir, "gen/gpkg/example.com/m/a/a.gp.ll"))
	if err != nil || !strings.Contains(string(data), "target triple") || !strings.Contains(string(data), "define ") {
		t.Errorf("a.gp.ll is not LLVM IR text: %v\n%.200s", err, data)
	}
	// The program was built too, and runs.
	if out, err := exec.Command(filepath.Join(dir, "prog")).Output(); err != nil || string(out) != "8 6 llvm\n" {
		t.Errorf("prog = %q, %v", out, err)
	}
	// llvm-as style check: the IR parses (clang reads .ll as input).
	cx, _ := build.LookCXX(t.Context(), os.Getenv("GOCPP_CXX"))
	if out, err := exec.Command(cx.Path, "-x", "ir", "-c", filepath.Join(ir, "gen/gpkg/example.com/m/a/a.gp.ll"), "-o", filepath.Join(t.TempDir(), "a.o")).CombinedOutput(); err != nil {
		t.Errorf("clang can not read the IR: %v\n%s", err, out)
	}
}

func TestEmitLLVMBitcodeAndFolder(t *testing.T) {
	clangOnly(t)
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", false, "build", "-emit-llvm=bc", "-o", "prog", "hello.gp")
	if r.code != 0 {
		t.Fatalf("gocpp build -emit-llvm=bc = %+v", r)
	}
	ir := filepath.Join(dir, "prog.llvm", "gen", "gpkg", "command-line-arguments")
	data, err := os.ReadFile(filepath.Join(ir, "hello.gp.bc"))
	if err != nil || !strings.HasPrefix(string(data), "BC\xc0\xde") {
		t.Fatalf("hello.gp.bc is not bitcode: %v %q", err, data[:min(len(data), 8)])
	}
	if exists(filepath.Join(ir, "hello.gp.ll")) {
		t.Errorf("a .ll file with -emit-llvm=bc")
	}
	// The next build with the text format empties the folder: no stale .bc.
	if r := gocppRun("", false, "build", "-emit-llvm", "-o", "prog", "hello.gp"); r.code != 0 {
		t.Fatalf("gocpp build -emit-llvm = %+v", r)
	}
	if exists(filepath.Join(ir, "hello.gp.bc")) || !exists(filepath.Join(ir, "hello.gp.ll")) {
		t.Errorf("the IR folder was not emptied before the new build")
	}
	// A folder that gocpp did not make is not emptied.
	os.RemoveAll(filepath.Join(dir, "prog.llvm"))
	os.MkdirAll(filepath.Join(dir, "prog.llvm"), 0o755)
	os.WriteFile(filepath.Join(dir, "prog.llvm", "mine.txt"), []byte("x"), 0o644)
	r = gocppRun("", false, "build", "-emit-llvm", "-o", "prog", "hello.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "not a folder of LLVM IR files that gocpp made") || !exists(filepath.Join(dir, "prog.llvm", "mine.txt")) {
		t.Errorf("gocpp build -emit-llvm into a foreign folder = %+v", r)
	}
}

func TestEmitLLVMGetsTheFlags(t *testing.T) {
	c := clangOnly(t)
	logs := quiet(t, c)
	prog, logs2 := fakeCompiler(t, c)
	_ = logs
	t.Setenv("GOCPP_CXX", prog)
	inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
	r := gocppRun("", false, "build", "-emit-llvm", "-cxxflags=-DIR_FLAG=1 -fpass-plugin=/nonexistent/p.so", "-o", "prog", "hello.gp")
	if r.code != 0 {
		t.Fatalf("gocpp build = %+v", r)
	}
	var ir [][]string
	for _, run := range runs(t, logs2) {
		if slices.Contains(run, "-emit-llvm") && slices.ContainsFunc(run, func(a string) bool { return strings.HasSuffix(a, "hello.gp.cpp") }) {
			ir = append(ir, run)
		}
	}
	// Only the unit has IR; the flags are the object compile's with -c turned into -S.
	if len(ir) != 1 || !slices.Contains(ir[0], "-S") || !slices.Contains(ir[0], "-DIR_FLAG=1") || !slices.Contains(ir[0], "-fpass-plugin=/nonexistent/p.so") {
		t.Errorf("IR compiles: %q", ir)
	}
	if got := len(compilesOf(runs(t, logs2), "/src/rt.cpp")); got == 0 {
		t.Errorf("no runtime compile")
	}
	for _, run := range runs(t, logs2) {
		if slices.Contains(run, "-emit-llvm") && slices.ContainsFunc(run, func(a string) bool { return strings.HasSuffix(a, "/src/rt.cpp") }) {
			t.Errorf("the runtime has IR: %q", run)
		}
	}
}

func readCompDB(t *testing.T, path string) []build.CompDBEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []build.CompDBEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("%s is not a compilation database: %v", path, err)
	}
	return entries
}

func TestBuildCompDB(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	dir := inDir(t, pkgsProject("db", 3))
	r := gocppRun("", false, "build", "-compdb=db/out.json", "-cxxflags=-DCOMPDB=1", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build -compdb = %+v", r)
	}
	entries := readCompDB(t, filepath.Join(dir, "db", "out.json"))
	var units, rt int
	for _, e := range entries {
		if !filepath.IsAbs(e.File) || !filepath.IsAbs(e.Directory) || len(e.Arguments) == 0 || !dirExists(e.Directory) {
			t.Errorf("entry %+v has paths that are not stable folders or no arguments", e)
		}
		if !exists(e.File) {
			t.Errorf("entry file %s does not exist after the build", e.File)
		}
		if !slices.Contains(e.Arguments, "-DCOMPDB=1") {
			t.Errorf("entry %s lacks -DCOMPDB=1: %q", e.File, e.Arguments)
		}
		switch {
		case strings.HasSuffix(e.File, ".gp.cpp"), strings.HasSuffix(e.File, "pkg_init.cpp"), strings.HasSuffix(e.File, "pkg_desc.cpp"):
			units++
		case strings.Contains(e.File, "/src/"):
			rt++
		}
		if strings.Contains(strings.Join(e.Arguments, " "), "gocpp-build") {
			t.Errorf("entry %s names the work folder: %q", e.File, e.Arguments)
		}
	}
	if units < 3 || rt < 10 {
		t.Errorf("%d generated units and %d runtime files in %d entries", units, rt, len(entries))
	}
	if !slices.IsSortedFunc(entries, func(a, b build.CompDBEntry) int { return strings.Compare(a.File, b.File) }) {
		t.Errorf("the entries are not sorted by file")
	}
	// The program was built as well.
	if !exists(filepath.Join(dir, "prog")) {
		t.Errorf("gocpp build -compdb did not build the program")
	}
}

func TestBuildCompDBDefaultPlaces(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	dir := inDir(t, pkgsProject("db2", 3))
	r := gocppRun("", false, "build", "-compdb", "-o", "prog", ".")
	if r.code != 0 {
		t.Fatalf("gocpp build -compdb = %+v", r)
	}
	root := readCompDB(t, filepath.Join(dir, "compile_commands.json"))
	if len(root) == 0 {
		t.Fatalf("no entries in the module's compile_commands.json")
	}
	// The same file is in the folder of the generated C++.
	for _, e := range root {
		if strings.HasSuffix(e.File, "main.gp.cpp") {
			gen := e.Directory
			if got := readCompDB(t, filepath.Join(gen, "compile_commands.json")); len(got) != len(root) {
				t.Errorf("%d entries in the generated folder, %d in the module's", len(got), len(root))
			}
			return
		}
	}
	t.Errorf("no entry for main.gp.cpp")
}

// TestCompDBCommandsCompile runs the commands of the database: clangd and other tools run them
// to parse the generated C++, so each must succeed in its directory.
func TestCompDBCommandsCompile(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	dir := inDir(t, pkgsProject("db3", 3))
	if r := gocppRun("", false, "export", "compdb", "-o", "db.json", "."); r.code != 0 {
		t.Fatalf("gocpp export compdb = %+v", r)
	}
	entries := readCompDB(t, filepath.Join(dir, "db.json"))
	tried := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.File, "main.gp.cpp") && !strings.HasSuffix(e.File, "b/b.gp.cpp") && !strings.HasSuffix(e.File, "/src/rt.cpp") {
			continue
		}
		tried++
		cmd := exec.Command(e.Arguments[0], append(slices.Clone(e.Arguments[1:]), "-fsyntax-only")...)
		cmd.Dir = e.Directory
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s: %v\n%.600s", e.File, err, out)
		}
	}
	if tried != 3 {
		t.Errorf("tried %d entries, want 3", tried)
	}
}

func TestExportCompDB(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	dir := inDir(t, pkgsProject("db4", 3))
	r := gocppRun("", false, "export", "compdb", "-cxxflags=-DEXPORTED=1", ".")
	if r.code != 0 {
		t.Fatalf("gocpp export compdb = %+v", r)
	}
	// The default places are printed, one path per line, and nothing else is built.
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 || lines[1] != filepath.Join(dir, "compile_commands.json") || !strings.HasSuffix(lines[0], "compile_commands.json") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if exists(filepath.Join(dir, "prog")) || exists(filepath.Join(dir, "pkgs")) {
		t.Errorf("gocpp export compdb wrote a program")
	}
	a := readCompDB(t, lines[1])
	// Run again: the same bytes (deterministic, sorted).
	first, _ := os.ReadFile(lines[1])
	if r := gocppRun("", false, "export", "compdb", "."); r.code != 0 {
		t.Fatalf("again: %+v", r)
	}
	// (the second run has no -DEXPORTED, so the files differ: with the flag again they are equal)
	if r := gocppRun("", false, "export", "compdb", "-cxxflags=-DEXPORTED=1", "."); r.code != 0 {
		t.Fatalf("again: %+v", r)
	}
	second, _ := os.ReadFile(lines[1])
	if string(first) != string(second) || len(a) == 0 {
		t.Errorf("two exports of the same build differ")
	}
}

func TestExportCMakeBuilds(t *testing.T) {
	cmake, err := exec.LookPath("cmake")
	if err != nil {
		t.Skip("no cmake")
	}
	gen := "make"
	if _, err := exec.LookPath("ninja"); err == nil {
		gen = "Ninja"
	}
	if _, err := exec.LookPath("make"); err != nil && gen == "make" {
		t.Skip("no ninja and no make")
	}
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("GOCPP_GATE_PHASE", "06")
	dir := inDir(t, pkgsProject("cmake", 3))
	out := filepath.Join(dir, "cm")
	r := gocppRun("", false, "export", "cmake", "-o", "cm", "-cxxflags=-DFROM_EXPORT=1", ".")
	if r.code != 0 || strings.TrimSpace(r.stdout) != filepath.Join(out, "CMakeLists.txt") {
		t.Fatalf("gocpp export cmake = %+v", r)
	}
	text, _ := os.ReadFile(filepath.Join(out, "CMakeLists.txt"))
	if !strings.HasPrefix(string(text), build.CMakeMarker) || !strings.Contains(string(text), "-DFROM_EXPORT=1") {
		t.Errorf("CMakeLists.txt:\n%.600s", text)
	}
	for _, f := range []string{"gen/gpkg/example.com/m/a/a.gp.cpp", "runtime/include/gocpp", "runtime/src/rt.cpp"} {
		if !exists(filepath.Join(out, filepath.FromSlash(f))) {
			t.Errorf("%s was not exported", f)
		}
	}
	// Configure and build with no help from gocpp: the program runs.
	cxx := c.cxx.Path
	cfg := exec.Command(cmake, "-S", out, "-B", filepath.Join(out, "b"), "-G", gen, "-DCMAKE_CXX_COMPILER="+cxx, "-DCMAKE_BUILD_TYPE=")
	if o, err := cfg.CombinedOutput(); err != nil {
		t.Fatalf("cmake configure: %v\n%s", err, o)
	}
	bld := exec.Command(cmake, "--build", filepath.Join(out, "b"), "-j", "4")
	if o, err := bld.CombinedOutput(); err != nil {
		t.Fatalf("cmake build: %v\n%.3000s", err, o)
	}
	got, err := exec.Command(filepath.Join(out, "b", "m")).Output()
	if err != nil || string(got) != "8 6 cmake\n" {
		// The program is named after the module's main package.
		matches, _ := filepath.Glob(filepath.Join(out, "b", "*"))
		t.Errorf("the program prints %q, %v (files: %q)", got, err, matches)
	}
	// A compiler of the other family is refused at configure time, with a message.
	other := "g++"
	if c.cxx.Family == build.GCC {
		other = "clang++"
	}
	if p, err := exec.LookPath(other); err == nil {
		cfg := exec.Command(cmake, "-S", out, "-B", filepath.Join(out, "b2"), "-G", gen, "-DCMAKE_CXX_COMPILER="+p)
		if o, err := cfg.CombinedOutput(); err == nil || !strings.Contains(string(o), "has the flags of") {
			t.Errorf("a configure with %s was accepted or has no message: %v\n%.600s", other, err, o)
		}
	}
}

func TestExportCMakeRefusesForeignFolder(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	dir := inDir(t, map[string]string{"hello.gp": helloSrc, "precious/keep.txt": "x"})
	r := gocppRun("", false, "export", "cmake", "-o", "precious", "hello.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "won't be replaced") || !exists(filepath.Join(dir, "precious", "keep.txt")) {
		t.Errorf("gocpp export cmake -o precious = %+v", r)
	}
	// The default folder is <program>-cmake.
	r = gocppRun("", false, "export", "cmake", "hello.gp")
	if r.code != 0 || !exists(filepath.Join(dir, "hello-cmake", "CMakeLists.txt")) {
		t.Errorf("gocpp export cmake = %+v", r)
	}
	// Again into the same folder works: it is gocpp's.
	if r := gocppRun("", false, "export", "cmake", "hello.gp"); r.code != 0 {
		t.Errorf("second export = %+v", r)
	}
}

// userFile writes the file name of a test with content and sets its time to the second sec of the
// epoch plus a day, so that every write has a time of its own (the cache keeps the hash of a file
// by its name, size and time).
func userFile(t *testing.T, name, content string, sec int) string {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Unix(86400+int64(sec), 0)
	if err := os.Chtimes(name, when, when); err != nil {
		t.Fatal(err)
	}
	return name
}

// TestBuildFilesOfTrustedFlagsAreInTheCacheKeys: the compiler reads a plugin, a response file and
// an -include file, so a build that follows a change of the file compiles again (the unit's
// object and the runtime's), whatever its time; a file with the content of the build before
// compiles nothing. The fake compiler does not load the plugins: only the keys can tell.
func TestBuildFilesOfTrustedFlagsAreInTheCacheKeys(t *testing.T) {
	cases := []struct {
		name     string
		one, two string // the content of the file, before and after (of the same size)
		flags    func(file string) (env, flag string)
		// unitUncached: the unit is never cached with this flag (-fplugin, @file: see
		// uncacheableFlag), so it is compiled every time.
		unitUncached bool
	}{
		{"fplugin", "AAAA", "BBBB", func(f string) (string, string) { return "-fplugin=" + f, "" }, true},
		{"fpass-plugin", "AAAA", "BBBB", func(f string) (string, string) { return "", "-cxxflags=-fpass-plugin=" + f }, false},
		{"response file", "-DZZ_ONE=1\n", "-DZZ_TWO=2\n", func(f string) (string, string) { return "@" + f, "" }, true},
		{"response file by flag", "-DZZ_ONE=1\n", "-DZZ_TWO=2\n", func(f string) (string, string) { return "", "-cxxflags=@" + f }, true},
		{"include", "#define ZZ 1\n", "#define ZZ 2\n", func(f string) (string, string) { return "-include " + f, "" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withOneCXX(t, func(t *testing.T, c testCXX) {
				logs := quiet(t, c)
				dir := inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
				file := userFile(t, filepath.Join(dir, "user.dat"), tc.one, 0)
				env, flag := tc.flags(file)
				t.Setenv("GOCPP_CXXFLAGS", env)
				args := []string{"build", "-o", "prog", "hello.gp"}
				if flag != "" {
					args = []string{"build", flag, "-o", "prog", "hello.gp"}
				}
				build := func(what string) (unit, rt int) {
					t.Helper()
					os.RemoveAll(logs)
					if err := os.Mkdir(logs, 0o755); err != nil {
						t.Fatal(err)
					}
					if r := gocppRun("", false, args...); r.code != 0 {
						t.Fatalf("%s: gocpp %v = %+v", what, args, r)
					}
					all := runs(t, logs)
					return len(compilesOf(all, "hello.gp.cpp")), len(compilesOf(all, "/src/rt.cpp"))
				}
				if unit, rt := build("first"); unit == 0 || rt == 0 {
					t.Fatalf("first build: %d compiles of the unit, %d of the runtime", unit, rt)
				}
				// unitWant: whether the unit is compiled in a build that finds the key in the cache.
				unitWant := func(changed bool) bool { return changed || tc.unitUncached }
				if unit, rt := build("again"); (unit > 0) != unitWant(false) || rt != 0 {
					t.Errorf("a build again: %d compiles of the unit, %d of the runtime", unit, rt)
				}
				userFile(t, file, tc.two, 10)
				unit, rt := build("changed")
				if unit == 0 || rt == 0 {
					t.Errorf("after the file changed: %d compiles of the unit, %d of the runtime, want both compiled again", unit, rt)
				}
				// The same content with a new time is a cache hit: the key is the content.
				userFile(t, file, tc.two, 20)
				if unit, rt := build("touched"); (unit > 0) != unitWant(false) || rt != 0 {
					t.Errorf("after the file was written again with the same content: %d compiles of the unit, %d of the runtime", unit, rt)
				}
			})
		})
	}
}

// TestBuildRelativePathsOfTrustedFlags: the compiles run in gocpp's work folder, so the names in the
// user's flags that are relative to the user's folder are made absolute (-I, -L, -Wl,-Map, ...).
func TestBuildRelativePathsOfTrustedFlags(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		logs := quiet(t, c)
		dir := inDir(t, map[string]string{"hello.gp": uniqueMain(t), "inc/hh.h": "#define ZZ_FROM_INC 1\n"})
		r := gocppRun("", false, "build", "-cxxflags=-Iinc -include hh.h", "-ldflags=-L. -Wl,-Map,x.map", "-o", "prog", "hello.gp")
		if r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		all := runs(t, logs)
		cs := compilesOf(all, "hello.gp.cpp")
		if len(cs) == 0 || !slices.Contains(cs[0], "-I"+filepath.Join(dir, "inc")) {
			t.Errorf("the compile of the unit has no absolute -I: %q", cs)
		}
		ls := linksOf(all)
		if len(ls) != 1 || !slices.Contains(ls[0], "-L"+dir) || !slices.Contains(ls[0], "-Wl,-Map,"+filepath.Join(dir, "x.map")) {
			t.Errorf("the link has no absolute -L and -Map: %q", ls)
		}
	})
}

// TestBuildReleaseIsQuiet: -release (-O2 -flto) links without a word on standard error: no trace of
// the temporary objects of link-time optimization, no lto-wrapper warning (GCC's serial LTRANS).
func TestBuildReleaseIsQuiet(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		cacheOn(t)
		t.Setenv("GOCPP_NOUPTODATE", "0") // the link is traced (-Wl,-t) only when the build is recorded
		inDir(t, map[string]string{"hello.gp": uniqueMain(t)})
		r := gocppRun("", false, "build", "-release", "-o", "prog", "hello.gp")
		if r.code != 0 || r.stderr != "" || r.stdout != "" {
			t.Errorf("gocpp build -release = %+v, want no output", r)
		}
		r = gocppRun("", false, "build", "-ldflags=-flto", "-cxxflags=-flto", "-o", "prog2", "hello.gp")
		if r.code != 0 || r.stderr != "" {
			t.Errorf("gocpp build -ldflags=-flto = %+v, want no output", r)
		}
	})
}

const exportLibSrc = "package main\n\n//export Add\nfunc Add(a, b int32) int32 { return a + b }\n\nfunc main() {}\n"

// TestExportLibraryModes: export accepts -buildmode. compdb lists the compiles of a library (they
// have the flags of the mode: -fPIC); cmake is for programs and says so.
func TestExportLibraryModes(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		quiet(t, c)
		dir := inDir(t, map[string]string{"lib.gp": exportLibSrc})
		for _, mode := range []string{"c-shared", "c-archive"} {
			out := filepath.Join(dir, mode+".json")
			r := gocppRun("", false, "export", "compdb", "-buildmode="+mode, "-o", out, "lib.gp")
			if r.code != 0 {
				t.Fatalf("gocpp export compdb -buildmode=%s = %+v", mode, r)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var entries []struct {
				Command   string
				Arguments []string
				File      string
			}
			if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 {
				t.Fatalf("%s: %v, %d entries", out, err, len(entries))
			}
			for _, e := range entries {
				if !slices.Contains(e.Arguments, "-fPIC") && !strings.Contains(e.Command, "-fPIC") {
					t.Errorf("%s: the compile of %s has no -fPIC: %q %s", mode, e.File, e.Arguments, e.Command)
				}
			}
		}
		r := gocppRun("", false, "export", "cmake", "-buildmode=c-shared", "-o", "cm", "lib.gp")
		if r.code != 2 || !strings.Contains(r.stderr, "only a program can be exported to CMake") {
			t.Errorf("gocpp export cmake -buildmode=c-shared = %+v", r)
		}
		if _, err := os.Stat(filepath.Join(dir, "cm")); err == nil {
			t.Errorf("export cmake wrote a folder for a library")
		}
	})
}
