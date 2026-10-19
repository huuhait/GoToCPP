package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWindowsLSPBench measures the language server's speed budgets (Phase 11 §2.1) on the real Windows PC (Phase 14 step
// 14.5, ADR-096): tools/lspbench is cross-built, copied to the PC with gocpp.exe, and runs there against the reference
// projects, with the same scenarios, the same statistics and the same gate as `make lsp-bench` on Linux. It is opt-in,
// because it takes minutes and the PC is shared:
//
//	GOCPP_WINDOWS_LSPBENCH=1 GOCPP_WINDOWS_SSH=user@host go test -run TestWindowsLSPBench ./tests
//
// GOCPP_WINDOWS_LSPBENCH_PROJECTS names the projects (default small,medium,large), GOCPP_WINDOWS_LSPBENCH_OUT a
// folder here that gets the report (lsp-windows.json and lsp-windows.md). The test fails when a row fails the gate. The
// PC's clock ticks every half millisecond or so (Go reads the interrupt time), so the figures under a millisecond are
// steps of that size; the budgets are 20 ms and more.
func TestWindowsLSPBench(t *testing.T) {
	if os.Getenv("GOCPP_WINDOWS_LSPBENCH") != "1" {
		t.Skip("GOCPP_WINDOWS_LSPBENCH=1 turns it on: it runs the language server benchmarks on a real Windows PC (minutes)")
	}
	n := nativeRemote(t)
	bin := filepath.Join(t.TempDir(), "lspbench.exe")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, "./tools/lspbench")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build lspbench: %v\n%s", err, out)
	}
	if err := n.r.Put(t.Context(), bin, "lspbench.exe"); err != nil {
		t.Fatal(err)
	}
	projects := os.Getenv("GOCPP_WINDOWS_LSPBENCH_PROJECTS")
	if projects == "" {
		projects = "small,medium,large"
	}
	n.cmd(t, "", nil, `mkdir bench`)
	// The server's environment is the benchmark's own (it adds the cache folders); the toolchain and the phase gate
	// come from the run's environment (nativeRemote). -max-load 0: there is no load average on Windows.
	line := `lspbench.exe -targets gocpp -gocpp ` + quote(n.abs("gocpp.exe")) + ` -projects ` + projects +
		` -runs 3 -max-load 0 -timeout 2m -workdir ` + quote(n.abs("bench")) +
		` -json report.json -md report.md -gate > bench.log 2>&1`
	timeout := 40 * time.Minute
	if d, ok := t.Deadline(); ok && time.Until(d)-deadlineMargin < timeout {
		timeout = time.Until(d) - deadlineMargin
	}
	res, err := n.r.Exec(t.Context(), timeout, n.env, line)
	if err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	for _, f := range []string{"bench.log", "report.json", "report.md"} {
		if err := n.r.Get(t.Context(), f, filepath.Join(local, f)); err != nil && f == "bench.log" {
			t.Fatal(err)
		}
	}
	log, _ := os.ReadFile(filepath.Join(local, "bench.log"))
	md, _ := os.ReadFile(filepath.Join(local, "report.md"))
	t.Logf("lspbench.log:\n%s", strings.TrimSpace(string(log)))
	t.Logf("report:\n%s", md)
	if out := os.Getenv("GOCPP_WINDOWS_LSPBENCH_OUT"); out != "" {
		os.MkdirAll(out, 0o755)
		for f, name := range map[string]string{"report.json": "lsp-windows.json", "report.md": "lsp-windows.md"} {
			if b, err := os.ReadFile(filepath.Join(local, f)); err == nil {
				os.WriteFile(filepath.Join(out, name), b, 0o644)
			}
		}
	}
	if res.TimedOut {
		t.Fatalf("lspbench did not finish in %v", timeout)
	}
	if res.Code != 0 {
		t.Errorf("lspbench exited with %d (a budget row failed the gate, or the run failed): see the log above", res.Code)
	}
}
