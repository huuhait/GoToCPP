package icheck

import (
	"context"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
	"github.com/huuhait/gocpp/stdlib"
)

// TestAcceptCorpus is the compatibility rule: every program of the repository
// that imports C/C++ headers and that opaque mode compiles (the interop tests, the
// header zoo, the examples, the multi-package modules) gets no error and no
// warning from the checker. A false error here is a program that typed mode would
// reject and opaque mode runs.

var (
	headerImport = regexp.MustCompile(`"(<[^">]+>|\./[^"]*\.(h|hpp|hh|hxx))"`)
	notAccepted  = regexp.MustCompile(`(?m)^// ERROR|// ERROR "|ExitCode: [1-9]|^// pending|^// WARNING`)
)

type program struct {
	name string // for the test name
	dir  string
	arg  string // a file name, or "."
}

func repoRoot(t testing.TB) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// corpus lists the programs of the repository that opaque mode accepts and
// that use headers.
func corpus(t testing.TB) []program {
	root := repoRoot(t)
	var out []program
	for _, d := range []string{"tests/interop", "tests/zoo", "tests/run", "tests/golden"} {
		files, err := filepath.Glob(filepath.Join(root, d, "*.gp"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if notAccepted.Match(b) || !headerImport.Match(b) {
				continue
			}
			out = append(out, program{d + "/" + filepath.Base(f), filepath.Join(root, d), filepath.Base(f)})
		}
	}
	var dirs []string
	for _, pat := range []string{"tests/interop/*", "examples/*", "tests/pkgs/*"} {
		m, err := filepath.Glob(filepath.Join(root, pat))
		if err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, m...)
	}
	for _, d := range dirs {
		st, err := os.Stat(d)
		if err != nil || !st.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, "main.gp")); err != nil {
			if _, err := os.Stat(filepath.Join(d, "gocpp.mod")); err != nil {
				continue
			}
		}
		bad := false
		any := false
		_ = filepath.WalkDir(d, func(p string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() || !strings.HasSuffix(p, ".gp") {
				return nil
			}
			b, _ := os.ReadFile(p)
			bad = bad || notAccepted.Match(b)
			any = any || headerImport.Match(b)
			return nil
		})
		if bad || !any {
			continue
		}
		rel, _ := filepath.Rel(root, d)
		out = append(out, program{rel, d, "."})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// checkAll loads the program and runs the opaque check, returning the packages
// that are not standard ones, or nil with the first error when it does not check.
func checkAll(t testing.TB, pr program) ([]*check.Package, *token.FileSet, string) {
	cxx := tsupport.CXX(t)
	cfg := load.Config{Dir: pr.dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	prog := load.Load(cfg, pr.arg)
	if prog.Errors.HasErrors() {
		return nil, nil, prog.Errors.Diagnostics()[0].Msg
	}
	r := check.Check(prog, check.Config{Phase: check.LastPhase, CXX: func(context.Context) (build.CXX, error) { return cxx, nil }})
	var pkgs []*check.Package
	for _, p := range r.Packages {
		if p.Load.Std {
			continue
		}
		if !p.Checked() || p.Errors.HasErrors() {
			msg := "did not check"
			if ds := p.Errors.Diagnostics(); len(ds) > 0 {
				msg = ds[0].Msg
			}
			return nil, nil, msg
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, prog.Fset, ""
}

func TestAcceptCorpus(t *testing.T) {
	clang := tsupport.Clang(t)
	progs := corpus(t)
	var checked, skipped, compiled, uses, silent atomic.Int64
	t.Run("programs", func(t *testing.T) {
		for _, pr := range progs {
			t.Run(pr.name, func(t *testing.T) {
				t.Parallel()
				pkgs, fset, why := checkAll(t, pr)
				if pkgs == nil {
					skipped.Add(1)
					t.Skipf("the opaque check rejects it here (%s)", why)
				}
				for _, p := range pkgs {
					res, err := Check(context.Background(), p, fset, Options{
						Analyze: typedinterop.Options{Clang: clang, Runner: new(probe.Runner)},
						Build:   build.Clang,
					})
					if err != nil {
						t.Fatalf("Check: %v", err)
					}
					if res.Skipped != "" && !strings.Contains(res.Skipped, "no C/C++") {
						t.Logf("%s: skipped: %s", p.Load.ImportPath, res.Skipped)
					}
					for _, d := range res.Diags {
						t.Errorf("%s:%d:%d: %s", d.Pos.Filename, d.Pos.Line, d.Pos.Column, d.Msg)
					}
					compiled.Add(int64(res.Stats.Compiled))
					uses.Add(int64(res.Stats.Sites))
					silent.Add(int64(len(res.Silent)))
				}
				checked.Add(1)
			})
		}
	})
	if t.Failed() {
		return
	}
	t.Logf("%d programs checked, %d skipped; %d probes compiled, %d uses, %d left to the compiler", checked.Load(), skipped.Load(), compiled.Load(), uses.Load(), silent.Load())
	if checked.Load() < 240 || compiled.Load() < 3500 || uses.Load() < 5000 {
		t.Errorf("the corpus is too small to mean anything: %d programs, %d probes, %d uses", checked.Load(), compiled.Load(), uses.Load())
	}
}
