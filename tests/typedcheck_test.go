package tests

import (
	"path/filepath"
	"strings"
	"testing"
)

// The compatibility rule of typed interop, measured on the whole corpus (Phase 12 step 12.12, exit
// criterion "no typed-mode error on programs that compile in opaque mode"): every program of
// tests/interop, tests/zoo, tests/run, tests/pkgs and examples that imports a C/C++ header passes
// `gocpp check -interop=opaque`, and must pass `gocpp check -interop=typed` too, with each C++
// compiler: with clang++ the checker's errors are errors, with g++ they are warnings (ADR-079), so
// neither may fail the check. TestCorpusTyped builds and runs the same programs (the slow half of
// the rule); this test is only the check, so it takes seconds and it names the first typed
// diagnostic when a program is rejected.
//
// The programs that typedModeSkip leaves out are the same as there: error tests of opaque mode,
// programs without a header import, and the three of typedExceptions.

// corpusPrograms returns the programs of the corpora that have C/C++ programs, for a check only: a
// program that needs a display is included (nothing runs).
func corpusPrograms(t *testing.T) []*program {
	t.Helper()
	var progs []*program
	for _, f := range []format{interopFormat, zooFormat, runFormat, pkgsFormat} {
		progs = append(progs, loadTests(t, f)...)
	}
	dir := filepath.Join(repoRoot, "examples")
	names, err := listExamples(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		rel := "examples/" + name
		exDir := filepath.Join(dir, name)
		p := &program{name: name, path: rel + "/main.gp", dir: exDir, arg: "."}
		if p.ex, p.err = loadExample(exDir, rel); p.err == nil {
			p.header = p.ex.header
		}
		progs = append(progs, p)
	}
	for _, p := range progs {
		if p.err == nil && p.header != nil {
			p.header.display = false
		}
	}
	return progs
}

// checkMode runs `gocpp check -interop=mode` on the program with the compiler.
func (c *config) checkMode(t *testing.T, p *program, cc compiler, mode string) result {
	j := gocppJob(p.dir, runEnv(p.header), cc.cxx, p.header.phase, "check", "-interop="+mode, p.arg)
	j.combined = true
	return c.run(t, j)
}

func TestTypedCheckCorpus(t *testing.T) {
	c := setup(t)
	t.Setenv("GOCPP_INTEROP", "typed") // typedModeSkip answers only in typed mode
	progs := corpusPrograms(t)
	var checked, skipped int
	for _, p := range progs {
		if p.err == nil && c.typedCheckSkip(p) == "" {
			checked++
		} else {
			skipped++
		}
	}
	t.Logf("corpora: %d programs, %d checked in typed mode, %d left out",
		len(progs), checked, skipped)
	if checked < 200 {
		t.Errorf("only %d programs are checked in typed mode, the corpora have about 290 "+
			"(a loader or a skip rule broke)", checked)
	}
	for _, p := range progs {
		t.Run(strings.ReplaceAll(p.path, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if p.err != nil {
				t.Fatal(p.err)
			}
			if why := c.typedCheckSkip(p); why != "" {
				t.Skip(why)
			}
			c.forEachCompiler(t, func(t *testing.T, cc compiler) {
				opaque := c.checkMode(t, p, cc, "opaque")
				if opaque.problem != "" || opaque.exit != 0 {
					t.Fatalf("%s: %s\nthe program does not pass the opaque check, "+
						"so it is not a case of the rule:\n%s",
						opaque.cmd, opaque.why(), indent(string(opaque.stdout)))
				}
				typed := c.checkMode(t, p, cc, "typed")
				if typed.problem != "" || typed.exit != 0 {
					t.Fatalf("%s: %s\ntyped mode rejects a program that opaque mode accepts:\n%s",
						typed.cmd, typed.why(), indent(string(typed.stdout)))
				}
				// Warnings are allowed (a g++ build turns the checker's errors into warnings),
				// but a clang++ build has none: the checker has nothing to add to a good program.
				if cc.name != "g++" && strings.TrimSpace(string(typed.stdout)) != "" {
					t.Errorf("%s printed:\n%s", typed.cmd, indent(string(typed.stdout)))
				}
			})
		})
	}
}

// typedCheckSkip returns why the program is not checked, or "". Unlike the build and run tests, a
// check needs no display and no phase beyond the gate of the program's own header.
func (c *config) typedCheckSkip(p *program) string {
	if why, _ := c.skipReason(p.header); why != "" {
		return why
	}
	if expectsCompileError(p.header) {
		return "the test expects the build to fail with an error at a line of the program: " +
			"it is no program that compiles"
	}
	return typedModeSkip(p)
}

// expectsCompileError reports whether the test is one of the error tests that are written as run
// tests: it exits with 1 (the status of a failed build) and its // Stderr: patterns name a position
// of the program ("x.gp:11:9:"). Opaque mode and the C++ compiler reject it too, so the
// compatibility rule does not cover it. A program that runs and then panics (exit 2) compiles: it is
// checked.
func expectsCompileError(h *header) bool {
	if h.exitCode != 1 {
		return false
	}
	for _, re := range h.stderr {
		if strings.Contains(re.String(), `\.gp:`) {
			return true
		}
	}
	return false
}
