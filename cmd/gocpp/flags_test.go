package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// The tests of the build flags of step 6.12 (flags.go): their parsing, their errors, what they
// put in the key of the up-to-date check, and their help text. The builds that use them are in
// buildflags_test.go.

func TestParseTags(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
		err  string
	}{
		{"", "", ""},
		{"a", "a", ""},
		{"a,b,c", "a,b,c", ""},
		{" a , b ", "a,b", ""},
		{"a,,b", "a,b", ""},
		{"a,b,a", "a,b", ""}, // each tag once
		{"go1.21,linux_amd64", "go1.21,linux_amd64", ""},
		{"a b", "", "separate the tags with commas"},
		{"a,b c,d", "", "separate the tags with commas"},
		{"a!", "", "invalid build tag"},
		{"-a", "", "invalid build tag"},
		{"a/b", "", "invalid build tag"},
	} {
		got, err := parseTags(c.in)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("parseTags(%q) = %q, %v; want an error with %q", c.in, got, err, c.err)
			}
			continue
		}
		if err != nil || strings.Join(got, ",") != c.want {
			t.Errorf("parseTags(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestTagsFlagLastWins(t *testing.T) {
	var tags []string
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(tagsFlag{&tags}, "tags", "")
	if err := fs.Parse([]string{"-tags", "a,b", "-tags=c"}); err != nil {
		t.Fatal(err)
	}
	// Like go build, the last -tags is the list.
	if !slices.Equal(tags, []string{"c"}) {
		t.Errorf("tags = %q, want [c]", tags)
	}
}

func TestListFlagAdds(t *testing.T) {
	var words []string
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(listFlag{&words}, "cxxflags", "")
	if err := fs.Parse([]string{"-cxxflags=-DA=1 '-DB=x y'", "-cxxflags", "-O2"}); err != nil {
		t.Fatal(err)
	}
	// Quotes keep spaces; a flag given twice adds.
	if want := []string{"-DA=1", "-DB=x y", "-O2"}; !slices.Equal(words, want) {
		t.Errorf("words = %q, want %q", words, want)
	}
	if err := fs.Parse([]string{"-cxxflags='-DA=unclosed"}); err == nil {
		t.Errorf("a quote that is not closed was accepted")
	}
}

func TestOptFlag(t *testing.T) {
	check := func(v string) error {
		if v != "ll" && v != "bc" {
			return flag.ErrHelp
		}
		return nil
	}
	for _, c := range []struct {
		args []string
		want string
		fail bool
	}{
		{nil, "", false},
		{[]string{"-e"}, "true", false},
		{[]string{"-e=ll"}, "ll", false},
		{[]string{"-e=bc"}, "bc", false},
		{[]string{"-e=false"}, "", false},
		{[]string{"-e=true"}, "true", false},
		{[]string{"-e=obj"}, "", true},
		{[]string{"-e="}, "", true},
		{[]string{"-e", "-e=bc"}, "bc", false},
	} {
		var v string
		fs := flag.NewFlagSet("x", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.Var(optFlag{dst: &v, valid: check}, "e", "")
		err := fs.Parse(c.args)
		if (err != nil) != c.fail || (!c.fail && v != c.want) {
			t.Errorf("%q: value %q, error %v; want %q (fail: %v)", c.args, v, err, c.want, c.fail)
		}
	}
}

func TestBuildFlagUsageErrors(t *testing.T) {
	inDir(t, map[string]string{"hello.gp": helloSrc})
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"build", "-emit-llvm=obj", "hello.gp"}, 2, "must be ll (text) or bc (bitcode)"},
		{[]string{"build", "-tags", "a b", "hello.gp"}, 2, "invalid build tag"},
		{[]string{"build", "-debug", "-release", "hello.gp"}, 2, "-debug and -release can't be used together"},
		{[]string{"build", "-race", "-asan", "hello.gp"}, 2, "-race and -asan can't be used together"},
		{[]string{"build", "-race", "-release", "hello.gp"}, 2, "-race and -release can't be used together"},
		{[]string{"build", "-cxxflags='-DX=open", "hello.gp"}, 2, "invalid value"},
		{[]string{"build", "-nope", "hello.gp"}, 2, "flag provided but not defined: -nope"},
		{[]string{"run", "-n", "hello.gp"}, 2, "flag provided but not defined: -n"},
		{[]string{"run", "-emit-llvm", "hello.gp"}, 2, "flag provided but not defined: -emit-llvm"},
		{[]string{"export", "compdb", "-compdb", "hello.gp"}, 2, "flag provided but not defined: -compdb"},
		{[]string{"emit", "-cxxflags=-DX", "hello.gp"}, 2, "flag provided but not defined: -cxxflags"},
		{[]string{"check", "-ldflags=-lm", "hello.gp"}, 2, "flag provided but not defined: -ldflags"},
	} {
		r := gocppRun("", false, c.args...)
		if r.code != c.code || !strings.Contains(r.stderr, c.msg) || r.stdout != "" {
			t.Errorf("gocpp %v = %+v, want exit %d and %q", c.args, r, c.code, c.msg)
		}
	}
}

func TestBuildFlagsOnWindowsRace(t *testing.T) {
	inDir(t, map[string]string{"hello.gp": helloSrc})
	t.Setenv("GOOS", "windows")
	r := gocppRun("", false, "build", "-race", "hello.gp")
	if r.code != 2 || !strings.Contains(r.stderr, "-race is not supported for windows") {
		t.Errorf("gocpp build -race for windows = %+v", r)
	}
}

func TestEmitLLVMError(t *testing.T) {
	if err := emitLLVMError("C++", build.CXX{Path: "/usr/bin/clang++", Family: build.Clang}); err != nil {
		t.Errorf("emitLLVMError(clang) = %v", err)
	}
	err := emitLLVMError("C++", build.CXX{Path: "/usr/bin/g++", Family: build.GCC})
	if err == nil || !strings.Contains(err.Error(), "-emit-llvm needs clang") || !strings.Contains(err.Error(), "GOCPP_CXX=clang++") {
		t.Errorf("emitLLVMError(g++) = %v, want a message that says what to do", err)
	}
	// The C compiler: its own name and variable.
	err = emitLLVMError("C", build.CXX{Path: "/usr/bin/gcc", Family: build.GCC})
	if err == nil || !strings.Contains(err.Error(), "the C compiler is gcc (GCC") || !strings.Contains(err.Error(), "GOCPP_CC=clang") || strings.Contains(err.Error(), "is g++") {
		t.Errorf("emitLLVMError(C, gcc) = %v", err)
	}
}

func TestExtraFlagsKeyParts(t *testing.T) {
	base := extraFlags{}
	same := extraFlags{}
	if !slices.Equal(base.keyParts(), same.keyParts()) {
		t.Fatalf("equal flags, different keys")
	}
	for name, e := range map[string]extraFlags{
		"tags":     {tags: []string{"a"}},
		"cflags":   {cflags: []string{"-DA"}},
		"cxxflags": {cxxflags: []string{"-DA"}},
		"ldflags":  {ldflags: []string{"-lm"}},
		"release":  {mode: build.Profile{Release: true}},
		"race":     {mode: build.Profile{Race: true}},
		"debug":    {mode: build.Profile{Debug: true}},
	} {
		if slices.Equal(base.keyParts(), e.keyParts()) {
			t.Errorf("%s does not change the key", name)
		}
	}
	// A flag is a word of its own: moving a space does not give the same key.
	a := extraFlags{cxxflags: []string{"-DA", "-DB"}}
	b := extraFlags{cxxflags: []string{"-DA -DB"}}
	if slices.Equal(a.keyParts(), b.keyParts()) {
		t.Errorf("two flags and one flag with a space have the same key")
	}
	c := extraFlags{cxxflags: []string{"-DA"}, ldflags: []string{"-DB"}}
	d := extraFlags{cxxflags: []string{"-DA", "-DB"}}
	if slices.Equal(c.keyParts(), d.keyParts()) {
		t.Errorf("a compile flag and a link flag have the same key")
	}
}

func TestExtraFlagsUnstamped(t *testing.T) {
	for name, c := range map[string]struct {
		e    extraFlags
		want string
	}{
		"none":         {extraFlags{}, ""},
		"flags":        {extraFlags{cxxflags: []string{"-DA"}, tags: []string{"t"}, verbose: true}, ""},
		"rebuild":      {extraFlags{rebuild: true}, "-a"},
		"dry run":      {extraFlags{dryRun: true}, "-n"},
		"emit llvm":    {extraFlags{llvm: "ll"}, "-emit-llvm"},
		"compdb":       {extraFlags{compdb: "true"}, "-compdb"},
		"compdb value": {extraFlags{compdb: "x.json"}, "-compdb"},
	} {
		got := c.e.unstamped()
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: unstamped = %q, want it to name %q", name, got, c.want)
		}
	}
}

func TestExtraFlagsLayered(t *testing.T) {
	if (&extraFlags{}).layered() {
		t.Errorf("no flags are layered")
	}
	for _, e := range []extraFlags{{cflags: []string{"-D"}}, {cxxflags: []string{"-D"}}, {ldflags: []string{"-l"}}, {mode: build.Profile{Race: true}}, {mode: build.Profile{Release: true}}} {
		if !e.layered() {
			t.Errorf("%+v is not layered", e)
		}
	}
}

func TestUserEnvLayersAndPlugins(t *testing.T) {
	t.Setenv("GOCPP_CFLAGS", "-DENV_C")
	t.Setenv("GOCPP_CXXFLAGS", "-fpass-plugin=env.so")
	t.Setenv("GOCPP_LDFLAGS", "-lenv")
	dir := filepath.FromSlash("/proj")
	e := extraFlags{cflags: []string{"-DFLAG_C"}, cxxflags: []string{"-fplugin=plugins/p.so"}, ldflags: []string{"-lflag"}, mode: build.Profile{Race: true}}
	env, err := e.userEnv(false, "linux", dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"-fsanitize=thread", "-fno-omit-frame-pointer", "-DENV_C", "-DFLAG_C"}; !slices.Equal(env.CFlags, want) {
		t.Errorf("CFlags = %q, want %q", env.CFlags, want)
	}
	// A plugin with a folder is made absolute (the compiles run in the work folder); a bare name stays.
	if want := []string{"-fpass-plugin=env.so", "-fplugin=" + filepath.Join(dir, "plugins", "p.so")}; !slices.Equal(env.CXXFlags, want) {
		t.Errorf("CXXFlags = %q, want %q", env.CXXFlags, want)
	}
	if want := []string{"-fsanitize=thread", "-lenv", "-lflag"}; !slices.Equal(env.LDFlags, want) {
		t.Errorf("LDFlags = %q, want %q", env.LDFlags, want)
	}
	t.Setenv("GOCPP_CXXFLAGS", "'-DX=open")
	if _, err := e.userEnv(false, "linux", dir); err == nil || !strings.Contains(err.Error(), "GOCPP_CXXFLAGS") {
		t.Errorf("userEnv with a bad GOCPP_CXXFLAGS = %v", err)
	}
}

func TestCompdbTargets(t *testing.T) {
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": mainSrc})
	d := &driver{name: "build", flags: &buildFlags{}, check: &checkCommand{dir: dir, goos: runtime.GOOS, goarch: runtime.GOARCH}}
	set := &buildSet{main: &check.Package{Load: &load.Package{Dir: dir, Module: &load.Module{Dir: dir}}}}
	gen := filepath.Join(dir, "gen")
	genDB := filepath.Join(gen, "compile_commands.json")
	modDB := filepath.Join(dir, "compile_commands.json")
	expect := func(what string, want ...string) {
		t.Helper()
		if got := d.compdbTargets(set, gen); !slices.Equal(got, want) {
			t.Errorf("%s: targets %q, want %q", what, got, want)
		}
	}
	// A debug build (nothing given, or -debug, or -race) writes both places by itself.
	expect("debug build", genDB, modDB)
	d.flags.extra.mode.Debug = true
	expect("-debug", genDB, modDB)
	d.flags.extra.mode = build.Profile{Race: true}
	expect("-race", genDB, modDB)
	// A release build and -n write nothing by themselves.
	d.flags.extra.mode = build.Profile{Release: true}
	expect("-release")
	d.flags.extra.mode = build.Profile{}
	d.flags.extra.dryRun = true
	expect("-n")
	d.flags.extra.dryRun = false
	// -compdb=false turns it off in a debug build.
	d.flags.extra.compdb = "false"
	expect("-compdb=false")
	if d.wantsCompdb() || d.wantsPlan() {
		t.Errorf("-compdb=false: the build still wants the plan")
	}
	// -compdb and -compdb=path work in every mode.
	d.flags.extra.compdb = "true"
	d.flags.extra.mode = build.Profile{Release: true}
	expect("-release -compdb", genDB, modDB)
	d.flags.extra.dryRun = true
	expect("-n -compdb", genDB, modDB)
	d.flags.extra.dryRun = false
	d.flags.extra.mode = build.Profile{}
	d.flags.extra.compdb = "out.json"
	expect("-compdb=out.json", filepath.Join(dir, "out.json"))
	abs := filepath.Join(os.TempDir(), "x.json")
	d.flags.extra.compdb = abs
	expect("-compdb="+abs, abs)
	// gocpp export writes its own files.
	d.flags.extra.compdb = ""
	d.afterPlan = func(*buildSet, *build.Plan, string) error { return nil }
	expect("export")
	d.afterPlan = nil
	// The module's copy is gocpp build's, and of a build for the host.
	for _, name := range []string{"run", "test", "debug"} {
		d.name = name
		expect("gocpp "+name, genDB)
	}
	d.name = "build"
	d.check.goos = "windows"
	if runtime.GOOS == "windows" {
		d.check.goos = "linux"
	}
	expect("cross build", genDB)
	d.check.goos = runtime.GOOS
	// A program of single .gp files has no module: no copy next to it.
	noMod := &buildSet{main: &check.Package{Load: &load.Package{Dir: dir}}}
	if got := d.compdbTargets(noMod, gen); !slices.Equal(got, []string{genDB}) {
		t.Errorf("no module: %q", got)
	}
	// A folder of generated files that is removed with the work folder: nothing is written
	// (the commands name files in it), whoever builds. -work keeps it.
	d.work = filepath.Join(dir, "work")
	for _, name := range []string{"build", "run"} {
		d.name = name
		if got := d.compdbTargets(set, filepath.Join(d.work, "gen")); got != nil {
			t.Errorf("gocpp %s, gen in the work folder: %q", name, got)
		}
	}
	d.flags.work = true // -work keeps it
	if got := d.compdbTargets(set, filepath.Join(d.work, "gen")); !slices.Equal(got, []string{filepath.Join(d.work, "gen", "compile_commands.json")}) {
		t.Errorf("run -work: %q", got)
	}
}

// TestBuildUsageDocumentsEveryFlag: every flag of gocpp build and run is described in the help
// text, and every flag of gocpp export too.
func TestBuildUsageDocumentsEveryFlag(t *testing.T) {
	for _, c := range []struct {
		name, usage string
		forBuild    bool
	}{{"build", buildUsage, true}, {"run", buildUsage, false}} {
		var f buildFlags
		fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
		f.register(fs, c.forBuild)
		if !c.forBuild {
			f.extra.registerCompdb(fs) // run has it too
		}
		fs.VisitAll(func(fl *flag.Flag) {
			if !strings.Contains(c.usage, "\t-"+fl.Name) {
				t.Errorf("gocpp %s: the flag -%s is not in the help text of gocpp build (buildUsage)", c.name, fl.Name)
			}
		})
	}
}

func TestHelpTexts(t *testing.T) {
	for _, c := range []struct {
		args []string
		want []string
	}{
		{[]string{"build", "-h"}, []string{"-cxxflags", "-emit-llvm", "-tags", "GOCPP_CXXFLAGS", "-fpass-plugin"}},
		{[]string{"export", "-h"}, []string{"gocpp export cmake", "gocpp export compdb", "compile_commands.json"}},
		{[]string{"list", "-h"}, []string{"-json", "CImports", "-deps"}},
		{[]string{"env", "-h"}, []string{"-json", "GOCPPCACHE"}},
	} {
		r := gocppRun("", false, c.args...)
		if r.code != 0 {
			t.Errorf("gocpp %v = %+v", c.args, r)
		}
		for _, w := range c.want {
			if !strings.Contains(r.stderr, w) {
				t.Errorf("gocpp %v: the help text lacks %q", c.args, w)
			}
		}
	}
}

func TestExportUsageErrors(t *testing.T) {
	inDir(t, map[string]string{"hello.gp": helloSrc})
	if r := gocppRun("", false, "export"); r.code != 2 || !strings.Contains(r.stderr, "usage: gocpp export") {
		t.Errorf("gocpp export = %+v", r)
	}
	if r := gocppRun("", false, "export", "ninja"); r.code != 2 || !strings.Contains(r.stderr, `unknown kind "ninja"`) {
		t.Errorf("gocpp export ninja = %+v", r)
	}
	if r := gocppRun("", false, "export", "cmake", "-n", "hello.gp"); r.code != 2 {
		t.Errorf("gocpp export cmake -n = %+v: -n is for build", r)
	}
	if r := gocppRun("", false, "export", "cmake", "-emit-llvm", "hello.gp"); r.code != 2 {
		t.Errorf("gocpp export cmake -emit-llvm = %+v: it is for build", r)
	}
}
