package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
)

// The C and C++ libraries of the modules in gocpp build and run (Phase 06
// step 6.1d, internal/build/cxxlib.go, docs/spec/modules.md).

// resolveLibs looks at the sources of the libraries of pkg's modules (the
// patterns of their recipes) before anything is compiled, so that a source that
// does not exist is an error at its line of gocpp.mod. It reports whether a
// library has a C file (the build then needs the C compiler), and the exit
// status, 0 when there is no error.
func (d *driver) resolveLibs(pkg *check.Package) (needC bool, code int) {
	var errs []diag.Diagnostic
	for _, lib := range pkg.Flags.Libs {
		srcs, lerrs := lib.Sources()
		errs = append(errs, lerrs...)
		if d.stamp != nil {
			// The patterns of the recipe were expanded by listing these folders, which the loader's
			// file system did not see: the stamp watches them, so that a new file that matches is a
			// change (a removed or edited one is seen by the compile's own dependencies).
			for _, dir := range lib.GlobDirs() {
				d.stamp.fsrec.ReadDir(dir)
			}
		}
		for _, s := range srcs {
			needC = needC || s.C
		}
	}
	if len(errs) == 0 {
		return needC, 0
	}
	fmt.Fprintf(d.stderr, "# %s\n", pkg.Load.ImportPath)
	for _, e := range errs {
		fmt.Fprintf(d.stderr, "%s:%d: %s\n", diag.ShortFileName(d.check.dir, e.Pos.Filename), e.Pos.Line, e.Msg)
	}
	return false, exitErrors
}

// libFailed prints the error of a source file of a library that didn't
// compile: an error of the build (the recipe, or the library for this compiler
// and these flags), never a bug of gocpp. It names the module and the line of
// the recipe that listed the file, and shows the compiler's output (the first
// maxLibLines lines; -cpperrors=full shows all). -x shows the command.
func (d *driver) libFailed(pkg *check.Package, e *build.CompileError) {
	lib := e.Lib
	fmt.Fprintf(d.stderr, "# %s\n", lib.Module)
	fmt.Fprintf(d.stderr, "the C or C++ library of module %s@%s did not build: %s did not compile (recipe %s:%d: %s)\n",
		lib.Module, lib.Version, strings.TrimPrefix(e.Source, lib.Dir+"/"), diag.ShortFileName(d.check.dir, lib.File), e.LibLine, e.LibText)
	out := strings.TrimRight(string(e.Output), "\n")
	if out == "" {
		out = fmt.Sprintf("%s: %v (the compiler printed nothing)", e.Command, e.Err)
	}
	lines := strings.Split(out, "\n")
	if !d.flags.cppErrors.full() && len(lines) > maxLibLines {
		lines = append(lines[:maxLibLines:maxLibLines], fmt.Sprintf("(and %d more lines: -cpperrors=full shows them)", len(lines)-maxLibLines))
	}
	fmt.Fprintln(d.stderr, strings.Join(lines, "\n"))
	for _, h := range shadowHints(lib.Public) {
		fmt.Fprintf(d.stderr, "(%s)\n", h)
	}
	if !d.flags.x {
		fmt.Fprintln(d.stderr, "(gocpp build -x shows the command)")
	}
}

// maxLibLines is how many lines of a library's compiler output are shown.
const maxLibLines = 40

// noRecipeHints returns the hints for a link error of pkg whose output is out:
// a module whose header the package imports has no cxx recipe, so none of its C
// or C++ files is built (the headers alone don't define anything). It is shown
// only for a link that failed for missing symbols, and only for a module whose
// folder has C or C++ files at all.
func noRecipeHints(pkg *check.Package, out []byte) []string {
	if !bytes.Contains(out, []byte("undefined reference")) && !bytes.Contains(out, []byte("undefined symbol")) {
		return nil
	}
	var hints []string
	for _, cm := range pkg.Load.CxxModules {
		if cm.Recipe == nil && dirHasSources(cm.Module.Dir) {
			hints = append(hints, fmt.Sprintf("(module %s has no cxx recipe, so its C and C++ files are not built: add to gocpp.mod  cxx %[1]s { src file.c ... }  or ask the library for a gocpp.mod)", cm.Module.Path))
		}
	}
	return hints
}

// maxHintEntries bounds the folder walk of dirHasSources.
const maxHintEntries = 20000

// dirHasSources reports whether the folder dir has a C or C++ file (.c, .cc,
// .cpp, .cxx) somewhere below it. It looks at no more than maxHintEntries
// entries.
func dirHasSources(dir string) bool {
	n, found := 0, false
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if n++; n > maxHintEntries || found {
			return filepath.SkipAll
		}
		if !d.IsDir() {
			switch filepath.Ext(p) {
			case ".c", ".cc", ".cpp", ".cxx":
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// stdHeaders are file names that a folder on the include path (-I) must not
// have: they are the names of the C and C++ standard headers, and a file with
// such a name in a folder of -I hides the system's for every compile that has
// the flag.
var stdHeaders = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`assert.h complex.h ctype.h errno.h fenv.h float.h inttypes.h iso646.h limits.h locale.h math.h
		setjmp.h signal.h stdalign.h stdarg.h stdatomic.h stdbool.h stddef.h stdint.h stdio.h stdlib.h string.h tgmath.h threads.h
		time.h uchar.h wchar.h wctype.h unistd.h pthread.h fcntl.h
		algorithm array atomic bit chrono cmath concepts cstddef cstdint cstdio cstdlib cstring ctime deque exception filesystem
		functional iostream limits list map memory mutex new numeric optional queue set span sstream stack string string_view
		thread tuple type_traits unordered_map unordered_set utility variant vector`) {
		stdHeaders[n] = true
	}
}

// shadowHints returns a hint for each -I folder of the compile flags flags
// that has a file named like a standard header: a recipe's `include .` with a
// stdint.h at the root of the library puts it before the system's, and the
// compiles fail in ways that do not point at the cause.
func shadowHints(flags []string) []string {
	var hints []string
	for i, f := range flags {
		dir := ""
		switch {
		case f == "-I" && i+1 < len(flags):
			dir = flags[i+1]
		case strings.HasPrefix(f, "-I") && len(f) > 2:
			dir = f[2:]
		default:
			continue
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() && stdHeaders[e.Name()] {
				hints = append(hints, fmt.Sprintf("the recipe's include folder %s has a file named %s, which hides the standard header of that name in every compile with -I %[1]s: "+
					"use a more specific include folder in the recipe, such as include include", dir, e.Name()))
				break
			}
		}
	}
	return hints
}
