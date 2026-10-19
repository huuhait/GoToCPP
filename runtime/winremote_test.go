//go:build linux

package rtfiles_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The remote runner: with GOCPP_WINDOWS_SSH=user@host in the environment, the Windows tests
// (windows_test.go, windows_gp_test.go) run their programs on a real Windows machine over ssh instead of
// under Wine. The programs are still built here, with the dev shell's cross compiler. The variable
// is the only place that names the machine; the repository never does.
//
// The rules of the runner, so that it can be used on somebody's own PC:
//   - everything lands in one folder, %USERPROFILE%\gocpp-wintest\<run id>\ (the ssh login's home),
//     which the test deletes at its end; nothing else on the machine is read, written or installed,
//     and there are no registry or system changes;
//   - a program that is still running when its time is up is ended with taskkill by its image name
//     (gp<pid>_<time>_<n>.exe: unique to this run, so no other session's program is touched);
//     programs that finished are never killed;
//   - one ssh connection is shared by all runs (ControlMaster, ControlPersist=60) in a short
//     folder of the test binary; BatchMode and ConnectTimeout make a dead host a quick failure;
//   - the host's key must be accepted without a prompt (key authentication, a known host key).
//
// A program that writes a megabyte or more to a pipe can stall in the ssh server's relay (seen with the
// Go programs of TestWindowsDiff: the reference binary hung at the end of a 1 MiB line on stderr), so
// those runs (viaFiles) send both streams to files on the machine and copy them back with scp.
//
// The login shell is cmd.exe (Windows OpenSSH's default), so a command is "cd ... && set ... &&
// program args": the arguments are quoted for the C runtime's rules (Go's syscall.EscapeArg), and
// the environment is set with set "NAME=value" first. The exit status is the program's low
// 8 bits (ssh's rule), so a crash with 0xc0000005 shows as 5.

type remoteWin struct {
	host    string
	dir     string // the control socket folder here
	runID   string
	mu      sync.Mutex
	uploads map[string]string // local path + size + mtime -> remote image name
	next    int
}

// sshOpts are the options of every ssh and scp command.
func (r *remoteWin) sshOpts() []string {
	return []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "LogLevel=ERROR",
		"-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=3",
		"-o", "ControlMaster=auto", "-o", "ControlPath=" + filepath.Join(r.dir, "%C"), "-o", "ControlPersist=60"}
}

// ssh runs a cmd.exe command line on the machine, with the context's bound.
func (r *remoteWin) ssh(ctx context.Context, command string) (stdout, stderr []byte, code int, err error) {
	cmd := exec.CommandContext(ctx, "ssh", append(r.sshOpts(), "--", r.host, command)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.Bytes(), errb.Bytes(), ee.ExitCode(), nil
	}
	return out.Bytes(), errb.Bytes(), 0, err
}

// hostRx is what GOCPP_WINDOWS_SSH may be: user@host, a name or an address; never an option for ssh.
var hostRx = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@:\[\]-]*$`)

func newRemoteWin(host string) (*remoteWin, error) {
	if !hostRx.MatchString(host) {
		return nil, fmt.Errorf("GOCPP_WINDOWS_SSH=%q: want user@host, with no space and no leading '-'", host)
	}
	// A control socket path has a length limit of about 104 bytes, and TMPDIR may be long (the
	// tests of the repo run with deep ones): the folder is a short one in /tmp.
	base := "/tmp"
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		base = os.TempDir()
	}
	if len(base) > 40 {
		return nil, fmt.Errorf("no short folder for the ssh control socket (%q is too long)", base)
	}
	removeStaleLocal(base)
	dir, err := os.MkdirTemp(base, "gp-ssh-")
	if err != nil {
		return nil, err
	}
	r := &remoteWin{host: host, dir: dir, uploads: make(map[string]string),
		runID: fmt.Sprintf("%d_%d", os.Getpid(), time.Now().Unix())}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Folders of earlier runs that were killed before they could clean up (a day old or more; only
	// this tool's folder, gocpp-wintest, and only folders in it), then this run's folder.
	r.ssh(ctx, `if exist gocpp-wintest forfiles /P gocpp-wintest /D -1 /C "cmd /c if @isdir==TRUE rmdir /S /Q @path" >nul 2>&1`)
	out, errb, code, err := r.ssh(ctx, `mkdir gocpp-wintest\`+r.runID+` && echo gocpp-ready`)
	if err != nil || code != 0 || !strings.Contains(string(out), "gocpp-ready") {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("GOCPP_WINDOWS_SSH: ssh to the Windows machine failed: exit %d %v\nstdout: %s\nstderr: %s", code, err, out, errb)
	}
	return r, nil
}

// removeStaleLocal removes the control socket folders (gp-ssh-*) that an earlier test binary left in
// base when it was killed, if they are a day old or more.
func removeStaleLocal(base string) {
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "gp-ssh-") {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
			os.RemoveAll(filepath.Join(base, e.Name()))
		}
	}
}

// close deletes the run's folder on the machine and closes the shared connection. (A program that
// ran out of time was ended by run at once; nothing else is ended here, because the folder's
// programs have finished.)
func (r *remoteWin) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r.ssh(ctx, `rmdir /S /Q gocpp-wintest\`+r.runID+` & rmdir gocpp-wintest 2>nul`)
	exec.Command("ssh", append(r.sshOpts(), "-O", "exit", "--", r.host)...).Run()
	os.RemoveAll(r.dir)
}

// upload copies the local program to the run's folder, once for each file, and returns its name
// there.
func (r *remoteWin) upload(t *testing.T, exe string) string {
	t.Helper()
	fi, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", exe, fi.Size(), fi.ModTime().UnixNano())
	r.mu.Lock()
	defer r.mu.Unlock()
	if name, ok := r.uploads[key]; ok {
		return name
	}
	r.next++
	name := fmt.Sprintf("gp%s_%d.exe", r.runID, r.next) // unique on the machine: other runs may share it
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "scp", append(r.sshOpts(), "-q", "--", exe, r.host+":gocpp-wintest/"+r.runID+"/"+name)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("scp %s: %v\n%s", exe, err, out)
	}
	r.uploads[key] = name
	return name
}

// winEscapeArg quotes an argument for the C runtime's parsing of a command line (syscall.EscapeArg
// of Go's Windows port, which the 14.1 tests agree with): backslashes before a quote are doubled,
// a quote is escaped, and an argument with a space, tab or quote or an empty one is quoted.
func winEscapeArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0 // backslashes just written
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			slashes++
			b.WriteByte(c)
		case '"':
			b.WriteString(strings.Repeat(`\`, slashes)) // each backslash before a quote is doubled
			b.WriteString(`\"`)
			slashes = 0
		default:
			slashes = 0
			b.WriteByte(c)
		}
	}
	b.WriteString(strings.Repeat(`\`, slashes)) // and before the closing quote
	b.WriteByte('"')
	return b.String()
}

// run runs the uploaded program with the arguments and extra environment, with the time bound.
func (r *remoteWin) run(t *testing.T, timeout time.Duration, exe string, env []string, args []string, viaFiles bool) winRun {
	t.Helper()
	name := r.upload(t, exe)
	var b strings.Builder
	b.WriteString(`cd /d gocpp-wintest\` + r.runID)
	for _, kv := range env {
		if strings.ContainsAny(kv, "\"%\r\n") {
			t.Fatalf("remote runner: the variable %q has a character that cmd.exe cannot carry", kv)
		}
		b.WriteString(` && set "` + kv + `"`)
	}
	var prog strings.Builder
	prog.WriteString(`.\` + name) // as typed: a path in the program's argument 0
	for _, a := range args {
		if strings.ContainsAny(a, "%\r\n&|<>^") || viaFiles && strings.ContainsAny(a, "\" ") {
			t.Fatalf("remote runner: the argument %q has a character that cmd.exe would read", a)
		}
		prog.WriteString(" " + winEscapeArg(a))
	}
	if viaFiles {
		// stdout and stderr go to files, which are copied back afterwards: see runViaFiles.
		b.WriteString(` && cmd /c "` + prog.String() + ` >out.txt 2>err.txt"`)
	} else {
		b.WriteString(` && ` + prog.String())
	}
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	out, errb, code, err := r.ssh(ctx, b.String())
	if ctx.Err() != nil {
		// Time is up: the program may still run on the machine.
		kctx, kcancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer kcancel()
		r.ssh(kctx, `taskkill /F /T /IM `+name+` >nul 2>&1`)
	}
	if err != nil {
		t.Fatalf("ssh %s %q: %v\nstdout:\n%s\nstderr:\n%s", name, args, err, out, errb)
	}
	if code == 255 && strings.Contains(string(errb), "ssh:") {
		t.Fatalf("ssh failed: %s", errb)
	}
	if viaFiles {
		for i, f := range []string{"out.txt", "err.txt"} {
			dst := filepath.Join(r.dir, f)
			cmd := exec.Command("scp", append(r.sshOpts(), "-q", "--", r.host+":gocpp-wintest/"+r.runID+"/"+f, dst)...)
			if o, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("scp %s back: %v\n%s", f, err, o)
			}
			b, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			os.Remove(dst)
			if i == 0 {
				out = b
			} else {
				errb = b
			}
		}
	}
	return winRun{out, errb, code}
}
