package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// A Target is a language server to measure: its command, how it is
// configured, and the file extension of the sources it is given.
type Target struct {
	Name     string   // "gocpp" or "gopls"
	Ext      string   // ".gp" or ".go"
	Command  []string // the command and its arguments
	Env      []string // added to the environment of the process
	Settings json.RawMessage
	// Start, if not nil, starts the server instead of running Command (the
	// tests of the harness use it for an in-process fake server).
	Start func(ctx context.Context, dir, cacheDir string) (*Proc, error)
}

// A Proc is a language server process and the client that talks to it.
type Proc struct {
	Target Target
	Client *Client
	cmd    *exec.Cmd
	stderr *lockedBuf
	start  time.Time
	waited chan struct{}
	pid    int
	stop   func() // a Proc that is not a process: how it ends
}

type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() > 1<<20 {
		return len(p), nil // enough to see what went wrong
	}
	return b.buf.Write(p)
}

func (b *lockedBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// StartProc starts the server of t in the folder dir, with the caches in
// cacheDir (a new folder is a cold start). The process is killed when ctx
// is done, and with the benchmark (Pdeathsig).
func StartProc(ctx context.Context, t Target, dir, cacheDir string) (*Proc, error) {
	if t.Start != nil {
		return t.Start(ctx, dir, cacheDir)
	}
	cmd := exec.CommandContext(ctx, t.Command[0], t.Command[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"XDG_CACHE_HOME="+cacheDir,
		"GOCPP_LSP_CACHE="+filepath.Join(cacheDir, "gocpp-lsp"),
		"GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1",
		"GOPROXY=off", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOWORK=off",
	)
	cmd.Env = append(cmd.Env, t.Env...)
	setProcAttr(cmd)
	cmd.Cancel = func() error { killGroup(cmd); return nil }
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p := &Proc{Target: t, cmd: cmd, stderr: new(lockedBuf), waited: make(chan struct{})}
	cmd.Stderr = p.stderr
	p.start = time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", t.Command[0], err)
	}
	p.pid = cmd.Process.Pid
	p.Client = NewClient(stdout, stdin, t.Settings)
	go func() {
		cmd.Wait()
		close(p.waited)
	}()
	return p, nil
}

// Started returns when the process was started.
func (p *Proc) Started() time.Time { return p.start }

// Stderr returns what the server wrote to its standard error.
func (p *Proc) Stderr() string { return p.stderr.String() }

// Stop shuts the server down (shutdown and exit) and waits for it to
// end, and kills it when it takes more than five seconds (and its whole
// process group, whatever it started).
func (p *Proc) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.Client.Call(ctx, "shutdown", nil, nil)
	p.Client.Notify("exit", nil)
	p.Client.Close()
	if p.cmd == nil {
		p.stop()
		return
	}
	select {
	case <-p.waited:
	case <-ctx.Done():
	}
	killGroup(p.cmd)
	<-p.waited
}

// Kill kills the server and its process group at once.
func (p *Proc) Kill() {
	if p.cmd == nil {
		p.stop()
		return
	}
	killGroup(p.cmd)
	<-p.waited
}

// Memory returns the resident set of the process now and its peak, in
// bytes (/proc on Linux, the working set on Windows); 0, 0 when unknown.
func (p *Proc) Memory() (rss, peak int64) {
	if p.cmd == nil {
		return 0, 0
	}
	return procMemory(p.pid)
}

// NewPipeProc returns a Proc for a server that is not a process: it reads
// from r and writes to w; stop is called when the proc is stopped or
// killed, and must end the server.
func NewPipeProc(t Target, r io.Reader, w io.WriteCloser, stop func()) *Proc {
	p := &Proc{Target: t, stderr: new(lockedBuf), waited: make(chan struct{}), start: time.Now()}
	p.Client = NewClient(r, w, t.Settings)
	p.stop = stop
	return p
}
