package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/version"
)

// exportUsage is the help text of "gocpp export".
const exportUsage = `usage: gocpp export <cmake | compdb> [-o path] [build flags] [packages]

gocpp export writes files that let other tools build or understand the C++ that gocpp makes for
a program. It does what gocpp build -n does (it writes the generated C++ and decides every
compile, but runs no compiler), then writes:

gocpp export compdb
	compile_commands.json, the JSON compilation database that clangd and other tools read: one
	entry for every translation unit of the program with the exact command that compiles it (the
	generated C++ of the packages, the packages' own C and C++ files, the sources of the libraries
	of modules, and gocpp's runtime). The entries are sorted by file, and the files and folders
	they name are stable (the folder of the generated files in the cache folder, the runtime's
	folder, the packages' folders), so a tool can open the generated C++ after the build. Without
	-o it is written to the folder of the generated files and to the module's folder (the folder
	of gocpp.mod; else the package's folder); -o names a file, or a folder that gets
	compile_commands.json. The paths are printed. gocpp build -compdb writes the same file as
	part of a build.

	clangd needs the compiler's include folders: with g++ or a compiler in a Nix shell, start
	it with --query-driver (for example --query-driver=/usr/bin/*).

gocpp export cmake
	a folder with a CMakeLists.txt that builds the program from the generated C++ without gocpp:
	gen/ (a copy of the generated files), runtime/ (gocpp's runtime) and CMakeLists.txt, which has the
	flags of every compile and of the link, exactly as the build has them, and checks that the
	compiler is of the family that they were made for (clang or g++). -o names the folder (default
	<program>-cmake). Build it with
		cmake -S <folder> -B build -DCMAKE_CXX_COMPILER=<compiler> && cmake --build build
	A folder that exists, is not empty and has no CMakeLists.txt that gocpp wrote is not touched.
	The limits: the project's own C and C++ files, its headers, the libraries of modules and the link
	flags are named by their absolute paths on this machine, the generated C++ names the .gp files
	and the headers of modules by absolute paths too (in #line and #include lines), and the export is
	for the compiler and the options of this build: run it again after the .gp files, the flags or the
	compiler change.

The build flags are those of gocpp build, except -n, -emit-llvm and -compdb, and -o means the file or
folder of the export. -buildmode is accepted: export compdb lists the compiles of a library (c-archive,
c-shared or cpp-lib) too; export cmake is for programs only (a library's link, with its export list and
header, is not a CMake target that gocpp writes: build it with gocpp build).

The flags are:
`

// runExport is "gocpp export".
func runExport(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "-help" || args[0] == "--help" {
		fmt.Fprint(stderr, exportUsage)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	kind := args[0]
	if kind != "cmake" && kind != "compdb" {
		fmt.Fprintf(stderr, "gocpp export: unknown kind %q (cmake or compdb)\nRun 'gocpp export -h' for usage.\n", kind)
		return 2
	}
	var f buildFlags
	var out string
	flags := flag.NewFlagSet("gocpp export "+kind, flag.ContinueOnError)
	flags.StringVar(&out, "o", "", "write to this file or folder")
	f.register(flags, false)
	f.registerMode(flags)
	if code, done := parseFlags("export", exportUsage, flags, args[1:], stderr); done {
		return code
	}
	if kind == "cmake" && f.mode != "" {
		fmt.Fprintf(stderr, "gocpp export cmake: -buildmode=%s: only a program can be exported to CMake (gocpp export compdb takes a library; gocpp build -buildmode=%s builds it)\n", f.mode, f.mode)
		return 2
	}
	d, code := newDriver("export", &f, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	d.afterPlan = func(set *buildSet, plan *build.Plan, name string) error {
		switch kind {
		case "compdb":
			if out != "" {
				d.flags.extra.compdb = out
			} else {
				d.flags.extra.compdb = "true"
			}
			for _, t := range d.compdbTargets(set, plan.GenRoot) {
				path, err := build.WriteCompDB(plan, t)
				if err != nil {
					return err
				}
				fmt.Fprintln(stdout, path)
			}
		case "cmake":
			dir := out
			if dir == "" {
				dir = name + "-cmake"
			}
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(d.check.dir, dir)
			}
			path, err := build.ExportCMake(plan, dir, name, version.Version)
			if err != nil {
				return err
			}
			fmt.Fprintln(stdout, path)
		}
		return nil
	}
	p, code := d.compile(flags.Args())
	if p == nil {
		return code
	}
	d.removeWork()
	return 0
}

// wantsPlan reports whether the build records its commands: for -compdb and for gocpp export.
func (d *driver) wantsPlan() bool {
	return d.flags.extra.compdb != "" || d.afterPlan != nil
}

// finishPlan is what compile does with the commands of the build once they are known: it writes
// the compilation database that -compdb asked for, and runs what gocpp export wants. It returns
// the exit status, 0 when nothing went wrong.
func (d *driver) finishPlan(set *buildSet, gen string, plan *build.Plan, name string) int {
	if d.afterPlan == nil { // gocpp export writes its own files
		for _, t := range d.compdbTargets(set, gen) {
			path, err := build.WriteCompDB(plan, t)
			if err != nil {
				fmt.Fprintf(d.stderr, "gocpp %s: compile_commands.json: %v\n", d.name, err)
				return exitErrors
			}
			if d.flags.x {
				fmt.Fprintf(d.stderr, "# compile_commands.json: %s\n", path)
			}
		}
	}
	if d.afterPlan != nil {
		if err := d.afterPlan(set, plan, name); err != nil {
			fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
			return exitErrors
		}
	}
	return 0
}
