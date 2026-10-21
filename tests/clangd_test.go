package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The tests of this file are the evidence for the exit criterion of Phase 06 "clangd opens
// generated code cleanly": they replace the manual check of an editor. What the user's editor does
// is run `gocpp build` (a debug build writes compile_commands.json, ADR-044; `gocpp export compdb`
// writes the same file without compiling) and open the generated C++ or the project's own C and
// C++ files in clangd. Here, for a few representative programs (single file, modules of several
// packages, generics, goroutines and channels, interfaces, macros and `!`, #Derive, the standard
// library written in gocpp, C and C++ sources in the module, a C header, sqlite3), gocpp writes the
// compilation database for each compiler of the matrix and `clangd --check=FILE` (batch mode, no
// editor) parses the generated .gp.cpp, pkg.hpp, pkg_desc.cpp and pkg_init.cpp files of the
// program (and the module's own C and C++ files).
//
// "Opens cleanly" means, for each file (clangdVerdict):
//
//   - clangd found the file's compile command in the compilation database that gocpp wrote (a
//     source file: "Compile command from CDB"; a header has no entry and takes the command of a
//     neighbor, as in an editor: "Compile command inferred from"), and the command names the
//     compiler of the run (so that a GOCPP_CXX that stops working cannot test one compiler twice);
//   - clangd read no configuration file (a developer's ~/.config/clangd/config.yaml can suppress
//     every diagnostic: the runs get an empty XDG_CONFIG_HOME);
//   - clangd printed no error and no warning: no line `E[...]` except the refusals of its
//     refactoring tweaks, which it tries at every token in this mode (`tweak: ExtractFunction ==>
//     FAIL ...`: "Cannot extract break/continue without a loop" is no diagnostic: clangd counts
//     those as errors, so its exit code and "N errors" are not usable as they are). In particular:
//     no "file not found", no "unknown type name", no "Unable to handle compilation";
//   - the summary line "All checks completed, N errors" is there and N is the number of those
//     refusals.
//
// Where clangd can be asked cheaply, go to definition works (TestClangdGoToDefinition: a call in
// main.gp.cpp finds the declaration of a function of another package; the symbols of the file
// include the function). The `#line` directives of the generated code point at .gp files that
// exist, at lines inside them (TestClangdLineDirectivesPointAtGP; it needs no clangd). clangd
// itself ignores `#line` for its diagnostics: it reports the line of the generated file
// (TestClangdChecksCatchErrors shows that), so a diagnostic points at the generated C++, not at
// the .gp line.
//
// Gate: without clangd 21 on PATH the tests skip with a visible reason; GOCPP_REQUIRE_CLANGD=1
// fails them instead (tests/README.md). clangd runs in a process group of its own under a time
// limit, and the group is killed when it ends, so no test leaves a child behind.

// clangdTimeout is the limit for one clangd run (a check takes about two seconds).
const clangdTimeout = 90 * time.Second

// clangdMajor is the version of clangd that the tests are written for (the dev shell has it).
const clangdMajor = 21

// clangdCase is a program whose generated C++ clangd must open cleanly.
type clangdCase struct {
	name string
	// src is a folder of the repository that gocpp builds as "." or a single .gp file, relative to
	// the repository root.
	src string
	// std are generated files of the standard library's own packages to check as well, relative to
	// the folder gpkg of the generated files.
	std []string
	// pkgConfig is a library that pkg-config must know, or "".
	pkgConfig string
	// why says what the program stands for.
	why string
}

var clangdCases = []clangdCase{
	{name: "hello", src: "examples/hello", why: "the smallest program"},
	{name: "multipkg", src: "tests/pkgs/basic_cross", why: "a module of three packages"},
	{name: "generics", src: "examples/generics", why: "generic functions and types"},
	{name: "workers", src: "examples/workers", why: "goroutines, channels, select"},
	{name: "interfaces", src: "examples/shapes", why: "interfaces and methods"},
	{name: "macros", src: "examples/macros", why: "macros and `!` across packages"},
	{name: "bang", src: "examples/must-errors", why: "`!` and must functions"},
	{name: "derive", src: "tests/pkgs/derive_cross", why: "#Derive across packages"},
	{
		name: "stdlib",
		src:  "examples/wc",
		std:  []string{"bufio/scan.gp.cpp", "strings/builder.gp.cpp"},
		why:  "packages of the standard library written in gocpp",
	},
	{
		name: "mixed",
		src:  "examples/mixed-sources",
		why:  "C and C++ files in the module, next to .gp",
	},
	{name: "cppstd", src: "examples/cpp-std", why: "C++ standard library types"},
	{name: "stdio", src: "tests/interop/stdio_calls.gp", why: "a C header (stdio.h)"},
	{
		name:      "sqlite",
		src:       "examples/sqlite",
		pkgConfig: "sqlite3",
		why:       "a C library found by pkg-config",
	},
}

// clangdVersionLine matches the first line of `clangd --version`.
var clangdVersionLine = regexp.MustCompile(`clangd version (\d+)\.`)

// clangdPath returns the clangd of PATH. When there is none, or it is not version 21, the test is
// skipped with the reason, or failed with GOCPP_REQUIRE_CLANGD=1.
func clangdPath(t *testing.T) string {
	t.Helper()
	require := false
	switch s := os.Getenv("GOCPP_REQUIRE_CLANGD"); s {
	case "", "0":
	case "1":
		require = true
	default:
		t.Fatalf("GOCPP_REQUIRE_CLANGD=%s: want 1 (clangd must run) or 0", s)
	}
	if runtime.GOOS == "windows" {
		t.Skip("the clangd tests run on Unix (the compilation database has this machine's paths)")
	}
	unusable := func(why string) {
		t.Helper()
		if require {
			t.Fatalf("%s; GOCPP_REQUIRE_CLANGD=1 needs clangd %d (unset it to skip)",
				why, clangdMajor)
		}
		t.Skip(why)
	}
	path, err := exec.LookPath("clangd")
	if err != nil {
		unusable(fmt.Sprintf("clangd is not on PATH (the dev shell, nix develop, has it): %v", err))
	}

	// The messages that the tests read are those of clangd 21.
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		unusable(fmt.Sprintf("%s --version: %v", path, err))
	}
	m := clangdVersionLine.FindSubmatch(out)
	if m == nil {
		unusable(fmt.Sprintf("%s --version printed %q: not a clangd version line", path, out))
	}
	if major, _ := strconv.Atoi(string(m[1])); major != clangdMajor {
		unusable(fmt.Sprintf("clangd %s is on PATH, the tests are written for clangd %d",
			m[1], clangdMajor))
	}
	return path
}

// isolatedClangdEnv returns the environment that clangd runs with: an empty folder for
// XDG_CONFIG_HOME, so that the user's ~/.config/clangd/config.yaml is not read. Such a file can
// change what clangd reports (`Diagnostics: {Suppress: "*"}` hides every error) and so make every
// check pass.
func isolatedClangdEnv(t *testing.T) []string {
	t.Helper()
	return []string{"XDG_CONFIG_HOME=" + t.TempDir()}
}

// A clangdLog is what one `clangd --check` run printed.
type clangdLog struct {
	file   string
	cmd    string
	output string
	// problems are the findings of clangdVerdict: empty when clangd opened the file cleanly.
	problems []string
	// noise is the number of lines of clangdNoise in the log.
	noise int
}

// completedLine is the summary line of clangd --check.
var completedLine = regexp.MustCompile(`All checks completed, (\d+) errors?`)

// tweakRefusal matches the line of a refactoring tweak that does not apply at a token.
var tweakRefusal = regexp.MustCompile(`^E\[[^\]]*\]\s+tweak: \w+ ==> FAIL`)

// compileCommandLine matches the line that says which command clangd uses for the file, and takes
// the compiler out of it. The command comes from the database for a source file, and is inferred
// from a neighbor's entry for a header.
var compileCommandLine = regexp.MustCompile(
	`Compile command (from CDB|inferred from \S+) is: \[[^\]]*\] (\S+)`)

// clangdNoise are error lines of clangd that are no diagnostic of the code, accepted in headers
// only. clangd 21 prints this one while it tries its refactoring tweaks at each token of a header
// that has a method with an inline body and a member function that is only declared, with no .cpp
// file for it (the DefineOutline tweak logs an empty URI). A plain C++ header does it too, so it
// is a bug of clangd, not of gocpp's code. clangd does not count it in "N errors". The test
// counts the lines and logs the number (clangdLog.noise); any other error line is a problem.
var clangdNoise = []string{"Failed to resolve URI : Scheme must be provided in URI:"}

// forbiddenInLog are phrases of clangd that say the generated code did not open, in any log line.
var forbiddenInLog = []string{"file not found", "unknown type name", "Unable to handle compilation"}

// A clangdWant says what the file that clangd checks is.
type clangdWant struct {
	// header is true for a header: it has no entry in the compilation database, clangd infers the
	// command, and the known noise is accepted. False: a source file with an entry of its own.
	header bool
	// compiler is the compiler that the command must name (compared by file name), or "" for no
	// check.
	compiler string
}

// clangdVerdict reads the log of `clangd --check=FILE` and returns the reasons why the file did not
// open cleanly (none when it did).
func clangdVerdict(log string, want clangdWant) []string {
	var problems []string
	tweakRefusals := 0
	for _, line := range strings.Split(log, "\n") {
		text := strings.TrimSpace(line)
		switch {
		case strings.Contains(line, "Loading config file"):
			problems = append(problems, "clangd read a configuration file: "+text)
		case tweakRefusal.MatchString(line):
			tweakRefusals++
		case hasForbiddenPhrase(line):
			problems = append(problems, "clangd says: "+text)
		case strings.HasPrefix(line, "E["):
			if !want.header || !isClangdNoise(line) {
				problems = append(problems, "error: "+text)
			}
		case strings.HasPrefix(line, "W["):
			problems = append(problems, "warning: "+text)
		}
	}

	m := completedLine.FindStringSubmatch(log)
	if m == nil {
		problems = append(problems,
			`no "All checks completed" line (clangd crashed, or stopped early)`)
	} else if n, _ := strconv.Atoi(m[1]); n != tweakRefusals {
		problems = append(problems, fmt.Sprintf(
			"clangd counted %d errors, %d of them refusals of tweaks", n, tweakRefusals))
	}

	command := compileCommandLine.FindStringSubmatch(log)
	switch {
	case command == nil:
		problems = append(problems, "clangd printed no line about the compile command of the file")
	case want.header && !strings.HasPrefix(command[1], "inferred"):
		problems = append(problems,
			"a header has no entry of its own: clangd should infer a command")
	case !want.header && command[1] != "from CDB":
		problems = append(problems,
			"clangd did not use a command of the compilation database for this file")
	}
	if command != nil && want.compiler != "" &&
		filepath.Base(command[2]) != filepath.Base(want.compiler) {
		problems = append(problems, fmt.Sprintf("the compile command names %s, the run is for %s",
			command[2], want.compiler))
	}
	return problems
}

// isClangdNoise reports whether an error line is one of clangdNoise.
func isClangdNoise(line string) bool {
	for _, noise := range clangdNoise {
		if strings.Contains(line, noise) {
			return true
		}
	}
	return false
}

// hasForbiddenPhrase reports whether the line names one of forbiddenInLog.
func hasForbiddenPhrase(line string) bool {
	for _, phrase := range forbiddenInLog {
		if strings.Contains(line, phrase) {
			return true
		}
	}
	return false
}

// checkWithClangd runs `clangd --check=file` and returns its log with the verdict. A run that does
// not finish in clangdTimeout, or that cannot start, is a failure of the test.
func checkWithClangd(t *testing.T, c *config, clangd, dir, file string, want clangdWant) clangdLog {
	t.Helper()
	limited := *c
	limited.timeout = clangdTimeout
	args := []string{"--check=" + file}
	r := limited.run(t, job{
		dir:      dir,
		env:      isolatedClangdEnv(t),
		name:     clangd,
		args:     args,
		show:     "clangd " + shellQuote(args[0]),
		combined: true,
	})
	if r.problem != "" {
		t.Fatalf("%s: %s\noutput:\n%s", r.cmd, r.problem, indent(string(r.stdout)))
	}
	out := string(r.stdout)
	noise := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "E[") && isClangdNoise(line) {
			noise++
		}
	}
	return clangdLog{
		file:     file,
		cmd:      r.cmd,
		output:   out,
		problems: clangdVerdict(out, want),
		noise:    noise,
	}
}

// A generated program is the C++ that gocpp made for one case and one compiler.
type generated struct {
	module string // the copy of the program that gocpp built (a folder of its own)
	gen    string // the folder of the generated C++, with its compile_commands.json
	arg    string // how gocpp got the program: "." or the .gp file
}

// generate copies the program of the case into a temporary folder (gocpp writes
// compile_commands.json next to the module) and has gocpp write the generated C++ and the
// compilation database, as the build of an editor user does.
func generate(t *testing.T, c *config, cs clangdCase, cxx string) generated {
	t.Helper()
	src := filepath.Join(repoRoot, filepath.FromSlash(cs.src))
	trackFiles(src)
	module := t.TempDir()
	arg := "."
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		if err := copyTree(src, module); err != nil {
			t.Fatal(err)
		}
	} else {
		arg = filepath.Base(src)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(module, arg), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Phase 15 opens the gates of the features of later phases: every program's C++ is wanted.
	r := c.run(t, gocppJob(module, nil, cxx, 15, "export", "compdb", arg))
	if r.problem != "" || r.exit != 0 {
		t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
	}
	paths := strings.Fields(string(r.stdout))
	if len(paths) != 2 {
		t.Fatalf("%s printed %q, want the two paths of compile_commands.json", r.cmd, paths)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s: %v", r.cmd, err)
		}
	}
	return generated{module: module, gen: filepath.Dir(paths[0]), arg: arg}
}

// A userFileSet is the generated files of the program's own packages that clangd must open.
type userFileSet struct {
	sources []string // the .gp.cpp files whose #line directives name files of the module
	units   []string // the pkg_desc.cpp and pkg_init.cpp of their folders
	headers []string // the pkg.hpp of their folders
}

// userFiles finds the generated files of the program's own packages (not those of the standard
// library, which also are in the folder of the generated files).
func (g generated) userFiles(t *testing.T) userFileSet {
	t.Helper()
	var set userFileSet
	marker := `"` + filepath.ToSlash(g.module)
	seen := map[string]bool{}
	root := filepath.Join(g.gen, "gpkg")
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(path, ".gp.cpp") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), marker) {
			return nil
		}
		set.sources = append(set.sources, path)
		dir := filepath.Dir(path)
		if seen[dir] {
			return nil
		}
		seen[dir] = true
		set.headers = append(set.headers, filepath.Join(dir, "pkg.hpp"))
		for _, name := range []string{"pkg_desc.cpp", "pkg_init.cpp"} {
			unit := filepath.Join(dir, name)
			if _, err := os.Stat(unit); err == nil {
				set.units = append(set.units, unit)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// moduleSources returns the C and C++ files of the module (its own, not the generated ones) that
// compile_commands.json lists.
func (g generated) moduleSources(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(g.module, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() && (e.Name() == "twin" || e.Name() == "testdata") {
			return fs.SkipDir
		}
		switch filepath.Ext(path) {
		case ".c", ".cc", ".cpp":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// subtestName names a checked file for the subtest: its path in the folder of the generated files,
// or else in the module.
func (g generated) subtestName(path string) string {
	for _, root := range []string{g.gen, g.module} {
		rel, err := filepath.Rel(root, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

// TestClangdOpensGeneratedCode is the exit criterion: for each program and each compiler, clangd
// opens the generated C++ (and the module's own C and C++ files) with no error.
func TestClangdOpensGeneratedCode(t *testing.T) {
	c := setup(t)
	clangd := clangdPath(t)
	for _, cs := range clangdCases {
		t.Run(cs.name, func(t *testing.T) {
			t.Parallel()
			skipWithoutLibrary(t, cs)
			for _, cc := range c.compilers {
				t.Run(cc.name, func(t *testing.T) {
					if cc.missing != "" {
						t.Skip(cc.missing)
					}
					checkCaseWithClangd(t, c, clangd, cs, cc)
				})
			}
		})
	}
}

// skipWithoutLibrary skips the test when the case needs a library that pkg-config does not know.
func skipWithoutLibrary(t *testing.T, cs clangdCase) {
	t.Helper()
	if cs.pkgConfig == "" {
		return
	}
	if err := exec.Command("pkg-config", "--exists", cs.pkgConfig).Run(); err != nil {
		t.Skipf("pkg-config does not know %s: %v", cs.pkgConfig, err)
	}
}

// checkCaseWithClangd generates the program of cs for cc and runs clangd over its files.
func checkCaseWithClangd(t *testing.T, c *config, clangd string, cs clangdCase, cc compiler) {
	g := generate(t, c, cs, cc.cxx)
	files := g.userFiles(t)
	if len(files.sources) == 0 {
		t.Fatalf("no generated .gp.cpp file of the program in %s", g.gen)
	}
	type target struct {
		path string
		want clangdWant
		what string
	}
	var targets []target
	source := clangdWant{compiler: cc.cxx}
	header := clangdWant{header: true, compiler: cc.cxx}
	for _, f := range files.sources {
		targets = append(targets, target{f, source, "generated source"})
	}
	for _, f := range files.units {
		targets = append(targets, target{f, source, "generated unit"})
	}
	for _, f := range files.headers {
		targets = append(targets, target{f, header, "generated header"})
	}
	for _, rel := range cs.std {
		path := filepath.Join(g.gen, "gpkg", filepath.FromSlash(rel))
		targets = append(targets, target{path, source, "generated std source"})
	}
	// The module's own C and C++ files: their commands come from the same database, with the
	// compiler of the build or its C counterpart, so the compiler is not compared.
	for _, f := range g.moduleSources(t) {
		targets = append(targets, target{f, clangdWant{}, "module's own source"})
	}

	for _, tg := range targets {
		t.Run(g.subtestName(tg.path), func(t *testing.T) {
			if _, err := os.Stat(tg.path); err != nil {
				t.Fatalf("%s: %v", tg.what, err)
			}
			l := checkWithClangd(t, c, clangd, g.module, tg.path, tg.want)
			if l.noise > 0 {
				t.Logf("%d lines of known clangd noise (clangdNoise)", l.noise)
			}
			if len(l.problems) > 0 {
				t.Errorf("clangd did not open the %s cleanly:\n%s\n%s\nclangd's log:\n%s",
					tg.what, strings.Join(l.problems, "\n"), l.cmd,
					indent(significantLog(l.output)))
			}
		})
	}
}

// significantLog returns the lines of a clangd log worth showing: the errors, the warnings and the
// summary (the rest is one line per token).
func significantLog(log string) string {
	var keep []string
	for _, line := range strings.Split(log, "\n") {
		switch {
		case strings.HasPrefix(line, "E["), strings.HasPrefix(line, "W["),
			strings.Contains(line, "Compile command"),
			strings.Contains(line, "Loading config file"),
			strings.Contains(line, "All checks completed"):
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// TestClangdVerdict tests the reading of a clangd log, without clangd: the logs are what clangd 21
// prints.
func TestClangdVerdict(t *testing.T) {
	const (
		fromCDB  = "I[00:00:00.001] Compile command from CDB is: [/x] /bin/g++ -c a.cpp\n"
		inferred = "I[00:00:00.001] Compile command inferred from /x/a.cpp is: " +
			"[/x] /bin/g++ -c a.cpp\n"
		done0 = "I[00:00:00.300] All checks completed, 0 errors\n"
		done1 = "I[00:00:00.300] All checks completed, 1 errors\n"
		done2 = "I[00:00:00.300] All checks completed, 2 errors\n"
		tweak = "E[00:00:00.200]     tweak: ExtractFunction ==> FAIL: " +
			"Cannot extract break/continue without corresponding loop/switch statement.\n"
		noise = "E[00:00:00.100] Failed to resolve URI : Scheme must be provided in URI: \n"
	)
	source := clangdWant{}
	header := clangdWant{header: true}
	tests := []struct {
		name string
		log  string
		want clangdWant
		// problem is a part of the first problem, or "" for a clean file.
		problem string
	}{
		{"clean", fromCDB + done0, source, ""},
		{"tweak refusals are not errors", fromCDB + tweak + tweak + done2, source, ""},
		{
			"undeclared name",
			fromCDB + "E[00:00:00.100] [undeclared_var_use] Line 2: " +
				"use of undeclared identifier 'x'\n" + done1,
			source, "use of undeclared identifier",
		},
		{
			"missing header",
			fromCDB + "E[00:00:00.100] [pp_file_not_found] Line 1: 'nothere.h' file not found\n" +
				done1,
			source, "file not found",
		},
		{
			"unknown type",
			fromCDB + "E[00:00:00.100] [unknown_typename] Line 4: " +
				"unknown type name 'Foo'\n" + done1,
			source, "unknown type name",
		},
		{
			"unable to handle compilation",
			fromCDB + "E[00:00:00.100] [drv_unable] Unable to handle compilation, " +
				"expected exactly one compiler job\n" + done1,
			source, "Unable to handle compilation",
		},
		{
			"an error that mentions a tweak is still an error",
			fromCDB + "E[00:00:00.100] [some_error] Line 3: tweak: nonsense\n" + done1,
			source, "tweak: nonsense",
		},
		{"known clangd noise in a header", inferred + noise + done0, header, ""},
		{
			"known clangd noise in a source file is a problem",
			fromCDB + noise + done0, source, "Failed to resolve URI",
		},
		{
			"another failure to resolve a URI in a header",
			inferred + "E[00:00:00.100] Failed to resolve URI : " +
				"Unknown scheme in URI: foo:x\n" + done0,
			header, "Failed to resolve URI",
		},
		{
			"a warning",
			fromCDB + "W[00:00:00.100] [unused_variable] Line 2: unused variable 'x'\n" + done0,
			source, "unused variable",
		},
		{
			"a configuration file was read",
			"I[00:00:00.001] Loading config file at /home/u/.config/clangd/config.yaml\n" +
				fromCDB + done0,
			source, "configuration file",
		},
		{"no summary", fromCDB, source, "All checks completed"},
		{"error count above the refusals", fromCDB + tweak + done2, source, "counted 2 errors"},
		{"no line about the compile command", done0, source, "no line about the compile command"},
		{
			"a source file with an inferred command",
			inferred + done0, source, "compilation database",
		},
		{"a header with a command from the database", fromCDB + done0, header, "infer a command"},
		{
			"the command names another compiler",
			fromCDB + done0, clangdWant{compiler: "clang++"},
			"names /bin/g++, the run is for clang++",
		},
		{
			"the command names the compiler by another path",
			fromCDB + done0, clangdWant{compiler: "/usr/bin/g++"}, "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := clangdVerdict(tt.log, tt.want)
			switch {
			case tt.problem == "" && len(problems) > 0:
				t.Errorf("problems = %q, want none", problems)
			case tt.problem != "" &&
				(len(problems) == 0 || !strings.Contains(problems[0], tt.problem)):
				t.Errorf("problems = %q, want one with %q first", problems, tt.problem)
			}
		})
	}
}

// TestClangdChecksCatchErrors is the control of the tests above: they would pass for everything if
// the check could not see errors. A file next to the generated C++ (it takes the compile command
// of its neighbor, as an editor does for a new file) with a mistake that a generator bug would
// make is found, and clangd reports the line of the generated file, not the .gp line of a #line
// directive.
func TestClangdChecksCatchErrors(t *testing.T) {
	c := setup(t)
	clangd := clangdPath(t)
	g, original, probe := generateWithProbe(t, c)
	tests := []struct {
		name  string
		extra string
		want  string
		// row is the line of the generated file that clangd must name, or 0.
		row int
	}{
		{
			"undeclared name",
			"int gocpp_probe = gocpp_undeclared_name;\n",
			"use of undeclared identifier",
			0,
		},
		{"missing header", "#include \"gpkg/gocpp_nothere/pkg.hpp\"\n", "file not found", 0},
		{"unknown type", "gocpp_NoSuchType gocpp_probe;\n", "unknown type name", 0},
		{
			"under a #line directive",
			"#line 3 \"" + filepath.ToSlash(filepath.Join(g.module, "main.gp")) + "\"\n" +
				"int gocpp_probe = gocpp_undeclared_name;\n",
			"use of undeclared identifier",
			len(strings.Split(string(original), "\n")) + 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := append(append([]byte{}, original...), tt.extra...)
			if err := os.WriteFile(probe, text, 0o644); err != nil {
				t.Fatal(err)
			}
			l := checkWithClangd(t, c, clangd, g.module, probe, clangdWant{})
			if len(l.problems) == 0 || !strings.Contains(strings.Join(l.problems, "\n"), tt.want) {
				t.Fatalf("problems = %q, want one with %q\nclangd's log:\n%s",
					l.problems, tt.want, indent(significantLog(l.output)))
			}
			if tt.row != 0 && !strings.Contains(l.output, fmt.Sprintf("Line %d:", tt.row)) {
				t.Errorf("clangd's log has no \"Line %d:\" (the line of the generated file)\n%s",
					tt.row, indent(significantLog(l.output)))
			}
		})
	}
}

// generateWithProbe generates the hello program and returns it with the text of its main source
// and the path of a file next to it that the test can write (removed at the end of the test).
func generateWithProbe(t *testing.T, c *config) (g generated, original []byte, probe string) {
	t.Helper()
	g = generate(t, c, clangdCase{name: "hello", src: "examples/hello"}, "")
	files := g.userFiles(t)
	if len(files.sources) == 0 {
		t.Fatalf("no generated source in %s", g.gen)
	}
	original, err := os.ReadFile(files.sources[0])
	if err != nil {
		t.Fatal(err)
	}
	probe = filepath.Join(filepath.Dir(files.sources[0]), "gocpp_probe.gp.cpp")
	t.Cleanup(func() { os.Remove(probe) })
	return g, original, probe
}

// TestClangdIgnoresDeveloperConfig shows that the user's own clangd configuration does not reach
// the checks. A config.yaml that suppresses every diagnostic makes clangd report no error for a
// file that has one; the runs of the tests get an empty XDG_CONFIG_HOME, so the error is found.
func TestClangdIgnoresDeveloperConfig(t *testing.T) {
	c := setup(t)
	clangd := clangdPath(t)
	g, original, probe := generateWithProbe(t, c)
	text := append(append([]byte{}, original...), "int gocpp_probe = gocpp_undeclared_name;\n"...)
	if err := os.WriteFile(probe, text, 0o644); err != nil {
		t.Fatal(err)
	}

	// The developer's configuration, as clangd finds it: $XDG_CONFIG_HOME/clangd/config.yaml.
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "clangd"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "Diagnostics:\n  Suppress: \"*\"\n"
	file := filepath.Join(home, "clangd", "config.yaml")
	if err := os.WriteFile(file, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", home)

	l := checkWithClangd(t, c, clangd, g.module, probe, clangdWant{})
	if !strings.Contains(strings.Join(l.problems, "\n"), "use of undeclared identifier") {
		t.Errorf("a configuration file of the user hid the error: problems = %q\n%s",
			l.problems, indent(significantLog(l.output)))
	}
}

// lineDirective matches `#line N "path"`.
var lineDirective = regexp.MustCompile(`(?m)^#line (\d+) "([^"]+)"$`)

// lineDirectiveProblems checks the #line directives of a generated file: each names a .gp file of
// the module (or of the embedded standard library, written $GOCPP/stdlib/...) that exists, with a
// line inside it, or the generated file itself (the directive that ends a run of .gp lines). It
// returns the problems, and how many directives name a .gp file.
func lineDirectiveProblems(file, text, module, gen string) (problems []string, gpDirectives int) {
	lines := func(s string) int { return strings.Count(s, "\n") + 1 }
	for _, m := range lineDirective.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(m[1])
		target := m[2]
		switch {
		case isGeneratedFile(target, file, gen):
			if n < 1 || n > lines(text) {
				problems = append(problems, fmt.Sprintf(
					"#line %d names the generated file, which has %d lines", n, lines(text)))
			}
			continue
		case strings.HasPrefix(target, "$GOCPP/"):
			rel := filepath.FromSlash(strings.TrimPrefix(target, "$GOCPP/"))
			target = filepath.Join(repoRoot, rel)
		}
		target = filepath.FromSlash(target)
		if !strings.HasSuffix(target, ".gp") {
			problems = append(problems,
				fmt.Sprintf("#line %d names %q, which is no .gp file", n, m[2]))
			continue
		}
		if !strings.HasPrefix(target, module) && !strings.HasPrefix(target, repoRoot) {
			problems = append(problems,
				fmt.Sprintf("#line %d names %q, which is outside the module", n, m[2]))
			continue
		}
		data, err := os.ReadFile(target)
		if err != nil {
			problems = append(problems, fmt.Sprintf("#line %d names %q: %v", n, m[2], err))
			continue
		}
		gpDirectives++
		if n < 1 || n > lines(string(data)) {
			problems = append(problems, fmt.Sprintf(
				"#line %d names %q, which has %d lines", n, m[2], lines(string(data))))
		}
	}
	return problems, gpDirectives
}

// isGeneratedFile reports whether the target of a #line directive is the generated file itself:
// its path, or, for the packages of the embedded standard library, $GOCPP/gen/ and its path in the
// folder of the generated files.
func isGeneratedFile(target, file, gen string) bool {
	if filepath.Clean(filepath.FromSlash(target)) == filepath.Clean(file) {
		return true
	}
	rel, err := filepath.Rel(gen, file)
	return err == nil && target == "$GOCPP/gen/"+filepath.ToSlash(rel)
}

// TestClangdLineDirectivesPointAtGP checks, for the same programs, that the #line directives of
// the generated sources point at .gp files and lines that exist: what a debugger shows, and what
// an editor that follows #line would open. It needs no clangd.
func TestClangdLineDirectivesPointAtGP(t *testing.T) {
	c := setup(t)
	for _, cs := range clangdCases {
		t.Run(cs.name, func(t *testing.T) {
			t.Parallel()
			skipWithoutLibrary(t, cs)
			g := generate(t, c, cs, "")
			sources := g.userFiles(t).sources
			for _, rel := range cs.std {
				sources = append(sources, filepath.Join(g.gen, "gpkg", filepath.FromSlash(rel)))
			}
			total := 0
			for _, file := range sources {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				problems, gp := lineDirectiveProblems(file, string(data), g.module, g.gen)
				total += gp
				for _, p := range problems {
					t.Errorf("%s: %s", file, p)
				}
			}
			if total == 0 {
				t.Errorf("the generated sources have no #line directive that names a .gp file")
			}
		})
	}
}

// TestClangdLineDirectiveProblems tests lineDirectiveProblems.
func TestClangdLineDirectiveProblems(t *testing.T) {
	module := t.TempDir()
	gp := filepath.Join(module, "a.gp")
	if err := os.WriteFile(gp, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generatedFile := filepath.Join(module, "gen", "a.gp.cpp")
	tests := []struct {
		name string
		text string
		want string // a part of the problem, or "" for none
	}{
		{"a line of the file", "#line 3 \"" + gp + "\"\nx\n", ""},
		{"the generated file itself", "#line 2 \"" + generatedFile + "\"\nx\n", ""},
		{"past the end of the .gp file", "#line 40 \"" + gp + "\"\nx\n", "which has 4 lines"},
		{"line zero", "#line 0 \"" + gp + "\"\nx\n", "which has 4 lines"},
		{
			"a file that is gone",
			"#line 3 \"" + filepath.Join(module, "gone.gp") + "\"\nx\n",
			"no such file",
		},
		{
			"not a .gp file",
			"#line 3 \"" + filepath.Join(module, "a.txt") + "\"\nx\n",
			"no .gp file",
		},
		{
			"the generated file of the embedded standard library",
			"#line 2 \"$GOCPP/gen/a.gp.cpp\"\nx\n",
			"",
		},
		{"the embedded standard library", "#line 52 \"$GOCPP/stdlib/bufio/bufio.gp\"\nx\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := filepath.Join(module, "gen")
			problems, _ := lineDirectiveProblems(generatedFile, tt.text, module, gen)
			switch {
			case tt.want == "" && len(problems) > 0:
				t.Errorf("problems = %q, want none", problems)
			case tt.want != "" && (len(problems) == 0 || !strings.Contains(problems[0], tt.want)):
				t.Errorf("problems = %q, want one with %q", problems, tt.want)
			}
		})
	}
}

// calleeInOtherPackage matches a call of a function of another package in generated C++:
// ::gpkg_example_dcom__shop_hgeom::New( .
var calleeInOtherPackage = regexp.MustCompile(`::(gpkg_[A-Za-z0-9_]+)::([A-Z][A-Za-z0-9_]*)\(`)

// TestClangdGoToDefinition starts clangd as an editor does (the language server, over a pipe) on
// the generated C++ of a module of several packages, and asks it for the symbols of main.gp.cpp
// and for the definition of a function of another package that main calls: the answer is a place
// in that package's generated files.
func TestClangdGoToDefinition(t *testing.T) {
	c := setup(t)
	clangd := clangdPath(t)
	cs := clangdCase{name: "multipkg", src: "tests/pkgs/basic_cross"}
	g := generate(t, c, cs, "")
	mainFile := filepath.Join(g.gen, "gpkg", "example.com", "shop", "main.gp.cpp")
	text, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	loc := calleeInOtherPackage.FindSubmatchIndex(text)
	if loc == nil {
		t.Fatalf("main.gp.cpp has no call of a function of another package:\n%s",
			indent(string(text)))
	}
	calleePackage := string(text[loc[2]:loc[3]])
	calleeName := string(text[loc[4]:loc[5]])
	before := text[:loc[4]]
	line := bytes.Count(before, []byte("\n"))
	column := len(before) - (bytes.LastIndexByte(before, '\n') + 1)

	s := startClangdServer(t, clangd, g.gen)
	uri := "file://" + mainFile
	s.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri": uri, "languageId": "cpp", "version": 1, "text": string(text),
		},
	})

	t.Run("symbols", func(t *testing.T) {
		var symbols []lspSymbol
		params := map[string]any{"textDocument": map[string]any{"uri": uri}}
		s.call(t, "textDocument/documentSymbol", params, &symbols)
		names := map[string]bool{}
		var collect func([]lspSymbol)
		collect = func(list []lspSymbol) {
			for _, sym := range list {
				names[sym.Name] = true
				collect(sym.Children)
			}
		}
		collect(symbols)
		for _, want := range []string{"gpkg_main", "main"} {
			if !names[want] {
				t.Errorf("the symbols of main.gp.cpp have no %q; they are %v", want, names)
			}
		}
	})

	t.Run("definition", func(t *testing.T) {
		var places []lspLocation
		s.call(t, "textDocument/definition", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": line, "character": column},
		}, &places)
		if len(places) == 0 {
			t.Fatalf("no definition of %s::%s at line %d, column %d of main.gp.cpp",
				calleePackage, calleeName, line+1, column+1)
		}
		file := strings.TrimPrefix(places[0].URI, "file://")
		if !strings.HasPrefix(file, filepath.Join(g.gen, "gpkg")) ||
			filepath.Base(file) == "main.gp.cpp" {
			t.Errorf("the definition of %s is in %s, want a generated file of its package",
				calleeName, file)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		row := places[0].Range.Start.Line
		if row >= len(lines) || !strings.Contains(lines[row], calleeName) {
			t.Errorf("line %d of %s does not name %s: %q",
				row+1, file, calleeName, lines[min(row, len(lines)-1)])
		}
	})
	s.shutdown(t)
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspLocation struct {
	URI   string `json:"uri"`
	Range struct {
		Start lspPosition `json:"start"`
	} `json:"range"`
}

type lspSymbol struct {
	Name     string      `json:"name"`
	Children []lspSymbol `json:"children"`
}

// A clangdServer is a clangd language server that the test talks to over its standard input and
// output.
type clangdServer struct {
	cmd      *exec.Cmd
	in       io.WriteCloser
	messages chan lspMessage
	nextID   int
}

type lspMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
	err error
}

// startClangdServer starts clangd on the folder root (where its compile_commands.json is) and
// does the initialize handshake. The server runs in a process group of its own that is killed when
// the test ends, and reads no configuration file of the user (isolatedClangdEnv).
func startClangdServer(t *testing.T, clangd, root string) *clangdServer {
	t.Helper()
	cmd := exec.Command(clangd, "--background-index=false", "--log=error")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), isolatedClangdEnv(t)...)
	inGroup(cmd)
	cmd.Cancel = nil // not made with CommandContext: the cleanup below kills the group
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("clangd: %v", err)
	}
	pgid := cmd.Process.Pid
	track(pgid, true)
	t.Cleanup(func() {
		killGroup(pgid)
		cmd.Wait()
		track(pgid, false)
	})
	s := &clangdServer{cmd: cmd, in: in, messages: make(chan lspMessage, 64)}
	go s.read(bufio.NewReader(out))

	// A server that never answers ends with the test: the time limit kills its group.
	timer := time.AfterFunc(clangdTimeout*2, func() { killGroup(pgid) })
	t.Cleanup(func() { timer.Stop() })

	var result json.RawMessage
	s.call(t, "initialize", map[string]any{
		"processId": os.Getpid(),
		"rootUri":   "file://" + root,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true},
			},
		},
	}, &result)
	s.notify("initialized", map[string]any{})
	return s
}

// read decodes the messages of the server until its output ends.
func (s *clangdServer) read(r *bufio.Reader) {
	defer close(s.messages)
	for {
		length := -1
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length: "); ok {
				length, _ = strconv.Atoi(v)
			}
		}
		if length < 0 {
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var m lspMessage
		if err := json.Unmarshal(body, &m); err != nil {
			m.err = err
		}
		s.messages <- m
	}
}

func (s *clangdServer) send(v any) {
	body, _ := json.Marshal(v)
	fmt.Fprintf(s.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func (s *clangdServer) notify(method string, params any) {
	s.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// call sends a request and decodes the result into out. Requests of the server to the client
// (work done progress, registrations) are answered with null.
func (s *clangdServer) call(t *testing.T, method string, params, out any) {
	t.Helper()
	s.nextID++
	id := s.nextID
	s.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	timeout := time.After(clangdTimeout)
	for {
		select {
		case m, ok := <-s.messages:
			if !ok {
				t.Fatalf("clangd ended before it answered %s", method)
			}
			if m.err != nil {
				t.Fatalf("clangd sent a message that is not JSON: %v", m.err)
			}
			if m.Method != "" && len(m.ID) > 0 {
				s.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": nil})
				continue
			}
			if string(m.ID) != strconv.Itoa(id) {
				continue
			}
			if m.Error != nil {
				t.Fatalf("clangd answered %s with an error: %s", method, m.Error.Message)
			}
			if err := json.Unmarshal(m.Result, out); err != nil {
				t.Fatalf("the answer to %s is not what was asked for: %v\n%s",
					method, err, m.Result)
			}
			return
		case <-timeout:
			t.Fatalf("clangd did not answer %s in %v", method, clangdTimeout)
		}
	}
}

// shutdown ends the server the way an editor does and checks that it exits.
func (s *clangdServer) shutdown(t *testing.T) {
	t.Helper()
	var result json.RawMessage
	s.call(t, "shutdown", nil, &result)
	s.notify("exit", nil)
	s.in.Close()
	done := make(chan struct{})
	go func() {
		for range s.messages {
		}
		close(done)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		t.Errorf("clangd did not exit after the exit notification")
	}
}
