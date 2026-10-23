package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
)

// TestMain keeps the test of the harness from downloading anything (the
// repo's rule), like every test package.
func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// maxLoad is the 1-minute load average above which the speed budgets are
// not measured: on a loaded machine every figure is noise, and the repo has
// had its share of flaky timing tests. GOCPP_LSP_MAXLOAD changes it.
func maxLoad() float64 {
	if s := os.Getenv("GOCPP_LSP_MAXLOAD"); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return float64(min(max(runtime.NumCPU()/8, 4), 16))
}

// buildGocpp builds the gocpp binary of this checkout into dir.
func buildGocpp(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "gocpp")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gocpp")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/gocpp: %v\n%s", err, out)
	}
	return bin
}

// TestLSPBudgets is the CI gate of the speed contract (Phase 11 README
// §2.1, ADR-015): it starts the real gocpp language server (and gopls, when
// it is on the PATH) on the small and medium reference projects, runs the
// scripted scenarios, and fails when a budget row is exceeded.
//
// It is made not to flake on a shared machine: a budget is a p95, taken as
// the median of five sessions; a row fails only when it is over three times
// its budget and, when gopls ran in the same test, over 2.5 times gopls's
// own p95 (a loaded machine slows both); a failing measurement is repeated
// once, and fails the test only when the repeat fails too; the test skips
// when the load average is over GOCPP_LSP_MAXLOAD (16 on a big machine) before it
// starts, and reports "inconclusive" instead of failing when the machine
// became busy while it ran.
//
// It is opt-in (GOCPP_LSP_BUDGETS=on, which make lsp-budgets sets): it takes
// a minute and a half on a quiet machine, builds gocpp, and uses gopls when
// it is installed, so it is not part of go test ./... and make ci. The load
// guard stays on when it runs.
func TestLSPBudgets(t *testing.T) {
	if testing.Short() {
		t.Skip("the speed budgets are measured in the long tests")
	}
	if os.Getenv("GOCPP_LSP_BUDGETS") != "on" {
		t.Skip("the speed budgets are measured only on request: GOCPP_LSP_BUDGETS=on go test -run TestLSPBudgets ./tools/lspbench (make lsp-budgets)")
	}
	limit := maxLoad()
	if load := bench.LoadAvg()[0]; load > limit {
		t.Skipf("the load average is %.1f, over %.1f: the figures would be noise (GOCPP_LSP_MAXLOAD changes the limit)", load, limit)
	}
	gocpp := buildGocpp(t, t.TempDir())
	work := t.TempDir()
	targets := []bench.Target{{Name: "gocpp", Ext: ".gp", Command: []string{gocpp, "lsp"}, Env: []string{"GOCPP_GATE_PHASE=15"}}}
	if path, err := exec.LookPath("gopls"); err == nil {
		targets = append(targets, bench.Target{Name: "gopls", Ext: ".go", Command: []string{path},
			Settings: []byte(`{"diagnosticsDelay":"30ms"}`)})
	} else {
		t.Log("gopls is not on the PATH: the rows are held to the absolute budgets only")
	}
	measure := func() []bench.Violation {
		var results []*bench.ProjectResult
		for _, spec := range []bench.Spec{bench.Small, bench.Medium} {
			proj := bench.Generate(spec)
			byTarget := map[string]*bench.ProjectResult{}
			for _, tg := range targets {
				if tg.Name == "gopls" && spec.Name != "small" {
					continue // gopls on the medium project doubles the time of the test
				}
				res, err := bench.RunProject(context.Background(), bench.Config{Runs: 5, Scale: 0.1, WorkDir: work, Timeout: 30 * time.Second}, proj, tg)
				if err != nil {
					t.Fatal(err)
				}
				results = append(results, res)
				byTarget[tg.Name] = res
			}
			// The memory budget is gopls's peak on the same project (bench.MemoryBudgetMet): where
			// gopls did not run, there is no memory verdict.
			if g, o := byTarget["gocpp"], byTarget["gopls"]; o != nil {
				if met, ok := bench.MemoryBudgetMet(g, o); ok && !met {
					t.Errorf("%s: peak memory %.0f MiB is over gopls's %.0f MiB", spec.Name, g.PeakRSS, o.PeakRSS)
				}
			}
		}
		rows := bench.Rows(results)
		for _, r := range rows {
			t.Logf("%-7s %-20s p95 %7.2f ms  budget %4.0f  %s", r.Project, r.Scenario, r.P95, r.Budget, r.Status())
		}
		return bench.Gate(rows, bench.DefaultGate)
	}
	bad := measure()
	if len(bad) == 0 {
		return
	}
	t.Logf("%d rows over the gate; measuring again to be sure", len(bad))
	bad = measure()
	if len(bad) == 0 {
		return
	}
	if load := bench.LoadAvg()[0]; load > 2*limit {
		t.Skipf("inconclusive: the load average went to %.1f during the test (%d rows over the gate: %v)", load, len(bad), bad)
	}
	for _, v := range bad {
		t.Errorf("speed budget: %v", v)
	}
	if t.Failed() {
		fmt.Fprintln(os.Stderr, "the speed budgets are in docs/benchmarks/lsp.md and plans/2026-10-13-gocpp/phase-11-language-server/README.md §2.1")
	}
}

// TestLoadGuard: the limit is a number of at least 4 on any machine.
func TestLoadGuard(t *testing.T) {
	t.Setenv("GOCPP_LSP_MAXLOAD", "")
	if l := maxLoad(); l < 4 || l > 16 {
		t.Errorf("maxLoad() = %v", l)
	}
	t.Setenv("GOCPP_LSP_MAXLOAD", "2.5")
	if l := maxLoad(); l != 2.5 {
		t.Errorf("maxLoad() = %v, want 2.5", l)
	}
}
