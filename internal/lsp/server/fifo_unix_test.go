//go:build unix

package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestNamedPipeSourceRequests: a named pipe with the name of a source in a package of the
// workspace (one that main does not import; the pipe comes before z.gp in the folder) does not
// freeze hover and workspace/symbol (the server's reads of the disk refuse a file that is not
// regular: load.ReadRegular), and the auto-import scan skips it. If a read waits for the pipe
// after all, the pipe gets a writer after a minute (so that the read ends and no goroutine
// outlives the test) and the test fails.
func TestNamedPipeSourceRequests(t *testing.T) {
	c := startDiag(t, testConfig(), map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"main.gp":     cfMain,
		"lib/util.gp": cfLib,
		"other/z.gp":  "package other\n\nfunc G() {}\n",
	}, nil)
	var stuck atomic.Bool
	pipe := filepath.Join(c.root, "other", "pipe.gp")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	timer := time.AfterFunc(time.Minute, func() {
		stuck.Store(true)
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	t.Cleanup(func() { timer.Stop() })
	c.notify("textDocument/didOpen", didOpen(c.uri("main.gp"), 1, cfMain))
	if text, _ := c.hover("main.gp", cfMain, "lib.|F"); !strings.Contains(text, "lib.F(x int) int") {
		t.Errorf("hover on lib.F = %q", text)
	}
	if resp := c.call("workspace/symbol", `{"query":"F"}`); resp.Error != nil || !strings.Contains(string(resp.Result), `"F"`) {
		t.Errorf("workspace/symbol F = %s, %v", resp.Result, resp.Error)
	}
	if resp := c.call("workspace/symbol", `{"query":"G"}`); resp.Error != nil {
		t.Errorf("workspace/symbol G = %s, %v", resp.Result, resp.Error)
	}
	// The completion of a package name that main does not import scans the module's packages
	// (readHeader reads the start of each source).
	edited := cfMain + "\nfunc h() {\n\toth\n}\n"
	c.notify("textDocument/didChange", fullChange(c.uri("main.gp"), 2, edited))
	if resp := c.call("textDocument/completion", c.posParams("main.gp", edited, "oth|")); resp.Error != nil || !strings.Contains(string(resp.Result), "example.com/m/other") {
		t.Errorf("completion of oth = %s, %v; want the package example.com/m/other", resp.Result, resp.Error)
	}
	if stuck.Load() {
		t.Fatal("a request waited for a writer of a named pipe (the pipes got one after a minute)")
	}
}
