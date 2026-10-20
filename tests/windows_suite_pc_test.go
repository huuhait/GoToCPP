package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/pclock"
	"github.com/huuhait/gocpp/internal/winssh"
)

// Tests of the parts of the Windows suite that only the real PC can answer: the clean-up of old run folders,
// the end of a run's processes, the removal of files, and an interrupted run. They need GOCPP_WINDOWS_SSH
// and skip without it. Each leaves nothing on the PC.

func pcOrSkip(t *testing.T) string {
	t.Helper()
	host := windowsHost()
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host runs this test on the real PC")
	}
	return host
}

// pcLock takes the PC lock of internal/pclock for a test that uses the PC itself.
func pcLock(t *testing.T) {
	t.Helper()
	release, err := pclock.Acquire(t.Context(), pclock.Options{Path: windowsLockPath(), Log: func(s string) { t.Log(s) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}

// psRun runs a PowerShell script on the PC (-EncodedCommand is not needed: the scripts here have no cmd.exe
// operators) and returns what it printed.
func psRun(t *testing.T, r *winssh.Remote, script string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	var res winssh.Result
	var err error
	for try := 0; try < 3; try++ {
		res, err = r.Exec(ctx, 2*time.Minute, nil, `powershell -NoProfile -Command "`+script+`"`)
		if err != nil || res.Code != 255 {
			break
		}
		t.Logf("ssh failed (exit 255, stderr %q), trying again", res.Stderr)
		time.Sleep(3 * time.Second)
	}
	if err != nil || res.TimedOut {
		t.Fatalf("powershell: %v %v\n%s\n%s", err, res.TimedOut, res.Stdout, res.Stderr)
	}
	if res.Code != 0 || len(res.Stderr) > 0 {
		t.Logf("powershell exit %d, stderr: %s", res.Code, res.Stderr)
	}
	return strings.TrimSpace(string(res.Stdout))
}

// runProcs returns the processes of the run id whose program file is below its folder.
func runProcs(t *testing.T, r *winssh.Remote, folder string) int {
	t.Helper()
	out := psRun(t, r, `$p = (Join-Path $env:USERPROFILE ('gocpp-wintest\`+folder+`')) + '\'; @(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($p, [StringComparison]::OrdinalIgnoreCase) }).Count`)
	n, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("process count %q for folder %q", out, folder)
	}
	return n
}

// waitProcs polls until the count of the folder's processes satisfies ok (at most 60 s).
func waitProcs(t *testing.T, r *winssh.Remote, folder string, ok func(int) bool) int {
	t.Helper()
	var n int
	for i := 0; i < 30; i++ {
		if n = runProcs(t, r, folder); ok(n) {
			return n
		}
		time.Sleep(2 * time.Second)
	}
	return n
}

// forceRemove kills the processes under a folder of gocpp-wintest and deletes it: for the clean-up of a test
// that failed half way. The folder name is a literal of the test.
func forceRemove(r *winssh.Remote, folders ...string) {
	for _, f := range folders {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		r.Exec(ctx, 2*time.Minute, nil, `powershell -NoProfile -Command "$p = (Join-Path $env:USERPROFILE ('gocpp-wintest\`+f+`')) + '\'; Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($p, [StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }; Start-Sleep -Seconds 1; Remove-Item -Recurse -Force -LiteralPath (Join-Path $env:USERPROFILE ('gocpp-wintest\`+f+`')) -ErrorAction SilentlyContinue"`)
		cancel()
	}
}

func buildTree(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "tree.exe")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", exe, "./internal/wintest/agent/testdata/tree")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the test program: %v\n%s", err, out)
	}
	return exe
}

// CleanStale removes the old <digits>_<digits> folders and nothing else.
func TestWindowsSuiteCleanStaleOnThePC(t *testing.T) {
	host := pcOrSkip(t)
	pcLock(t)
	r, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	oldDirs := []string{"99999991_1", "x99999991_1", "99999991_1x", "99999991-1", "99999991_1_3", "99999991", "zzold dir_1", "_1", "99999991_"}
	all := append(append([]string(nil), oldDirs...), "99999991_2", "99999991_9", "keep.txt")
	cleanup := func() {
		var names []string
		for _, n := range all {
			names = append(names, "'"+n+"'")
		}
		psRun(t, r, `$d = Join-Path $env:USERPROFILE 'gocpp-wintest'; foreach ($n in `+strings.Join(names, ",")+`) { Remove-Item -Recurse -Force -LiteralPath (Join-Path $d $n) -ErrorAction SilentlyContinue }`)
	}
	defer cleanup()
	var dirs []string
	for _, n := range oldDirs {
		dirs = append(dirs, "'"+n+"'")
	}
	psRun(t, r, `$d = Join-Path $env:USERPROFILE 'gocpp-wintest'; `+
		`foreach ($n in `+strings.Join(dirs, ",")+`) { $p = Join-Path $d $n; New-Item -ItemType Directory -Force -Path $p | Out-Null; Set-Content -LiteralPath (Join-Path $p 'f.txt') -Value x; (Get-Item -LiteralPath $p).LastWriteTime = (Get-Date).AddHours(-30) }; `+
		`$p = Join-Path $d '99999991_2'; New-Item -ItemType Directory -Force -Path $p | Out-Null; `+ // recent
		`foreach ($n in '99999991_9','keep.txt') { $p = Join-Path $d $n; Set-Content -LiteralPath $p -Value x; (Get-Item -LiteralPath $p).LastWriteTime = (Get-Date).AddHours(-30) }`)
	list := func() map[string]bool {
		out := psRun(t, r, `Get-ChildItem -LiteralPath (Join-Path $env:USERPROFILE 'gocpp-wintest') | ForEach-Object { $_.Name }`)
		m := map[string]bool{}
		for _, l := range strings.Split(out, "\n") {
			m[strings.TrimSpace(l)] = true
		}
		return m
	}
	before := list()
	for _, n := range all {
		if !before[n] {
			t.Fatalf("the decoy %q was not made: %v", n, before)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	if err := (sshRemote{r}).CleanStale(ctx, 12*time.Hour); err != nil {
		t.Fatal(err)
	}
	after := list()
	if after["99999991_1"] {
		t.Errorf("the old run folder 99999991_1 was not removed")
	}
	for _, n := range all {
		if n != "99999991_1" && !after[n] {
			t.Errorf("%q was removed: only an old <digits>_<digits> folder may go", n)
		}
	}
	if !after[r.RunID()] {
		t.Errorf("this run's own folder %s was removed", r.RunID())
	}
}

// KillRun ends the processes of its run and no others, also those of a folder whose name only starts like the run id.
func TestWindowsSuiteKillRunOnlyItsOwnOnThePC(t *testing.T) {
	host := pcOrSkip(t)
	pcLock(t)
	tree := buildTree(t)
	a, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	lookalike := a.RunID() + "9" // a folder of another tool or run whose name starts with a's id
	defer forceRemove(a, lookalike)
	ctx := t.Context()
	for _, r := range []*winssh.Remote{a, b} {
		if err := r.Put(ctx, tree, "tree.exe"); err != nil {
			t.Fatal(err)
		}
	}
	// Run a's program, b's program and the look-alike's, each in its own ssh session.
	pa, err := a.Start(ctx, nil, `.\tree.exe sleep`)
	if err != nil {
		t.Fatal(err)
	}
	defer pa.Kill()
	pb, err := b.Start(ctx, nil, `.\tree.exe sleep`)
	if err != nil {
		t.Fatal(err)
	}
	defer pb.Kill()
	if out := psRun(t, b, `New-Item -ItemType Directory -Force -Path (Join-Path $env:USERPROFILE ('gocpp-wintest\`+lookalike+`')) | Out-Null; Copy-Item -LiteralPath .\tree.exe -Destination (Join-Path $env:USERPROFILE ('gocpp-wintest\`+lookalike+`\tree.exe'))`); out != "" {
		t.Logf("look-alike: %s", out)
	}
	pc, err := b.Start(ctx, nil, `cd /d ..\`+lookalike+` && .\tree.exe sleep`)
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Kill()
	for _, f := range []string{a.RunID(), b.RunID(), lookalike} {
		if n := waitProcs(t, b, f, func(n int) bool { return n >= 1 }); n < 1 {
			t.Fatalf("the program of %s did not start", f)
		}
	}
	a.KillRun()
	if n := waitProcs(t, b, a.RunID(), func(n int) bool { return n == 0 }); n != 0 {
		t.Errorf("KillRun left %d processes of its own run", n)
	}
	if n := runProcs(t, b, b.RunID()); n < 1 {
		t.Errorf("a's KillRun ended b's program (%d processes left)", n)
	}
	if n := runProcs(t, b, lookalike); n < 1 {
		t.Errorf("a's KillRun ended the program of %s, a folder that only starts with a's id", lookalike)
	}
	b.KillRun()
	if n := waitProcs(t, b, b.RunID(), func(n int) bool { return n == 0 }); n != 0 {
		t.Errorf("b's KillRun left %d processes", n)
	}
}

// Remove deletes the named files and folders of the run's folder and only those.
func TestWindowsSuiteRemoveOnThePC(t *testing.T) {
	host := pcOrSkip(t)
	pcLock(t)
	r, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	work := t.TempDir()
	for _, n := range []string{"f1.txt", "keep.txt", "b1.tgz"} {
		os.WriteFile(filepath.Join(work, n), []byte("x"), 0o644)
		if err := r.Put(t.Context(), filepath.Join(work, n), n); err != nil {
			t.Fatal(err)
		}
	}
	res, err := r.Exec(t.Context(), time.Minute, nil, `mkdir b1\exe & echo x > b1\exe\1.exe & dir /b`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Stdout), "b1") {
		t.Fatalf("setup: %s", res.Stdout)
	}
	if err := r.Remove(t.Context(), "b1", "b1.tgz", "missing-file", "f1.txt"); err != nil {
		t.Fatal(err)
	}
	res, _ = r.Exec(t.Context(), time.Minute, nil, `dir /b`)
	got := strings.Fields(string(res.Stdout))
	sort.Strings(got)
	if strings.Join(got, " ") != "keep.txt" {
		t.Errorf("left in the run folder: %v, want only keep.txt", got)
	}
	if err := r.Remove(t.Context(), "keep.txt", "../x"); err == nil {
		t.Error("a name with a separator was accepted")
	}
	res, _ = r.Exec(t.Context(), time.Minute, nil, `dir /b keep.txt`)
	if !strings.Contains(string(res.Stdout), "keep.txt") {
		t.Error("a bad name in the list removed the good ones before the check")
	}
}
