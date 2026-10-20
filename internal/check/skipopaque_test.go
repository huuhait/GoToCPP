package check

import (
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// TestSkipOpaque checks the claim behind Config.SkipOpaque over the programs
// of the repository (the gate cases, the examples, tests/run and tests/errors,
// in all about 2,200): a package without C in it or in its imports has an
// empty Opaque, and the diagnostics of every package are the same with the
// pass left out.
// A corpusProgram is a program of the repository to check.
type corpusProgram struct {
	name, dir string
	pattern   string
	tests     bool
}

// corpusPrograms returns the programs of the repository: the gate cases, the
// examples, tests/run and tests/errors, and every standard package (about 2,300).
func corpusPrograms(t *testing.T) []corpusProgram {
	t.Helper()
	var programs []corpusProgram
	for _, tc := range gateCases {
		files := tc.files
		if files == nil {
			files = map[string]string{"main.gp": tc.src}
		}
		programs = append(programs, corpusProgram{"gate/" + tc.name, writeTree(t, files), ".", tc.tests})
	}
	examples, _ := filepath.Glob("../../examples/*/main.gp")
	for _, m := range examples {
		dir, err := filepath.Abs(filepath.Dir(m))
		if err != nil {
			t.Fatal(err)
		}
		programs = append(programs, corpusProgram{"examples/" + filepath.Base(dir), dir, ".", true})
	}
	for _, folder := range []string{"run", "errors"} {
		dir, err := filepath.Abs("../../tests/" + folder)
		if err != nil {
			t.Fatal(err)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.gp"))
		for _, f := range files {
			programs = append(programs, corpusProgram{folder + "/" + filepath.Base(f), dir, f, false})
		}
	}
	// Every standard package (Check leaves the gates off for them, but the passes run).
	var stdDirs []string
	fs.WalkDir(stdlib.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(name, ".gp") && !strings.HasSuffix(name, "_test.gp") {
			if dir := path.Dir(name); dir != "." && !slices.Contains(stdDirs, dir) {
				stdDirs = append(stdDirs, dir)
			}
		}
		return nil
	})
	for _, dir := range stdDirs {
		programs = append(programs, corpusProgram{"std/" + dir, t.TempDir(), dir, false})
	}
	if len(programs) < 100 {
		t.Skipf("only %d programs: the repository's tests are not here", len(programs))
	}
	return programs
}

// loadCorpus loads the program pr (fresh: Check changes the syntax trees).
func loadCorpus(pr corpusProgram) *load.Program {
	cfg := load.Config{Dir: pr.dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64", Tests: pr.tests}
	if filepath.Base(pr.dir) == "win32" {
		cfg.GOOS = "windows"
	}
	return load.Load(cfg, pr.pattern)
}

// TestSkipOpaque checks the claim behind Config.SkipOpaque over the programs
// of the repository (see corpusPrograms): a package without C in it or in its
// imports, and with no error, has an empty Opaque, and the diagnostics of every
// package are the same with the pass left out.
func TestSkipOpaque(t *testing.T) {
	programs := corpusPrograms(t)
	// pkgInfo is what one run knows of a package: whether the pass was left out,
	// whether there is no C in it, and how many opaque expressions the pass found.
	type pkgInfo struct {
		skipped, hasC bool
		opaque        int
	}
	run := func(pr corpusProgram, skip bool) (text string, pkgs map[string]pkgInfo) {
		prog := loadCorpus(pr)
		ccfg := fakeProbeConfig(LastPhase)
		ccfg.SkipOpaque = skip
		r := Check(prog, ccfg)
		pkgs = make(map[string]pkgInfo)
		for _, p := range r.Packages {
			dep, _ := r.Dep(p.Load)
			info := pkgInfo{skipped: p.typesClean, hasC: dep.HasC}
			if p.Opaque != nil {
				info.opaque = len(p.Opaque.exprs) + len(p.Opaque.typeExprs)
			} else if p.Checked() {
				info.opaque = -1
			}
			pkgs[p.Load.ImportPath+" "+p.Load.ForTest] = info
		}
		return errorText(t, prog, r, pr.dir, true), pkgs
	}
	var mu sync.Mutex
	var skippedPkgs, ranPkgs int
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, pr := range programs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			want, pkgs := run(pr, false)
			got, skipped := run(pr, true)
			mu.Lock()
			defer mu.Unlock()
			if got != want {
				t.Errorf("%s: the diagnostics differ with SkipOpaque\nwithout:\n%s\nwith:\n%s", pr.name, want, got)
			}
			for key, sk := range skipped {
				full := pkgs[key]
				switch {
				case sk.opaque < 0:
					t.Errorf("%s: %s: a checked package has no Opaque", pr.name, key)
				case sk.skipped && full.opaque != 0:
					t.Errorf("%s: %s: the pass was left out, but it finds %d opaque expressions", pr.name, key, full.opaque)
				case !sk.skipped && sk.opaque != full.opaque:
					t.Errorf("%s: %s: %d opaque expressions with SkipOpaque, %d without", pr.name, key, sk.opaque, full.opaque)
				case sk.skipped && sk.hasC:
					t.Errorf("%s: %s: the pass was left out for a package with C", pr.name, key)
				}
				if sk.skipped {
					skippedPkgs++
				} else {
					ranPkgs++
				}
			}
		}()
	}
	wg.Wait()
	t.Logf("%d programs, %d packages without the pass, %d with it", len(programs), skippedPkgs, ranPkgs)
	if skippedPkgs < 1000 || ranPkgs < 100 {
		t.Errorf("%d packages without the pass and %d with it: the corpus does not cover both", skippedPkgs, ranPkgs)
	}
}

// TestOptimizationsFocused is a bounded equivalence check, independent of the
// repository-wide audit tests above/below. It covers valid and broken Go-like
// programs, repeated/recursive generic types, and C/header syntax.
func TestOptimizationsFocused(t *testing.T) {
	cases := []string{
		"package main\nfunc main() { println(1) }\n",
		"package main\nfunc main() { x := missing; _ = x.Field }\n",
		"package main\ntype Node[T any] struct { Next *Node[T]; Value T }\nfunc f(x Node[int], y Node[int]) Node[int] { return x }\nfunc main() {}\n",
		"package main\ntype I interface { M() int }\ntype T int\nfunc (T) M() int { return 1 }\nvar x I = T(0)\nfunc main() { println(x.M()) }\n",
		"package main\nfunc main() { m := map[string]chan int{}; _ = m; go func() {}() }\n",
		"package main\nimport \"<vector>\"\nfunc main() { v := std.vector[int]{}; v.push_back(1) }\n",
		"package main\nfunc main() { var n C.int; _ = n }\n",
	}
	defer func() { noTypeUsesMemo = false }()
	for i, src := range cases {
		pr := corpusProgram{name: "focused", dir: writeTree(t, map[string]string{"main.gp": src}), pattern: "."}
		for _, phase := range []int{2, 5, 12, LastPhase} {
			var want string
			for mode := 0; mode < 3; mode++ {
				noTypeUsesMemo = mode == 0
				prog := loadCorpus(pr)
				cfg := fakeProbeConfig(phase)
				cfg.SkipOpaque = mode == 2
				got := errorText(t, prog, Check(prog, cfg), pr.dir, true)
				if mode == 0 {
					want = got
				} else if got != want {
					t.Errorf("case %d phase %d mode %d changed diagnostics\nfull:\n%s\noptimized:\n%s", i, phase, mode, want, got)
				}
			}
		}
	}
}

// TestGateTypeMemo checks that remembering what the walk of a type asks the
// gate (typeUsesOnce) changes no gate error: at phases from 2 to 15 the
// diagnostics of the programs of the repository are the same with and without
// the memo.
func TestGateTypeMemo(t *testing.T) {
	programs := corpusPrograms(t)
	for _, phase := range []int{2, 4, 5, 8, 12, LastPhase} {
		results := make([][2]string, len(programs))
		for i, memo := range []bool{true, false} {
			noTypeUsesMemo = !memo
			var wg sync.WaitGroup
			sem := make(chan struct{}, 6)
			for j, pr := range programs {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					prog := loadCorpus(pr)
					r := Check(prog, fakeProbeConfig(phase))
					results[j][i] = errorText(t, prog, r, pr.dir, true)
				}()
			}
			wg.Wait()
		}
		noTypeUsesMemo = false
		gates := 0
		for j, pr := range programs {
			if results[j][0] != results[j][1] {
				t.Errorf("phase %d, %s: the gate errors differ with the memo\nwith:\n%s\nwithout:\n%s", phase, pr.name, results[j][0], results[j][1])
			}
			gates += strings.Count(results[j][0], "(Phase ")
		}
		if phase < 8 && gates < 200 {
			t.Errorf("phase %d: only %d gate errors in the corpus", phase, gates)
		}
	}
}
