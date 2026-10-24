package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
)

// The build flags of Phase 06 step 6.12 (ADR-044): -tags, the trusted flags of the user (-cflags,
// -cxxflags, -ldflags), the mode (-debug, -release, -race, -strip), -a, -n, -v, -emit-llvm and -compdb.
// buildFlags (build.go) has the older ones (-o, -p, -x, -work, -asan, -nochecks, -cpperrors).

// extraFlags are the flags that step 6.12 added to gocpp build and gocpp run.
type extraFlags struct {
	tags     []string // -tags: more build tags
	cflags   []string // -cflags: every C and C++ compile
	cxxflags []string // -cxxflags: every C++ compile
	ldflags  []string // -ldflags: the link
	mode     build.Profile

	rebuild bool   // -a: compile everything again
	dryRun  bool   // -n: print the commands, run nothing
	verbose bool   // -v: print the packages
	llvm    string // -emit-llvm: "", "ll" or "bc"
	compdb  string // -compdb: "" (not given), "true" (the default places), "false" (never) or a file or folder
}

// A listFlag is a flag whose value is a list of words, split like GOCPP_CXXFLAGS is (quotes keep
// spaces). A flag given twice adds to the list.
type listFlag struct {
	dst *[]string
}

func (l listFlag) String() string {
	if l.dst == nil {
		return ""
	}
	return strings.Join(*l.dst, " ")
}

func (l listFlag) Set(v string) error {
	words, err := build.SplitCommand(v)
	if err != nil {
		return err
	}
	*l.dst = append(*l.dst, words...)
	return nil
}

// A tagsFlag is -tags: a comma-separated list of build tags, as go build's.
type tagsFlag struct {
	dst *[]string
}

func (t tagsFlag) String() string {
	if t.dst == nil {
		return ""
	}
	return strings.Join(*t.dst, ",")
}

func (t tagsFlag) Set(v string) error {
	tags, err := parseTags(v)
	if err != nil {
		return err
	}
	*t.dst = tags // like go build, the last -tags is the list
	return nil
}

// tagPattern is what a build tag looks like: letters, digits, '_' and '.' (go/build/constraint).
var tagPattern = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

// parseTags splits the value of -tags at commas, like go build. A tag that has a space or another
// character that a //go:build line can't have is an error (go build's older space-separated list
// is not accepted: it is deprecated in Go).
func parseTags(v string) ([]string, error) {
	var tags []string
	for _, t := range strings.Split(v, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !tagPattern.MatchString(t) {
			if strings.ContainsAny(t, " \t") {
				return nil, fmt.Errorf("invalid build tag %q: separate the tags with commas", t)
			}
			return nil, fmt.Errorf("invalid build tag %q: a tag has letters, digits, _ and dots", t)
		}
		if !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	return tags, nil
}

// An optFlag is a flag that can be given without a value (-emit-llvm) or with one
// (-emit-llvm=bc): "true" is the bare form. valid, when it is not nil, checks a value.
type optFlag struct {
	dst   *string
	valid func(v string) error
	// keepFalse says that -flag=false is a value of its own ("false" is stored) instead of the
	// same as not giving the flag ("" is stored): -compdb=false turns off what a debug build does.
	keepFalse bool
}

func (o optFlag) String() string {
	if o.dst == nil {
		return ""
	}
	return *o.dst
}

func (o optFlag) IsBoolFlag() bool { return true }

func (o optFlag) Set(v string) error {
	switch v {
	case "false":
		*o.dst = ""
		if o.keepFalse {
			*o.dst = "false"
		}
		return nil
	case "":
		return fmt.Errorf("the value is empty")
	}
	if o.valid != nil && v != "true" {
		if err := o.valid(v); err != nil {
			return err
		}
	}
	*o.dst = v
	return nil
}

// register adds the flags to flags. forBuild is true for gocpp build, which alone has -n,
// -emit-llvm and -compdb (gocpp run builds into a folder that it removes).
func (e *extraFlags) register(flags *flag.FlagSet, forBuild bool) {
	flags.Var(tagsFlag{&e.tags}, "tags", "a comma-separated `list` of build tags that are satisfied, like go build -tags")
	flags.Var(listFlag{&e.cflags}, "cflags", "trusted `flags` for every C and C++ compile, like GOCPP_CFLAGS")
	flags.Var(listFlag{&e.cxxflags}, "cxxflags", "trusted `flags` for every C++ compile, like GOCPP_CXXFLAGS: may load compiler plugins (-fpass-plugin=x.so, -fplugin=x.so)")
	flags.Var(listFlag{&e.ldflags}, "ldflags", "trusted `flags` for the link, like GOCPP_LDFLAGS")
	flags.BoolVar(&e.mode.Debug, "debug", false, "build with -O0 -g (what a build is without a flag)")
	flags.BoolVar(&e.mode.Release, "release", false, "build with -O2 -flto and without debug information; a crash prints only its message")
	flags.BoolVar(&e.mode.Trace, "trace", false, "with -release: keep the line tables (-g1), so a crash prints Go's trace with .gp file:line (a debug build has it already)")
	flags.BoolVar(&e.mode.Strip, "strip", false, "drop the symbol table and the debug information from the program")
	flags.BoolVar(&e.mode.Race, "race", false, "build with ThreadSanitizer")
	flags.BoolVar(&e.rebuild, "a", false, "compile everything again: no object cache and no up-to-date check")
	flags.BoolVar(&e.verbose, "v", false, "print the import paths of the packages that are built")
	if !forBuild {
		return
	}
	flags.BoolVar(&e.dryRun, "n", false, "print the commands, but run no compiler and no linker, and write no program")
	flags.Var(optFlag{dst: &e.llvm, valid: func(v string) error {
		if v != "ll" && v != "bc" {
			return fmt.Errorf("must be ll (text) or bc (bitcode)")
		}
		return nil
	}}, "emit-llvm", "also write the LLVM IR of every translation unit (clang only) next to the program: `ll` (text, the default) or bc (bitcode)")
	e.registerCompdb(flags)
}

// registerCompdb adds -compdb: gocpp build, run, test and debug have it (gocpp export has its own
// way to choose the file).
func (e *extraFlags) registerCompdb(flags *flag.FlagSet) {
	flags.Var(optFlag{dst: &e.compdb, keepFalse: true}, "compdb", "write compile_commands.json, the compilation database of clangd and other tools. A debug build (the default) writes it without the flag (into the folder of the generated C++, and gocpp build also into the module's folder); a -release build only with the flag. -compdb: those places; -compdb=`path`: that file or folder; -compdb=false: never")
}

// layered reports whether the build flags change the trusted flags of the build.
func (e *extraFlags) layered() bool {
	return len(e.cflags)+len(e.cxxflags)+len(e.ldflags) > 0 || e.mode.Race || e.mode.Release || e.mode.Trace || e.mode.Strip
}

// unstamped returns why a build with these flags is not kept for the up-to-date check, or "":
// a build that lists the commands, or writes the IR or the compilation database, has files to
// make that the check can't see, and -a means compile again.
func (e *extraFlags) unstamped() string {
	switch {
	case e.rebuild:
		return "-a compiles everything again"
	case e.dryRun:
		return "-n runs nothing"
	case e.llvm != "":
		return "-emit-llvm writes files that the check does not look at"
	case e.compdbExplicit():
		return "-compdb writes files that the check does not look at"
	}
	return ""
}

// keyParts returns what the flags put in the key of the up-to-date check: every flag that changes
// what the build does (the flags of a plugin by the stat of its file).
func (e *extraFlags) keyParts() []string {
	parts := []string{
		"tags=" + strings.Join(e.tags, ","),
		"cflags=" + strings.Join(e.cflags, "\x00"),
		"cxxflags=" + strings.Join(e.cxxflags, "\x00"),
		"ldflags=" + strings.Join(e.ldflags, "\x00"),
		fmt.Sprint("mode=", e.mode.Debug, e.mode.Release, e.mode.Race, e.mode.Strip, e.mode.Trace),
		fmt.Sprint("compdb-off=", e.compdb == "false"), // a build that wrote the file is not the build that did not
	}
	return parts
}

// userEnv returns the trusted flags of the build: the environment's with the mode's and the
// build flags' around them (build.EnvFlags.Layered), with the paths of plugins made absolute
// from the folder dir. A value of GOCPP_CFLAGS, GOCPP_CXXFLAGS or GOCPP_LDFLAGS that does not
// parse is an error.
func (e *extraFlags) userEnv(asan bool, goos, dir string) (build.EnvFlags, error) {
	env, err := build.ReadEnvFlags()
	if err != nil {
		return build.EnvFlags{}, err
	}
	mc, ml, err := build.ProfileFlags(e.mode, asan, goos)
	if err != nil {
		return build.EnvFlags{}, err
	}
	env = env.Layered(mc, ml, e.cflags, e.cxxflags, e.ldflags)
	env.CFlags = build.AbsUserPaths(env.CFlags, dir)
	env.CXXFlags = build.AbsUserPaths(env.CXXFlags, dir)
	env.LDFlags = build.AbsUserPaths(env.LDFlags, dir)
	return env, nil
}

// pluginKey returns the key parts for the plugin files that the trusted flags load.
func pluginKey(env build.EnvFlags) []string {
	return build.PluginStamp(env.CXXCompile())
}

// compdbModuleDir returns the folder of the module of the program (the folder of gocpp.mod), and
// whether there is one: a program of single .gp files has no module.
func compdbModuleDir(set *buildSet) (string, bool) {
	if m := set.main.Load.Module; m != nil && m.Dir != "" && !set.main.Load.Std {
		return m.Dir, true
	}
	return "", false
}

// compdbModuleRoot returns the folder that the bare -compdb writes compile_commands.json to: the
// folder of the module of the program, or the folder of the package.
func compdbModuleRoot(set *buildSet) string {
	if dir, ok := compdbModuleDir(set); ok {
		return dir
	}
	return set.main.Load.Dir
}

// compdbExplicit reports whether the user asked for compile_commands.json: -compdb or -compdb=path.
func (e *extraFlags) compdbExplicit() bool {
	return e.compdb != "" && e.compdb != "false"
}

// compdbAuto reports whether this build writes compile_commands.json although the flag was not
// given (ADR-044, amended 2026-10-19): a debug build does, because the person who debugs the
// program steps into the generated C++ and wants clangd to understand it. A -release build, -n
// (nothing is written), -compdb=false and gocpp export (which writes its own files) do not.
func (d *driver) compdbAuto() bool {
	e := &d.flags.extra
	return e.compdb == "" && !e.mode.Release && !e.dryRun && d.afterPlan == nil
}

// wantsCompdb reports whether the build writes compile_commands.json.
func (d *driver) wantsCompdb() bool {
	return d.flags.extra.compdbExplicit() || d.compdbAuto()
}

// compdbTargets returns where the build writes compile_commands.json. With the bare flag, the
// folder of the generated files and the folder of the module; with a value, that file or folder
// (a relative one from the current folder). Without the flag, a debug build writes the same two
// places, except that
//   - the module's folder gets its copy only from gocpp build for the host, of a program in a
//     module (gocpp run and test build many things of one module, a cross-compile's commands are
//     not those of the host: the file that clangd reads there is not replaced by them; a program
//     of single .gp files has no module and its folder may be anywhere), and
//   - nothing is written when the folder of the generated files is not kept (the object cache's
//     gen folder, or -work's): without a cache folder the build is in a work folder that is
//     removed, and the commands name files in it.
func (d *driver) compdbTargets(set *buildSet, gen string) []string {
	v := d.flags.extra.compdb
	switch v {
	case "false":
		return nil
	case "":
		if !d.compdbAuto() {
			return nil
		}
		if !d.genKept(gen) {
			return nil
		}
		targets := []string{filepath.Join(gen, "compile_commands.json")}
		if set != nil && d.name == "build" && isHostTarget(d.check.goos, d.check.goarch) {
			if dir, ok := compdbModuleDir(set); ok {
				targets = append(targets, filepath.Join(dir, "compile_commands.json"))
			}
		}
		return targets
	case "true":
		return []string{filepath.Join(gen, "compile_commands.json"), filepath.Join(compdbModuleRoot(set), "compile_commands.json")}
	}
	if !filepath.IsAbs(v) {
		// (Join drops a trailing slash, which says that the name is a folder)
		folder := os.IsPathSeparator(v[len(v)-1])
		v = filepath.Join(d.check.dir, v)
		if folder {
			v += string(filepath.Separator)
		}
	}
	return []string{v}
}

// genKept reports whether the folder gen of the generated files is still there after the build:
// it is not when it is in the work folder, which is removed (without -work).
func (d *driver) genKept(gen string) bool {
	if d.flags.work || d.work == "" {
		return true
	}
	rel, err := filepath.Rel(d.work, gen)
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// emitLLVMError returns the error for -emit-llvm with a compiler that is not clang, or nil. kind
// says which compiler cx is ("C++" or "C").
func emitLLVMError(kind string, cx build.CXX) error {
	if cx.Family == build.Clang {
		return nil
	}
	family, fix := "not clang", "set GOCPP_CXX=clang++"
	if cx.Family == build.GCC {
		family = "GCC"
	}
	if kind == "C" {
		fix = "set GOCPP_CC=clang"
	}
	return fmt.Errorf("-emit-llvm needs clang, but the %s compiler is %s (%s, which writes no LLVM IR; %s)", kind, cx.Name(), family, fix)
}

// outputPath returns where gocpp build writes the program named name: -o's file, or -o's folder
// (or a name that ends in a slash) with name in it, or name in the current folder.
func (d *driver) outputPath(name string) string {
	out := d.flags.output
	switch {
	case out == "":
		out = name
	case strings.HasSuffix(out, "/") || strings.HasSuffix(out, string(filepath.Separator)) || isDir(out):
		out = filepath.Join(out, name)
	default:
		// A program for Windows is an .exe: -o prog names prog.exe (a name that has an extension keeps it).
		out = withExeSuffix(out, d.check.goos)
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(d.check.dir, out)
	}
	return out
}

// withExeSuffix returns the file name out of a program for the operating system goos: for Windows
// ".exe" is added to a name without an extension, so that "-o prog" gives prog.exe, which is what
// CreateProcess, cmd.exe and the shell's PATHEXT look for.
func withExeSuffix(out, goos string) string {
	if goos == "windows" && filepath.Ext(out) == "" {
		return out + ".exe"
	}
	return out
}

// ensureNoDir removes nothing: it only reports whether the folder exists (for tests).
func dirExists(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi.IsDir()
}
