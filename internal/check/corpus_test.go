package check

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The "no false errors" corpus (Phase 01 README §5, step 1.15) is
// testdata/corpus: interop-heavy .gp files, each checked as a program of
// its own (like gocpp check file.gp) in the module corpus, with the real
// namespace probe and without the phase gates (Config.Phase LastPhase):
// most of what the corpus uses is gated until Phases 03 to 08, and the
// corpus checks the front-end, not the gates. testdata/corpus/README.md
// describes the files.
//
// A file without // ERROR comments must check without any error or
// warning. A file with them (the seeded mistakes) must report exactly
// those errors on those lines, with the rules of tests/errors (a regexp in
// double quotes, matched against the message; one regexp uses up every
// error of its line that it matches), and nothing else.
//
// The probe runs the real compilers by the rules of the other probe tests
// (forEachCompiler: GOCPP_CXX, GOCPP_REQUIRE_CXX). A file that needs a
// library (a //gocpp:pkg-config directive, or a "// pkg-config: lib..."
// comment for a header found without the directive) is skipped when
// pkg-config doesn't find it, like the tests in tests/interop.

// A corpusWant is an error that a corpus file wants: the regexp of a
// // ERROR comment and its line.
type corpusWant struct {
	line int
	expr string
	rx   *regexp.Regexp
}

var (
	// corpusErrorRx finds a // ERROR comment and the text after it.
	corpusErrorRx = regexp.MustCompile(`// ERROR( .*)?$`)
	// corpusQuotedRx matches one regexp in double quotes.
	corpusQuotedRx = regexp.MustCompile(`^"([^"]*)"\s*`)
	// corpusPkgConfigRx matches the libraries a file needs.
	corpusPkgConfigRx = regexp.MustCompile(`(?m)^//(?:gocpp:| )pkg-config:? (.*)$`)
)

// corpusWants reads the // ERROR comments of a corpus file.
func corpusWants(t *testing.T, name string, src []byte) []corpusWant {
	t.Helper()
	var wants []corpusWant
	for i, line := range strings.Split(string(src), "\n") {
		m := corpusErrorRx.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		rest := strings.TrimSpace(m[1])
		if rest == "" {
			t.Fatalf("%s:%d: want a regexp in double quotes after ERROR", name, i+1)
		}
		for rest != "" {
			q := corpusQuotedRx.FindStringSubmatch(rest)
			if q == nil {
				t.Fatalf("%s:%d: want a regexp in double quotes, not %q", name, i+1, rest)
			}
			rx, err := regexp.Compile(q[1])
			if err != nil {
				t.Fatalf("%s:%d: %v", name, i+1, err)
			}
			wants = append(wants, corpusWant{line: i + 1, expr: q[1], rx: rx})
			rest = rest[len(q[0]):]
		}
	}
	return wants
}

// corpusLibraries returns the pkg-config packages that a corpus file
// needs.
func corpusLibraries(src []byte) []string {
	var libs []string
	for _, m := range corpusPkgConfigRx.FindAllStringSubmatch(string(src), -1) {
		libs = append(libs, strings.Fields(m[1])...)
	}
	return libs
}

// skipWithoutLibraries skips the test when pkg-config doesn't find one of
// the libraries.
func skipWithoutLibraries(t *testing.T, libs []string) {
	t.Helper()
	if len(libs) == 0 {
		return
	}
	if _, err := exec.LookPath("pkg-config"); err != nil {
		t.Skipf("needs %s, but pkg-config is not installed", strings.Join(libs, " "))
	}
	for _, lib := range libs {
		if err := exec.Command("pkg-config", "--exists", lib).Run(); err != nil {
			t.Skipf("pkg-config doesn't find %s", lib)
		}
	}
}

// corpusErrors checks the corpus file name in dir with the compiler cxx
// and returns the problems: the wanted errors that are missing, and
// everything else that Check (or the loader) reported.
func corpusErrors(t *testing.T, dir, name string, cxx build.CXX, wants []corpusWant) []string {
	t.Helper()
	cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	prog := load.Load(cfg, name)
	r := Check(prog, probeConfig(cxx, LastPhase))
	type reported struct {
		file string
		line int
		msg  string
		used bool
	}
	var got []*reported
	for _, d := range prog.Errors.Diagnostics() {
		got = append(got, &reported{d.Pos.Filename, d.Pos.Line, "load: " + d.Msg, false})
	}
	for _, p := range r.Packages {
		if !p.Checked() {
			msg := "package " + p.Load.ImportPath + " was not checked"
			got = append(got, &reported{"", 0, msg, false})
		}
		for _, d := range p.Errors.Diagnostics() {
			msg := d.Msg
			if d.Severity != diag.Error {
				msg = d.Severity.String() + ": " + msg
			}
			got = append(got, &reported{d.Pos.Filename, d.Pos.Line, msg, false})
		}
	}
	var problems []string
	for _, w := range wants {
		matched := false
		for _, g := range got {
			onLine := g.line == w.line && filepath.Base(g.file) == name
			if !g.used && onLine && w.rx.MatchString(g.msg) {
				g.used, matched = true, true
			}
		}
		if !matched {
			problems = append(problems,
				fmt.Sprintf("%s:%d: missing error %q", name, w.line, w.expr))
		}
	}
	for _, g := range got {
		if !g.used {
			rel, err := filepath.Rel(dir, g.file)
			if err != nil || g.file == "" {
				rel = g.file
			}
			problems = append(problems, fmt.Sprintf("unexpected: %s:%d: %s", rel, g.line, g.msg))
		}
	}
	return problems
}

// TestCorpus checks the files of the corpus (see above).
func TestCorpus(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 10 {
		t.Fatalf("only %d corpus files in %s", len(paths), dir)
	}
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		for _, path := range paths {
			name := filepath.Base(path)
			t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
				t.Parallel()
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				skipWithoutLibraries(t, corpusLibraries(src))
				wants := corpusWants(t, name, src)
				if problems := corpusErrors(t, dir, name, cxx, wants); len(problems) > 0 {
					t.Errorf("%s with %s:\n\t%s", name, cxx.Path, strings.Join(problems, "\n\t"))
				}
			})
		}
	})
}
