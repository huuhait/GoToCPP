package tests

import (
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// mustTwin returns the path of the hand-written Go twin of a run test that uses the `!` forms of
// Phase 10 (tests/run/testdata/must_twin/NAME.go for tests/run/NAME.gp), or "" if the test has none.
//
// `!` is sugar for the `if err != nil` code that a Go programmer writes, so a test program with
// `!` has a twin that is valid Go and does the same by hand (its errors.Wrapf is a small Go
// function). TestRunVsGo runs the twin with real Go and compares what it prints with the test's
// // Output: block, so the expected output of a `!` program comes from real Go, not from
// gocpp (tests/run/README.md, "Programs with !").
//
// The same goes for the tests of the user macros (step 10.9), umacro_*.gp, whose twins are in
// tests/run/testdata/macro_twin: a macro is sugar for a function and its call, so the twin writes
// them by hand (the hidden arguments as constants, a lazy argument as a func literal, a wrap
// macro's code as a func literal) and real Go runs it.
//
// The tests of the built-in macros (step 10.8), macro_*.gp, have twins in the same folder: the built-ins are
// written as the constants the compiler puts in their place (#text as a literal, #line as a number, #func as a
// call that asks the Go runtime for the name of its caller, so the names are those of real Go).
//
// A test of another folder (a race test with a macro, tests/race) has its twin in testdata/twin of that folder.
//
// And for the tests of #Derive (step 10.10), derive_*.gp, whose twins are in
// tests/run/testdata/derive_twin: the twin writes the generated methods by hand.
func mustTwin(p *program) string {
	dir := ""
	switch {
	case strings.HasPrefix(p.name, "must_"):
		dir = "must_twin"
	case strings.HasPrefix(p.name, "umacro_"), strings.HasPrefix(p.name, "macro_"):
		dir = "macro_twin"
	case strings.HasPrefix(p.name, "derive_"):
		dir = "derive_twin"
	default:
		// Any other test (a race test, say) with a twin in testdata/twin.
		dir = "twin"
	}
	twin := filepath.Join(p.dir, "testdata", dir, p.name+".go")
	if _, err := os.Stat(twin); err != nil {
		return ""
	}
	return twin
}

// pkgsTwin returns the folder of the hand-written Go twin of a module test of tests/pkgs
// (tests/pkgs/NAME/twin), or "" if the test has none. A module that uses `!`, macros or #Derive
// across packages cannot be built by Go, so its twin is the same module written as plain Go: a
// folder with its own go.mod and .go files (the macros and the rewrites of `!` written out by hand,
// the places that a macro gets as literals). TestPkgsVsGo builds the twin in place of the module
// and compares its output with the // Output: block of main.gp, so the expected output comes from
// real Go.
func pkgsTwin(p *program) string {
	if p.arg != "." {
		return ""
	}
	twin := filepath.Join(p.dir, "twin")
	if _, err := os.Stat(filepath.Join(twin, "go.mod")); err != nil {
		return ""
	}
	return twin
}

// copyTree copies the folder src to dst, file for file.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if e.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
}

// usesPhase10Syntax reports whether a .gp file of the test program p has the syntax of Phase 10 that Go cannot
// build: a must call or function (`F!(x)`, `func f!()`), a macro declaration or use (`macro`, `lazy`, `#Name(...)`),
// a `#Derive` line. A single-file program is its file; a module folder (tests/pkgs) is all its .gp files except
// those of its twin. TestRunVsGo and its relatives use it: a program with this syntax that Go rejects must have a
// twin, or nothing would be compared with real Go (harness_test.go, runVsGoTest).
func usesPhase10Syntax(p *program) bool {
	var files []string
	if p.arg == "." {
		filepath.WalkDir(p.dir, func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if e.IsDir() && (e.Name() == "twin" || e.Name() == "testdata") {
				return fs.SkipDir
			}
			if !e.IsDir() && strings.HasSuffix(path, ".gp") {
				files = append(files, path)
			}
			return nil
		})
	} else {
		files = []string{filepath.Join(p.dir, p.arg)}
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		file, _ := parser.ParseFile(token.NewFileSet(), f, src, parser.Phase10Syntax|parser.SkipObjectResolution)
		if file != nil && len(gpast.Leftovers(file)) > 0 {
			return true
		}
	}
	return false
}
