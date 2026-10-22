package server

import (
	"io/fs"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A file that another program holds for longer than the repeats of the read (Phase 14 step 14.5, ADR-096): the server
// says so once, reads the file again in the background, and diagnoses when it can; no event is needed.
func TestBusyFileIsReadAgain(t *testing.T) {
	checkGoroutines(t)
	var busy atomic.Bool
	busy.Store(true)
	var reads atomic.Int64
	cfg := testConfig()
	cfg.ReadFile = func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "util.gp") && busy.Load() {
			reads.Add(1)
			return nil, &fs.PathError{Op: "open", Path: path, Err: cache.ErrBusy}
		}
		return os.ReadFile(path)
	}
	main := "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.F() }\n"
	c := startDiag(t, cfg, map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"main.gp":     main,
		"lib/util.gp": "package lib\n\nfunc F() {}\n",
	}, func(s *Server) { s.retry.base, s.retry.max = 5*time.Millisecond, 20*time.Millisecond })
	c.open("main.gp", main)
	c.next("main.gp")
	// The library cannot be read: the client is told, once, in the log and by window/logMessage.
	note := c.readNotification("window/logMessage")
	if !strings.Contains(note, "cannot read") || !strings.Contains(note, "util.gp") || !strings.Contains(note, "in use by another program") {
		t.Errorf("window/logMessage: %s", note)
	}
	if locs := c.definitions("textDocument/definition", "main.gp", main, "lib.|F()"); len(locs) != 0 {
		t.Errorf("a definition in a file that cannot be read: %v", locs)
	}
	busy.Store(false)
	// The file is free: the server reads it again by itself, makes a new snapshot and finds the definition.
	waitFor(t, "the log says the file can be read again", func() bool { return strings.Contains(c.logs.String(), "can be read again") })
	if locs := c.definitions("textDocument/definition", "main.gp", main, "lib.|F()"); len(locs) != 1 || !strings.HasPrefix(locs[0], "lib/util.gp:") {
		t.Errorf("the definition after the file was read again: %v", locs)
	}
	if n := strings.Count(c.logs.String(), "cannot read"); n != 1 {
		t.Errorf("the failure was reported %d times, want once:\n%s", n, c.logs.String())
	}
}

func noErrorsOf(p *publish) bool {
	for _, d := range p.Diagnostics {
		if d.Severity == protocol.SeverityError {
			return false
		}
	}
	return true
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(time.Minute); !ok(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("after a minute: %s", what)
		}
	}
}

// After shutdown the retries stop.
func TestBusyFileRetryStops(t *testing.T) {
	checkGoroutines(t)
	cfg := testConfig()
	cfg.ReadFile = func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "util.gp") {
			return nil, &fs.PathError{Op: "open", Path: path, Err: cache.ErrBusy}
		}
		return os.ReadFile(path)
	}
	main := "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.F() }\n"
	c := startDiag(t, cfg, map[string]string{
		"gocpp.mod": "module example.com/m\n", "main.gp": main, "lib/util.gp": "package lib\n\nfunc F() {}\n",
	}, func(s *Server) { s.retry.base, s.retry.max = 5*time.Millisecond, 5*time.Millisecond })
	c.open("main.gp", main)
	c.next("main.gp")
	c.readNotification("window/logMessage")
	waitFor(t, "a retry is running", func() bool {
		c.s.retry.mu.Lock()
		defer c.s.retry.mu.Unlock()
		return len(c.s.retry.active) > 0
	})
	c.call("shutdown", "")
	c.notify("exit", "")
	c.wait()
	waitFor(t, "the retry ended with the server", func() bool {
		c.s.retry.mu.Lock()
		defer c.s.retry.mu.Unlock()
		return len(c.s.retry.active) == 0
	})
}
