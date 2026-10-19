//go:build linux

// Package winssh runs Windows programs on a real Windows machine over ssh, for gocpp's tests (Phase 06
// step 6.13, ADR-055). It is the harness's copy of the runner of runtime/winremote_test.go (step 14.1:
// that one is a test file of another package and can't be imported); the rules are the same:
//
//   - the machine is named by the caller (the tests read GOCPP_WINDOWS_SSH=user@host; the repository
//     never names a machine), and its key must be accepted without a prompt;
//   - everything lands in one folder, %USERPROFILE%\gocpp-wintest\<run id>\, which Close deletes;
//     nothing else is read, written or installed there, and there are no registry or system changes;
//   - a program that is still running when its time is up is ended with taskkill by its image name
//     (gp<pid>_<time>_<n>.exe, unique to this run), so no other session's program is touched;
//   - one ssh connection is shared (ControlMaster, ControlPersist=60) in a short folder of /tmp.
//
// The login shell is cmd.exe: a command is "cd ... && set ... && program args", the arguments are quoted
// for the C runtime's rules, and the exit status is the program's low 8 bits (ssh's rule). A program that
// writes a megabyte or more to a pipe can stall in the ssh server's relay, so Run can send both streams
// to files on the machine and copy them back with scp.
package winssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// A Remote is a connection to the Windows machine and this run's folder on it.
type Remote struct {
	host    string
	dir     string // the control socket folder here
	runID   string
	mu      sync.Mutex
	uploads map[string]string // local path + size + mtime -> remote image name
	next    int
	home    string
}

var hostRx = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@:\[\]-]*$`)

// New connects to host (user@host, a name or an address; never an option for ssh) and makes the
// run's folder. It fails at once when the machine can't be reached with key authentication.
func New(host string) (*Remote, error) {
	if !hostRx.MatchString(host) {
		return nil, fmt.Errorf("GOCPP_WINDOWS_SSH=%q: want user@host, with no space and no leading '-'", host)
	}
	// A control socket path has a length limit of about 104 bytes, and TMPDIR may be long: the
	// folder is a short one in /tmp.
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
	r := &Remote{host: host, dir: dir, uploads: make(map[string]string),
		runID: fmt.Sprintf("%d_%d", os.Getpid(), time.Now().Unix())}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Folders of earlier runs that were killed before they could clean up (a day old or more; only
	// this tool's folder, gocpp-wintest), then this run's folder.
	r.ssh(ctx, `if exist gocpp-wintest forfiles /P gocpp-wintest /D -1 /C "cmd /c if @isdir==TRUE rmdir /S /Q @path" >nul 2>&1`)
	out, errb, code, err := r.ssh(ctx, `mkdir gocpp-wintest\`+r.runID+` && echo gocpp-ready`)
	if err != nil || code != 0 || !strings.Contains(string(out), "gocpp-ready") {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("ssh to the Windows machine failed: exit %d %v\nstdout: %s\nstderr: %s", code, err, out, errb)
	}
	return r, nil
}

func (r *Remote) sshOpts() []string {
	return []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "LogLevel=ERROR",
		"-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=3",
		"-o", "ControlMaster=auto", "-o", "ControlPath=" + filepath.Join(r.dir, "%C"), "-o", "ControlPersist=60"}
}

// ssh runs a cmd.exe command line on the machine, with the context's bound.
func (r *Remote) ssh(ctx context.Context, command string) (stdout, stderr []byte, code int, err error) {
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

// removeStaleLocal removes the control socket folders that an earlier test binary left in base when it
// was killed, if they are a day old or more.
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

// Close deletes the run's folder on the machine and closes the shared connection.
func (r *Remote) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r.ssh(ctx, `rmdir /S /Q gocpp-wintest\`+r.runID+` & rmdir gocpp-wintest 2>nul`)
	exec.Command("ssh", append(r.sshOpts(), "-O", "exit", "--", r.host)...).Run()
	os.RemoveAll(r.dir)
}

// upload copies the local program to the run's folder, once for each file, and returns its name there.
func (r *Remote) upload(exe string) (string, error) {
	fi, err := os.Stat(exe)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", exe, fi.Size(), fi.ModTime().UnixNano())
	r.mu.Lock()
	defer r.mu.Unlock()
	if name, ok := r.uploads[key]; ok {
		return name, nil
	}
	r.next++
	name := fmt.Sprintf("gp%s_%d.exe", r.runID, r.next) // unique on the machine: other runs may share it
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "scp", append(r.sshOpts(), "-q", "--", exe, r.host+":gocpp-wintest/"+r.runID+"/"+name)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("scp %s: %v\n%s", exe, err, out)
	}
	r.uploads[key] = name
	return name, nil
}

// EscapeArg quotes an argument for the C runtime's parsing of a command line (syscall.EscapeArg of
// Go's Windows port): backslashes before a quote are doubled, a quote is escaped, and an argument
// with a space, tab or quote, or an empty one, is quoted.
func EscapeArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			slashes++
			b.WriteByte(c)
		case '"':
			b.WriteString(strings.Repeat(`\`, slashes))
			b.WriteString(`\"`)
			slashes = 0
		default:
			slashes = 0
			b.WriteByte(c)
		}
	}
	b.WriteString(strings.Repeat(`\`, slashes))
	b.WriteByte('"')
	return b.String()
}

// A Result is what the program did on the machine.
type Result struct {
	Stdout, Stderr []byte
	Code           int  // the low 8 bits of the exit status
	TimedOut       bool // it was ended after the time given
}

// Run uploads exe and runs it with the arguments and the extra environment (NAME=value), for at most
// timeout. With viaFiles the streams go through files (for output of a megabyte or more); the
// arguments may then hold neither a space nor a quote. An error is a failure of the connection or a
// character that cmd.exe would read, not the program's exit.
func (r *Remote) Run(ctx context.Context, timeout time.Duration, exe string, env, args []string, viaFiles bool) (Result, error) {
	name, err := r.upload(exe)
	if err != nil {
		return Result{}, err
	}
	var b strings.Builder
	b.WriteString(`cd /d gocpp-wintest\` + r.runID)
	for _, kv := range env {
		if strings.ContainsAny(kv, "\"%\r\n") {
			return Result{}, fmt.Errorf("the variable %q has a character that cmd.exe cannot carry", kv)
		}
		b.WriteString(` && set "` + kv + `"`)
	}
	var prog strings.Builder
	prog.WriteString(`.\` + name)
	for _, a := range args {
		if strings.ContainsAny(a, "%\r\n&|<>^") || viaFiles && strings.ContainsAny(a, "\" ") {
			return Result{}, fmt.Errorf("the argument %q has a character that cmd.exe would read", a)
		}
		prog.WriteString(" " + EscapeArg(a))
	}
	if viaFiles {
		b.WriteString(` && cmd /c "` + prog.String() + ` >out.txt 2>err.txt"`)
	} else {
		b.WriteString(` && ` + prog.String())
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, errb, code, err := r.ssh(cctx, b.String())
	res := Result{Stdout: out, Stderr: errb, Code: code}
	if cctx.Err() != nil {
		// Time is up: the program may still run on the machine.
		res.TimedOut = true
		kctx, kcancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer kcancel()
		r.ssh(kctx, `taskkill /F /T /IM `+name+` >nul 2>&1`)
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("ssh %s %q: %v\nstderr:\n%s", name, args, err, errb)
	}
	if code == 255 && strings.Contains(string(errb), "ssh:") {
		return res, fmt.Errorf("ssh failed: %s", errb)
	}
	if viaFiles {
		for i, f := range []string{"out.txt", "err.txt"} {
			dst := filepath.Join(r.dir, f)
			cmd := exec.Command("scp", append(r.sshOpts(), "-q", "--", r.host+":gocpp-wintest/"+r.runID+"/"+f, dst)...)
			if o, err := cmd.CombinedOutput(); err != nil {
				return res, fmt.Errorf("scp %s back: %v\n%s", f, err, o)
			}
			data, err := os.ReadFile(dst)
			if err != nil {
				return res, err
			}
			os.Remove(dst)
			if i == 0 {
				res.Stdout = data
			} else {
				res.Stderr = data
			}
		}
	}
	return res, nil
}

// vtRx matches the terminal control sequences that the pseudo console (ConPTY) writes around a
// program's output: CSI sequences (ESC [ ... letter) and OSC sequences (ESC ] ... BEL).
var vtRx = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]|\x1b\\][^\x07]*\x07")

// A markerWriter collects what the console shows and tells, once, when the text marker has appeared in it.
type markerWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	marker []byte
	seen   chan struct{}
	done   bool
}

func (w *markerWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	if !w.done && bytes.Contains(w.buf.Bytes(), w.marker) {
		w.done = true
		close(w.seen)
	}
	return len(p), nil
}

func (w *markerWriter) sawMarker() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.done
}

func (w *markerWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// RunConsole runs exe with a console (ssh -tt: the program's standard handles are the console of a pseudo
// terminal, as in a terminal window), types the text stdin at it once the console shows prompt (the text
// that the program writes before it reads: a fixed pause would be a guess), and returns what appeared on
// the screen with the control sequences taken out and "\r\n" turned into "\n". The console shows what the
// program writes to both streams, and the characters that are typed (echo), in one stream. The exit code
// is the program's. An error says the screen if prompt never appeared before the time was up.
func (r *Remote) RunConsole(ctx context.Context, timeout time.Duration, exe, prompt, stdin string) (output string, code int, err error) {
	name, err := r.upload(exe)
	if err != nil {
		return "", 0, err
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "ssh", append(r.sshOpts(), "-tt", "--", r.host, `cd /d gocpp-wintest\`+r.runID+` && .\`+name)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		return "", 0, err
	}
	out := &markerWriter{marker: []byte(prompt), seen: make(chan struct{})}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		return "", 0, err
	}
	// The text is typed when the prompt is on the screen; the pipe stays open until the program ends (a
	// closed input ends the session before the output has come).
	go func() {
		select {
		case <-out.seen:
			in.Write([]byte(stdin))
		case <-cctx.Done():
		}
	}()
	err = cmd.Wait()
	in.Close()
	text := vtRx.ReplaceAllString(out.String(), "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var ee *exec.ExitError
	switch {
	case cctx.Err() != nil:
		kctx, kcancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer kcancel()
		r.ssh(kctx, `taskkill /F /T /IM `+name+` >nul 2>&1`)
		return text, 0, fmt.Errorf("the program did not end in %v (prompt %q seen: %v); screen:\n%s", timeout, prompt, out.sawMarker(), text)
	case errors.As(err, &ee):
		return text, ee.ExitCode(), nil
	}
	return text, 0, err
}

// RunID returns the name of this run's folder (the last element of Dir).
func (r *Remote) RunID() string { return r.runID }

// Dir returns this run's folder as a path relative to the user's home on the machine, with
// backslashes: gocpp-wintest\<run id>. A command that Exec runs starts there.
func (r *Remote) Dir() string { return `gocpp-wintest\` + r.runID }

// Home returns the user's home folder on the machine (%USERPROFILE%), asked once.
func (r *Remote) Home(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.home != "" {
		return r.home, nil
	}
	out, errb, code, err := r.ssh(ctx, `echo %USERPROFILE%`)
	if err != nil || code != 0 {
		return "", fmt.Errorf("asking for the home folder: exit %d %v %s", code, err, errb)
	}
	r.home = strings.TrimSpace(string(out))
	return r.home, nil
}

// Abs returns the absolute path on the machine of rel, a path below this run's folder (slashes or
// backslashes).
func (r *Remote) Abs(ctx context.Context, rel string) (string, error) {
	home, err := r.Home(ctx)
	if err != nil {
		return "", err
	}
	rel = strings.ReplaceAll(rel, "/", `\`)
	if rel == "" {
		return home + `\` + r.Dir(), nil
	}
	return home + `\` + r.Dir() + `\` + rel, nil
}

// Put copies a local file or folder to rel, a path below this run's folder (with slashes). A folder
// is copied whole as rel; its parent is made. rel must not exist yet when local is a folder.
func (r *Remote) Put(ctx context.Context, local, rel string) error {
	if strings.ContainsAny(rel, "\"\r\n%&|<>^") || strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
		return fmt.Errorf("the remote path %q is not a plain relative path", rel)
	}
	fi, err := os.Stat(local)
	if err != nil {
		return err
	}
	if parent := path.Dir(rel); parent != "." {
		out, errb, code, err := r.ssh(ctx, `cd gocpp-wintest\`+r.runID+` && if not exist "`+strings.ReplaceAll(parent, "/", `\`)+`" mkdir "`+strings.ReplaceAll(parent, "/", `\`)+`"`)
		if err != nil || code != 0 {
			return fmt.Errorf("making %s: exit %d %v %s%s", parent, code, err, out, errb)
		}
	}
	args := append(r.sshOpts(), "-q")
	if fi.IsDir() {
		args = append(args, "-r")
	}
	args = append(args, "--", local, r.host+":gocpp-wintest/"+r.runID+"/"+rel)
	cmd := exec.CommandContext(ctx, "scp", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp %s: %v\n%s", local, err, out)
	}
	return nil
}

// Get copies the file rel (below this run's folder) to the local file dst.
func (r *Remote) Get(ctx context.Context, rel, dst string) error {
	cmd := exec.CommandContext(ctx, "scp", append(r.sshOpts(), "-q", "--", r.host+":gocpp-wintest/"+r.runID+"/"+rel, dst)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp %s back: %v\n%s", rel, err, out)
	}
	return nil
}

// Exec runs a cmd.exe command line in this run's folder, with the extra environment (NAME=value, set
// before the command) and at most timeout. The line is cmd.exe's own syntax: the caller quotes it.
// The output of a command that writes megabytes should go to a file of the run (> out.txt) and be
// fetched with Get. When the time is up, every process whose command line names this run's folder
// is ended (the programs of this run only), and TimedOut is set.
func (r *Remote) Exec(ctx context.Context, timeout time.Duration, env []string, line string) (Result, error) {
	var b strings.Builder
	b.WriteString(`cd /d ` + r.Dir())
	for _, kv := range env {
		if strings.ContainsAny(kv, "\"%\r\n") {
			return Result{}, fmt.Errorf("the variable %q has a character that cmd.exe cannot carry", kv)
		}
		b.WriteString(` && set "` + kv + `"`)
	}
	b.WriteString(` && ` + line)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, errb, code, err := r.ssh(cctx, b.String())
	res := Result{Stdout: out, Stderr: errb, Code: code}
	if cctx.Err() != nil {
		res.TimedOut = true
		r.killRun()
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("ssh %q: %v\nstderr:\n%s", line, err, errb)
	}
	if code == 255 && strings.Contains(string(errb), "ssh:") {
		return res, fmt.Errorf("ssh failed: %s", errb)
	}
	return res, nil
}

// killRun ends the processes of this run: those whose command line names the run's folder (the
// PowerShell that looks is left out: its own command line names it too).
func (r *Remote) killRun() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r.ssh(ctx, `powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*gocpp-wintest\`+r.runID+`*' -and $_.Name -ne 'powershell.exe' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }" >nul 2>&1`)
}

// A Process is a program that runs on the machine with its standard streams connected to this side
// (ssh without a terminal): a language server that a test drives over its standard input and output.
// The streams are binary: ssh adds and removes nothing.
type Process struct {
	Stdin  io.WriteCloser // the program's standard input; closing it is the end of its input
	Stdout io.Reader      // the program's standard output
	r      *Remote
	cmd    *exec.Cmd
	stderr *stderrBuf
	done   chan struct{}
	code   int
	werr   error
}

type stderrBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *stderrBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() < 4<<20 {
		b.buf.Write(p)
	}
	return len(p), nil
}

func (b *stderrBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Start runs the cmd.exe command line in this run's folder, with the extra environment (NAME=value),
// and returns at once. The program ends when its input ends, when Kill is called, or when ctx is done;
// the ssh client is killed with the test (process group, Pdeathsig), and so is the program's tree on the
// machine (the server of the connection ends the session's processes; Kill also ends those that name
// this run's folder).
func (r *Remote) Start(ctx context.Context, env []string, line string) (*Process, error) {
	var b strings.Builder
	b.WriteString(`cd /d ` + r.Dir())
	for _, kv := range env {
		if strings.ContainsAny(kv, "\"%\r\n") {
			return nil, fmt.Errorf("the variable %q has a character that cmd.exe cannot carry", kv)
		}
		b.WriteString(` && set "` + kv + `"`)
	}
	b.WriteString(` && ` + line)
	cmd := exec.CommandContext(ctx, "ssh", append(r.sshOpts(), "-T", "--", r.host, b.String())...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p := &Process{Stdin: in, Stdout: out, r: r, cmd: cmd, stderr: new(stderrBuf), done: make(chan struct{})}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := cmd.Wait()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			p.code, err = ee.ExitCode(), nil
		}
		p.werr = err
		close(p.done)
	}()
	return p, nil
}

// Stderr returns what the program has written to its standard error so far (the first 4 MiB).
func (p *Process) Stderr() string { return p.stderr.String() }

// Done is closed when the program has ended.
func (p *Process) Done() <-chan struct{} { return p.done }

// Wait waits for the end of the program and returns its exit code (the low 8 bits).
func (p *Process) Wait(ctx context.Context) (int, error) {
	select {
	case <-p.done:
		return p.code, p.werr
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Kill ends the connection and the processes of this run on the machine, and waits for the end.
func (p *Process) Kill() {
	syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	p.r.killRun()
	<-p.done
}
