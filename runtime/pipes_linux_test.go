package rtfiles_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// bigSize is how many bytes the "big" case of test/diff.cpp writes to stdout with one
// fmt::Print, before "done" on stderr. "bigerr" writes them and a newline to stderr with
// one println, then "still alive\n" to stderr, and exits with code 3.
const bigSize = 1 << 20

// runPipes checks writes to pipes that Go handles and plain write(2) loops get wrong: a
// non-blocking stdout or stderr that is full (the program must wait, not drop the rest), and
// a broken pipe (fmt's writes to stdout end the program with SIGPIPE, also when the parent
// left SIGPIPE ignored; the builtin println ignores it). goBin is the Go side of the
// differential test, to check the expectations against Go, or "" without Go.
func runPipes(t *testing.T, bin, goBin string) {
	t.Run("nonblocking", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			fd   int // the fd that gets the non-blocking pipe
			size int
			code int
		}{{"big", 1, bigSize, 0}, {"bigerr", 2, bigSize + 1 + len("still alive\n"), 3}} {
			n, status := runNonblocking(t, bin, tc.name, tc.fd)
			if n != tc.size || !status.Exited() || status.ExitStatus() != tc.code {
				t.Errorf("%s into a full non-blocking pipe on fd %d: the reader got %d bytes, %s; want %d bytes and exit code %d",
					tc.name, tc.fd, n, describe(status), tc.size, tc.code)
			}
		}
	})
	t.Run("broken", func(t *testing.T) {
		for _, tc := range []struct {
			name, redirect, want string
		}{
			{"big", "", "signal: broken pipe"},           // fmt.Print to os.Stdout
			{"bigerr", "2>&1 >/dev/null", "exit code 3"}, // println goes on; "still alive" is lost
		} {
			for _, trap := range []string{"", "trap '' PIPE; "} {
				script := trap + `exec "$0" "$1" ` + tc.redirect
				if goBin != "" {
					if got := runBroken(t, goBin, tc.name, script); got != tc.want {
						t.Fatalf("Go: %s into a broken pipe (sh -c %q): %s; the test wants %s", tc.name, script, got, tc.want)
					}
				}
				if got := runBroken(t, bin, tc.name, script); got != tc.want {
					t.Errorf("%s into a broken pipe (sh -c %q): %s; want %s, like Go", tc.name, script, got, tc.want)
				}
			}
		}
	})
}

// describe says how a process ended: "exit code N" or "signal: <name>".
func describe(ws syscall.WaitStatus) string {
	switch {
	case ws.Exited():
		return fmt.Sprintf("exit code %d", ws.ExitStatus())
	case ws.Signaled():
		return "signal: " + ws.Signal().String()
	}
	return fmt.Sprintf("status %#x", uint32(ws))
}

// runBroken runs bin with the case name through sh -c script, with stdout a pipe whose read
// end is already closed, and says how it ended.
func runBroken(t *testing.T, bin, name, script string) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	cmd := exec.Command("sh", "-c", script, bin, name)
	cmd.Stdout = w
	cmd.WaitDelay = cmdTimeout
	err = cmd.Start()
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	timer := time.AfterFunc(cmdTimeout, func() { cmd.Process.Kill() })
	err = cmd.Wait()
	timer.Stop()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatal(err)
	}
	return describe(cmd.ProcessState.Sys().(syscall.WaitStatus))
}

// runNonblocking runs bin with the case name, with a non-blocking pipe as the fd fd (the other
// ones go to /dev/null), and reads the pipe only once the program waits for it. It returns how
// many bytes the reader got and how the program ended. exec.Cmd can't do this: os.File.Fd
// makes the fd blocking again, so the child gets raw fds from syscall.ForkExec.
func runNonblocking(t *testing.T, bin, name string, fd int) (int, syscall.WaitStatus) {
	t.Helper()
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	r := os.NewFile(uintptr(p[0]), "pipe") // non-blocking, so it uses the poller: deadlines work
	defer r.Close()
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	files := []uintptr{null.Fd(), null.Fd(), null.Fd()}
	files[fd] = uintptr(p[1])
	pid, err := syscall.ForkExec(bin, []string{bin, name}, &syscall.ProcAttr{
		Dir:   filepath.Dir(bin),
		Env:   os.Environ(),
		Files: files,
	})
	syscall.Close(p[1])
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(cmdTimeout)
	killed := false
	kill := func() {
		if !killed {
			syscall.Kill(pid, syscall.SIGKILL)
			killed = true
		}
	}

	// Wait until the program has written something and then sleeps (state S) or has ended
	// (Z). It writes more than a pipe holds, so with something written, sleeping means it
	// waits in poll for the full pipe: the case the test is about. Without the wait, write
	// fails with EAGAIN at once and the program ends (Z) with the rest lost. So a slow
	// machine only makes the test wait longer; it can't make a lost write pass.
	for {
		state := procState(pid)
		if state == 'Z' || state == 0 || (state == 'S' && pipeBytes(t, p[0]) > 0) {
			break
		}
		if time.Now().After(deadline) {
			kill()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	r.SetReadDeadline(deadline)
	var got bytes.Buffer
	if _, err := io.Copy(&got, r); err != nil {
		kill()
		t.Errorf("reading the pipe: %v", err)
	}
	var status syscall.WaitStatus
	for {
		_, err := syscall.Wait4(pid, &status, 0, nil)
		if err == nil {
			break
		}
		if err != syscall.EINTR {
			t.Fatal(err)
		}
	}
	if killed {
		t.Errorf("%s: killed after %v", name, cmdTimeout)
	}
	return got.Len(), status
}

// procState returns the state letter of the process pid from /proc (R, S, D, Z, ...), or 0 when
// it can't be read.
func procState(pid int) byte {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	// "pid (comm) S ...": comm can hold spaces and parentheses, so look after the last ')'.
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	if i < 0 || i+2 >= len(s) {
		return 0
	}
	return s[i+2]
}

// pipeBytes returns how many bytes wait in the pipe whose read end is fd (FIONREAD).
func pipeBytes(t *testing.T, fd int) int {
	t.Helper()
	var n int32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCINQ, uintptr(unsafe.Pointer(&n))); e != 0 {
		t.Fatalf("FIONREAD: %v", e)
	}
	return int(n)
}
