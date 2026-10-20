package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
	"github.com/huuhait/gocpp/internal/wintest"
)

// TestWindowsSuiteAgent runs the suite's agent on the real PC (GOCPP_WINDOWS_SSH; it skips without it) through the
// whole driver, with programs that misbehave: a hung program with a child (the job object ends the tree), a
// program that leaves a child behind, one that prints without end, an exit code, and the TEMP and working
// folders. It then asks the PC whether any process of the run is left and whether the run's files are gone.
func TestWindowsSuiteAgent(t *testing.T) {
	host := windowsHost()
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host runs the agent on the real PC")
	}
	work := t.TempDir()
	agentExe, err := buildAgent(t.Context(), work)
	if err != nil {
		t.Fatal(err)
	}
	progs := filepath.Join(work, "tree.exe")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", progs, "./internal/wintest/agent/testdata/tree")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the test program: %v\n%s", err, out)
	}
	r, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	lock := &wintest.FileLock{Path: windowsLockPath(), Wait: 3 * time.Hour, Log: func(s string) { t.Log(s) }}
	var items []wintest.Item
	for _, mode := range []string{"hang", "leak", "flood", "exit7", "tmp"} {
		items = append(items, wintest.Item{Name: "agent/" + mode, Group: "agent", Args: []string{mode}, Key: mode})
	}
	items[0].Timeout = 3 * time.Second // hang
	results := filepath.Join(work, "results.json")
	cfg := wintest.Config{
		Remote: sshRemote{r}, Lock: lock, AgentExe: agentExe, Work: work, Jobs: 5, BuildJobs: 5, BatchSize: 10,
		Timeout: time.Minute, MaxOutput: 64 << 10, ResultsPath: results, Generation: "agent-test", Target: "real PC",
		Log: func(s string) { t.Log(s) },
		Build: func(context.Context, wintest.Item) (string, error) {
			// the driver deletes a packed program: give each item its own copy
			p := filepath.Join(work, fmt.Sprintf("p%d.exe", time.Now().UnixNano()))
			data, err := os.ReadFile(progs)
			if err != nil {
				return "", err
			}
			return p, os.WriteFile(p, data, 0o755)
		},
		Check: func(it wintest.Item, o wintest.Outcome) []string {
			switch it.Args[0] {
			case "leak":
				if string(o.Stdout) != "started\n" || o.Exit != 0 {
					return []string{fmt.Sprintf("leak: %q exit %d", o.Stdout, o.Exit)}
				}
			case "exit7":
				if o.Exit != 7 {
					return []string{fmt.Sprintf("exit %d, want 7", o.Exit)}
				}
			case "tmp":
				lines := strings.Split(strings.TrimSpace(string(o.Stdout)), "\n")
				if len(lines) != 2 || !strings.Contains(lines[0], `\tmp\`) || !strings.Contains(lines[1], `\work\`) || !strings.Contains(lines[0], r.RunID()) {
					return []string{fmt.Sprintf("TEMP and working folder: %q", o.Stdout)}
				}
			}
			return nil
		},
	}
	start := time.Now()
	rep, err := wintest.New(cfg).Run(t.Context(), items)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Minute {
		t.Errorf("the run took %v", time.Since(start))
	}
	want := map[string]wintest.Status{"agent/hang": wintest.Fail, "agent/flood": wintest.Fail, "agent/leak": wintest.Pass, "agent/exit7": wintest.Pass, "agent/tmp": wintest.Pass}
	for name, st := range want {
		e := rep.Results.Tests[name]
		if e == nil || e.Status != st {
			t.Errorf("%s: %+v, want %s", name, e, st)
		}
	}
	if e := rep.Results.Tests["agent/hang"]; e != nil && !strings.Contains(e.Reason, "timed out after 3s") {
		t.Errorf("hang: %s", e.Reason)
	}
	if e := rep.Results.Tests["agent/flood"]; e != nil && !strings.Contains(e.Reason, "printed more than") {
		t.Errorf("flood: %s", e.Reason)
	}
	// Nothing of the run is left: no process (the hung program, its child and the leaked child) and no file.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	res, err := r.Exec(ctx, time.Minute, nil, `powershell -NoProfile -Command "@(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -like '*gocpp-wintest\`+r.RunID()+`*' }).Count"`)
	if err != nil || strings.TrimSpace(string(res.Stdout)) != "0" {
		t.Errorf("processes of the run left: %v %q %q", err, res.Stdout, res.Stderr)
	}
	res, err = r.Exec(ctx, time.Minute, nil, `dir /b`)
	if err != nil || strings.TrimSpace(string(res.Stdout)) != wintest.AgentName {
		t.Errorf("files of the run left: %v %q", err, res.Stdout)
	}
}
