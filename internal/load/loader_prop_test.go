package load

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
)

// TestExcludedInternalTest checks a package whose only test file of its
// own package is left out by the build constraints (by its name or by
// //go:build ignore): for its test it is the plain package, also when an
// earlier root loaded it, so the program has one c and go/types sees one
// type c.T.
func TestExcludedInternalTest(t *testing.T) {
	for name, src := range map[string]string{
		"c/w_windows_test.gp": "package c\n",
		"c/w_test.gp":         "//go:build ignore\n\npackage c\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := writeTree(t, map[string]string{
				"gocpp.mod":   "module example.com/v\n",
				"c/c.gp":      "package c\n\ntype T int\n",
				name:          src,
				"c/x_test.gp": "package c_test\n\nimport (\n\t\"example.com/v/c\"\n\t\"example.com/v/q\"\n)\n\nvar _ c.T = q.V\n",
				"q/q.gp":      "package q\n\nimport \"example.com/v/c\"\n\nvar V c.T\n",
				"d/d.gp":      "package d\n\nimport (\n\t_ \"example.com/v/c\"\n\t_ \"example.com/v/q\"\n)\n",
			})
			for _, patterns := range [][]string{{"./d", "./c"}, {"./c", "./d"}, {"./c"}} {
				prog := loadIn(t, Config{Tests: true, GOOS: "linux", GOARCH: "amd64"}, root, patterns...)
				wantErrors(t, prog, root, "")
				n := 0
				for _, p := range prog.Packages {
					if p.ImportPath == "example.com/v/c" {
						n++
						if p.ForTest != "" {
							t.Errorf("%v: c is a test variant", patterns)
						}
					}
				}
				if n != 1 {
					t.Errorf("%v: %d packages example.com/v/c", patterns, n)
				}
				if got := typeCheckProgram(t, prog, root); got != "" {
					t.Errorf("%v: go/types errors:\n%s", patterns, got)
				}
			}
		})
	}
}

// TestTestLayouts is a property test of the loader with Config.Tests. It
// makes random small modules (randomLayout) and loads each one for linux
// and for windows, with every order of its root patterns, and also with
// the patterns written another way (a pattern twice, an import path for a
// folder, and from a sub-folder as Config.Dir). For one system, every run
// must give:
//
//   - at most one plain package (ForTest "") with files per import path (a
//     Tests root that has only external test files is not the plain
//     package, so a plain package without files can be there too, with
//     its "no non-test .gp files" error);
//   - the same sorted list of (package as go test shows it, error message)
//     pairs, duplicates kept (positions may differ: a package-wide error
//     is at the import of the package that loaded it first);
//   - the same sorted list of Roots as go test shows them, and of the
//     packages with their numbers of files;
//   - no internal error;
//   - when there is no error, no error from stock go/types either.
//
// A run without Tests must give the same errors for each plain package
// that has files in both runs. The layouts come from a fixed seed, so a
// failure can be repeated; the message shows the files.
func TestTestLayouts(t *testing.T) {
	n := 300
	if testing.Short() {
		n = 40
	}
	rng := rand.New(rand.NewPCG(1, 9))
	for i := range n {
		files, roots := randomLayout(rng)
		variants := patternVariants(rng, roots)
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			root := writeTree(t, files)
			for _, goos := range []string{"linux", "windows"} {
				checkLayout(t, root, goos, roots, variants, files)
			}
		})
	}
}

// A patternRun is a way to name the roots: the patterns, relative to the
// sub-folder dir of the module ("" for the top).
type patternRun struct {
	dir      string
	patterns []string
}

// patternVariants returns other ways to write the root patterns roots
// ("./p0", ...): with a pattern twice, with an import path for a folder,
// and from the folder of the first root.
func patternVariants(rng *rand.Rand, roots []string) []patternRun {
	var runs []patternRun
	twice := append(slices.Clone(roots), roots[rng.IntN(len(roots))])
	runs = append(runs, patternRun{patterns: twice})
	mixed := slices.Clone(roots)
	k := rng.IntN(len(mixed))
	mixed = append(mixed, "example.com/r/"+strings.TrimPrefix(mixed[k], "./"))
	rng.Shuffle(len(mixed), func(i, j int) { mixed[i], mixed[j] = mixed[j], mixed[i] })
	runs = append(runs, patternRun{patterns: mixed})
	sub := strings.TrimPrefix(roots[0], "./")
	var fromSub []string
	for _, r := range roots {
		if r == roots[0] {
			fromSub = append(fromSub, ".")
		} else {
			fromSub = append(fromSub, "../"+strings.TrimPrefix(r, "./"))
		}
	}
	runs = append(runs, patternRun{dir: sub, patterns: fromSub})
	return runs
}

// A loadSignature is what must be the same in every run for one system.
type loadSignature struct {
	errs, roots, pkgs []string
}

// checkLayout loads the module in root for goos in every run (see
// TestTestLayouts) and checks the properties.
func checkLayout(t *testing.T, root, goos string, roots []string, variants []patternRun, files map[string]string) {
	t.Helper()
	var runs []patternRun
	for order := range permutations(roots) {
		runs = append(runs, patternRun{patterns: order})
	}
	runs = append(runs, variants...)
	var first loadSignature
	var firstRun patternRun
	var plainTests map[string][]string
	for i, run := range runs {
		prog := Load(Config{Dir: filepath.Join(root, run.dir), Stdlib: fakeStd, GoStd: goStd,
			GOOS: goos, GOARCH: "amd64", Tests: true}, run.patterns...)
		checkProgram(t, prog)
		fail := func(format string, args ...any) {
			t.Helper()
			t.Errorf("%s, Dir %q, roots %v: %s\nfiles:\n%s", goos, run.dir, run.patterns,
				fmt.Sprintf(format, args...), layoutText(files))
		}
		plain := make(map[string]int)
		for _, p := range prog.Packages {
			if p.ForTest == "" && len(p.Files) > 0 {
				if plain[p.ImportPath]++; plain[p.ImportPath] == 2 {
					fail("two plain packages %s with files", p.ImportPath)
				}
			}
		}
		sig := signature(prog)
		for _, e := range sig.errs {
			if strings.Contains(e, "internal error") {
				fail("%s", e)
			}
		}
		if i == 0 {
			first, firstRun = sig, run
			plainTests = plainErrors(prog)
		} else {
			for _, c := range []struct {
				what      string
				got, want []string
			}{{"errors", sig.errs, first.errs}, {"roots", sig.roots, first.roots}, {"packages", sig.pkgs, first.pkgs}} {
				if !slices.Equal(c.got, c.want) {
					fail("%s differ from those of Dir %q, roots %v:\n%s\nthere:\n%s", c.what, firstRun.dir,
						firstRun.patterns, strings.Join(c.got, "\n"), strings.Join(c.want, "\n"))
				}
			}
		}
		if !prog.HasErrors() {
			if got := typeCheckProgram(t, prog, root); got != "" {
				fail("go/types errors:\n%s", got)
			}
		}
	}

	// Without Tests: the plain packages have the same errors.
	prog := Load(Config{Dir: root, Stdlib: fakeStd, GoStd: goStd, GOOS: goos, GOARCH: "amd64"}, roots...)
	checkProgram(t, prog)
	for path, errs := range plainErrors(prog) {
		if want, ok := plainTests[path]; ok && !slices.Equal(errs, want) {
			t.Errorf("%s, roots %v: without Tests, %s has the errors\n%s\nwith Tests:\n%s\nfiles:\n%s", goos, roots, path,
				strings.Join(errs, "\n"), strings.Join(want, "\n"), layoutText(files))
		}
	}
}

// signature returns what must be the same in every run: the errors of
// prog as sorted "package: message" lines (the related messages included,
// without positions, duplicates kept), the sorted roots, and the sorted
// packages with their numbers of files.
func signature(prog *Program) loadSignature {
	var sig loadSignature
	for _, d := range prog.Errors.Diagnostics() {
		sig.errs = append(sig.errs, "program: "+d.Msg)
	}
	for _, p := range prog.Packages {
		for _, d := range p.Errors.Diagnostics() {
			sig.errs = append(sig.errs, shown(p)+": "+message(d))
		}
		sig.pkgs = append(sig.pkgs, fmt.Sprintf("%s %d", shown(p), len(p.Files)))
	}
	for _, r := range prog.Roots {
		sig.roots = append(sig.roots, shown(r))
	}
	slices.Sort(sig.errs)
	slices.Sort(sig.roots)
	slices.Sort(sig.pkgs)
	return sig
}

// plainErrors returns the sorted error messages of each plain package
// with files, by import path.
func plainErrors(prog *Program) map[string][]string {
	errs := make(map[string][]string)
	for _, p := range prog.Packages {
		if p.ForTest != "" || len(p.Files) == 0 {
			continue
		}
		msgs := []string{}
		for _, d := range p.Errors.Diagnostics() {
			msgs = append(msgs, message(d))
		}
		slices.Sort(msgs)
		errs[p.ImportPath] = msgs
	}
	return errs
}

// message returns the message of d with its related messages.
func message(d diag.Diagnostic) string {
	msg := d.Msg
	for _, r := range d.Related {
		msg += " / " + r.Msg
	}
	return msg
}

// randomLayout returns the files of a random module and its root patterns:
// 2 to 4 packages p0, p1, ...; a package imports only packages with a
// smaller number, so the plain packages have no cycle; 0 to 2 internal and
// 0 to 2 external test files each, which can import any package (and so
// close a cycle in a test), the external ones sometimes with a dot import
// (and a header with the name that it brings) or a name that sorts before
// main.gp. Now and then a package has no main.gp, or one that its build
// constraints exclude, and a file has a broken //go:build line or a syntax
// error; a test file can also have a wrong package clause, //go:build
// ignore, or a _windows_test.gp name.
func randomLayout(rng *rand.Rand) (map[string]string, []string) {
	n := 2 + rng.IntN(3)
	files := map[string]string{"gocpp.mod": "module example.com/r\n"}
	path := func(k int) string { return fmt.Sprintf("example.com/r/p%d", k) }
	hasMain := make([]bool, n)
	// uses returns an import block and declarations that use the packages
	// ks, so that no import is unused; dot dot-imports the first one. A
	// package without a main file declares no T, so it is imported with _.
	uses := func(ks []int, dot bool) (string, string) {
		if len(ks) == 0 {
			return "", ""
		}
		var imp, decl strings.Builder
		imp.WriteString("import (\n")
		for i, k := range ks {
			switch {
			case !hasMain[k]:
				fmt.Fprintf(&imp, "\t_ %q\n", path(k))
			case i == 0 && dot:
				fmt.Fprintf(&imp, "\t. %q\n\n\t\"<T.h>\"\n", path(k))
				decl.WriteString("var _ T = V\n")
			default:
				fmt.Fprintf(&imp, "\t%q\n", path(k))
				fmt.Fprintf(&decl, "var _ p%d.T = p%d.V\n", k, k)
			}
		}
		imp.WriteString(")\n\n")
		return imp.String(), decl.String()
	}
	pick := func(ks []int) []int {
		var out []int
		for _, k := range ks {
			if rng.IntN(2) == 0 {
				out = append(out, k)
			}
		}
		return out
	}
	// spoil makes a file wrong now and then. A wrong clause and excluding
	// are only for test files: a main file alone with another name is a
	// fine package (a missing main file is chosen on its own).
	spoil := func(name, clause, body string, test bool) (string, string) {
		header := ""
		switch r := rng.IntN(24); {
		case r == 0 && test:
			clause = "wrong" + strings.TrimPrefix(clause, strings.TrimSuffix(clause, "_test"))
		case r == 1:
			header = "//go:build linux &&\n\n"
		case r == 2:
			body += "var = (\n"
		case r == 3 && test:
			header = "//go:build ignore\n\n"
		case r == 4 && test:
			name = strings.TrimSuffix(name, "_test.gp") + "_windows_test.gp"
		}
		return name, header + "package " + clause + "\n\n" + body
	}
	var roots []string
	for k := range n {
		dir := fmt.Sprintf("p%d", k)
		files[dir+"/README"] = "The folder is there even without .gp files.\n"
		var lower, others []int
		for j := range n {
			if j < k {
				lower = append(lower, j)
			}
			if j != k {
				others = append(others, j)
			}
		}
		imp, decl := uses(pick(lower), false)
		switch r := rng.IntN(12); {
		case r == 0:
			// No main file: the package has no non-test files.
		case r == 1:
			files[dir+"/main.gp"] = "//go:build ignore\n\npackage " + dir + "\n\n" + imp + "type T int\n\nvar V T\n\n" + decl
		default:
			hasMain[k] = true
			name, src := spoil(dir+"/main.gp", dir, imp+"type T int\n\nvar V T\n\n"+decl, false)
			files[name] = src
		}
		for i := range rng.IntN(3) {
			imp, decl := uses(pick(others), false) // may close a cycle in the test
			name, src := spoil(fmt.Sprintf("%s/in%d_test.gp", dir, i), dir, imp+decl, true)
			files[name] = src
		}
		for i := range rng.IntN(3) {
			ks := append([]int{k}, pick(others)...)
			rng.Shuffle(len(ks), func(a, b int) { ks[a], ks[b] = ks[b], ks[a] })
			imp, decl := uses(ks, rng.IntN(4) == 0)
			file := fmt.Sprintf("%s/x%d_test.gp", dir, i)
			if rng.IntN(4) == 0 {
				file = fmt.Sprintf("%s/a%d_test.gp", dir, i) // before main.gp
			}
			name, src := spoil(file, dir+"_test", imp+decl, true)
			files[name] = src
		}
		if rng.IntN(3) > 0 {
			roots = append(roots, "./"+dir)
		}
	}
	if len(roots) == 0 {
		roots = []string{"./p0"}
	}
	return files, roots
}

// permutations yields every order of s.
func permutations(s []string) func(yield func([]string) bool) {
	return func(yield func([]string) bool) {
		var rec func(k int) bool
		rec = func(k int) bool {
			if k == len(s) {
				return yield(slices.Clone(s))
			}
			for i := k; i < len(s); i++ {
				s[k], s[i] = s[i], s[k]
				if !rec(k + 1) {
					return false
				}
				s[k], s[i] = s[i], s[k]
			}
			return true
		}
		rec(0)
	}
}

// layoutText returns the files, sorted by name, for a failure message.
func layoutText(files map[string]string) string {
	var names []string
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "-- %s --\n%s", name, files[name])
	}
	return b.String()
}
