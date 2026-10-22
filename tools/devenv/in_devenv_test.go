package devenv

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The fake nix: `print-dev-env` appends a line to $FAKE_NIX_LOG, makes the --profile link, and
// prints a bash environment; `develop` appends a line and runs the command after -c. Set
// FAKE_NIX_FAIL=1 to make print-dev-env fail, FAKE_NIX_ODD=1 to write the temporary folder line
// in a form the script does not know.
const fakeNix = `#!/bin/sh
case "$1" in
print-dev-env)
	echo print-dev-env >>"$FAKE_NIX_LOG"
	[ -n "$FAKE_NIX_FAIL" ] && { echo "fake nix: evaluation failed" >&2; exit 1; }
	profile=
	while [ $# -gt 0 ]; do
		[ "$1" = "--profile" ] && profile=$2
		shift
	done
	sleep 0.2
	ln -sf "$FAKE_NIX_LOG" "$profile"
	echo 'export FAKE_ENV=cached'
	echo "export EVAL_CWD='$PWD'"
	if [ -n "$FAKE_NIX_ODD" ]; then
		echo 'export NIX_BUILD_TOP="$(mktemp -d -t nix-shell.XXXXXX)" # odd'
	else
		echo 'export NIX_BUILD_TOP="$(mktemp -d -t nix-shell.XXXXXX)"'
	fi
	echo 'export TMPDIR="$NIX_BUILD_TOP"'
	;;
develop)
	echo develop >>"$FAKE_NIX_LOG"
	while [ "$1" != "-c" ]; do shift; done
	shift
	FAKE_ENV=fallback exec "$@"
	;;
esac
`

// sandbox is a copy of the files the script needs (it finds the flake files from its own
// place), a fake nix first in PATH, and a private cache and temporary folder.
type sandbox struct {
	t     *testing.T
	root  string
	log   string
	cache string
	tmp   string
	env   []string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("scripts/in-devenv is for Linux with Nix")
	}
	for _, tool := range []string{"bash", "flock", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	box := &sandbox{t: t, root: filepath.Join(dir, "repo")}
	box.log = filepath.Join(dir, "nix.log")
	box.cache = filepath.Join(dir, "cache")
	box.tmp = filepath.Join(dir, "tmp")

	folders := []string{
		filepath.Join(box.root, "scripts"),
		box.tmp,
		filepath.Join(dir, "bin"),
	}
	for _, folder := range folders {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join("scripts", "in-devenv")
	copyFile(t, filepath.Join(repo, script), filepath.Join(box.root, script), 0o755)
	copyFile(t, filepath.Join(repo, "flake.nix"), filepath.Join(box.root, "flake.nix"), 0o644)
	copyFile(t, filepath.Join(repo, "flake.lock"), filepath.Join(box.root, "flake.lock"), 0o644)
	writeFile(t, filepath.Join(dir, "bin", "nix"), fakeNix, 0o755)
	writeFile(t, box.log, "", 0o644)

	box.env = append(os.Environ(),
		"PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOCPP_DEVENV_DIR="+box.cache,
		"FAKE_NIX_LOG="+box.log,
		"TMPDIR="+box.tmp,
		"IN_NIX_SHELL=",
		"GOCPP_IN_DEVENV=",
		"FAKE_NIX_FAIL=",
		"FAKE_NIX_ODD=",
		"GOCPP_DEVENV_KEEP_DAYS=30",
	)
	return box
}

func copyFile(t *testing.T, from, to string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, to, string(data), mode)
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// run starts scripts/in-devenv with args and returns stdout, stderr and the exit status.
func (box *sandbox) run(extraEnv []string, args ...string) (stdout, stderr string, status int) {
	box.t.Helper()
	cmd := exec.Command(filepath.Join(box.root, "scripts", "in-devenv"), args...)
	cmd.Dir = box.root
	cmd.Env = append(append([]string{}, box.env...), extraEnv...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out.String(), errOut.String(), exitErr.ExitCode()
	}
	if err != nil {
		box.t.Fatal(err)
	}
	return out.String(), errOut.String(), 0
}

// key is the cache key for the flake files as they are now.
func (box *sandbox) key() string {
	box.t.Helper()
	out, errOut, status := box.run(nil, "--key")
	if status != 0 {
		box.t.Fatalf("--key failed (%d): %s", status, errOut)
	}
	return strings.TrimSpace(out)
}

// count returns how many times the fake nix was called with the subcommand.
func (box *sandbox) count(subcommand string) int {
	box.t.Helper()
	data, err := os.ReadFile(box.log)
	if err != nil {
		box.t.Fatal(err)
	}
	return strings.Count(string(data), subcommand+"\n")
}

func (box *sandbox) append(name, text string) {
	box.t.Helper()
	file, err := os.OpenFile(filepath.Join(box.root, name), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		box.t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		box.t.Fatal(err)
	}
}

func TestKeyChangesWithFlakeLock(t *testing.T) {
	box := newSandbox(t)
	before := box.key()

	box.append("flake.lock", "\n")
	after := box.key()

	if before == after {
		t.Fatalf("the key %s did not change when flake.lock changed", before)
	}
}

func TestKeyChangesWithFlakeNix(t *testing.T) {
	box := newSandbox(t)
	before := box.key()

	box.append("flake.nix", "\n# changed\n")
	after := box.key()

	if before == after {
		t.Fatalf("the key %s did not change when flake.nix changed", before)
	}
}

func TestKeyIsStableForSameFiles(t *testing.T) {
	box := newSandbox(t)

	first := box.key()
	second := box.key()

	if first != second || len(first) != 32 {
		t.Fatalf("keys %q and %q: want the same 32 hex digits", first, second)
	}
}

func TestEnvironmentIsMadeOnceAndReused(t *testing.T) {
	box := newSandbox(t)

	for i := 0; i < 3; i++ {
		out, errOut, status := box.run(nil, "sh", "-c", "echo $FAKE_ENV")
		if status != 0 || strings.TrimSpace(out) != "cached" {
			t.Fatalf("run %d: status %d, stdout %q, stderr %q: want the cached environment",
				i, status, out, errOut)
		}
	}

	if got := box.count("print-dev-env"); got != 1 {
		t.Fatalf("print-dev-env ran %d times, want 1", got)
	}
	if got := box.count("develop"); got != 0 {
		t.Fatalf("nix develop ran %d times, want 0", got)
	}
}

func TestEnvironmentIsMadeAgainWhenFlakeLockChanges(t *testing.T) {
	box := newSandbox(t)
	box.run(nil, "true")

	box.append("flake.lock", "\n")
	box.run(nil, "true")

	if got := box.count("print-dev-env"); got != 2 {
		t.Fatalf("print-dev-env ran %d times, want 2 (one per key)", got)
	}
}

func TestEnvironmentFileHasNoTemporaryFolderLine(t *testing.T) {
	box := newSandbox(t)

	out, errOut, status := box.run(nil, "--file")
	if status != 0 {
		t.Fatalf("--file failed (%d): %s", status, errOut)
	}
	data, err := os.ReadFile(strings.TrimSpace(out))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "mktemp") {
		t.Fatalf("the environment file still makes its own temporary folder:\n%s", data)
	}
}

func TestParallelCallsMakeTheEnvironmentOnce(t *testing.T) {
	box := newSandbox(t)

	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, _, _ := box.run(nil, "sh", "-c", "echo $FAKE_ENV")
			results[i] = strings.TrimSpace(out)
		}()
	}
	wg.Wait()

	for i, got := range results {
		if got != "cached" {
			t.Errorf("call %d printed %q, want the cached environment", i, got)
		}
	}
	if got := box.count("print-dev-env"); got != 1 {
		t.Fatalf("print-dev-env ran %d times for 8 parallel calls, want 1", got)
	}
}

func TestFallsBackToNixDevelopWhenTheEnvironmentFails(t *testing.T) {
	box := newSandbox(t)

	out, errOut, status := box.run([]string{"FAKE_NIX_FAIL=1"}, "sh", "-c", "echo $FAKE_ENV")

	if status != 0 || strings.TrimSpace(out) != "fallback" {
		t.Fatalf("status %d, stdout %q: want the nix develop fallback", status, out)
	}
	if !strings.Contains(errOut, "plain 'nix develop -c'") {
		t.Fatalf("stderr %q does not say why the fallback was used", errOut)
	}
	if !strings.Contains(errOut, ".err") || !strings.Contains(errOut, "evaluation failed") {
		t.Fatalf("stderr %q does not point at the error file and its last line", errOut)
	}
	if got := box.count("develop"); got != 1 {
		t.Fatalf("nix develop ran %d times, want 1", got)
	}
}

func TestExitStatusOfTheCommandIsKept(t *testing.T) {
	box := newSandbox(t)

	_, _, status := box.run(nil, "sh", "-c", "exit 7")

	if status != 7 {
		t.Fatalf("exit status %d, want 7", status)
	}
}

func TestTemporaryFolderIsRemovedAfterTheCommand(t *testing.T) {
	box := newSandbox(t)

	out, _, _ := box.run(nil, "sh", "-c", `echo "$TMPDIR"; [ -d "$TMPDIR" ]`)
	folder := strings.TrimSpace(out)

	if !strings.HasPrefix(folder, box.tmp+"/nix-shell.") {
		t.Fatalf("TMPDIR %q is not a nix-shell.* folder under %s", folder, box.tmp)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("the temporary folder %s is still there (err %v)", folder, err)
	}
}

func TestStdinReachesTheCommand(t *testing.T) {
	box := newSandbox(t)
	cmd := exec.Command(filepath.Join(box.root, "scripts", "in-devenv"), "cat")
	cmd.Dir = box.root
	cmd.Env = box.env
	cmd.Stdin = strings.NewReader("from stdin")

	out, err := cmd.Output()

	if err != nil || string(out) != "from stdin" {
		t.Fatalf("output %q, error %v: want the input back", out, err)
	}
}

func TestRunsDirectlyInsideADevShell(t *testing.T) {
	box := newSandbox(t)

	out, _, status := box.run([]string{"IN_NIX_SHELL=impure"}, "sh", "-c", "echo direct")

	if status != 0 || strings.TrimSpace(out) != "direct" {
		t.Fatalf("status %d, stdout %q: want the command to run as it is", status, out)
	}
	if got := box.count("print-dev-env") + box.count("develop"); got != 0 {
		t.Fatalf("nix was called %d times inside a dev shell, want 0", got)
	}
}

func TestRunsDirectlyWithoutNix(t *testing.T) {
	box := newSandbox(t)
	bin := filepath.Join(filepath.Dir(box.log), "plain")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"sh", "dirname"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not installed", tool)
		}
		if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}

	out, _, status := box.run([]string{"PATH=" + bin}, "sh", "-c", "echo direct")

	if status != 0 || strings.TrimSpace(out) != "direct" {
		t.Fatalf("status %d, stdout %q: want the command to run without nix", status, out)
	}
}

func TestRefreshMakesTheEnvironmentAgain(t *testing.T) {
	box := newSandbox(t)
	box.run(nil, "true")

	_, errOut, status := box.run(nil, "--refresh")

	if status != 0 {
		t.Fatalf("--refresh failed (%d): %s", status, errOut)
	}
	if got := box.count("print-dev-env"); got != 2 {
		t.Fatalf("print-dev-env ran %d times, want 2", got)
	}
}

func TestMissingGCRootMakesTheEnvironmentAgain(t *testing.T) {
	box := newSandbox(t)
	box.run(nil, "true")
	profile := filepath.Join(box.cache, box.key()+".profile")
	if err := os.Remove(profile); err != nil {
		t.Fatal(err)
	}

	out, _, _ := box.run(nil, "sh", "-c", "echo $FAKE_ENV")

	if strings.TrimSpace(out) != "cached" || box.count("print-dev-env") != 2 {
		t.Fatalf("stdout %q, print-dev-env %d times: want the environment made again",
			out, box.count("print-dev-env"))
	}
}

func TestFlakeIsFoundFromTheCurrentFolder(t *testing.T) {
	box := newSandbox(t)
	inRepo := box.key()
	sub := filepath.Join(box.root, "internal", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	other := newSandbox(t)
	other.append("flake.lock", "\n# a worktree with its own lock\n")

	cmd := exec.Command(filepath.Join(box.root, "scripts", "in-devenv"), "--key")
	cmd.Env = box.env

	cmd.Dir = sub
	fromSub, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(filepath.Join(box.root, "scripts", "in-devenv"), "--key")
	cmd.Env = box.env
	cmd.Dir = other.root
	fromOther, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(fromSub)) != inRepo {
		t.Errorf("key from a subfolder %q, want the repository's %q", fromSub, inRepo)
	}
	if strings.TrimSpace(string(fromOther)) == inRepo {
		t.Errorf("key from another worktree equals %q: the script's own flake was used", inRepo)
	}
}

func TestKeyChangesWithTheSystem(t *testing.T) {
	box := newSandbox(t)
	before := box.key()
	stubs := t.TempDir()
	writeFile(t, filepath.Join(stubs, "uname"), "#!/bin/sh\necho Linux other-arch\n", 0o755)

	out, errOut, status := box.run([]string{"PATH=" + stubs + ":" + os.Getenv("PATH")}, "--key")

	if status != 0 {
		t.Fatalf("--key failed (%d): %s", status, errOut)
	}
	if strings.TrimSpace(out) == before {
		t.Fatalf("the key %s did not change with the system", before)
	}
}

func TestDoubleDashSeparatesTheCommand(t *testing.T) {
	box := newSandbox(t)

	out, _, status := box.run(nil, "--", "sh", "-c", "echo $FAKE_ENV")

	if status != 0 || strings.TrimSpace(out) != "cached" {
		t.Fatalf("status %d, stdout %q: want the command after -- in the environment", status, out)
	}
	_, _, status = box.run(nil, "--")
	if status != 2 {
		t.Fatalf("-- alone exited %d, want the usage error 2", status)
	}
}

func TestArgumentsReachTheCommandUnchanged(t *testing.T) {
	box := newSandbox(t)
	args := []string{"a b", "it's", `say "hi"`, "line1\nline2", "", "$HOME", "*", `back\slash`, "-n"}
	script := `for a in "$@"; do printf '[%s]' "$a"; done`

	out, _, status := box.run(nil, append([]string{"sh", "-c", script, "sh"}, args...)...)

	var want strings.Builder
	for _, arg := range args {
		want.WriteString("[" + arg + "]")
	}
	if status != 0 || out != want.String() {
		t.Fatalf("status %d, output %q, want %q", status, out, want.String())
	}
}

func TestEnvironmentIsMadeFromInsideTheFlakeCopy(t *testing.T) {
	box := newSandbox(t)

	out, _, _ := box.run(nil, "sh", "-c", "echo $EVAL_CWD")

	want := filepath.Join(box.cache, box.key()+".flake")
	if got := strings.TrimSpace(out); got != want {
		t.Fatalf("nix print-dev-env ran in %q, want %q (not the caller's folder)", got, want)
	}
}

func TestUnknownTemporaryFolderLineIsRefused(t *testing.T) {
	box := newSandbox(t)

	out, errOut, _ := box.run([]string{"FAKE_NIX_ODD=1"}, "sh", "-c", "echo $FAKE_ENV")

	if strings.TrimSpace(out) != "fallback" {
		t.Fatalf("stdout %q, stderr %q: want the fallback instead of an environment that leaks folders",
			out, errOut)
	}
	left, err := filepath.Glob(filepath.Join(box.cache, "*.sh*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("files left behind: %v", left)
	}
}

func TestFlakeIsFoundWithoutDirname(t *testing.T) {
	box := newSandbox(t)
	want := box.key()
	bin := t.TempDir()
	for _, tool := range []string{"sh", "cat", "uname", "sha256sum", "cut"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not installed", tool)
		}
		if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	// A folder with no flake above it: the search runs up to / and must stop there.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(box.root, "scripts", "in-devenv"), "--key")
	cmd.Dir = t.TempDir()
	cmd.Env = append(append([]string{}, box.env...), "PATH="+bin)

	out, err := cmd.Output()

	if err != nil || strings.TrimSpace(string(out)) != want {
		t.Fatalf("output %q, error %v, want the key %s of the script's own repository", out, err, want)
	}
}

func TestOldEntriesArePrunedAndOthersKept(t *testing.T) {
	box := newSandbox(t)
	if err := os.MkdirAll(box.cache, 0o755); err != nil {
		t.Fatal(err)
	}
	oldKey := strings.Repeat("a", 32)
	freshKey := strings.Repeat("b", 32)
	orphanOld := strings.Repeat("c", 32)
	orphanFresh := strings.Repeat("d", 32)
	old := time.Now().Add(-60 * 24 * time.Hour)

	oldFiles := []string{
		oldKey + ".sh", oldKey + ".err", oldKey + ".lock", oldKey + ".sh.tmp.123",
		oldKey + ".flake", oldKey + ".profile-1-link", orphanOld + ".lock",
	}
	keptFiles := []string{
		freshKey + ".sh", freshKey + ".err", orphanFresh + ".lock",
		"mytool.sh", "mytool.err", "mytool.profile", "mytool.flake", "notes.lock", "A" + oldKey[1:] + ".sh",
	}
	for _, name := range append(append([]string{}, oldFiles...), keptFiles...) {
		writeFile(t, filepath.Join(box.cache, name), "x", 0o644)
	}
	for _, name := range append(append([]string{}, oldFiles...), "mytool.sh", "mytool.err", "mytool.profile",
		"mytool.flake", "notes.lock", "A"+oldKey[1:]+".sh") {
		if err := os.Chtimes(filepath.Join(box.cache, name), old, old); err != nil {
			t.Fatal(err)
		}
	}

	box.run(nil, "true")

	for _, name := range oldFiles {
		if _, err := os.Lstat(filepath.Join(box.cache, name)); !os.IsNotExist(err) {
			t.Errorf("%s of an unused key is still there (err %v)", name, err)
		}
	}
	for _, name := range keptFiles {
		if _, err := os.Lstat(filepath.Join(box.cache, name)); err != nil {
			t.Errorf("%s was removed: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(box.cache, box.key()+".sh")); err != nil {
		t.Errorf("the current environment is gone: %v", err)
	}
}

// longRunning starts the wrapper around a command that waits for signals and prints which one
// it got. It returns the process and a reader of its output positioned after "ready".
func longRunning(t *testing.T, box *sandbox) (*exec.Cmd, *bufio.Reader) {
	t.Helper()
	script := `trap "echo got-int; exit 5" INT; trap "echo got-quit; exit 6" QUIT; ` +
		`trap "echo got-term; exit 7" TERM; echo ready; while :; do sleep 0.05; done`
	cmd := exec.Command(filepath.Join(box.root, "scripts", "in-devenv"), "sh", "-c", script)
	cmd.Dir = box.root
	cmd.Env = box.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Nothing may outlive the test, whatever happens to it.
	guard := time.AfterFunc(20*time.Second, func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	t.Cleanup(func() {
		guard.Stop()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("first line %q, error %v, want ready", line, err)
	}
	return cmd, reader
}

func TestSignalsReachTheCommand(t *testing.T) {
	signals := []struct {
		name   string
		signal syscall.Signal
		output string
		status int
	}{
		{"INT", syscall.SIGINT, "got-int\n", 5},
		{"QUIT", syscall.SIGQUIT, "got-quit\n", 6},
		{"TERM", syscall.SIGTERM, "got-term\n", 7},
	}
	targets := []struct {
		name  string
		group bool
	}{
		{"to the process group", true},
		{"to the wrapper only", false},
	}
	for _, sig := range signals {
		for _, target := range targets {
			t.Run(sig.name+" "+target.name, func(t *testing.T) {
				box := newSandbox(t)
				cmd, reader := longRunning(t, box)
				pid := cmd.Process.Pid
				if target.group {
					pid = -pid
				}

				if err := syscall.Kill(pid, sig.signal); err != nil {
					t.Fatal(err)
				}
				rest, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				waitErr := cmd.Wait()

				status := 0
				if exitErr, ok := waitErr.(*exec.ExitError); ok {
					status = exitErr.ExitCode()
				}
				if string(rest) != sig.output || status != sig.status {
					t.Fatalf("output %q and status %d, want %q and %d", rest, status, sig.output, sig.status)
				}
				left, _ := filepath.Glob(filepath.Join(box.tmp, "nix-shell.*"))
				if len(left) != 0 {
					t.Fatalf("temporary folders left: %v", left)
				}
			})
		}
	}
}

func TestCommandGetsTermWhenTheWrapperIsKilled(t *testing.T) {
	if _, err := exec.LookPath("setpriv"); err != nil {
		t.Skip("setpriv is not installed: the command is not told when the wrapper dies")
	}
	box := newSandbox(t)
	cmd, reader := longRunning(t, box)

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if string(rest) != "got-term\n" {
		t.Fatalf("output %q after killing the wrapper, want got-term", rest)
	}
}
