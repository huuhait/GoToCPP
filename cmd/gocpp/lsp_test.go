package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
)

// An lspProcess is "gocpp lsp" as a process of its own: the test binary as
// gocpp (GOCPP_TEST_MAIN=1, see TestMain), which starts like gocpp with the
// test code on top.
type lspProcess struct {
	t      *testing.T
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *protocol.Reader
	stderr lockedBuffer
	start  time.Time
}

// A lockedBuffer is a bytes.Buffer that exec's copying goroutine can write
// while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func startLSP(t *testing.T, args ...string) *lspProcess {
	t.Helper()
	p := &lspProcess{t: t, cmd: exec.Command(os.Args[0], append([]string{"lsp"}, args...)...)}
	p.cmd.Env = append(os.Environ(), "GOCPP_TEST_MAIN=1")
	p.cmd.Stderr = &p.stderr
	var err error
	if p.in, err = p.cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.out = protocol.NewReader(stdout)
	p.start = time.Now()
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	timer := time.AfterFunc(30*time.Second, func() { p.cmd.Process.Kill() })
	t.Cleanup(func() {
		timer.Stop()
		p.cmd.Process.Kill()
		p.cmd.Wait()
	})
	return p
}

func (p *lspProcess) send(body string) {
	p.t.Helper()
	if _, err := fmt.Fprintf(p.in, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		p.t.Fatal(err)
	}
}

func (p *lspProcess) read() string {
	p.t.Helper()
	body, err := p.out.Read()
	if err != nil {
		p.t.Fatalf("reading from gocpp lsp: %v; stderr:\n%s", err, p.stderr.String())
	}
	return string(body)
}

// wait closes stdin and returns the exit code. gocpp lsp must have written
// nothing more to stdout.
func (p *lspProcess) wait() int {
	p.t.Helper()
	p.in.Close()
	if body, err := p.out.Read(); err != io.EOF {
		p.t.Errorf("at the end, gocpp lsp wrote %q (%v)", body, err)
	}
	err := p.cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	} else if err != nil {
		p.t.Fatal(err)
	}
	return 0
}

var (
	lspInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"clientInfo":{"name":"lsp_test"},"rootUri":null,"capabilities":{}}}`
	lspInitResult = `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"positionEncoding":"utf-16","textDocumentSync":{"openClose":true,"change":2,"save":{}}},"serverInfo":{"name":"gocpp","version":` + strconv.Quote(version.Version) + `}}}`
	lspShutdown   = `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	lspExit       = `{"jsonrpc":"2.0","method":"exit"}`
)

// TestLSPProcess runs gocpp lsp over pipes, like an editor does: it must
// answer initialize well within the budget (under 100 ms from the start of
// the process, phase 11 README §2.1), keep stdout for LSP messages only,
// and exit with status 0 after shutdown and exit.
func TestLSPProcess(t *testing.T) {
	const runs = 5
	var times []time.Duration
	for i := range runs {
		p := startLSP(t)
		p.send(lspInitialize)
		if got := p.read(); got != lspInitResult {
			t.Fatalf("initialize: got %s, want %s", got, lspInitResult)
		}
		times = append(times, time.Since(p.start))
		p.send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
		p.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///w/a.gp","languageId":"gocpp","version":1,"text":"package main\n"}}}`)
		p.send(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"file:///w/a.gp","version":2},"contentChanges":[{"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":0}},"text":"// 😀\n"}]}}`)
		p.send(`{"jsonrpc":"2.0","id":"h","method":"textDocument/hover","params":{}}`)
		if got, want := p.read(), `{"jsonrpc":"2.0","id":"h","error":{"code":-32601,"message":"method not found: textDocument/hover"}}`; got != want {
			t.Errorf("hover: got %s, want %s", got, want)
		}
		p.send(lspShutdown)
		if got, want := p.read(), `{"jsonrpc":"2.0","id":2,"result":null}`; got != want {
			t.Errorf("shutdown: got %s, want %s", got, want)
		}
		p.send(lspExit)
		if code := p.wait(); code != 0 {
			t.Errorf("run %d: exit status %d, want 0; stderr:\n%s", i, code, p.stderr.String())
		}
		if !strings.Contains(p.stderr.String(), "initialize: client lsp_test") {
			t.Errorf("the log on stderr:\n%s", p.stderr.String())
		}
	}
	slices.Sort(times)
	t.Logf("process start → initialize answered, %d runs: min %v, median %v, max %v",
		runs, times[0], times[runs/2], times[runs-1])
	// The fastest run: a busy machine (make ci runs the packages' tests
	// at the same time) can slow some runs down, but not every one.
	if times[0] >= 100*time.Millisecond {
		t.Errorf("initialize was answered after %v at the fastest, want well under 100 ms", times[0])
	}
}

func TestLSPProcessExitWithoutShutdown(t *testing.T) {
	p := startLSP(t)
	p.send(lspInitialize)
	p.read()
	p.send(lspExit)
	if code := p.wait(); code != 1 {
		t.Errorf("exit without shutdown: exit status %d, want 1", code)
	}
	// The end of stdin without exit.
	p = startLSP(t)
	p.send(lspInitialize)
	p.read()
	if code := p.wait(); code != 1 {
		t.Errorf("the end without shutdown: exit status %d, want 1", code)
	}
}

func TestLSPLogfile(t *testing.T) {
	logfile := filepath.Join(t.TempDir(), "lsp.log")
	if err := os.WriteFile(logfile, []byte("an older log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := startLSP(t, "-logfile", logfile)
	p.send(lspInitialize)
	p.read()
	p.send(lspShutdown)
	p.read()
	p.send(lspExit)
	if code := p.wait(); code != 0 {
		t.Errorf("exit status %d, want 0", code)
	}
	if p.stderr.Len() != 0 {
		t.Errorf("with -logfile, stderr has:\n%s", p.stderr.String())
	}
	data, err := os.ReadFile(logfile)
	if err != nil {
		t.Fatal(err)
	}
	if log := string(data); !strings.HasPrefix(log, "an older log\n") || !strings.Contains(log, "initialize: client lsp_test") {
		t.Errorf("the log file:\n%s", log)
	}
}

func TestLSPUsage(t *testing.T) {
	code, out, errOut := gocpp("lsp", "extra")
	if code != 2 || out != "" || errOut != "usage: gocpp lsp [-logfile file]\n" {
		t.Errorf("gocpp lsp extra: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, out, errOut = gocpp("lsp", "-h")
	if code != 0 || out != "" || !strings.HasPrefix(errOut, lspUsage) || !strings.Contains(errOut, "-logfile file") {
		t.Errorf("gocpp lsp -h: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = gocpp("lsp", "-nope")
	if code != 2 || !strings.Contains(errOut, "flag provided but not defined: -nope") {
		t.Errorf("gocpp lsp -nope: exit %d, stderr %q", code, errOut)
	}
	missing := filepath.Join(t.TempDir(), "no", "such", "dir", "lsp.log")
	code, out, errOut = gocpp("lsp", "-logfile", missing)
	if code != 2 || out != "" || !strings.HasPrefix(errOut, "gocpp lsp: open "+missing) {
		t.Errorf("gocpp lsp -logfile %s: exit %d, stdout %q, stderr %q", missing, code, out, errOut)
	}
}

// TestLSPInProcess runs the command in the test process with a whole
// session on stdin.
func TestLSPInProcess(t *testing.T) {
	var in strings.Builder
	for _, body := range []string{lspInitialize, lspShutdown, lspExit} {
		fmt.Fprintf(&in, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	code, out, errOut := gocppStdin(in.String(), "lsp")
	if code != 0 {
		t.Errorf("exit %d, want 0; stderr:\n%s", code, errOut)
	}
	var bodies []string
	r := protocol.NewReader(strings.NewReader(out))
	for {
		body, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("stdout is not only LSP messages: %v\n%q", err, out)
		}
		bodies = append(bodies, string(body))
	}
	want := []string{lspInitResult, `{"jsonrpc":"2.0","id":2,"result":null}`}
	if !slices.Equal(bodies, want) {
		t.Errorf("stdout has\n%s\nwant\n%s", strings.Join(bodies, "\n"), strings.Join(want, "\n"))
	}
}
