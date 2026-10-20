package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The compatibility rule of typed interop (Phase 12 step 12.11, ADR-101, docs/spec/typed-interop.md
// §13): a program that compiles and runs in opaque mode compiles and prints the same in typed mode.
// TestCorpusTyped runs the corpora of the repository that import C/C++ headers (tests/interop,
// tests/zoo, tests/run, tests/pkgs and examples) with GOCPP_INTEROP=typed and with each compiler,
// against the same expectations the opaque suites use (the // Output: blocks, exit codes and stderr
// patterns, which opaque mode meets). So "ok in both" is exactly "TestInterop and TestCorpusTyped
// pass", and a program that typed mode rejects, or runs differently, fails here.
//
// What is not run in typed mode (typedModeSkip):
//   - a program without a header import (typed mode does nothing for it);
//   - an error test of opaque mode (// ERROR comments): typed mode may accept it (that is what it
//     is for) or reject it in other words: they are covered by internal/typedinterop/icheck and
//     tests/typed;
//   - the programs of typedExceptions, each with its reason.
//
// It is the same test with the mode set in the environment, so the cost is TestInterop's: go
// test -run TestCorpusTyped ./tests (with GOCPP_TEST_PHASE=15 as the suites need).

// typedExceptions are the corpus programs that do not run in typed mode with their opaque
// expectation, by path, each with the reason. All of them are tests of a mistake that opaque mode rejects (an
// ExitCode: 1 test with a // Stderr: pattern instead of // ERROR comments): they are not programs that
// compile in opaque mode, so the compatibility rule does not cover them.
var typedExceptions = map[string]string{
	"tests/interop/implicit_conversion.gp": "opaque mode rejects `var iv int = cmath.pow(10, 30)`; typed mode accepts it, with C++'s conversion written as gp::cgo::as (ADR-087)",
	"tests/interop/assign_to_call.gp":      "opaque mode rejects `v.at(0) = 5`; typed mode accepts it, a T& result is an lvalue (ADR-087)",
	"tests/interop/addr_loopvar_error.gp":  "the program is wrong twice: &i of a loop variable (the lowering's error, which the test expects) and a C.int pointer passed as *Counter (the typed checker's error comes first with clang++)",
}

var headerImportRE = regexp.MustCompile(`"(<[^">]+>|\./[^"]*\.(h|hpp|hh|hxx))"`)

// typedModeSkip returns why the program p is not run in typed mode, or "". It answers only while
// the suite runs in typed mode (GOCPP_INTEROP=typed).
func typedModeSkip(p *program) string {
	if os.Getenv("GOCPP_INTEROP") != "typed" {
		return ""
	}
	if why := typedExceptions[p.path]; why != "" {
		return "typed mode: " + why
	}
	if p.wants != nil {
		return "an error test of opaque mode (typed mode words those errors itself: icheck and tests/typed)"
	}
	if !importsHeader(p) {
		return "no C/C++ header is imported: typed mode changes nothing"
	}
	return ""
}

// importsHeader reports whether the program (one file, or the folder of a module) imports a header.
func importsHeader(p *program) bool {
	if p.arg != "." {
		b, err := os.ReadFile(filepath.Join(p.dir, p.arg))
		return err != nil || headerImportRE.Match(b)
	}
	found := false
	_ = filepath.WalkDir(p.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found || d.IsDir() || !strings.HasSuffix(path, ".gp") {
			return nil
		}
		if b, err := os.ReadFile(path); err == nil && headerImportRE.Match(b) {
			found = true
		}
		return nil
	})
	return found
}

// TestCorpusTyped runs the suites that have C/C++ programs with GOCPP_INTEROP=typed.
func TestCorpusTyped(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	t.Run("interop", TestInterop)
	t.Run("zoo", TestZoo)
	t.Run("run", TestRun)
	t.Run("pkgs", TestPkgs)
	t.Run("examples", TestExamples)
}
