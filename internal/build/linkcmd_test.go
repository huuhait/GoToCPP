package build

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Tests of the link command that Link runs (linkplan.go), with a fake compiler that records
// its command lines: what comes in which order, and that it is the same every time.

type linkRun struct {
	e      *rtEnv
	work   string
	link   []string // the arguments of the link command
	logged []string // all the commands of the compiler
}

// linkProgram runs Link for pkgs with the fake compiler and returns the link command.
func linkProgram(t *testing.T, e *rtEnv, f func(b *Builder), pkgs func(dir string) []PackageBuild) linkRun {
	t.Helper()
	t.Setenv("GOCPP_NOOBJCACHE", "1")
	dir := t.TempDir()
	b := e.builder(f)
	b.Cache = e.cache
	ps := pkgs(dir)
	for _, p := range ps {
		for _, u := range p.Units {
			if err := os.WriteFile(u, []byte("// unit\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := b.Link(context.Background(), ps, filepath.Join(dir, "out")); err != nil {
		t.Fatal(err)
	}
	r := linkRun{e: e, work: b.Work, logged: lines(t, e.cxx.Path)}
	for _, l := range r.logged {
		if !strings.Contains(l, " -c ") && strings.Contains(l, "-o "+filepath.Join(dir, "out")) {
			r.link = strings.Fields(l)
		}
	}
	if r.link == nil {
		t.Fatalf("no link command in\n%s", strings.Join(r.logged, "\n"))
	}
	return r
}

func twoPackages(dir string) []PackageBuild {
	return []PackageBuild{
		{Path: "dep", Dir: dir, Units: []string{filepath.Join(dir, "dep.cpp")}, Flags: PackageFlags{Link: []string{"-lz", "-lm"}}},
		{Path: "main", Dir: dir, Units: []string{filepath.Join(dir, "main.cpp")}, Flags: PackageFlags{Link: []string{"-lpng", "-lm", "-L/opt/png"}}},
	}
}

func TestLinkCommandOrder(t *testing.T) {
	e := newRTEnv(t)
	r := linkProgram(t, e, nil, twoPackages)
	idx := func(sub string) int {
		for i, w := range r.link {
			if strings.Contains(w, sub) {
				return i
			}
		}
		t.Fatalf("%q is not in the link command %q", sub, r.link)
		return -1
	}
	dep, main, rt := idx("00-dep.o"), idx("01-main.o"), idx(rtLibName)
	png, z, m, out := idx("-lpng"), idx("-lz"), slices.Index(r.link, "-lm"), slices.Index(r.link, "-o")
	if !(dep < main && main < rt && rt < png && png < z) {
		t.Errorf("want dep.o < main.o < libgocpprt.a < -lpng < -lz: %q", r.link)
	}
	if n := slices.Index(r.link, "-lm"); slices.Contains(r.link[n+1:], "-lm") {
		t.Errorf("-lm is not once: %q", r.link)
	}
	if m < z {
		t.Errorf("-lm (shared by both) must come after the last one that needs it: %q", r.link)
	}
	if out != len(r.link)-2 || filepath.Base(r.link[out+1]) != "out" {
		t.Errorf("-o is not last: %q", r.link)
	}
	// --gc-sections before the objects.
	if gc := idx("--gc-sections"); gc > dep {
		t.Errorf("--gc-sections after the objects: %q", r.link)
	}
	// One runtime input, and no object file of the runtime.
	for _, w := range r.link {
		if strings.HasSuffix(w, ".o") && strings.Contains(w, "src_") {
			t.Errorf("an object file of the runtime is on the link line: %s", w)
		}
	}
}

func TestLinkCommandReproducible(t *testing.T) {
	// The same program with other work folders and many times over: the same command, with the
	// work folder written as $WORK (what -x prints), whatever the order in which the units
	// compiled.
	e := newRTEnv(t)
	var first string
	for i := range 6 {
		r := linkProgram(t, e, nil, func(dir string) []PackageBuild {
			ps := twoPackages(dir)
			ps[0].Units = append(ps[0].Units, filepath.Join(dir, "dep2.cpp"), filepath.Join(dir, "dep3.cpp"))
			return ps
		})
		line := strings.ReplaceAll(strings.Join(r.link, " "), r.work, "$WORK")
		// the output file is in another temp folder: its folder is named by the test, normalize
		line = line[:strings.LastIndex(line, "-o ")] + "-o OUT"
		if i == 0 {
			first = line
		} else if line != first {
			t.Fatalf("run %d:\n%s\nrun 0:\n%s", i, line, first)
		}
	}
	if o := strings.Fields(first); !(slices.Index(o, "$WORK/00-dep.o") < slices.Index(o, "$WORK/00-dep2.o") && slices.Index(o, "$WORK/00-dep2.o") < slices.Index(o, "$WORK/00-dep3.o") && slices.Index(o, "$WORK/00-dep3.o") < slices.Index(o, "$WORK/01-main.o")) {
		t.Errorf("objects not in the order of the packages and units: %s", first)
	}
}

func TestLinkCommandResponseFile(t *testing.T) {
	e := newRTEnv(t)
	old := responseThreshold
	responseThreshold = 50
	t.Cleanup(func() { responseThreshold = old })
	r := linkProgram(t, e, nil, twoPackages)
	var rsp string
	for _, w := range r.link {
		if strings.HasPrefix(w, "@") {
			rsp = w[1:]
		}
	}
	if rsp == "" || filepath.Base(rsp) != "link.rsp" {
		t.Fatalf("no response file in %q", r.link)
	}
	// The file is where the build put its work: it exists while the link runs (here, the
	// work folder of the build is a temporary folder of the test, still there).
	data := readText(t, rsp)
	for _, want := range []string{"00-dep.o", "01-main.o", rtLibName, "-lpng", "-lz", "-lm"} {
		if !strings.Contains(data, want) {
			t.Errorf("the response file lacks %s:\n%s", want, data)
		}
	}
	// The driver flags and -o stay on the command line.
	if !slices.Contains(r.link, "-Wl,--gc-sections") || !slices.Contains(r.link, "-o") {
		t.Errorf("the command line: %q", r.link)
	}
	if slices.ContainsFunc(r.link, func(w string) bool { return strings.HasSuffix(w, ".o") }) {
		t.Errorf("an object file stayed on the command line: %q", r.link)
	}
}

func TestLinkCommandLTO(t *testing.T) {
	e := newRTEnv(t)
	r := linkProgram(t, e, func(b *Builder) { b.LTO = true }, twoPackages)
	if !slices.Contains(r.link, "-flto=auto") {
		t.Errorf("no -flto=auto in the link: %q", r.link)
	}
	// The compiles of the units and of the runtime have it, too.
	n := 0
	for _, l := range r.logged {
		if strings.Contains(l, " -c ") {
			n++
			if !strings.Contains(l, "-flto=auto") {
				t.Errorf("a compile without -flto=auto: %s", l)
			}
		}
	}
	if n < nSources()+2 {
		t.Errorf("only %d compiles", n)
	}
	// And the runtime has a library of its own: another one than without LTO.
	plain := linkProgram(t, e, nil, twoPackages)
	if got := e.libFolders(); len(got) != 2 {
		t.Errorf("lib/ holds %q: want one library with LTO and one without", got)
	}
	if slices.Contains(plain.link, "-flto=auto") {
		t.Errorf("-flto=auto without -lto: %q", plain.link)
	}
	libOf := func(l []string) string {
		for _, w := range l {
			if strings.HasSuffix(w, rtLibName) {
				return w
			}
		}
		return ""
	}
	if libOf(r.link) == libOf(plain.link) {
		t.Error("LTO and the plain build link the same library")
	}
}

func TestLinkCommandSanitizerRuntime(t *testing.T) {
	e := newRTEnv(t)
	plain := linkProgram(t, e, nil, twoPackages)
	asan := linkProgram(t, e, func(b *Builder) { b.ASan = true }, twoPackages)
	nochecks := linkProgram(t, e, func(b *Builder) { b.NoChecks = true }, twoPackages)
	libOf := func(l []string) string {
		for _, w := range l {
			if strings.HasSuffix(w, rtLibName) {
				return w
			}
		}
		return ""
	}
	libs := map[string]bool{libOf(plain.link): true, libOf(asan.link): true, libOf(nochecks.link): true}
	if len(libs) != 3 || libs[""] {
		t.Errorf("the plain, the sanitized and the -nochecks build share a library: %v", libs)
	}
	if !slices.Contains(asan.link, "-fsanitize=address,undefined") {
		t.Errorf("no sanitizer flag at the link: %q", asan.link)
	}
}

func TestLinkCommandFlagsDedupAcrossPackages(t *testing.T) {
	e := newRTEnv(t)
	r := linkProgram(t, e, func(b *Builder) { b.Env = EnvFlags{LDFlags: []string{"-lm"}} }, func(dir string) []PackageBuild {
		ps := twoPackages(dir)
		ps[0].Flags.Link = append(ps[0].Flags.Link, "-pthread", "-L/opt/png")
		return ps
	})
	count := func(w string) (n int) {
		for _, x := range r.link {
			if x == w {
				n++
			}
		}
		return n
	}
	for _, w := range []string{"-pthread", "-L/opt/png", "-lz", "-lpng"} {
		if count(w) != 1 {
			t.Errorf("%s appears %d times: %q", w, count(w), r.link)
		}
	}
	// GOCPP_LDFLAGS are the user's: kept as given, after everything else (so -lm twice is fine).
	if got := count("-lm"); got != 2 {
		t.Errorf("-lm appears %d times, want 2 (one of the packages, one of GOCPP_LDFLAGS): %q", got, r.link)
	}
}

// The link line of a Windows target keeps the flags of the target (crosstarget.go): a static
// program with the 8 MiB stack, whatever else the link plan does.
func TestLinkPartsKeepTargetFlags(t *testing.T) {
	for _, c := range []struct {
		name string
		cxx  CXX
		lto  bool
		want []string
	}{
		{"mingw g++", CXX{Family: GCC, Path: "/bin/x86_64-w64-mingw32-g++", GOOS: "windows", GOARCH: "amd64"}, false, []string{"-static", "-Wl,--stack,8388608", "-pthread"}},
		{"mingw g++ with LTO", CXX{Family: GCC, Path: "/bin/x86_64-w64-mingw32-g++", GOOS: "windows", GOARCH: "amd64"}, true, []string{"-static", "-Wl,--stack,8388608", "-pthread", "-flto=auto"}},
		{"llvm-mingw clang++", CXX{Family: Clang, Path: "/bin/x86_64-w64-mingw32-clang++", GOOS: "windows", GOARCH: "amd64"}, false, []string{"-static", "-Wl,--stack,8388608"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := &Builder{CXX: c.cxx, GOOS: "windows", Work: t.TempDir(), LTO: c.lto}
			args, err := b.linkArgs([]string{"a.o"}, nil, []string{"libgocpprt.a"}, []string{"-lm"}, "out.exe")
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !slices.Contains(args, w) {
					t.Errorf("the link line has no %q: %q", w, args)
				}
			}
			if i := slices.Index(args, "a.o"); i < 0 || slices.Index(args, "-static") > i || slices.Index(args, "-Wl,--stack,8388608") > i {
				t.Errorf("the target flags must come before the objects: %q", args)
			}
		})
	}
	// A Linux link has none of them.
	b := &Builder{CXX: CXX{Family: GCC, Path: "/bin/g++", GOOS: "linux", GOARCH: "amd64"}, GOOS: "linux", Work: t.TempDir()}
	args, _ := b.linkArgs([]string{"a.o"}, nil, nil, nil, "out")
	if slices.Contains(args, "-static") {
		t.Errorf("-static on a Linux link: %q", args)
	}
}

// -lto and -flto in GOCPP_LDFLAGS give one -flto=auto on a g++ link line; the plan keeps the whole
// command, not a response file that the build deletes.
func TestLinkLTOFlagOnceAndPlanArgsInFull(t *testing.T) {
	old := responseThreshold
	responseThreshold = 50
	defer func() { responseThreshold = old }()
	b := &Builder{CXX: CXX{Family: GCC, Path: "/bin/g++"}, GOOS: "linux", Work: t.TempDir(), LTO: true, Env: EnvFlags{LDFlags: []string{"-flto", "-Wl,-O1"}}}
	p, err := b.linkParts([]string{"a.o", "b.o", "c.o"}, nil, []string{"libgocpprt.a"}, []string{"-lm"})
	if err != nil {
		t.Fatal(err)
	}
	full := slices.Concat(p.pre, p.objs, p.post)
	n := 0
	for _, f := range full {
		if strings.HasPrefix(f, "-flto") {
			n++
		}
	}
	if n != 1 || !slices.Contains(full, "-Wl,-O1") {
		t.Errorf("want one -flto=auto and the other ldflags: %q", full)
	}
}

// A link long enough for a response file: the command that runs has @file, the plan (gocpp build -n,
// gocpp export) keeps the files and flags themselves, because the work folder is deleted.
func TestLinkPlanArgsInFull(t *testing.T) {
	old := responseThreshold
	responseThreshold = 100
	defer func() { responseThreshold = old }()
	e := newRTEnv(t)
	plan := &Plan{}
	r := linkProgram(t, e, func(b *Builder) { b.Plan = plan }, twoPackages)
	if !slices.ContainsFunc(r.link, func(a string) bool { return strings.HasPrefix(a, "@") }) {
		t.Fatalf("the test needs a response file: %q", r.link)
	}
	if plan.Link == nil {
		t.Fatal("no link in the plan")
	}
	if slices.ContainsFunc(plan.Link.Args, func(a string) bool { return strings.HasPrefix(a, "@") }) {
		t.Errorf("the plan names a response file: %q", plan.Link.Args)
	}
	for _, want := range []string{"-lpng", "-lz", "-o"} {
		if !slices.Contains(plan.Link.Args, want) {
			t.Errorf("the plan's link has no %q: %q", want, plan.Link.Args)
		}
	}
}
