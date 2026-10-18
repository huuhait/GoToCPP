package tests

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPkgs runs the programs in tests/pkgs, each a module in a folder of its own with a main.gp
// and the packages it imports (Phase 06 step 6.2): like TestRun, stdout must match the
// // Output: block at the end of main.gp. See tests/pkgs/README.md.
func TestPkgs(t *testing.T) {
	c := setup(t)
	c.runPrograms(t, "tests/pkgs", loadTests(t, pkgsFormat), c.runTest)
}

// TestPkgsVsGo checks the expectations of the tests in tests/pkgs (the // Output: block,
// // ExitCode: and the // Stderr: regexps) against real Go, like TestRunVsGo: the module is
// copied to a temporary folder with its .gp files renamed to .go. GOCPP_RUN_VS_GO=1 turns it on.
func TestPkgsVsGo(t *testing.T) {
	c := setup(t)
	if !c.runVsGo {
		t.Skip("GOCPP_RUN_VS_GO=1 turns it on")
	}
	var n vsGoCount
	t.Cleanup(func() {
		compared, rejected, diverges := n.compared.Load(), n.rejected.Load(), n.diverges.Load()
		t.Logf("tests/pkgs: compared %d with real Go, skipped %d (go build rejects them) and %d (// divergence:)", compared, rejected, diverges)
		if compared == 0 {
			t.Error("no test was compared with real Go")
		}
	})
	all := *c
	all.phase, all.phaseFrom = lastPhase, "every phase"
	all.runPrograms(t, "tests/pkgs", loadTests(t, pkgsFormat), func(t *testing.T, p *program) {
		c.runVsGoTest(t, p, &n)
	})
}

// TestPkgErrors checks the module folders in tests/pkgs/errors with gocpp check: it must report
// the errors of the // ERROR comments of any .gp file of the folder, at their lines, and no others.
// Each folder has a main.gp whose first line is the phase line. See tests/pkgs/README.md.
func TestPkgErrors(t *testing.T) {
	c := setup(t)
	dir := filepath.Join(repoRoot, "tests", "pkgs", "errors")
	names, err := listExamples(dir)
	if err != nil {
		t.Fatal(err)
	}
	var progs []*program
	for _, name := range names {
		d := filepath.Join(dir, name)
		trackFiles(d)
		p := &program{name: name, path: "tests/pkgs/errors/" + name + "/main.gp", dir: d, arg: "."}
		p.header, p.wants, p.err = loadFolderErrors(d, p.path)
		progs = append(progs, p)
	}
	c.runPrograms(t, "tests/pkgs/errors", progs, c.folderErrorsTest)
}

// loadFolderErrors reads the header of main.gp in the folder dir and the // ERROR and // WARNING
// comments of all its .gp files.
func loadFolderErrors(dir, path string) (*header, []wantError, error) {
	src, err := os.ReadFile(filepath.Join(dir, "main.gp"))
	if err != nil {
		return nil, nil, err
	}
	h, err := errorsFormat.parse(path, src)
	if err != nil {
		return nil, nil, err
	}
	var wants []wantError
	err = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".gp") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		ws, err := parseErrors(filepath.ToSlash(rel), src)
		if err != nil {
			return err
		}
		for i := range ws {
			ws[i].file = filepath.ToSlash(rel)
		}
		wants = append(wants, ws...)
		return nil
	})
	return h, wants, err
}

// folderErrorsTest checks a module folder with gocpp check -e ./... : the errors must be those
// of the // ERROR comments.
func (c *config) folderErrorsTest(t *testing.T, p *program) {
	r := c.run(t, func() job {
		j := gocppJob(p.dir, nil, "", p.header.phase, "check", "-e", "./...")
		j.combined = true
		return j
	}())
	if r.problem != "" {
		t.Fatalf("%s: %s\noutput:\n%s", r.cmd, r.problem, indent(string(r.stdout)))
	}
	diags, other := parseDiagnostics(string(r.stdout))
	problems := matchErrors("", p.wants, diags, other)
	wantExit := 0
	if wantsErrors(p.wants) {
		wantExit = 1
	}
	if r.exit != wantExit {
		problems = append([]string{fmt.Sprintf("%s, want exit code %d", r.status, wantExit)}, problems...)
	}
	if len(problems) > 0 {
		t.Errorf("%s:\n\t%s\noutput:\n%s", r.cmd, strings.Join(problems, "\n\t"), indent(string(r.stdout)))
	}
}

// copyAsGoModule copies the module folder src to dst as a Go module: the .gp files are renamed to
// .go, gocpp.mod becomes a go.mod with the module line, and the other files stay.
func copyAsGoModule(src, dst string) error {
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
		switch {
		case filepath.Base(rel) == "gocpp.mod":
			rel = filepath.Join(filepath.Dir(rel), "go.mod") // also the one of a replaced module
			data = []byte(goModOf(string(data)))
		case strings.HasSuffix(rel, ".gp"):
			rel = strings.TrimSuffix(rel, ".gp") + ".go"
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
}

// anyGoError matches an error of go build in any file of a module.
var anyGoError = regexp.MustCompile(`(?m)^(\./)?\S+\.go:\d+(:\d+)?: `)

// goModOf returns the go.mod for the text of a gocpp.mod: its module, require and replace
// lines (also in blocks), with the go line; the gocpp lines (cxx, cflags, ...) are left out.
func goModOf(text string) string {
	var out strings.Builder
	keep, inBlock := false, false
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(strings.SplitN(line, "//", 2)[0])
		switch {
		case inBlock:
			if len(f) > 0 && f[0] == ")" {
				inBlock = false
			}
			if keep {
				out.WriteString(line + "\n")
			}
			continue
		case len(f) == 0:
			continue
		}
		keep = f[0] == "module" || f[0] == "require" || f[0] == "replace"
		inBlock = strings.HasSuffix(strings.TrimSpace(strings.SplitN(line, "//", 2)[0]), "(")
		if keep {
			out.WriteString(line + "\n")
		}
	}
	out.WriteString("\ngo 1.25\n")
	return out.String()
}
