package vet

import (
	"context"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"text/scanner"

	"golang.org/x/tools/txtar"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

func TestMain(m *testing.M) {
	// The tests must not download modules or read the user's cache.
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// A program is a loaded, checked program for the tests.
type program struct {
	dir  string
	prog *load.Program
	res  *check.Result
}

// fakeProbeConfig is check's test configuration: no C++ compiler runs, every
// name that looks like a namespace is one and every header is fine (the
// tests use headers like <lib.h> that don't exist).
func fakeProbeConfig() check.Config {
	return check.Config{
		Phase: check.LastPhase,
		CXX:   func(context.Context) (build.CXX, error) { return build.CXX{}, nil },
		Probe: func(_ context.Context, _ build.CXX, _ string, _, _, candidates []string) (interop.Result, error) {
			kinds := make(map[string]interop.Kind)
			for _, c := range candidates {
				kinds[c] = interop.Namespace
			}
			return interop.Result{Kinds: kinds}, nil
		},
	}
}

// loadProgram loads and checks the patterns of the files. It fails the test
// when there is an error that isn't a phase gate.
func loadProgram(t testing.TB, files map[string]string, patterns ...string) *program {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	all := map[string]string{"gocpp.mod": "module example.com/m\n"}
	for k, v := range files {
		all[k] = v
	}
	for name, src := range all {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return loadDir(t, root, load.Config{}, patterns...)
}

func loadDir(t testing.TB, root string, cfg load.Config, patterns ...string) *program {
	t.Helper()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	cfg.Dir, cfg.Stdlib, cfg.GoStd = root, stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	cfg.AllErrors = true
	cfg.Tests = true         // like gocpp vet
	cfg.Phase10Syntax = true // the gate phase is LastPhase (fakeProbeConfig)
	prog := load.Load(cfg, patterns...)
	res := check.Check(prog, fakeProbeConfig())
	p := &program{dir: root, prog: prog, res: res}
	var errs []string
	for _, d := range prog.Errors.Diagnostics() {
		errs = append(errs, d.Pos.String()+": "+d.Msg)
	}
	for _, lp := range prog.Packages {
		cp := res.Package(lp)
		if cp == nil {
			continue
		}
		for _, d := range cp.Errors.Diagnostics() {
			if d.Severity == diag.Error && !d.Gate {
				errs = append(errs, d.Pos.String()+": "+d.Msg)
			}
		}
	}
	if len(errs) > 0 {
		t.Fatalf("the program has errors:\n%s", strings.Join(errs, "\n"))
	}
	return p
}

// vet runs the analyzers (by name; none means all) on the root packages.
func (p *program) vet(t testing.TB, names ...string) *Report {
	t.Helper()
	var as []*Analyzer
	if len(names) == 0 {
		as = Analyzers
	}
	for _, n := range names {
		a := Lookup(n)
		if a == nil {
			t.Fatalf("no analyzer %q", n)
		}
		as = append(as, a)
	}
	rep, err := Run(p.prog, p.res, Config{Roots: p.prog.Roots, Analyzers: as})
	if err != nil {
		t.Fatal(err)
	}
	if c := rep.Crashed(); len(c) > 0 {
		t.Fatalf("analyzers crashed: %+v", c)
	}
	return rep
}

// A want is an expectation of a `// want "regexp"` comment.
type want struct {
	file string
	line int
	re   *regexp.Regexp
	used bool
}

// parseWants reads the `// want "re1" "re2"` comments of a file, like
// analysistest does: each regexp must match the message of one finding on
// the line. A regexp can be in "..." or `...`.
func parseWants(t *testing.T, name, text string) []*want {
	var out []*want
	for i, line := range strings.Split(text, "\n") {
		_, rest, ok := strings.Cut(line, "// want ")
		if !ok {
			continue
		}
		var s scanner.Scanner
		s.Init(strings.NewReader(rest))
		s.Error = func(*scanner.Scanner, string) {}
		for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
			if tok != scanner.String && tok != scanner.RawString {
				t.Fatalf("%s:%d: want: expected a string, got %q", name, i+1, s.TokenText())
			}
			pat, err := strconv.Unquote(s.TokenText())
			if err != nil {
				t.Fatalf("%s:%d: want: %v", name, i+1, err)
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				t.Fatalf("%s:%d: want: %v", name, i+1, err)
			}
			out = append(out, &want{file: name, line: i + 1, re: re})
		}
	}
	return out
}

// expect checks the findings of the roots against the want comments of
// their files: every want is matched, and every finding is wanted. Only the
// files under the folder prefix count ("" is all).
func (p *program) expect(t *testing.T, rep *Report, prefix string) {
	t.Helper()
	var wants []*want
	for _, lp := range p.prog.Roots {
		for _, f := range lp.Files {
			if !p.under(f.Path, prefix) {
				continue
			}
			text, err := os.ReadFile(f.Path)
			if err != nil {
				t.Fatal(err)
			}
			wants = append(wants, parseWants(t, f.Path, string(text))...)
		}
	}
	for _, f := range rep.Findings {
		if !p.under(f.Pos.Filename, prefix) {
			continue
		}
		matched := false
		for _, w := range wants {
			if !w.used && w.file == f.Pos.Filename && w.line == f.Pos.Line && w.re.MatchString(f.Message) {
				w.used, matched = true, true
				break
			}
		}
		if !matched {
			t.Errorf("unexpected finding: %s: %s [%s]", p.short(f.Pos), f.Message, f.Analyzer)
		}
	}
	for _, w := range wants {
		if !w.used {
			t.Errorf("%s:%d: no finding matches %q", p.short(token.Position{Filename: w.file}), w.line, w.re)
		}
	}
}

// under reports whether the file is in the folder prefix of the program.
func (p *program) under(file, prefix string) bool {
	rel, err := filepath.Rel(p.dir, file)
	return err == nil && strings.HasPrefix(filepath.ToSlash(rel), prefix)
}

func (p *program) short(pos token.Position) string {
	rel, err := filepath.Rel(p.dir, pos.Filename)
	if err != nil {
		rel = pos.Filename
	}
	pos.Filename = rel
	return pos.String()
}

// An archive of testdata is a set of cases: the files of one folder each
// ("case_name/main.gp", "case_name/lib/lib.gp"), loaded together as one
// module (gocpp.mod is added). The comment of the archive has "key: value"
// lines: analyzers (a comma-separated list, default all), skip (the reason
// the archive is not run yet). Every case is a subtest: the findings under
// its folder must be exactly its want comments.
type archive struct {
	name      string
	files     map[string]string
	cases     []string // the first path elements, sorted
	patterns  []string // the folders with .gp files
	analyzers []string
	skip      string
}

func readArchive(t *testing.T, file string) archive {
	t.Helper()
	ar, err := txtar.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	a := archive{name: strings.TrimSuffix(filepath.Base(file), ".txtar"), files: make(map[string]string)}
	for _, line := range strings.Split(string(ar.Comment), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "analyzers":
			a.analyzers = strings.Split(val, ",")
		case "skip":
			a.skip = val
		}
	}
	seen, dirs := make(map[string]bool), make(map[string]bool)
	for _, f := range ar.Files {
		a.files[f.Name] = string(f.Data)
		if dir := path.Dir(f.Name); strings.HasSuffix(f.Name, ".gp") && !dirs[dir] {
			dirs[dir] = true
			a.patterns = append(a.patterns, "./"+dir)
		}
		if c, _, ok := strings.Cut(f.Name, "/"); ok && !seen[c] {
			seen[c] = true
			a.cases = append(a.cases, c)
		}
	}
	slices.Sort(a.cases)
	slices.Sort(a.patterns)
	return a
}

// TestTestdata runs every archive of testdata.
func TestTestdata(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < minArchives {
		t.Fatalf("only %d archives in testdata", len(paths))
	}
	slices.Sort(paths)
	cases := 0
	for _, path := range paths {
		a := readArchive(t, path)
		if a.skip != "" {
			t.Run(a.name, func(t *testing.T) { t.Skip(a.skip) })
			continue
		}
		cases += len(a.cases)
		t.Run(a.name, func(t *testing.T) {
			p := loadProgram(t, a.files, a.patterns...)
			rep := p.vet(t, a.analyzers...)
			if os.Getenv("VET_SHOW") != "" { // for writing a case: what is found
				for _, f := range rep.Findings {
					t.Logf("%s: %s [%s]", p.short(f.Pos), f.Message, f.Analyzer)
				}
			}
			for _, c := range a.cases {
				t.Run(c, func(t *testing.T) { p.expect(t, rep, c+"/") })
			}
		})
	}
	if cases < minCases {
		t.Errorf("only %d cases in testdata, want at least %d", cases, minCases)
	}
}

// minArchives and minCases guard against testdata that went missing.
const (
	minArchives = 1
	minCases    = 1
)
