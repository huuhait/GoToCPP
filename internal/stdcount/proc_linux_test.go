package stdcount

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// processGone reports whether the process has ended: it does not exist, or it is a zombie that its
// new parent has not reaped yet.
func processGone(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	stat := string(data)
	state := stat[strings.LastIndex(stat, ")")+2:]
	return strings.HasPrefix(state, "Z")
}

// waitUntilGone waits for a process that was killed to disappear; the limit only bounds a failure.
func waitUntilGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("process %d is still running", pid)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestKillGroupEndsProcessesTheCommandStarted(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 300 & echo $!; wait")
	setProcessGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("the shell did not print the pid of its child: %v", err)
	}
	sleeper, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(sleeper, syscall.SIGKILL) })

	killGroup(cmd)
	cmd.Wait()

	waitUntilGone(t, sleeper)
}

func TestKillGroupOfCommandThatNeverStartedDoesNothing(t *testing.T) {
	cmd := exec.Command("sh", "-c", "true")

	killGroup(cmd)
}

// hangingScript is a stand-in for a test binary: it names a running test, starts a process of its
// own, writes the pid of that process to the file named by SLEEPER_PID, and then hangs.
const hangingScript = `echo "=== RUN   TestHang"
sleep 300 &
echo $! > "$SLEEPER_PID"
wait
`

func writeScript(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExecRoundKillsHangingBinaryAndItsProcesses(t *testing.T) {
	script := writeScript(t, hangingScript)
	pidFile := filepath.Join(t.TempDir(), "sleeper.pid")
	env := append(os.Environ(), "SLEEPER_PID="+pidFile)

	res := execRound(context.Background(), "sh", []string{script}, t.TempDir(), env, 5*time.Second)

	if !res.HardTimeout || res.Err == nil {
		t.Fatalf("round = %+v; want a hard timeout with an error", res)
	}
	if !strings.Contains(string(res.Out), "=== RUN   TestHang") {
		t.Fatalf("out = %q; want the line the binary printed before it hung", res.Out)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the script did not get as far as starting its process within the limit: %v", err)
	}
	sleeper, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(sleeper, syscall.SIGKILL) })
	waitUntilGone(t, sleeper)
}

func TestExecRoundOfBinaryThatFailsIsNotHardTimeout(t *testing.T) {
	script := writeScript(t, "echo FAIL output\nexit 3\n")

	res := execRound(context.Background(), "sh", []string{script}, t.TempDir(), os.Environ(),
		time.Minute)

	if res.Err == nil || res.HardTimeout {
		t.Fatalf("round = %+v; want an error that is not a hard timeout", res)
	}
	if string(res.Out) != "FAIL output\n" {
		t.Fatalf("out = %q", res.Out)
	}
}

func TestExecRoundOfBinaryThatPassesHasNoError(t *testing.T) {
	script := writeScript(t, "echo ok\n")

	res := execRound(context.Background(), "sh", []string{script}, t.TempDir(), os.Environ(),
		time.Minute)

	if res.Err != nil || res.HardTimeout || string(res.Out) != "ok\n" {
		t.Fatalf("round = %+v", res)
	}
}

func TestExecRoundOfCanceledContextIsNotHardTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	script := writeScript(t, "sleep 300\n")

	res := execRound(ctx, "sh", []string{script}, t.TempDir(), os.Environ(), time.Minute)

	if res.Err == nil || res.HardTimeout {
		t.Fatalf("round = %+v; want an error that is not a hard timeout", res)
	}
}
