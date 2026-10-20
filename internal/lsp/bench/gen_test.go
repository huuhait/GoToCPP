package bench

import (
	"bytes"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

func TestMain(m *testing.M) {
	// No module downloads in tests (the repo's test rule).
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// The same seed gives byte-identical trees; another seed does not.
func TestGenerateDeterministic(t *testing.T) {
	for _, spec := range Specs() {
		a, b := Generate(spec), Generate(spec)
		if len(a.Files) != len(b.Files) || a.Hot != b.Hot || a.Lines != b.Lines {
			t.Fatalf("%s: two runs differ in shape", spec.Name)
		}
		for i := range a.Files {
			if a.Files[i] != b.Files[i] {
				t.Fatalf("%s: file %s differs between two runs", spec.Name, a.Files[i].Rel)
			}
		}
		other := spec
		other.Seed += 100
		c := Generate(other)
		same := len(c.Files) == len(a.Files)
		for i := 0; same && i < len(a.Files); i++ {
			same = a.Files[i] == c.Files[i]
		}
		if same {
			t.Errorf("%s: seed %d and %d give the same tree", spec.Name, spec.Seed, other.Seed)
		}
	}
	// Written twice, the trees are byte-identical on disk too.
	d1, d2 := t.TempDir(), t.TempDir()
	for _, dir := range []string{d1, d2} {
		if err := Generate(Small).Write(dir, ".gp"); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range Generate(Small).Files {
		x, _ := os.ReadFile(filepath.Join(d1, filepath.FromSlash(f.Rel)+".gp"))
		y, _ := os.ReadFile(filepath.Join(d2, filepath.FromSlash(f.Rel)+".gp"))
		if len(x) == 0 || !bytes.Equal(x, y) {
			t.Errorf("%s differs on disk", f.Rel)
		}
	}
}

// The sizes of the reference projects: what the plan says (files and
// packages), and the lines of the large one near the 50k of the budgets.
func TestGenerateSizes(t *testing.T) {
	count := func(p *Project) (files, pkgs int) {
		dirs := map[string]bool{}
		for _, f := range p.Files {
			if f.Kind == Source {
				files++
				dirs[filepath.Dir(f.Rel)] = true
			}
		}
		return files, len(dirs)
	}
	for _, c := range []struct {
		spec                 Spec
		files, pkgs          int
		minLines, maxLines   int
		hotImports, hotUsers bool
	}{
		{Small, 3*6 + 2 + 1 + 1, 3 + 2 + 1, 800, 5000, true, true},
		{Medium, 20*10 + 2 + 1 + 1, 20 + 2 + 1, 8000, 40000, true, true},
		{Large, 100*15 + 2 + 1 + 1, 100 + 2 + 1, 50000, 100000, true, true},
	} {
		p := Generate(c.spec)
		files, pkgs := count(p)
		if files != c.files || pkgs != c.pkgs {
			t.Errorf("%s: %d files in %d packages, want %d in %d", c.spec.Name, files, pkgs, c.files, c.pkgs)
		}
		if p.Lines < c.minLines || p.Lines > c.maxLines {
			t.Errorf("%s: %d lines, want %d to %d", c.spec.Name, p.Lines, c.minLines, c.maxLines)
		}
		for _, rel := range []string{p.Hot, p.Dep, p.Core, p.Dependent} {
			if p.Source(rel) == "" {
				t.Errorf("%s: no file %q", c.spec.Name, rel)
			}
		}
		if p.Hot == p.Dep || filepath.Dir(p.Hot) == filepath.Dir(p.Dependent) || filepath.Dir(p.Hot) == filepath.Dir(p.Dep) {
			t.Errorf("%s: hot %s, dep %s, dependent %s are not in three packages", c.spec.Name, p.Hot, p.Dep, p.Dependent)
		}
		// The Dependent imports Hot's package.
		if !strings.Contains(p.Source(p.Dependent), "/"+filepath.Dir(p.Hot)+"\"") && !dependsOn(p, p.Dependent, filepath.Dir(p.Hot)) {
			t.Errorf("%s: %s does not import %s", c.spec.Name, p.Dependent, filepath.Dir(p.Hot))
		}
		t.Logf("%s: %d files, %d packages, %d lines", c.spec.Name, files, pkgs, p.Lines)
	}
	// The files the scenarios search must be in every file of every project.
	for _, spec := range []Spec{Small, Medium, Large} {
		p := Generate(spec)
		for _, rel := range p.SourceFiles(true) {
			if strings.HasPrefix(rel, "pkg") {
				for _, needle := range AnchorNeedles {
					if !strings.Contains(p.Source(rel), needle) {
						t.Fatalf("%s: %s has no %q", spec.Name, rel, needle)
					}
				}
			}
		}
	}
	// The long file of the format budget has about a thousand lines.
	if n := strings.Count(Generate(Small).Source(Generate(Small).Big), "\n"); n < 1000 || n > 1300 {
		t.Errorf("the big file has %d lines", n)
	}
	cp := Generate(Cpp)
	if cp.Typo == "" || cp.Source(cp.Typo) == "" {
		t.Errorf("cpp: no typo file")
	}
	// The macro project: every file of its macro packages uses the forms the scenarios look for, the
	// Go copy has none of them, and the other projects have no macro file.
	mp := Generate(Macro)
	src := mp.Source(mp.MacroFile)
	for _, needle := range []string{"#mac00.Assert(", "#mac00.Dbg(", "#mac00.Trace()", "#mac00.Count(", "#Derive(Equal", "strconv.Atoi!(a)", "os.Open!(path)", "f.Stat!()", "\tdata := os.ReadFile(path)", "\ttotal := #mac00.Dbg("} {
		if !strings.Contains(src, needle) {
			t.Errorf("macro: %s has no %q", mp.MacroFile, needle)
		}
	}
	for _, rel := range mp.SourceFiles(true) {
		if strings.HasPrefix(rel, "mac") {
			t.Errorf("macro: %s is in the Go copy", rel)
		}
	}
	if Generate(Small).MacroFile != "" {
		t.Errorf("small has a macro file")
	}
	t.Logf("macro: %d lines", mp.Lines)
}

// dependsOn reports whether any file of the package of rel imports the
// package dir.
func dependsOn(p *Project, rel, dir string) bool {
	pkg := filepath.Dir(rel)
	for _, f := range p.Files {
		if f.Kind == Source && filepath.Dir(f.Rel) == pkg && strings.Contains(f.Content, "/"+dir+"\"") {
			return true
		}
	}
	return false
}

// The generated code is gofmt-clean (so that formatting it changes nothing).
func TestGenerateGofmt(t *testing.T) {
	for _, spec := range []Spec{Small, Medium} {
		for _, f := range Generate(spec).Files {
			if f.Kind != Source || isCppPath(f.Rel) {
				continue
			}
			out, err := format.Source([]byte(f.Content))
			if err != nil {
				t.Fatalf("%s: %v", f.Rel, err)
			}
			if string(out) != f.Content {
				t.Errorf("%s is not gofmt-clean", f.Rel)
			}
		}
	}
}

// The projects type-check with gocpp check (gates open), with no
// diagnostics at all.
func TestGenerateChecksWithGocpp(t *testing.T) {
	for _, spec := range []Spec{Small, Medium, Macro} {
		dir := t.TempDir()
		if err := Generate(spec).Write(dir, ".gp"); err != nil {
			t.Fatal(err)
		}
		var patterns []string
		seen := map[string]bool{}
		for _, f := range Generate(spec).Files {
			if d := filepath.Dir(f.Rel); f.Kind == Source && !seen[d] {
				seen[d] = true
				patterns = append(patterns, "./"+d)
			}
		}
		prog := load.Load(load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64", AllErrors: true}, patterns...)
		r := check.Check(prog, check.Config{Phase: check.LastPhase, GOOS: "linux", GOARCH: "amd64"})
		var out strings.Builder
		pr := diag.Printer{All: true}
		lists := []*diag.List{prog.Errors}
		for _, cp := range r.Packages {
			lists = append(lists, cp.Errors)
		}
		for _, l := range lists {
			if err := pr.Print(&out, l); err != nil {
				t.Fatal(err)
			}
		}
		if out.Len() > 0 {
			t.Errorf("%s: gocpp check says:\n%.2000s", spec.Name, out.String())
		}
	}
}

// And with the Go toolchain (where it is Go: not the C++ packages): the
// same source is what gopls reads.
func TestGenerateBuildsWithGo(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool")
	}
	for _, spec := range []Spec{Small, Medium, Cpp, Macro} {
		dir := t.TempDir()
		if err := Generate(spec).Write(dir, ".go"); err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == "windows" {
			t.Skip()
		}
		cmd := exec.Command(goTool, "vet", "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOCACHE="+filepath.Join(t.TempDir(), "gocache"))
		done := make(chan struct{})
		var out []byte
		go func() { out, err = cmd.CombinedOutput(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Minute):
			cmd.Process.Kill()
			t.Fatalf("%s: go vet takes too long", spec.Name)
		}
		if err != nil {
			t.Errorf("%s: go vet: %v\n%.2000s", spec.Name, err, out)
		}
	}
}
